# Midnight Rollover Fix TODO

## Fix Missing Properties in Midnight Task Refresh

### Task 1: Update WSOnMidnightTaskRefresh function
- [ ] Update `WSOnMidnightTaskRefresh` in `websockets_custom_events.go` to include new properties
- [ ] Locate the `createTaskParams` section (around line 754-773)
- [ ] Add the missing properties to preserve them in new tasks:

```go
// insert a new task with the same properties
createTaskParams := database.CreateTaskParams{
    ID:          uuid.New(),
    Title:       task.Title,
    Description: task.Description,
    CreatedAt:   timeNow.In(time.UTC),
    CompletedAt: sql.NullTime{
        Valid: false,
    },
    Duration: "00:00:00",
    Category: task.Category,
    Tags:     task.Tags,
    ToggledAt: sql.NullInt64{
        Int64: ternary(task.ToggledAt.Int64 == 0, 0, lastEpochMs),
        Valid: ternary(task.ToggledAt.Int64 == 0, false, true),
    },
    IsActive:       task.IsActive,
    IsCompleted:    false,
    UserID:         task.UserID,
    LastModifiedAt: lastEpochMs,
    // ADD THESE MISSING PROPERTIES:
    Priority:           task.Priority,           // Copy from original task
    DueAt:              task.DueAt,              // Copy from original task
    ShowBeforeDueTime:  task.ShowBeforeDueTime,  // Copy from original task
}
```

### Task 2: Test the fix
- [ ] Start server and verify it starts without errors
- [ ] Create a task with priority, due date, and show_before_due_time
- [ ] Wait for midnight rollover or manually trigger the function
- [ ] Verify the new task preserves all properties from the original task
- [ ] Check that priority, due date, and show timing are maintained

### Task 3: Verify existing functionality
- [ ] Test that tasks without new properties still work correctly
- [ ] Verify that tasks with null values for new properties are handled properly
- [ ] Confirm that the rollover process completes without errors
- [ ] Check that all connected clients receive the updated task list

## Notes
- This fix ensures that recurring tasks maintain their priority, due dates, and show timing
- The new properties are copied directly from the original task to preserve all settings
- All existing functionality remains unchanged
- This is a critical fix for maintaining task properties across daily rollovers
