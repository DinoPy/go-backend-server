package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/dinopy/taskbar2_server/internal/database"
	"github.com/dinopy/taskbar2_server/internal/metrics"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

type ConnectionError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Code    int    `json:"code"`
}

const (
	ErrorGoogleUIDMismatch = "google_uid_mismatch"
	ErrorInvalidGoogleUID  = "invalid_google_uid"
	ErrorUserCreation      = "user_creation_failed"
	ErrorDatabaseError     = "database_error"
)

func sendError(c *websocket.Conn, errorType, message string, code int) error {
	log.Printf("sendError triggered: type=%s code=%d message=%s", errorType, code, message)
	errorResponse := map[string]interface{}{
		"event": "connection_error",
		"data": ConnectionError{
			Type:    errorType,
			Message: message,
			Code:    code,
		},
	}

	payload, _ := json.Marshal(errorResponse)
	return c.Write(context.Background(), websocket.MessageText, payload)
}

func logDBError(context string, err error) {
	if err == nil {
		return
	}
	log.Printf("%s: %v", context, err)
}

func isUndefinedTableError(err error, table string) bool {
	if err == nil {
		return false
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		if pqErr.Code == "42P01" {
			return true
		}
	}
	// Fallback string check in case driver differs.
	return strings.Contains(strings.ToLower(err.Error()), fmt.Sprintf(`relation "%s"`, strings.ToLower(table)))
}

func (cfg *config) getClientBySID(SID uuid.UUID) (*Client, bool) {
	cfg.WSClientManager.mu.RLock()
	defer cfg.WSClientManager.mu.RUnlock()
	client, ok := cfg.WSClientManager.clients[SID]
	return client, ok
}

func (cfg *config) WSOnConnect(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("connect").Observe(time.Since(start).Seconds())
	}()

	var connectionData struct {
		Data User `json:"data"`
	}
	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return sendError(c, "invalid_data", "Invalid connection data", 400)
	}

	// Validate Google UID
	if connectionData.Data.GoogleUID == "" {
		return sendError(c, ErrorInvalidGoogleUID, "Google UID is required", 400)
	}

	// Check if user exists by email
	existingUser, err := cfg.DB.GetUserByEmail(ctx, connectionData.Data.Email)
	if err != nil && err != sql.ErrNoRows {
		return sendError(c, ErrorDatabaseError, "Database error", 500)
	}

	var user database.User

	if err == sql.ErrNoRows {
		// User doesn't exist - create new user
		user, err = cfg.DB.CreateUser(ctx, database.CreateUserParams{
			Email:     connectionData.Data.Email,
			FirstName: connectionData.Data.FirstName,
			LastName:  connectionData.Data.LastName,
			GoogleUid: sql.NullString{
				String: connectionData.Data.GoogleUID,
				Valid:  true,
			},
		})
		if err != nil {
			return sendError(c, ErrorUserCreation, "Failed to create user", 500)
		}
	} else {
		// User exists - verify Google UID matches
		if !existingUser.GoogleUid.Valid && existingUser.GoogleUid.String != connectionData.Data.GoogleUID {
			return sendError(c, ErrorGoogleUIDMismatch, "Google UID does not match", 403)
		}
		user = existingUser
	}

	// Success - continue with normal connection flow
	cfg.WSClientManager.AddClient(&Client{
		SID:  SID,
		Conn: c,
		User: user,
	})

	tasks, err := cfg.DB.GetActiveTaskByUUIDWithTiming(ctx, user.ID)
	if err != nil {
		logDBError("Failed to load tasks for user "+user.ID.String(), err)
		return sendError(c, ErrorDatabaseError, "Failed to load tasks", 500)
	}

	notificationsParams := database.ListNotificationsByUserParams{
		UserID:            user.ID,
		Statuses:          []string{"unseen", "seen"},
		OffsetVal:         sql.NullInt32{Int32: 0, Valid: true},
		LimitVal:          sql.NullInt32{Int32: 10, Valid: true},
		IncludeSnoozed:    sql.NullBool{Valid: true, Bool: false},
		ExpiredOnly:       sql.NullBool{Valid: true, Bool: false},
		NotificationTypes: nil,
		Priorities:        nil,
	}

	notifications := []database.Notification{}

	notificationsResult, err := cfg.DB.ListNotificationsByUserWithTiming(ctx, notificationsParams)
	if err != nil {
		if isUndefinedTableError(err, "notifications") {
			log.Printf("Notifications table missing when loading for user %s; returning empty list", user.ID.String())
		} else {
			logDBError("Failed to load notifications for user "+user.ID.String(), err)
			return sendError(c, ErrorDatabaseError, "Failed to load notifications", 500)
		}
	} else {
		notifications = notificationsResult
	}

	unseenCount := int64(0)
	unseenCountValue, err := cfg.DB.CountUnseenNotificationsWithTiming(ctx, user.ID)
	if err != nil {
		if isUndefinedTableError(err, "notifications") {
			log.Printf("Notifications table missing when counting unseen for user %s; defaulting to 0", user.ID.String())
		} else {
			logDBError("Failed to load notification metadata for user "+user.ID.String(), err)
			return sendError(c, ErrorDatabaseError, "Failed to load notifications metadata", 500)
		}
	} else {
		unseenCount = unseenCountValue
	}

	var category string
	var keyCommands string

	if user.Categories.Valid {
		category = user.Categories.String
	}
	if user.KeyCommands.Valid {
		keyCommands = user.KeyCommands.String
	}

	type finalUser struct {
		SID                    uuid.UUID               `json:"sid"`
		ID                     uuid.UUID               `json:"id"`
		FirstName              string                  `json:"first_name"`
		LastName               string                  `json:"last_name"`
		Email                  string                  `json:"email"`
		CreatedAt              time.Time               `json:"created_at"`
		UpdatedAt              time.Time               `json:"updated_at"`
		Categories             string                  `json:"categories"`
		KeyCommands            string                  `json:"key_commands"`
		Tasks                  []database.Task         `json:"tasks"`
		Notifications          []database.Notification `json:"notifications"`
		NotificationsUnseenCnt int64                   `json:"notifications_unseen_count"`
	}

	cfg.WSClientManager.SendToClient(ctx, "connected", SID, finalUser{
		SID:                    SID,
		ID:                     user.ID,
		FirstName:              user.FirstName,
		LastName:               user.LastName,
		Email:                  user.Email,
		CreatedAt:              user.CreatedAt,
		UpdatedAt:              user.UpdatedAt,
		Categories:             category,
		KeyCommands:            keyCommands,
		Tasks:                  tasks,
		Notifications:          notifications,
		NotificationsUnseenCnt: unseenCount,
	})
	return nil
}

