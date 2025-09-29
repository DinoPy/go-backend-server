# Task Duplication TODO

## Add Task Duplication Functionality

### Task 1: Add WebSocket event handler
- [x] Add new case in `WebSocketsHandler` switch statement in `websockets.go`:
  ```go
  case "task_duplicate":
      err := cfg.WSOnTaskDuplicate(ctx, c, SID, data)
      if err != nil {
          log.Println("Error occurred in onTaskDuplicate function:", err)
          return
      }
  ```

### Task 2: Create WSOnTaskDuplicate function
- [x] Add new function `WSOnTaskDuplicate` in `websockets_custom_events.go`:
  ```go
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
          ID:          uuid.New(),                    // New ID
          Title:       originalTask.Title,            // Copy
          Description: originalTask.Description,      // Copy
          CreatedAt:   time.Now().UTC(),              // Current time
          CompletedAt: sql.NullTime{Valid: false},    // Null (not completed)
          Duration:    "00:00:00",                    // Reset to zero
          Category:    originalTask.Category,         // Copy
          Tags:        originalTask.Tags,             // Copy
          ToggledAt:   sql.NullInt64{Valid: false},   // Reset to null
          IsActive:    false,                         // Reset to false
          IsCompleted: false,                         // Reset to false
          UserID:      originalTask.UserID,           // Copy
          LastModifiedAt: time.Now().UnixMilli(),     // Current time
          Priority:    originalTask.Priority,         // Copy
          DueAt:       originalTask.DueAt,            // Copy
          ShowBeforeDueTime: originalTask.ShowBeforeDueTime, // Copy
      })

      if err != nil {
          return err
      }

      // Emit the new task via new_task_created event
      cfg.WSClientManager.BroadcastToSameUserNoIssuer(
          ctx,
          "new_task_created",
          cfg.WSClientManager.clients[SID].User.ID,
          SID,
          duplicateTask,
      )

      return nil
  }
  ```

### Task 3: Add GetTaskByID database query
- [ ] Add new query to `sql/queries/tasks.sql`:
  ```sql
  -- name: GetTaskByID :one
  SELECT * FROM tasks WHERE id = $1;
  ```

### Task 4: Regenerate database models
- [ ] Run `sqlc generate` to update Go models
- [ ] Verify `GetTaskByID` function is generated in `internal/database/tasks.sql.go`

### Task 5: Add timing wrapper for GetTaskByID
- [ ] Add timing wrapper to `internal/database/db_prometheus_timing.go`:
  ```go
  func (q *Queries) GetTaskByIDWithTiming(ctx context.Context, id uuid.UUID) (Task, error) {
      start := time.Now()
      defer func() {
          metrics.DatabaseQueryDuration.WithLabelValues("get_task_by_id").Observe(time.Since(start).Seconds())
      }()
      return q.GetTaskByID(ctx, id)
  }
  ```

### Task 6: Test the functionality
- [ ] Start server and verify it starts without errors
- [ ] Test task duplication with valid task ID:
  ```json
  {
    "event": "task_duplicate",
    "data": {
      "task_id": "existing-task-uuid"
    }
  }
  ```
- [ ] Verify the duplicated task has all properties copied except the reset ones
- [ ] Check that the new task is emitted via `new_task_created` event
- [ ] Test with non-existent task ID (should return error)
- [ ] Test with task belonging to different user (should return unauthorized error)

### Task 7: Verify property copying
- [ ] Create a task with all properties set (priority, due date, show timing, etc.)
- [ ] Duplicate the task
- [ ] Verify the duplicate has:
  - ✅ New ID
  - ✅ Current creation time
  - ✅ Duration reset to "00:00:00"
  - ✅ IsActive = false
  - ✅ IsCompleted = false
  - ✅ CompletedAt = null
  - ✅ ToggledAt = null
  - ✅ All other properties copied (title, description, category, tags, priority, due date, show timing)

## Notes
- The duplicate task will appear in the frontend via the `new_task_created` event
- All task properties are preserved except the ones that should be reset for a new task
- User authorization is checked to prevent duplicating other users' tasks
- The function includes proper error handling and metrics
- Backward compatible - no impact on existing functionality
