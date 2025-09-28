package database

import (
	"context"
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