func (cfg *config) WSOnTaskCreate(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("task_create").Observe(time.Since(start).Seconds())
	}()

	type taskT struct {
		ID                uuid.UUID  `json:"id"`
		Title             string     `json:"title"`
		Description       string     `json:"descripiton"`
		CreatedAt         time.Time  `json:"created_at"`
		CompletedAt       time.Time  `json:"completed_at"`
		Duration          string     `json:"duration"`
		Category          string     `json:"category"`
		Tags              []string   `json:"tags"`
		ToggledAt         int64      `json:"toggled_at"`
		IsCompleted       bool       `json:"is_completed"`
		IsActive          bool       `json:"is_active"`
		LastModifiedAt    int64      `json:"last_modified_at"`
		Priority          *int32     `json:"priority"`
		DueAt             *time.Time `json:"due_at"`
		ShowBeforeDueTime *int32     `json:"show_before_due_time"`
	}
	var connectionData struct {
		Data taskT `json:"data"`
	}
	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return err
	}

	fmt.Printf("%+v", connectionData.Data.Duration)

	// Handle nullable fields
	var priority sql.NullInt32
	if connectionData.Data.Priority != nil {
		priority = sql.NullInt32{
			Int32: *connectionData.Data.Priority,
			Valid: true,
		}
	}

	var dueAt sql.NullTime
	if connectionData.Data.DueAt != nil {
		dueAt = sql.NullTime{
			Time:  *connectionData.Data.DueAt,
			Valid: true,
		}
	}

	var showBeforeDueTime sql.NullInt32
	if connectionData.Data.ShowBeforeDueTime != nil {
		showBeforeDueTime = sql.NullInt32{
			Int32: *connectionData.Data.ShowBeforeDueTime,
			Valid: true,
		}
	} else {
		showBeforeDueTime = sql.NullInt32{
			Int32: 0,
			Valid: true,
		}
	}

	task, err := cfg.DB.CreateTaskWithTiming(ctx, database.CreateTaskParams{
		ID:          connectionData.Data.ID,
		Title:       connectionData.Data.Title,
		Description: connectionData.Data.Description,
		CreatedAt:   connectionData.Data.CreatedAt,
		CompletedAt: sql.NullTime{
			Valid: true,
			Time:  connectionData.Data.CompletedAt,
		},
		Duration: connectionData.Data.Duration,
		Category: connectionData.Data.Category,
		Tags:     connectionData.Data.Tags,
		ToggledAt: sql.NullInt64{
			Int64: connectionData.Data.ToggledAt,
			Valid: true,
		},
		IsCompleted:       connectionData.Data.IsCompleted,
		IsActive:          connectionData.Data.IsActive,
		LastModifiedAt:    connectionData.Data.LastModifiedAt,
		UserID:            cfg.WSClientManager.clients[SID].User.ID,
		Priority:          priority,
		DueAt:             dueAt,
		ShowBeforeDueTime: showBeforeDueTime,
	})

	if err != nil {
		return err
	}

	cfg.WSClientManager.BroadcastToSameUserNoIssuer(
		ctx,
		"new_task_created",
		cfg.WSClientManager.clients[SID].User.ID,
		SID,
		task,
	)

	return nil
}

func (cfg *config) WSOnTaskToggle(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("task_toggle").Observe(time.Since(start).Seconds())
	}()

	type taskT struct {
		UUID           uuid.UUID `json:"uuid"`
		ToggledAt      int64     `json:"toggled_at"`
		IsActive       bool      `json:"is_active"`
		Duration       string    `json:"duration"`
		LastModifiedAt int64     `json:"last_modified_at"`
	}

	var connectionData struct {
		Data taskT `json:"data"`
	}
	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return err
	}

	task, err := cfg.DB.ToggleTaskWithTiming(ctx, database.ToggleTaskParams{
		ID: connectionData.Data.UUID,
		ToggledAt: sql.NullInt64{
			Int64: connectionData.Data.ToggledAt,
			Valid: true,
		},
		IsActive:       connectionData.Data.IsActive,
		Duration:       connectionData.Data.Duration,
		LastModifiedAt: connectionData.Data.LastModifiedAt,
	})
	if err != nil {
		return err
	}
	cfg.WSClientManager.BroadcastToSameUserNoIssuer(
		ctx,
		"related_task_toggled",
		cfg.WSClientManager.clients[SID].User.ID,
		SID,
		task,
	)
	return nil
}

func (cfg *config) WSOnTaskCompleted(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("task_completed").Observe(time.Since(start).Seconds())
	}()

	type taskT struct {
		ID             uuid.UUID `json:"id"`
		CompletedAt    time.Time `json:"completed_at"`
		Duration       string    `json:"duration"`
		LastModifiedAt int64     `json:"last_modified_at"`
	}
	var connectionData struct {
		Data taskT `json:"Data"`
	}

	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return err
	}
	fmt.Println(connectionData)

	task, err := cfg.DB.CompleteTaskWithTiming(ctx, database.CompleteTaskParams{
		ID:       connectionData.Data.ID,
		Duration: connectionData.Data.Duration,
		CompletedAt: sql.NullTime{
			Valid: true,
			Time:  connectionData.Data.CompletedAt.In(time.UTC),
		},
		LastModifiedAt: connectionData.Data.LastModifiedAt,
	})

	cfg.WSClientManager.BroadcastToSameUserNoIssuer(
		ctx,
		"related_task_deleted",
		cfg.WSClientManager.clients[SID].User.ID,
		SID,
		struct {
			ID uuid.UUID `json:"id"`
		}{
			ID: task.ID,
		},
	)

	return nil
}

