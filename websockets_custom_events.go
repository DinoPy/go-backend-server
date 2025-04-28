package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/coder/websocket"
	"github.com/dinopy/taskbar2_server/internal/database"
	"github.com/google/uuid"
)

func (cfg *config) WSOnConnect(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	var connectionData struct {
		Data User `json:"data"`
	}
	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return err
	}

	user, err := cfg.DB.CreateUser(ctx, database.CreateUserParams{
		Email:     connectionData.Data.Email,
		FirstName: connectionData.Data.FirstName,
		LastName:  connectionData.Data.LastName,
	})
	if err != nil {
		return err
	}

	cfg.WSClientManager.AddClient(&Client{
		SID:  SID,
		Conn: c,
		User: user,
	})

	tasks, err := cfg.DB.GetActiveTaskByUUID(ctx, user.ID)
	if err != nil {
		return err
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
	type taskT struct {
		ID             uuid.UUID `json:"id"`
		Title          string    `json:"title"`
		Description    string    `json:"descripiton"`
		CreatedAt      time.Time `json:"created_at"`
		CompletedAt    time.Time `json:"completed_at"`
		Duration       string    `json:"duration"`
		Category       string    `json:"category"`
		Tags           string    `json:"tags"`
		ToggledAt      int64     `json:"toggled_at"`
		IsCompleted    bool      `json:"is_completed"`
		IsActive       bool      `json:"is_active"`
		LastModifiedAt int64     `json:"last_modified_at"`
	}
	var connectionData struct {
		Data taskT `json:"data"`
	}
	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return err
	}

	task, err := cfg.DB.CreateTask(ctx, database.CreateTaskParams{
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
		Tags: sql.NullString{
			Valid:  false,
			String: "",
		},
		ToggledAt: sql.NullInt64{
			Int64: connectionData.Data.ToggledAt,
			Valid: true,
		},
		IsCompleted:    connectionData.Data.IsCompleted,
		IsActive:       connectionData.Data.IsActive,
		LastModifiedAt: connectionData.Data.LastModifiedAt,
		UserID:         cfg.WSClientManager.clients[SID].User.ID,
	})

	if err != nil {
		return err
	}

	cfg.WSClientManager.BroadcastToSameUser(
		ctx,
		"task_create_response",
		cfg.WSClientManager.clients[SID].User.ID,
		task,
	)

	return nil
}

func (cfg config) WSOnTaskToggle(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	type taskT struct {
		UUID           uuid.UUID `json:"uuid"`
		ToggledAt      int64     `json:"toggle_at"`
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

	task, err := cfg.DB.ToggleTask(ctx, database.ToggleTaskParams{
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
	cfg.WSClientManager.BroadcastToSameUser(
		ctx,
		"toggle_task_confirmation",
		cfg.WSClientManager.clients[SID].User.ID,
		task,
	)
	return nil
}

func (cfg config) WSOnTaskEdit(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
	type taskT struct {
		ID             uuid.UUID `json:"id"`
		Title          string    `json:"title"`
		Description    string    `json:"description"`
		Category       string    `json:"category"`
		Tags           string    `json:"tags"`
		LastModifiedAt int64     `json:"last_modified_at"`
	}

	var connectionData struct {
		Data taskT `json:"data"`
	}
	err := json.Unmarshal(data, &connectionData)
	if err != nil {
		return err
	}

	task, err := cfg.DB.EditTask(ctx, database.EditTaskParams{
		ID:          connectionData.Data.ID,
		Title:       connectionData.Data.Title,
		Description: connectionData.Data.Description,
		Category:    connectionData.Data.Category,
		Tags: sql.NullString{
			String: connectionData.Data.Tags,
			Valid:  true,
		},
		LastModifiedAt: connectionData.Data.LastModifiedAt,
	})

	cfg.WSClientManager.BroadcastToSameUser(
		ctx,
		"task_edit_response",
		cfg.WSClientManager.clients[SID].User.ID,
		task,
	)
	return nil
}

func (cfg config) WSOnTaskDelete(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
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

	err = cfg.DB.DeleteTask(ctx, connectionData.Data.ID)
	if err != nil {
		return err
	}

	cfg.WSClientManager.BroadcastToSameUser(
		ctx,
		"task_delete_response",
		cfg.WSClientManager.clients[SID].User.ID,
		struct  {
			ID	uuid.UUID	`json:"id"`
		}{
			ID: connectionData.Data.ID,
		},
	)
	return nil
}
