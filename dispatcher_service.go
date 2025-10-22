package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/dinopy/taskbar2_server/internal/database"
	"github.com/google/uuid"
)

type DispatcherService struct {
	queries      *database.Queries
	batchSize    int
	sendToClient func(userID uuid.UUID, notification database.Notification) error
}

func NewDispatcherService(queries *database.Queries, batchSize int, sendToClient func(userID uuid.UUID, notification database.Notification) error) *DispatcherService {
	return &DispatcherService{
		queries:      queries,
		batchSize:    batchSize,
		sendToClient: sendToClient,
	}
}

func (s *DispatcherService) Tick(ctx context.Context) error {
	log.Printf("DispatcherService: Starting dispatcher tick at %s", time.Now().Format(time.RFC3339))

	// Claim due notification jobs
	jobs, err := s.queries.ClaimDueNotificationJobs(ctx, int32(s.batchSize))
	if err != nil {
		log.Printf("DispatcherService: Failed to claim due jobs: %v", err)
		return err
	}

	log.Printf("DispatcherService: Claimed %d due notification jobs", len(jobs))

	for _, job := range jobs {
		if err := s.processJob(ctx, job); err != nil {
			log.Printf("DispatcherService: Failed to process job %s: %v", job.ID, err)
			// Continue processing other jobs
			continue
		}
	}

	log.Printf("DispatcherService: Completed dispatcher tick")
	return nil
}

func (s *DispatcherService) processJob(ctx context.Context, job database.ClaimDueNotificationJobsRow) error {
	log.Printf("DispatcherService: Processing job %s for user %s", job.ID, job.UserID)

	// Parse job payload
	var payload map[string]interface{}
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		log.Printf("DispatcherService: Failed to parse job payload: %v", err)
		return err
	}

	// Determine notification type and content based on kind and offset
	notificationType := "reminder"
	title := "Reminder"
	description := ""
	priority := "normal"

	kind, _ := payload["kind"].(string)

	if job.OffsetMinutes == 0 {
		// At-time notification (only for reminders now)
		if kind == "reminder" {
			notificationType = "reminder"
			title = "Reminder"
		}
	} else {
		// Pre-notification (only for tasks now)
		if kind == "task" {
			notificationType = "reminder"
			title = "Upcoming Task"
		}
	}

	// Set description based on payload
	if taskTitle, ok := payload["title"].(string); ok {
		if job.OffsetMinutes == 0 {
			// At-time notification (only for reminders)
			if kind == "reminder" {
				description = "Reminder: " + taskTitle
			}
		} else {
			// Pre-notification (only for tasks)
			if kind == "task" {
				hours := job.OffsetMinutes / 60
				if hours >= 24 {
					days := hours / 24
					description = "Your task '" + taskTitle + "' is due in " + formatDays(days) + "."
				} else {
					description = "Your task '" + taskTitle + "' is due in " + formatHours(hours) + "."
				}
			}
		}
	}

	// Set priority based on offset
	if job.OffsetMinutes <= 60 {
		priority = "high"
	} else if job.OffsetMinutes <= 360 {
		priority = "normal"
	} else {
		priority = "low"
	}

	// Create notification in database
	notification, err := s.queries.CreateNotification(ctx, database.CreateNotificationParams{
		ID:               uuid.New(),
		UserID:           job.UserID,
		Title:            title,
		Description:      sql.NullString{String: description, Valid: description != ""},
		Status:           "unseen",
		NotificationType: notificationType,
		Payload:          job.Payload,
		Priority:         priority,
		ExpiresAt:        sql.NullTime{Time: time.Now().Add(24 * time.Hour), Valid: true},
		LastModifiedAt:   time.Now().UnixMilli(),
	})
	if err != nil {
		log.Printf("DispatcherService: Failed to create notification: %v", err)
		return err
	}

	// Send to client via WebSocket
	if s.sendToClient != nil {
		if err := s.sendToClient(job.UserID, notification); err != nil {
			log.Printf("DispatcherService: Failed to send notification to client: %v", err)
			// Don't return error - notification is already in database
		}
	}

	log.Printf("DispatcherService: Successfully processed job %s", job.ID)
	return nil
}

// Helper functions for formatting time descriptions
func formatDays(days int32) string {
	if days == 1 {
		return "1 day"
	}
	return fmt.Sprintf("%d days", days)
}

func formatHours(hours int32) string {
	if hours == 1 {
		return "1 hour"
	}
	return fmt.Sprintf("%d hours", hours)
}