func (cfg *config) WSOnTaskEdit(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("task_edit").Observe(time.Since(start).Seconds())
	}()

	type taskT struct {
		ID                uuid.UUID  `json:"id"`
		Title             string     `json:"title"`
		Description       string     `json:"description"`
		Category          string     `json:"category"`
		Tags              []string   `json:"tags"`
		LastModifiedAt    int64      `json:"last_modified_at"`
		Priority          *int32     `json:"priority"`
		DueAt             *time.Time `json:"due_at"`
		ShowBeforeDueTime *int32     `json:"show_before_due_time"`
	}

	var connectionData struct {
		Data taskT `json:"data"`
	}
	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return err
	}

	fmt.Printf("%+v", connectionData.Data)

	// Handle nullable fields
	var priority sql.NullInt32
	if connectionData.Data.Priority != nil {
		priority = sql.NullInt32{
			Int32: *connectionData.Data.Priority,
			Valid: true,
		}
	}

	var dueAt sql.NullTime
	if connectionData.Data.DueAt != nil {
		dueAt = sql.NullTime{
			Time:  *connectionData.Data.DueAt,
			Valid: true,
		}
	}

	var showBeforeDueTime sql.NullInt32
	if connectionData.Data.ShowBeforeDueTime != nil {
		showBeforeDueTime = sql.NullInt32{
			Int32: *connectionData.Data.ShowBeforeDueTime,
			Valid: true,
		}
	}

	task, err := cfg.DB.EditTaskWithTiming(ctx, database.EditTaskParams{
		ID:                connectionData.Data.ID,
		Title:             connectionData.Data.Title,
		Description:       connectionData.Data.Description,
		Category:          connectionData.Data.Category,
		Tags:              connectionData.Data.Tags,
		LastModifiedAt:    connectionData.Data.LastModifiedAt,
		Priority:          priority,
		DueAt:             dueAt,
		ShowBeforeDueTime: showBeforeDueTime,
	})

	cfg.WSClientManager.BroadcastToSameUserNoIssuer(
		ctx,
		"related_task_edited",
		cfg.WSClientManager.clients[SID].User.ID,
		SID,
		task,
	)
	return nil
}

func (cfg *config) WSOnTaskDelete(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("task_delete").Observe(time.Since(start).Seconds())
	}()

	type taskT struct {
		ID uuid.UUID `json:"id"`
	}

	var connectionData struct {
		Data taskT `json:"data"`
	}
	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return err
	}

	err = cfg.DB.DeleteTaskWithTiming(ctx, connectionData.Data.ID)
	if err != nil {
		return err
	}

	cfg.WSClientManager.BroadcastToSameUserNoIssuer(
		ctx,
		"related_task_deleted",
		cfg.WSClientManager.clients[SID].User.ID,
		SID,
		struct {
			ID uuid.UUID `json:"id"`
		}{
			ID: connectionData.Data.ID,
		},
	)
	return nil
}

func (cfg *config) WSOnGetCompletedTasks(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("get_completed_tasks").Observe(time.Since(start).Seconds())
	}()

	type searchT struct {
		Category    string    `json:"category"`
		StartDate   time.Time `json:"start_date"`
		EndDate     time.Time `json:"end_date"`
		SearchQuery string    `json:"search_query"`
		Tags        []string  `json:"tags"`
	}

	var connectionData struct {
		Data searchT `json:"data"`
	}

	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return err
	}
	fmt.Printf("Data received from the app: \n%+v\n\n", connectionData.Data)

	queryFilters := database.GetCompletedTasksByUUIDParams{}
	queryFilters.UserID = cfg.WSClientManager.clients[SID].User.ID
	queryFilters.Tags = connectionData.Data.Tags
	if !connectionData.Data.StartDate.IsZero() {
		queryFilters.StartDate = sql.NullTime{
			Valid: true,
			Time:  connectionData.Data.StartDate.In(time.UTC),
		}
	} else {
		now := time.Now()
		startOfDay := time.Date(
			now.Year(), now.Month(), now.Day(),
			0, 0, 0, 0, time.UTC,
		)
		queryFilters.StartDate = sql.NullTime{
			Valid: true,
			Time:  startOfDay,
		}
	}
	if !connectionData.Data.EndDate.IsZero() {
		endDateWithTime := time.Date(
			connectionData.Data.EndDate.Year(),
			connectionData.Data.EndDate.Month(),
			connectionData.Data.EndDate.Day(),
			23, 59, 59, 0, time.UTC,
		)
		queryFilters.EndDate = sql.NullTime{
			Valid: true,
			Time:  endDateWithTime,
		}
	} else {
		now := time.Now()
		endOfDay := time.Date(
			now.Year(), now.Month(), now.Day(),
			23, 59, 59, 0, time.UTC,
		)
		queryFilters.EndDate = sql.NullTime{
			Valid: true,
			Time:  endOfDay,
		}
	}
	if connectionData.Data.Category != "" {
		queryFilters.Category = sql.NullString{
			String: connectionData.Data.Category,
			Valid:  true,
		}
	}
	if connectionData.Data.SearchQuery != "" {
		queryFilters.SearchQuery = sql.NullString{
			String: "%" + connectionData.Data.SearchQuery + "%",
			Valid:  true,
		}
	}
	fmt.Printf("Final filters used for the query:\n%+v\n\n", queryFilters)

	tasks, err := cfg.DB.GetCompletedTasksByUUIDWithTiming(ctx, queryFilters)
	if err != nil {
		return err
	}
	cfg.WSClientManager.SendToClient(ctx, "get_completed_tasks", SID, tasks)

	return nil
}

