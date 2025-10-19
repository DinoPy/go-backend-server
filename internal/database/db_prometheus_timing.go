package database

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dinopy/taskbar2_server/internal/metrics"
	"github.com/google/uuid"
)

// Wrapper functions with timing metrics
func (q *Queries) CreateUserWithTiming(ctx context.Context, arg CreateUserParams) (User, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("create_user").Observe(time.Since(start).Seconds())
	}()
	return q.CreateUser(ctx, arg)
}

func (q *Queries) GetActiveTaskByUUIDWithTiming(ctx context.Context, userID uuid.UUID) ([]Task, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("get_active_tasks").Observe(time.Since(start).Seconds())
	}()
	return q.GetActiveTaskByUUID(ctx, userID)
}

func (q *Queries) GetTasksDueForVisibilityWithTiming(ctx context.Context, userID uuid.UUID) ([]Task, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("get_tasks_due_for_visibility").Observe(time.Since(start).Seconds())
	}()
	return q.GetTasksDueForVisibility(ctx, userID)
}

func (q *Queries) GetTasksDueForNotificationsWithTiming(ctx context.Context, userID uuid.UUID) ([]Task, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("get_tasks_due_for_notifications").Observe(time.Since(start).Seconds())
	}()
	return q.GetTasksDueForNotifications(ctx, userID)
}

func (q *Queries) CreateNotificationWithTiming(ctx context.Context, arg CreateNotificationParams) (Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("create_notification").Observe(time.Since(start).Seconds())
	}()
	return q.CreateNotification(ctx, arg)
}

func (q *Queries) GetNotificationByTaskAndTypeWithTiming(ctx context.Context, userID uuid.UUID, notificationType string, taskID string) (Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("get_notification_by_task_and_type").Observe(time.Since(start).Seconds())
	}()

	// Create payload JSON for the task ID
	payload := map[string]interface{}{
		"task_id": taskID,
	}
	payloadJSON, _ := json.Marshal(payload)

	return q.GetNotificationByTaskAndType(ctx, GetNotificationByTaskAndTypeParams{
		UserID:           userID,
		NotificationType: notificationType,
		Payload:          payloadJSON,
	})
}

func (q *Queries) CreateTaskWithTiming(ctx context.Context, arg CreateTaskParams) (Task, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("create_task").Observe(time.Since(start).Seconds())
	}()
	return q.CreateTask(ctx, arg)
}

func (q *Queries) ToggleTaskWithTiming(ctx context.Context, arg ToggleTaskParams) (Task, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("toggle_task").Observe(time.Since(start).Seconds())
	}()
	return q.ToggleTask(ctx, arg)
}

func (q *Queries) CompleteTaskWithTiming(ctx context.Context, arg CompleteTaskParams) (Task, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("complete_task").Observe(time.Since(start).Seconds())
	}()
	return q.CompleteTask(ctx, arg)
}

func (q *Queries) EditTaskWithTiming(ctx context.Context, arg EditTaskParams) (Task, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("edit_task").Observe(time.Since(start).Seconds())
	}()
	return q.EditTask(ctx, arg)
}

func (q *Queries) DeleteTaskWithTiming(ctx context.Context, id uuid.UUID) error {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("delete_task").Observe(time.Since(start).Seconds())
	}()
	return q.DeleteTask(ctx, id)
}

func (q *Queries) GetCompletedTasksByUUIDWithTiming(ctx context.Context, arg GetCompletedTasksByUUIDParams) ([]Task, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("get_completed_tasks").Observe(time.Since(start).Seconds())
	}()
	return q.GetCompletedTasksByUUID(ctx, arg)
}

func (q *Queries) GetUserSettingsWithTiming(ctx context.Context, id uuid.UUID) (GetUserSettingsRow, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("get_user_settings").Observe(time.Since(start).Seconds())
	}()
	return q.GetUserSettings(ctx, id)
}

func (q *Queries) UpdateUserCategoriesWithTiming(ctx context.Context, arg UpdateUserCategoriesParams) (User, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("update_user_categories").Observe(time.Since(start).Seconds())
	}()
	return q.UpdateUserCategories(ctx, arg)
}

