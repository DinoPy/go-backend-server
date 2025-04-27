package main

import (
	"context"
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
		ID:        connectionData.Data.ID,
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
		User: User(connectionData.Data),
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