type TaskNoNullable struct {
	ID                uuid.UUID  `json:"id"`
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	CreatedAt         time.Time  `json:"created_at"`
	CompletedAt       time.Time  `json:"completed_at"`
	Duration          string     `json:"duration"`
	Category          string     `json:"category"`
	Tags              []string   `json:"tags"`
	ToggledAt         int64      `json:"toggled_at"`
	IsActive          bool       `json:"is_active"`
	IsCompleted       bool       `json:"is_completed"`
	UserID            uuid.UUID  `json:"user_id"`
	LastModifiedAt    int64      `json:"last_modified_at"`
	Priority          *int32     `json:"priority"`
	DueAt             *time.Time `json:"due_at"`
	ShowBeforeDueTime *int32     `json:"show_before_due_time"`
}

func (cfg *config) WSOnRequestHardRefresh(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("hard_refresh").Observe(time.Since(start).Seconds())
	}()

	settings, err := cfg.DB.GetUserSettingsWithTiming(ctx, cfg.WSClientManager.clients[SID].User.ID)
	if err != nil {
		return nil
	}

	tasks, err := cfg.DB.GetActiveTaskByUUIDWithTiming(ctx, cfg.WSClientManager.clients[SID].User.ID)
	if err != nil {
		return nil
	}

	response := struct {
		Categories  string          `json:"categories"`
		KeyCommands string          `json:"key_commands"`
		Tasks       []database.Task `json:"tasks"`
	}{
		Tasks: tasks,
	}

	if settings.Categories.Valid {
		response.Categories = settings.Categories.String
	}
	if settings.KeyCommands.Valid {
		response.KeyCommands = settings.KeyCommands.String
	}

	cfg.WSClientManager.SendToClient(ctx, "request_hard_refresh", SID, response)

	return nil
}

func (cfg *config) WSOnUserUpdatedCategories(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	var connectionData struct {
		Data []string `json:"data"`
	}

	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return err
	}

	fmt.Printf("%+v\n", connectionData)

	updatedUser, err := cfg.DB.UpdateUserCategoriesWithTiming(ctx, database.UpdateUserCategoriesParams{
		ID: cfg.WSClientManager.clients[SID].User.ID,
		Categories: sql.NullString{
			String: strings.Join(connectionData.Data, ","),
			Valid:  true,
		},
	})
	if err != nil {
		return err
	}

	cfg.WSClientManager.BroadcastToSameUserNoIssuer(
		ctx,
		"related_user_updated_categories",
		cfg.WSClientManager.clients[SID].User.ID,
		SID,
		updatedUser.Categories,
	)

	return nil
}

func (cfg *config) WSOnNewCommandAdded(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	var connectionData struct {
		Data string `json:"data"`
	}

	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return err
	}

	fmt.Printf("%+v\n", connectionData)

	user, err := cfg.DB.UpdateUserCommandsWithTiming(ctx, database.UpdateUserCommandsParams{
		ID: cfg.WSClientManager.clients[SID].User.ID,
		KeyCommands: sql.NullString{
			String: connectionData.Data,
			Valid:  true,
		},
	})

	cfg.WSClientManager.BroadcastToSameUserNoIssuer(
		ctx,
		"related_command_updated",
		cfg.WSClientManager.clients[SID].User.ID,
		SID,
		user.KeyCommands,
	)

	return nil
}

func durationStrToInt(duration string) (int64, error) {
	strSlice := strings.Split(duration, ":")
	seconds, err := strconv.ParseInt(strSlice[0], 10, 32)
	if err != nil {
		return 0, err
	}

	minutes, err := strconv.ParseInt(strSlice[1], 10, 32)
	if err != nil {
		return 0, err
	}

	hours, err := strconv.ParseInt(strSlice[2], 10, 32)
	if err != nil {
		return 0, err
	}

	durInt := (seconds*60*60 + minutes*60 + hours) * 1000

	return durInt, nil
}

func durationIntToStr(duration int64) (string, error) {
	seconds := int(duration % 60)
	minutes := int((duration / 60) % 60)
	hours := int(duration / 60 / 60)

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds), nil
}

func ternary[T any](condition bool, ifTrue T, ifFalse T) T {
	if condition {
		return ifTrue
	}
	return ifFalse
}

