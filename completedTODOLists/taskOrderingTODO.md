# Task Ordering TODO

## Update SQL Queries for Consistent Ordering

### Task 1: Update GetTasks query
- [x] Update `sql/queries/tasks.sql`:
  ```sql
  -- name: GetTasks :many
  SELECT * FROM TASKS ORDER BY created_at ASC;
  ```

### Task 2: Update GetCompletedTasksByUUID query
- [x] Update `sql/queries/tasks.sql`:
  ```sql
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
  ```

### Task 3: Update GetActiveTaskByUUID query
- [x] Update `sql/queries/tasks.sql`:
  ```sql
  -- name: GetActiveTaskByUUID :many
  SELECT * 
  FROM tasks
  WHERE user_id = $1 AND is_completed = FALSE
  ORDER BY created_at ASC;
  ```

### Task 4: Regenerate database models
- [x] Run `sqlc generate` to update Go models
- [x] Verify no compilation errors

### Task 5: Test the changes
- [x] Start server and verify it starts without errors
- [x] Test WebSocket connection
- [x] Verify tasks are returned in chronological order (oldest first)
- [x] Test active tasks ordering
- [x] Test completed tasks ordering

## Notes
- All multi-task queries now return tasks ordered by `created_at ASC` (oldest to newest)
- `GetNonCompletedTasks` remains unchanged (used for midnight refresh, order doesn't matter)
- Changes are backward compatible
- Frontend will receive tasks in consistent chronological order
