package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/dinopy/taskbar2_server/internal/database"
	"github.com/dinopy/taskbar2_server/internal/metrics"
	"github.com/google/uuid"
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
		return sendError(c, ErrorDatabaseError, "Failed to load tasks", 500)
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
		SID         uuid.UUID       `json:"sid"`
		ID          uuid.UUID       `json:"id"`
		FirstName   string          `json:"first_name"`
		LastName    string          `json:"last_name"`
		Email       string          `json:"email"`
		CreatedAt   time.Time       `json:"created_at"`
		UpdatedAt   time.Time       `json:"updated_at"`
		Categories  string          `json:"categories"`
		KeyCommands string          `json:"key_commands"`
		Tasks       []database.Task `json:"tasks"`
	}

	cfg.WSClientManager.SendToClient(ctx, "connected", SID, finalUser{
		SID:         SID,
		ID:          user.ID,
		FirstName:   user.FirstName,
		LastName:    user.LastName,
		Email:       user.Email,
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Categories:  category,
		KeyCommands: keyCommands,
		Tasks:       tasks,
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
			IsActive:       task.IsActive,
			IsCompleted:    false,
			UserID:         task.UserID,
			LastModifiedAt: lastEpochMs,
			// ADD THESE MISSING PROPERTIES:
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
