package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/dinopy/taskbar2_server/internal/database"
	"github.com/dinopy/taskbar2_server/internal/metrics"
	"github.com/google/uuid"
)

type EventMessage struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

type User struct {
	ID        uuid.UUID `json:"id"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	GoogleUID string    `json:"google_uid"`
}

type Client struct {
	SID  uuid.UUID
	Conn *websocket.Conn
	User database.User
}

type ClientManager struct {
	clients map[uuid.UUID]*Client
	mu      sync.RWMutex
}

func NewClientManager() *ClientManager {
	return &ClientManager{
		clients: make(map[uuid.UUID]*Client),
	}
}

func (m *ClientManager) AddClient(c *Client) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[c.SID] = c
	metrics.WebSocketConnections.WithLabelValues(c.User.ID.String()).Inc()
	log.Println("Client added:", c.SID)
}

func (m *ClientManager) RemoveClient(id uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if client, exists := m.clients[id]; exists {
		metrics.WebSocketConnections.WithLabelValues(client.User.ID.String()).Dec()
	}
	delete(m.clients, id)
	log.Println("Client removed:", id)
}

func (m *ClientManager) Broadcast(ctx context.Context, event string, data []byte) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, client := range m.clients {
		sendEvent(ctx, client.Conn, event, client.SID)
	}
}

func (m *ClientManager) BroadcastToSameUser(ctx context.Context, event string, UID uuid.UUID, data interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, client := range m.clients {
		if client.User.ID == UID {
			sendEvent(ctx, client.Conn, event, data)
		}
	}
}

func (m *ClientManager) BroadcastToSameUserNoIssuer(ctx context.Context, event string, UID uuid.UUID, SID uuid.UUID, data interface{}) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, client := range m.clients {
		if client.User.ID == UID && client.SID != SID {
			sendEvent(ctx, client.Conn, event, data)
		}
	}
}

func (m *ClientManager) SendToClient(ctx context.Context, event string, SID uuid.UUID, data interface{}) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	client, exists := m.clients[SID]
	if !exists {
		return nil
	}
	return sendEvent(ctx, client.Conn, event, data)
}

func sendEvent(ctx context.Context, c *websocket.Conn, event string, data interface{}) error {
	msg := EventMessage{
		Event: event,
		Data:  data,
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return c.Write(ctx, websocket.MessageText, payload)
}

func (cfg config) wsPing(ctx context.Context, c *websocket.Conn, pongCh chan struct{}) {
	ticker := time.NewTicker(cfg.WSCfg.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := sendEvent(ctx, c, "ping", "")
			if err != nil {
				log.Println("Failed to sendping:", err)
				c.Close(websocket.StatusInternalError, "failed to send ping")
				return
			}

			select {
			case <-pongCh:
			case <-time.After(cfg.WSCfg.pingTimeout):
				log.Println("No pong received, closing connection")
				c.Close(websocket.StatusNormalClosure, "no pong response")
				return
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (cfg *config) WebSocketsHandler(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		fmt.Println("accept error", err)
		return
	}
	SID := uuid.New()

	defer func() {
		c.Close(websocket.StatusInternalError, "server error")
		if SID != uuid.Nil {
			cfg.WSClientManager.RemoveClient(SID)
		}
	}()

	ctx := r.Context()
	pongCh := make(chan struct{})
	go cfg.wsPing(ctx, c, pongCh)

	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusGoingAway {
				log.Printf("Client %s disconnected.\n", SID)
			} else {
				log.Println("read error: ", err)
			}
			return
		}

		var msg EventMessage
		err = json.Unmarshal(data, &msg)
		if err != nil {
			log.Println("Could not unmarshal event, err:", err)
			continue
		}

		if msg.Event != "pong" {
			log.Println("Received", msg.Event, "from", SID.String())
		}

		switch msg.Event {
		case "pong":
			select {
			case pongCh <- struct{}{}:
			default:
			}
		case "connect":
			err := cfg.WSOnConnect(ctx, c, SID, data)
			if err != nil {
				log.Println("Error occured in onConnect function:", err)
				return
			}
		case "task_create":
			err := cfg.WSOnTaskCreate(ctx, c, SID, data)
			if err != nil {
				log.Println("Error occured in onTaskCreate function:", err)
				return
			}
		case "task_toggle":
			err := cfg.WSOnTaskToggle(ctx, c, SID, data)
			if err != nil {
				log.Println("Error occured in onTaskToggle function:", err)
				return
			}
		case "task_edit":
			err := cfg.WSOnTaskEdit(ctx, c, SID, data)
			if err != nil {
				log.Println("Error occured in onTaskEdit function: ", err)
			}
		case "task_completed":
			err := cfg.WSOnTaskCompleted(ctx, c, SID, data)
			if err != nil {
				log.Println("Error occured in onTaskComplete function: ", err)
			}
		case "task_delete":
			err := cfg.WSOnTaskDelete(ctx, c, SID, data)
			if err != nil {
				log.Println("Error occured in onTaskDelete function: ", err)
			}
		case "get_completed_tasks":
			err := cfg.WSOnGetCompletedTasks(ctx, c, SID, data)
			if err != nil {
				log.Println("Error occured in OnGetCompletedTasks function: ", err)
			}
		case "request_hard_refresh":
			err := cfg.WSOnRequestHardRefresh(ctx, c, SID, data)
			if err != nil {
				log.Println("Error occured in OnRequestHardRefresh function: ", err)
			}
		case "user_updated_categories":
			err := cfg.WSOnUserUpdatedCategories(ctx, c, SID, data)
			if err != nil {
				log.Println("Error occured in OnUserUpdatedCategories function: ", err)
			}
		case "new_command_added":
			err := cfg.WSOnNewCommandAdded(ctx, c, SID, data)
			if err != nil {
				log.Println("Error occured in OnNewCommandAdded function: ", err)
			}
		case "command_removed":
			err := cfg.WSOnNewCommandAdded(ctx, c, SID, data)
			if err != nil {
				log.Println("Error occured in OnNewCommandAdded function: ", err)
			}
		case "taskbar-update":
			cfg.WSClientManager.BroadcastToSameUser(ctx, "taskbar-ack", cfg.WSClientManager.clients[SID].User.ID, "From "+SID.String())
		default:
			log.Println("Unknown event:", msg.Event)
		}
	}
}