func (cfg *config) WSOnMidnightTaskRefresh() {
	userIDs := make(map[uuid.UUID]struct{})
	loc, _ := time.LoadLocation("Europe/Bucharest")
	timeNow := time.Now().In(loc)
	lastEpochMs := time.Now().UnixMilli()

	// get all the tasks that are not completed from db
	tasks, err := cfg.DB.GetNonCompletedTasksWithTiming(context.Background())
	if err != nil {
		log.Println(err)
	}

	for _, task := range tasks {
		// save the id of the user, we must try to send the refresher to.
		userIDs[task.UserID] = struct{}{}

		// calculate duration to int
		durationInt, err := durationStrToInt(task.Duration)
		if err != nil {
			log.Println(err)
		}
		// exclude tasks that with total duration 0
		if durationInt == 0 && task.ToggledAt.Int64 == 0 {
			continue
		}

		var currentSegmentDurationMs int64
		if task.ToggledAt.Valid && task.ToggledAt.Int64 != 0 {
			currentSegmentDurationMs = lastEpochMs - task.ToggledAt.Int64

			if currentSegmentDurationMs < 0 {
				log.Printf("Warning: task %s ToggledAt (%d) is in the future compared to current time (%d)",
					task.ID, task.ToggledAt.Int64, lastEpochMs)
				currentSegmentDurationMs = 0
			}
		} else {
			currentSegmentDurationMs = 0
		}

		duration := (durationInt + currentSegmentDurationMs) / 1000
		durationStr, err := durationIntToStr(duration)
		if err != nil {
			log.Println(err)
		}

		// update current task to new duration, complete status, last modified at , completed_at
		completeTaskParams := database.CompleteTaskParams{
			ID:       task.ID,
			Duration: durationStr,
			CompletedAt: sql.NullTime{
				Time:  timeNow.In(time.UTC),
				Valid: true,
			},
			LastModifiedAt: lastEpochMs,
		}

		_, err = cfg.DB.CompleteTaskWithTiming(context.Background(), completeTaskParams)
		if err != nil {
			log.Println(err)
		}

		// insert a new task with the same properties
		createTaskParams := database.CreateTaskParams{
			ID:          uuid.New(),
			Title:       task.Title,
			Description: task.Description,
			CreatedAt:   timeNow.In(time.UTC),
			CompletedAt: sql.NullTime{
				Valid: false,
			},
			Duration: "00:00:00",
			Category: task.Category,
			Tags:     task.Tags,
			ToggledAt: sql.NullInt64{
				Int64: ternary(task.ToggledAt.Int64 == 0, 0, lastEpochMs),
				Valid: ternary(task.ToggledAt.Int64 == 0, false, true),
			},
			IsActive:          task.IsActive,
			IsCompleted:       false,
			UserID:            task.UserID,
			LastModifiedAt:    lastEpochMs,
			Priority:          task.Priority,          // Copy from original task
			DueAt:             task.DueAt,             // Copy from original task
			ShowBeforeDueTime: task.ShowBeforeDueTime, // Copy from original task
		}

		_, err = cfg.DB.CreateTaskWithTiming(context.Background(), createTaskParams)
		if err != nil {
			log.Println(err)
		}
	}

	// emit a refresher to all connected devices
	for _, client := range cfg.WSClientManager.clients {
		if _, ok := userIDs[client.User.ID]; !ok {
			continue
		}
		log.Printf("Currently processing client SID: %s\n", client.SID.String())

		tasks, err := cfg.DB.GetActiveTaskByUUIDWithTiming(context.Background(), client.User.ID)
		if err != nil {
			log.Println(err)
		}

		user, err := cfg.DB.GetUserSettingsWithTiming(context.Background(), client.User.ID)
		if err != nil {
			log.Println(err)
		}

		var category string
		var keyCommands string

		if user.Categories.Valid {
			category = user.Categories.String
		}
		if user.KeyCommands.Valid {
			keyCommands = user.KeyCommands.String
		}

		type refresher struct {
			Categories  string          `json:"categories"`
			KeyCommands string          `json:"key_commands"`
			Tasks       []database.Task `json:"tasks"`
		}

		cfg.WSClientManager.BroadcastToSameUser(context.Background(), "tasks_refresher", client.User.ID, refresher{
			Categories:  category,
			KeyCommands: keyCommands,
			Tasks:       tasks,
		})
	}
}

func (cfg *config) WSOnTaskDuplicate(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("task_duplicate").Observe(time.Since(start).Seconds())
	}()

	type duplicateRequest struct {
		TaskID uuid.UUID `json:"task_id"`
	}

	var request struct {
		Data duplicateRequest `json:"data"`
	}
	err := json.Unmarshal(data, &request)
	if err != nil {
		return err
	}

	// Get the original task from database
	// Note: We'll need to add a GetTaskByID query first
	originalTask, err := cfg.DB.GetTaskByIDWithTiming(ctx, request.Data.TaskID)
	if err != nil {
		return err
	}

	// Verify the task belongs to the requesting user
	if originalTask.UserID != cfg.WSClientManager.clients[SID].User.ID {
		return sendError(c, "unauthorized", "Task does not belong to user", 403)
	}

	// Create duplicate task with modified properties
	duplicateTask, err := cfg.DB.CreateTaskWithTiming(ctx, database.CreateTaskParams{
		ID:                uuid.New(),                     // New ID
		Title:             originalTask.Title,             // Copy
		Description:       originalTask.Description,       // Copy
		CreatedAt:         time.Now().UTC(),               // Current time
		CompletedAt:       sql.NullTime{Valid: false},     // Null (not completed)
		Duration:          "00:00:00",                     // Reset to zero
		Category:          originalTask.Category,          // Copy
		Tags:              originalTask.Tags,              // Copy
		ToggledAt:         sql.NullInt64{Valid: false},    // Reset to null
		IsActive:          false,                          // Reset to false
		IsCompleted:       false,                          // Reset to false
		UserID:            originalTask.UserID,            // Copy
		LastModifiedAt:    time.Now().UnixMilli(),         // Current time
		Priority:          originalTask.Priority,          // Copy
		DueAt:             originalTask.DueAt,             // Copy
		ShowBeforeDueTime: originalTask.ShowBeforeDueTime, // Copy
	})

	if err != nil {
		return err
	}

	// Emit the new task via new_task_created event
	cfg.WSClientManager.BroadcastToSameUser(
		ctx,
		"new_task_created",
		cfg.WSClientManager.clients[SID].User.ID,
		duplicateTask,
	)

	return nil
}

