# Task Split Feature TODO

## Add Task Split Functionality

### Task 1: Add WebSocket event handler
- [x] Add new case in `WebSocketsHandler` switch statement in `websockets.go`:
  ```go
  case "task_split":
      err := cfg.WSOnTaskSplit(ctx, c, SID, data)
      if err != nil {
          log.Println("Error occurred in onTaskSplit function:", err)
          return
      }
  ```

### Task 2: Create WSOnTaskSplit function
- [x] Add new function `WSOnTaskSplit` in `websockets_custom_events.go`:
  ```go
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

      // Get the original task from database
      originalTask, err := cfg.DB.GetTaskByIDWithTiming(ctx, request.Data.TaskID)
      if err != nil {
          return err
      }

      // Verify the task belongs to the requesting user
      if originalTask.UserID != cfg.WSClientManager.clients[SID].User.ID {
          return sendError(c, "unauthorized", "Task does not belong to user", 403)
      }

      // Start database transaction
      tx, err := cfg.DB.db.BeginTx(ctx, nil)
      if err != nil {
          return err
      }
      defer tx.Rollback()

      queries := cfg.DB.WithTx(tx)

      // Delete the original task
      err = queries.DeleteTaskWithTiming(ctx, originalTask.ID)
      if err != nil {
          return err
      }

      // Create split tasks
      var splitTasks []database.Task
      currentTime := time.Now().UTC()
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

          splitTask, err := queries.CreateTaskWithTiming(ctx, database.CreateTaskParams{
              ID:          uuid.New(),
              Title:       split.Title,
              Description: split.Description,
              CreatedAt:   originalTask.CreatedAt, // Keep original creation time
              CompletedAt: sql.NullTime{Valid: false},
              Duration:    split.Duration,
              Category:    originalTask.Category,
              Tags:        originalTask.Tags,
              ToggledAt:   toggledAt,
              IsActive:    originalTask.IsActive, // Keep original active state
              IsCompleted: false, // Always reset to false
              UserID:      originalTask.UserID,
              LastModifiedAt: lastEpochMs,
              Priority:    originalTask.Priority,
              DueAt:       originalTask.DueAt,
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
          // Emit task deleted event for original task
          cfg.WSClientManager.BroadcastToSameUserNoIssuer(
              ctx,
              "related_task_deleted",
              cfg.WSClientManager.clients[SID].User.ID,
              SID,
              struct {
                  ID uuid.UUID `json:"id"`
              }{
                  ID: originalTask.ID,
              },
          )

          // Emit new task created events for each split
          for _, splitTask := range splitTasks {
              cfg.WSClientManager.BroadcastToSameUserNoIssuer(
                  ctx,
                  "new_task_created",
                  cfg.WSClientManager.clients[SID].User.ID,
                  SID,
                  splitTask,
              )
          }
      }

      return nil
  }
  ```

### Task 3: Test the functionality
- [x] Start server and verify it starts without errors
- [x] Test task split with valid task ID and splits:
  ```json
  {
    "event": "task_split",
    "data": {
      "task_id": "existing-task-uuid",
      "splits": [
        {
          "title": "Part 1",
          "description": "First part description",
          "duration": "01:30:00"
        },
        {
          "title": "Part 2",
          "description": "Second part description", 
          "duration": "00:45:00"
        }
      ]
    }
  }
  ```

### Task 4: Test edge cases
- [ ] Test with empty splits array (should return error)
- [ ] Test with non-existent task ID (should return error)
- [ ] Test with task belonging to different user (should return unauthorized error)
- [ ] Test splitting a completed task (should not emit events)
- [ ] Test splitting an active task (should preserve active state and set toggled_at)

### Task 5: Verify property handling
- [ ] Create a task with all properties set (priority, due date, show timing, etc.)
- [ ] Split the task into multiple parts
- [ ] Verify each split task has:
  - ✅ New ID
  - ✅ Split title and description
  - ✅ Split duration
  - ✅ Original creation time preserved
  - ✅ Original category, tags, priority, due date, show timing preserved
  - ✅ IsCompleted = false (always reset)
  - ✅ ToggledAt = 0 (or current time if original was active)
  - ✅ IsActive = original task's active state

### Task 6: Verify event emission
- [ ] Test splitting an incomplete task:
  - Should emit `related_task_deleted` for original
  - Should emit `new_task_created` for each split
- [ ] Test splitting a completed task:
  - Should NOT emit any events
  - Task should still be split in database

### Task 7: Test transaction rollback
- [ ] Test with invalid split data that causes database error
- [ ] Verify that original task is not deleted if split creation fails
- [ ] Verify transaction rollback works correctly

## Notes
- Uses database transaction to ensure atomicity
- Only emits events if original task was not completed
- Preserves original task's active state and toggled_at timing
- All task properties are copied except duration, title, description
- IsCompleted is always reset to false for splits
- Follows existing error handling and metrics patterns
- Backward compatible - no impact on existing functionality
