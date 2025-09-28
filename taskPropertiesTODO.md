# Task Properties Enhancement TODO

## Database Schema Changes

### Task 1: Create database migration for new task properties
- [x] Create new migration file: `sql/schema/10_add_task_properties.sql`
- [x] Add the following SQL:
  ```sql
  -- +goose Up
  ALTER TABLE tasks ADD COLUMN priority INTEGER;
  ALTER TABLE tasks ADD COLUMN due_at TIMESTAMP;
  ALTER TABLE tasks ADD COLUMN show_before_due_time INTEGER; -- minutes before due date
  
  -- +goose Down
  ALTER TABLE tasks DROP COLUMN priority;
  ALTER TABLE tasks DROP COLUMN due_at;
  ALTER TABLE tasks DROP COLUMN show_before_due_time;
  ```
- [x] Run migration: `goose up` ✅ COMPLETED

### Task 2: Update SQL queries
- [x] Update `sql/queries/tasks.sql` to include new fields in all queries: ✅ COMPLETED
  ```sql
  -- name: GetTasks :many
  SELECT * FROM TASKS ORDER BY created_at ASC;

  -- name: GetNonCompletedTasks :many
  SELECT *
  FROM TASKS
  WHERE is_completed = FALSE
  ORDER BY user_id;

  -- name: GetCompletedTasksByUUID :many
  SELECT * 
  FROM tasks
  WHERE user_id = @user_id
      AND is_completed = TRUE
      AND (
        sqlc.narg(start_date)::timestamp IS NULL OR completed_at >= sqlc.narg(start_date)::timestamp
      )
      AND (
        sqlc.narg(end_date)::timestamp IS NULL OR completed_at <= sqlc.narg(end_date)::timestamp
      )
      AND (
          cardinality(@tags::text[]) = 0
          OR EXISTS (
              SELECT 1
              FROM unnest(@tags::text[]) AS tag_filter
              WHERE tag_filter ILIKE ANY (tags)
          )
      )
        AND (
          sqlc.narg(search_query)::text IS NULL OR title ILIKE sqlc.narg(search_query)::text
        )
        AND (
          sqlc.narg(category)::text IS NULL OR category = sqlc.narg(category)::text
        )
  ORDER BY created_at ASC;

  -- name: GetActiveTaskByUUID :many
  SELECT * 
  FROM tasks
  WHERE user_id = $1 AND is_completed = FALSE
  ORDER BY created_at ASC;

  -- name: CreateTask :one
  INSERT INTO tasks (
      id,
      title,
      description,
      created_at,
      completed_at,
      duration,
      category,
      tags,
      toggled_at,
      is_active,
      is_completed,
      user_id,
      last_modified_at,
      priority,
      due_at,
      show_before_due_time
  ) VALUES (
      $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16
  ) RETURNING *;

  -- name: ToggleTask :one
  UPDATE tasks
  SET 
      is_active = $2,
      toggled_at = $3,
      duration = $4,
      last_modified_at = $5
  WHERE 
      id = $1
  RETURNING *;

  -- name: CompleteTask :one
  UPDATE tasks
  SET
      is_active = FALSE,
      is_completed = TRUE,
      duration = $2,
      completed_at = $3,
      last_modified_at = $4
  WHERE id = $1
  RETURNING *;

  -- name: EditTask :one
  UPDATE tasks
  SET
      title = $2,
      description = $3,
      category = $4,
      tags = $5,
      last_modified_at = $6,
      priority = $7,
      due_at = $8,
      show_before_due_time = $9
  WHERE id = $1
  RETURNING *;

  -- name: DeleteTask :exec
  DELETE FROM tasks
  WHERE id = $1;
  ```

### Task 3: Regenerate database models
- [x] Run `sqlc generate` to update Go models ✅ COMPLETED
- [x] Verify `internal/database/models.go` includes new fields: ✅ COMPLETED
  ```go
  type Task struct {
      ID                 uuid.UUID      `json:"id"`
      Title              string         `json:"title"`
      Description        string         `json:"description"`
      CreatedAt          time.Time      `json:"created_at"`
      CompletedAt        sql.NullTime   `json:"completed_at"`
      Duration           string         `json:"duration"`
      Category           string         `json:"category"`
      Tags               []string       `json:"tags"`
      ToggledAt          sql.NullInt64  `json:"toggled_at"`
      IsActive           bool           `json:"is_active"`
      IsCompleted        bool           `json:"is_completed"`
      UserID             uuid.UUID      `json:"user_id"`
      LastModifiedAt     int64          `json:"last_modified_at"`
      Priority           sql.NullInt32  `json:"priority"`
      DueAt              sql.NullTime   `json:"due_at"`
      ShowBeforeDueTime  sql.NullInt32  `json:"show_before_due_time"`
  }
  ```

## WebSocket Event Updates

### Task 4: Update WebSocket task struct
- [x] Update `taskT` struct in `websockets_custom_events.go` to include new fields: ✅ COMPLETED
  ```go
  type taskT struct {
      ID                 uuid.UUID  `json:"id"`
      Title              string     `json:"title"`
      Description        string     `json:"description"`
      CreatedAt          time.Time  `json:"created_at"`
      CompletedAt        time.Time  `json:"completed_at"`
      Duration           string     `json:"duration"`
      Category           string     `json:"category"`
      Tags               []string   `json:"tags"`
      ToggledAt          int64      `json:"toggled_at"`
      IsCompleted        bool       `json:"is_completed"`
      IsActive           bool       `json:"is_active"`
      LastModifiedAt     int64      `json:"last_modified_at"`
      Priority           *int32     `json:"priority"`
      DueAt              *time.Time `json:"due_at"`
      ShowBeforeDueTime  *int32     `json:"show_before_due_time"`
  }
  ```