func (cfg *config) WSOnTaskSplit(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("task_split").Observe(time.Since(start).Seconds())
	}()

	type splitTask struct {
		Title       string `json:"title"`
		Description string `json:"description"`
		Duration    string `json:"duration"`
	}

	type splitRequest struct {
		TaskID uuid.UUID   `json:"task_id"`
		Splits []splitTask `json:"splits"`
	}

	var request struct {
		Data splitRequest `json:"data"`
	}
	err := json.Unmarshal(data, &request)
	if err != nil {
		return err
	}

	// Validate splits
	if len(request.Data.Splits) == 0 {
		return sendError(c, "invalid_request", "At least one split is required", 400)
	}

	// Validate task ID format
	if request.Data.TaskID == uuid.Nil {
		return sendError(c, "invalid_request", "Invalid task ID format", 400)
	}

	// Get the original task from database
	originalTask, err := cfg.DB.GetTaskByID(ctx, request.Data.TaskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return sendError(c, "not_found", "Task not found", 404)
		}
		return err
	}

	// Verify the task belongs to the requesting user
	if originalTask.UserID != cfg.WSClientManager.clients[SID].User.ID {
		return sendError(c, "unauthorized", "Task does not belong to user", 403)
	}

	// Start database transaction
	tx, err := cfg.DBPool.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	queries := cfg.DB.WithTx(tx)

	// Delete the original task
	err = queries.DeleteTask(ctx, originalTask.ID)
	if err != nil {
		return err
	}

	// Create split tasks
	var splitTasks []database.Task
	lastEpochMs := time.Now().UnixMilli()

	for _, split := range request.Data.Splits {
		// Determine toggled_at value
		var toggledAt sql.NullInt64
		if originalTask.IsActive {
			toggledAt = sql.NullInt64{
				Int64: lastEpochMs,
				Valid: true,
			}
		} else {
			toggledAt = sql.NullInt64{Valid: false}
		}

		splitTask, err := queries.CreateTask(ctx, database.CreateTaskParams{
			ID:                uuid.New(),
			Title:             split.Title,
			Description:       split.Description,
			CreatedAt:         originalTask.CreatedAt,   // Keep original creation time
			CompletedAt:       originalTask.CompletedAt, // Keep original completion time
			Duration:          split.Duration,
			Category:          originalTask.Category,
			Tags:              originalTask.Tags,
			ToggledAt:         toggledAt,
			IsActive:          originalTask.IsActive,    // Keep original active state
			IsCompleted:       originalTask.IsCompleted, // Keep original completion status
			UserID:            originalTask.UserID,
			LastModifiedAt:    lastEpochMs,
			Priority:          originalTask.Priority,
			DueAt:             originalTask.DueAt,
			ShowBeforeDueTime: originalTask.ShowBeforeDueTime,
		})

		if err != nil {
			return err
		}

		splitTasks = append(splitTasks, splitTask)
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		return err
	}

	// Emit events only if original task was not completed
	if !originalTask.IsCompleted {
		log.Printf("Emitting events for task split - original task ID: %s, splits: %d", originalTask.ID, len(splitTasks))

		// Emit task deleted event for original task
		cfg.WSClientManager.BroadcastToSameUser(
			ctx,
			"related_task_deleted",
			cfg.WSClientManager.clients[SID].User.ID,
			struct {
				ID uuid.UUID `json:"id"`
			}{
				ID: originalTask.ID,
			},
		)

		// Emit new task created events for each split
		for _, splitTask := range splitTasks {
			log.Printf("Emitting new_task_created for split task ID: %s", splitTask.ID)
			cfg.WSClientManager.BroadcastToSameUser(
				ctx,
				"new_task_created",
				cfg.WSClientManager.clients[SID].User.ID,
				splitTask,
			)
		}
	} else {
		log.Printf("Task split completed but original task was already completed - not emitting events")
	}

	return nil
}

func (cfg *config) WSOnNotificationsFetch(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("notifications_fetch").Observe(time.Since(start).Seconds())
	}()

	client, ok := cfg.getClientBySID(SID)
	if !ok {
		return fmt.Errorf("client not found for SID %s", SID)
	}

	type fetchRequest struct {
		Offset            int32    `json:"offset"`
		Limit             int32    `json:"limit"`
		Statuses          []string `json:"statuses"`
		NotificationTypes []string `json:"notification_types"`
		Priorities        []string `json:"priorities"`
		IncludeSnoozed    *bool    `json:"include_snoozed"`
		ExpiredOnly       *bool    `json:"expired_only"`
	}

	var payload struct {
		Data fetchRequest `json:"data"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	const defaultPageSize int32 = 10
	const maxPageSize int32 = 100

	offset := payload.Data.Offset
	if offset < 0 {
		offset = 0
	}

	limit := payload.Data.Limit
	if limit <= 0 {
		limit = defaultPageSize
	}
	if limit > maxPageSize {
		limit = maxPageSize
	}

	params := database.ListNotificationsByUserParams{
		UserID:            client.User.ID,
		Statuses:          payload.Data.Statuses,
		NotificationTypes: payload.Data.NotificationTypes,
		Priorities:        payload.Data.Priorities,
		OffsetVal:         sql.NullInt32{Int32: offset, Valid: true},
		LimitVal:          sql.NullInt32{Int32: limit, Valid: true},
	}

	if payload.Data.IncludeSnoozed != nil {
		params.IncludeSnoozed = sql.NullBool{Bool: *payload.Data.IncludeSnoozed, Valid: true}
	}
	if payload.Data.ExpiredOnly != nil {
		params.ExpiredOnly = sql.NullBool{Bool: *payload.Data.ExpiredOnly, Valid: true}
	}

	notifications, err := cfg.DB.ListNotificationsByUserWithTiming(ctx, params)
	if err != nil {
		return err
	}

	response := struct {
		Notifications []database.Notification `json:"notifications"`
		Offset        int32                   `json:"offset"`
		Limit         int32                   `json:"limit"`
		HasMore       bool                    `json:"has_more"`
	}{
		Notifications: notifications,
		Offset:        offset,
		Limit:         limit,
		HasMore:       int32(len(notifications)) == limit,
	}

	return cfg.WSClientManager.SendToClient(ctx, "notifications_batch", SID, response)
}

func (cfg *config) WSOnNotificationMarkSeen(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("notification_mark_seen").Observe(time.Since(start).Seconds())
	}()

	client, ok := cfg.getClientBySID(SID)
	if !ok {
		return fmt.Errorf("client not found for SID %s", SID)
	}

	var payload struct {
		Data struct {
			NotificationIDs []uuid.UUID `json:"notification_ids"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	if len(payload.Data.NotificationIDs) == 0 {
		return nil
	}

	lastModified := time.Now().UnixMilli()
	updates, err := cfg.DB.MarkNotificationsSeenWithTiming(ctx, database.MarkNotificationsSeenParams{
		LastModifiedAt:  lastModified,
		UserID:          client.User.ID,
		NotificationIds: payload.Data.NotificationIDs,
	})
	if err != nil {
		return err
	}

	cfg.broadcastNotificationSet(ctx, "notifications_marked_seen", client.User.ID, updates)
	cfg.emitNotificationUnseenCount(ctx, client.User.ID)
	return nil
}