func (q *Queries) UpdateUserCommandsWithTiming(ctx context.Context, arg UpdateUserCommandsParams) (User, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("update_user_commands").Observe(time.Since(start).Seconds())
	}()
	return q.UpdateUserCommands(ctx, arg)
}

func (q *Queries) GetNonCompletedTasksWithTiming(ctx context.Context) ([]Task, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("get_non_completed_tasks").Observe(time.Since(start).Seconds())
	}()
	return q.GetNonCompletedTasks(ctx)
}

func (q *Queries) GetTaskByIDWithTiming(ctx context.Context, id uuid.UUID) (Task, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("get_task_by_id").Observe(time.Since(start).Seconds())
	}()
	return q.GetTaskByID(ctx, id)
}

func (q *Queries) GetNotificationByIDWithTiming(ctx context.Context, id uuid.UUID) (Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("get_notification_by_id").Observe(time.Since(start).Seconds())
	}()
	return q.GetNotificationByID(ctx, id)
}

func (q *Queries) ListNotificationsByUserWithTiming(ctx context.Context, arg ListNotificationsByUserParams) ([]Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("list_notifications_by_user").Observe(time.Since(start).Seconds())
	}()
	return q.ListNotificationsByUser(ctx, arg)
}

func (q *Queries) MarkNotificationSeenWithTiming(ctx context.Context, arg MarkNotificationSeenParams) (Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("mark_notification_seen").Observe(time.Since(start).Seconds())
	}()
	return q.MarkNotificationSeen(ctx, arg)
}

func (q *Queries) MarkNotificationsSeenWithTiming(ctx context.Context, arg MarkNotificationsSeenParams) ([]Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("mark_notifications_seen").Observe(time.Since(start).Seconds())
	}()
	return q.MarkNotificationsSeen(ctx, arg)
}

func (q *Queries) MarkAllNotificationsSeenWithTiming(ctx context.Context, arg MarkAllNotificationsSeenParams) ([]Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("mark_all_notifications_seen").Observe(time.Since(start).Seconds())
	}()
	return q.MarkAllNotificationsSeen(ctx, arg)
}

func (q *Queries) ArchiveNotificationWithTiming(ctx context.Context, arg ArchiveNotificationParams) (Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("archive_notification").Observe(time.Since(start).Seconds())
	}()
	return q.ArchiveNotification(ctx, arg)
}

func (q *Queries) ArchiveAllNotificationsWithTiming(ctx context.Context, arg ArchiveAllNotificationsParams) ([]Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("archive_all_notifications").Observe(time.Since(start).Seconds())
	}()
	return q.ArchiveAllNotifications(ctx, arg)
}

func (q *Queries) SnoozeNotificationWithTiming(ctx context.Context, arg SnoozeNotificationParams) (Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("snooze_notification").Observe(time.Since(start).Seconds())
	}()
	return q.SnoozeNotification(ctx, arg)
}

func (q *Queries) UpdateNotificationDetailsWithTiming(ctx context.Context, arg UpdateNotificationDetailsParams) (Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("update_notification_details").Observe(time.Since(start).Seconds())
	}()
	return q.UpdateNotificationDetails(ctx, arg)
}

func (q *Queries) CountUnseenNotificationsWithTiming(ctx context.Context, userID uuid.UUID) (int64, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("count_unseen_notifications").Observe(time.Since(start).Seconds())
	}()
	return q.CountUnseenNotifications(ctx, userID)
}

func (q *Queries) GetNotificationsByTypeWithTiming(ctx context.Context, arg GetNotificationsByTypeParams) ([]Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("get_notifications_by_type").Observe(time.Since(start).Seconds())
	}()
	return q.GetNotificationsByType(ctx, arg)
}

func (q *Queries) GetExpiredNotificationsWithTiming(ctx context.Context) ([]Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("get_expired_notifications").Observe(time.Since(start).Seconds())
	}()
	return q.GetExpiredNotifications(ctx)
}

func (q *Queries) ReleaseDueSnoozedNotificationsWithTiming(ctx context.Context, lastModifiedAt int64) ([]Notification, error) {
	start := time.Now()
	defer func() {
		metrics.DatabaseQueryDuration.WithLabelValues("release_due_snoozed_notifications").Observe(time.Since(start).Seconds())
	}()
	return q.ReleaseDueSnoozedNotifications(ctx, lastModifiedAt)
}