### Task 5: Update WSOnTaskCreate function
- [x] Update `WSOnTaskCreate` in `websockets_custom_events.go`: ✅ COMPLETED
  ```go
  func (cfg *config) WSOnTaskCreate(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
      type taskT struct {
          ID                 uuid.UUID  `json:"id"`
          Title              string     `json:"title"`
          Description        string     `json:"description"`
          CreatedAt          time.Time  `json:"created_at"`
          CompletedAt        time.Time  `json:"completed_at"`
          Duration           string     `json:"duration"`
          Category           string     `json:"category"`
          Tags               []string   `json:"tags"`
          ToggledAt          int64      `json:"toggled_at"`
          IsCompleted        bool       `json:"is_completed"`
          IsActive           bool       `json:"is_active"`
          LastModifiedAt     int64      `json:"last_modified_at"`
          Priority           *int32     `json:"priority"`
          DueAt              *time.Time `json:"due_at"`
          ShowBeforeDueTime  *int32     `json:"show_before_due_time"`
      }
      
      var connectionData struct {
          Data taskT `json:"data"`
      }
      err := json.Unmarshal(data, &connectionData)
      if err != nil {
          return err
      }

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
          ID:                 connectionData.Data.ID,
          Title:              connectionData.Data.Title,
          Description:        connectionData.Data.Description,
          CreatedAt:          connectionData.Data.CreatedAt,
          CompletedAt:        sql.NullTime{
              Valid: true,
              Time:  connectionData.Data.CompletedAt,
          },
          Duration:           connectionData.Data.Duration,
          Category:           connectionData.Data.Category,
          Tags:               connectionData.Data.Tags,
          ToggledAt:          sql.NullInt64{
              Int64: connectionData.Data.ToggledAt,
              Valid: true,
          },
          IsCompleted:        connectionData.Data.IsCompleted,
          IsActive:           connectionData.Data.IsActive,
          LastModifiedAt:     connectionData.Data.LastModifiedAt,
          UserID:             cfg.WSClientManager.clients[SID].User.ID,
          Priority:           priority,
          DueAt:              dueAt,
          ShowBeforeDueTime:  showBeforeDueTime,
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
  ```

### Task 6: Update WSOnTaskEdit function
- [x] Update `WSOnTaskEdit` in `websockets_custom_events.go`: ✅ COMPLETED
  ```go
  func (cfg *config) WSOnTaskEdit(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
      type taskT struct {
          ID                 uuid.UUID  `json:"id"`
          Title              string     `json:"title"`
          Description        string     `json:"description"`
          Category           string     `json:"category"`
          Tags               []string   `json:"tags"`
          LastModifiedAt     int64      `json:"last_modified_at"`
          Priority           *int32     `json:"priority"`
          DueAt              *time.Time `json:"due_at"`
          ShowBeforeDueTime  *int32     `json:"show_before_due_time"`
      }

      var connectionData struct {
          Data taskT `json:"data"`
      }
      err := json.Unmarshal(data, &connectionData)
      if err != nil {
          return err
      }

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
          ID:                 connectionData.Data.ID,
          Title:              connectionData.Data.Title,
          Description:        connectionData.Data.Description,
          Category:           connectionData.Data.Category,
          Tags:               connectionData.Data.Tags,
          LastModifiedAt:     connectionData.Data.LastModifiedAt,
          Priority:           priority,
          DueAt:              dueAt,
          ShowBeforeDueTime:  showBeforeDueTime,
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
  ```

## Testing

### Task 7: Test database migration
- [ ] Run `goose up` and verify new columns are added
- [ ] Run `goose down` and verify columns are removed
- [ ] Run `goose up` again to restore the columns

### Task 8: Test WebSocket events
- [ ] Start server and verify it starts without errors
- [ ] Test task creation with new properties:
  ```json
  {
    "event": "task_create",
    "data": {
      "id": "uuid",
      "title": "Test Task",
      "description": "Test Description",
      "created_at": "2024-01-01T00:00:00Z",
      "completed_at": "2024-01-01T00:00:00Z",
      "duration": "00:00:00",
      "category": "Test",
      "tags": ["test"],
      "toggled_at": 0,
      "is_completed": false,
      "is_active": false,
      "last_modified_at": 1640995200000,
      "priority": 5,
      "due_at": "2024-12-31T23:59:59Z",
      "show_before_due_time": 300
    }
  }
  ```
- [ ] Test task creation without new properties (backward compatibility):
  ```json
  {
    "event": "task_create",
    "data": {
      "id": "uuid",
      "title": "Test Task",
      "description": "Test Description",
      "created_at": "2024-01-01T00:00:00Z",
      "completed_at": "2024-01-01T00:00:00Z",
      "duration": "00:00:00",
      "category": "Test",
      "tags": ["test"],
      "toggled_at": 0,
      "is_completed": false,
      "is_active": false,
      "last_modified_at": 1640995200000
    }
  }
  ```
- [ ] Test task editing with new properties
- [ ] Verify tasks are returned with new properties

### Task 9: Verify frontend compatibility
- [ ] Check that all existing functionality still works
- [ ] Verify new properties are included in task responses
- [ ] Test that null values are handled correctly

## Notes
- All changes are backward compatible
- New properties are nullable to support existing tasks
- Frontend will handle filtering based on due dates
- Priority is an integer (1-10 or similar scale)
- ShowBeforeDueTime is in minutes
- DueAt is a timestamp for exact due date/time
- All existing functionality remains unchanged
- WebSocket structs use pointer types for nullable fields
- Database models use sql.NullInt32 and sql.NullTime for nullable fields
- Proper null handling in WebSocket event functions