func (cfg *config) WSOnNotificationMarkAllSeen(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("notification_mark_all_seen").Observe(time.Since(start).Seconds())
	}()

	client, ok := cfg.getClientBySID(SID)
	if !ok {
		return fmt.Errorf("client not found for SID %s", SID)
	}

	var payload struct {
		Data struct {
			NotificationIDs []uuid.UUID `json:"notification_ids"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	lastModified := time.Now().UnixMilli()
	var (
		updates []database.Notification
		err     error
	)

	if len(payload.Data.NotificationIDs) > 0 {
		updates, err = cfg.DB.MarkNotificationsSeenWithTiming(ctx, database.MarkNotificationsSeenParams{
			LastModifiedAt:  lastModified,
			UserID:          client.User.ID,
			NotificationIds: payload.Data.NotificationIDs,
		})
	} else {
		updates, err = cfg.DB.MarkAllNotificationsSeenWithTiming(ctx, database.MarkAllNotificationsSeenParams{
			UserID:         client.User.ID,
			LastModifiedAt: lastModified,
		})
	}

	if err != nil {
		return err
	}

	cfg.broadcastNotificationSet(ctx, "notifications_marked_seen", client.User.ID, updates)
	cfg.emitNotificationUnseenCount(ctx, client.User.ID)
	return nil
}

func (cfg *config) WSOnNotificationArchive(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("notification_archive").Observe(time.Since(start).Seconds())
	}()

	client, ok := cfg.getClientBySID(SID)
	if !ok {
		return fmt.Errorf("client not found for SID %s", SID)
	}

	var payload struct {
		Data struct {
			NotificationID uuid.UUID `json:"notification_id"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	if payload.Data.NotificationID == uuid.Nil {
		return fmt.Errorf("notification_id is required")
	}

	lastModified := time.Now().UnixMilli()
	notification, err := cfg.DB.ArchiveNotificationWithTiming(ctx, database.ArchiveNotificationParams{
		ID:             payload.Data.NotificationID,
		UserID:         client.User.ID,
		LastModifiedAt: lastModified,
	})
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	cfg.broadcastSingleNotification(ctx, "notification_archived", client.User.ID, notification)
	cfg.emitNotificationUnseenCount(ctx, client.User.ID)
	return nil
}

func (cfg *config) WSOnNotificationSnooze(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	start := time.Now()
	defer func() {
		metrics.WebSocketEventDuration.WithLabelValues("notification_snooze").Observe(time.Since(start).Seconds())
	}()

	client, ok := cfg.getClientBySID(SID)
	if !ok {
		return fmt.Errorf("client not found for SID %s", SID)
	}

	var payload struct {
		Data struct {
			NotificationID uuid.UUID `json:"notification_id"`
			SnoozeUntil    *int64    `json:"snooze_until"`   // epoch millis
			SnoozeMinutes  *int64    `json:"snooze_minutes"` // minutes from now
			SnoozeSeconds  *int64    `json:"snooze_seconds"` // seconds from now
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &payload); err != nil {
		return err
	}

	if payload.Data.NotificationID == uuid.Nil {
		return fmt.Errorf("notification_id is required")
	}

	var snoozeUntil time.Time
	now := time.Now()
	switch {
	case payload.Data.SnoozeUntil != nil:
		snoozeUntil = time.UnixMilli(*payload.Data.SnoozeUntil)
	case payload.Data.SnoozeMinutes != nil:
		snoozeUntil = now.Add(time.Duration(*payload.Data.SnoozeMinutes) * time.Minute)
	case payload.Data.SnoozeSeconds != nil:
		snoozeUntil = now.Add(time.Duration(*payload.Data.SnoozeSeconds) * time.Second)
	default:
		snoozeUntil = now.Add(5 * time.Minute)
	}

	if snoozeUntil.Before(now.Add(5 * time.Second)) {
		snoozeUntil = now.Add(5 * time.Minute)
	}

	lastModified := time.Now().UnixMilli()
	notification, err := cfg.DB.SnoozeNotificationWithTiming(ctx, database.SnoozeNotificationParams{
		ID:             payload.Data.NotificationID,
		SnoozedUntil:   sql.NullTime{Time: snoozeUntil.UTC(), Valid: true},
		UserID:         client.User.ID,
		LastModifiedAt: lastModified,
	})
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}

	cfg.broadcastSingleNotification(ctx, "notification_snoozed", client.User.ID, notification)
	cfg.emitNotificationUnseenCount(ctx, client.User.ID)
	return nil
}

func (cfg *config) emitNotificationUnseenCount(ctx context.Context, userID uuid.UUID) {
	count, err := cfg.DB.CountUnseenNotificationsWithTiming(ctx, userID)
	if err != nil {
		log.Println("failed to compute unseen notification count:", err)
		return
	}

	cfg.WSClientManager.BroadcastToSameUser(ctx, "notifications_unseen_count", userID, struct {
		Count int64 `json:"count"`
	}{
		Count: count,
	})
}

func (cfg *config) broadcastNotificationSet(ctx context.Context, event string, userID uuid.UUID, notifications []database.Notification) {
	if len(notifications) == 0 {
		return
	}

	cfg.WSClientManager.BroadcastToSameUser(ctx, event, userID, struct {
		Notifications []database.Notification `json:"notifications"`
	}{
		Notifications: notifications,
	})
}

func (cfg *config) broadcastSingleNotification(ctx context.Context, event string, userID uuid.UUID, notification database.Notification) {
	cfg.WSClientManager.BroadcastToSameUser(ctx, event, userID, notification)
}

func (cfg *config) DispatchDueNotifications() {
	log.Printf("DispatchDueNotifications cron job started at %s UTC", time.Now().UTC().Format(time.RFC3339))
	ctx := context.Background()
	lastModified := time.Now().UnixMilli()

	// Handle snoozed notifications
	notifications, err := cfg.DB.ReleaseDueSnoozedNotificationsWithTiming(ctx, lastModified)
	if err != nil {
		log.Println("failed to release snoozed notifications:", err)
		return
	}

	if len(notifications) > 0 {
		userBuckets := make(map[uuid.UUID][]database.Notification)
		for _, notification := range notifications {
			userBuckets[notification.UserID] = append(userBuckets[notification.UserID], notification)
		}

		for userID, bucket := range userBuckets {
			cfg.broadcastNotificationSet(ctx, "notifications_reemitted", userID, bucket)
			cfg.emitNotificationUnseenCount(ctx, userID)
		}
	}

	cfg.dispatchTaskVisibility(ctx)
	cfg.dispatchTaskDueNotifications(ctx, lastModified)
	log.Printf("DispatchDueNotifications cron job completed at %s UTC", time.Now().UTC().Format(time.RFC3339))
}

func (cfg *config) dispatchTaskVisibility(ctx context.Context) {
	log.Println("dispatchTaskVisibility: Starting task visibility check")
	tasks, err := cfg.DB.GetTasksDueForVisibilityAllWithTiming(ctx)
	if err != nil {
		log.Printf("Failed to fetch tasks due for visibility: %v", err)
		return
	}
	log.Printf("dispatchTaskVisibility: Found %d tasks due for visibility", len(tasks))
	if len(tasks) == 0 {
		log.Println("dispatchTaskVisibility: No tasks found, returning")
		return
	}

	buckets := make(map[uuid.UUID][]database.Task)
	for _, task := range tasks {
		buckets[task.UserID] = append(buckets[task.UserID], task)
	}

	for userID, bucket := range buckets {
		cfg.WSClientManager.BroadcastToSameUser(ctx, "tasks_became_visible", userID, struct {
			Tasks []database.Task `json:"tasks"`
		}{
			Tasks: bucket,
		})
		log.Printf("Broadcasted %d tasks becoming visible for user %s", len(bucket), userID)
	}
}

type dueStage struct {
	ID          string
	Duration    time.Duration
	Title       string
	Description func(task database.Task) string
	Priority    string
}

var taskDueStages = []dueStage{
	{
		ID:       "48h",
		Duration: 48 * time.Hour,
		Title:    "Task due in 48 hours",
		Description: func(task database.Task) string {
			return fmt.Sprintf("Your task '%s' is due in 48 hours.", task.Title)
		},
		Priority: "low",
	},
	{
		ID:       "24h",
		Duration: 24 * time.Hour,
		Title:    "Task due in 24 hours",
		Description: func(task database.Task) string {
			return fmt.Sprintf("Your task '%s' is due in 24 hours.", task.Title)
		},
		Priority: "low",
	},
	{
		ID:       "12h",
		Duration: 12 * time.Hour,
		Title:    "Task due in 12 hours",
		Description: func(task database.Task) string {
			return fmt.Sprintf("Your task '%s' is due in 12 hours.", task.Title)
		},
		Priority: "normal",
	},
	{
		ID:       "6h",
		Duration: 6 * time.Hour,
		Title:    "Task due in 6 hours",
		Description: func(task database.Task) string {
			return fmt.Sprintf("Your task '%s' is due in 6 hours.", task.Title)
		},
		Priority: "normal",
	},
	{
		ID:       "3h",
		Duration: 3 * time.Hour,
		Title:    "Task due in 3 hours",
		Description: func(task database.Task) string {
			return fmt.Sprintf("Your task '%s' is due in 3 hours.", task.Title)
		},
		Priority: "normal",
	},
	{
		ID:       "1h",
		Duration: time.Hour,
		Title:    "Task due in 1 hour",
		Description: func(task database.Task) string {
			return fmt.Sprintf("Your task '%s' is due in 1 hour.", task.Title)
		},
		Priority: "high",
	},
}

func determineDueStage(diff time.Duration) (dueStage, bool) {
	const triggerWindow = time.Minute
	for _, stage := range taskDueStages {
		if diff <= stage.Duration && diff > stage.Duration-triggerWindow {
			return stage, true
		}
	}
	return dueStage{}, false
}

func (cfg *config) dispatchTaskDueNotifications(ctx context.Context, lastModified int64) {
	tasks, err := cfg.DB.GetUpcomingTasksForNotificationsWithTiming(ctx)
	if err != nil {
		log.Printf("Failed to fetch upcoming tasks for notifications: %v", err)
		return
	}
	if len(tasks) == 0 {
		return
	}

	now := time.Now()
	for _, task := range tasks {
		if !task.DueAt.Valid {
			continue
		}
		diff := task.DueAt.Time.Sub(now)
		if diff <= 0 {
			continue
		}

		stage, ok := determineDueStage(diff)
		if !ok {
			continue
		}

		hasNotification, err := cfg.DB.HasNotificationForTaskStageWithTiming(ctx, task.UserID, "due_task", task.ID.String(), stage.ID)
		if err != nil {
			log.Printf("Failed to check existing notification for task %s stage %s: %v", task.ID, stage.ID, err)
			continue
		}
		if hasNotification {
			continue
		}

		payload := map[string]interface{}{
			"task_id":        task.ID.String(),
			"task_title":     task.Title,
			"due_at":         task.DueAt.Time,
			"stage":          stage.ID,
			"due_in_seconds": int(diff.Seconds()),
			"category":       task.Category,
		}
		payloadJSON, _ := json.Marshal(payload)

		notification, err := cfg.DB.CreateNotificationWithTiming(ctx, database.CreateNotificationParams{
			ID:               uuid.New(),
			UserID:           task.UserID,
			Title:            stage.Title,
			Description:      sql.NullString{String: stage.Description(task), Valid: true},
			Status:           "unseen",
			NotificationType: "due_task",
			Payload:          payloadJSON,
			Priority:         stage.Priority,
			ExpiresAt: sql.NullTime{
				Time:  task.DueAt.Time.Add(24 * time.Hour),
				Valid: true,
			},
			LastModifiedAt: lastModified,
		})
		if err != nil {
			log.Printf("Failed to create due notification for task %s stage %s: %v", task.ID, stage.ID, err)
			continue
		}

		cfg.broadcastSingleNotification(ctx, "notification_created", task.UserID, notification)
		cfg.emitNotificationUnseenCount(ctx, task.UserID)

		log.Printf("Created due notification stage %s for task '%s' (user %s)", stage.ID, task.Title, task.UserID)
	}
}
