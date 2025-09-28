# Google OAuth UID Verification TODO

## Database Schema Changes

### Task 1: Create database migration for Google UID
- [x] Create new migration file: `sql/schema/9_add_google_uid.sql`
- [x] Add the following SQL:
  ```sql
  -- +goose Up
  ALTER TABLE users ADD COLUMN google_uid TEXT UNIQUE;
  
  -- +goose Down
  ALTER TABLE users DROP COLUMN google_uid;
  ```
- [x] Run migration: `goose up`

### Task 2: Update SQL queries
- [x] Update `sql/queries/users.sql`:
  ```sql
  -- name: CreateUser :one
  INSERT INTO users (
      first_name,
      last_name,
      email,
      google_uid
  ) VALUES (
      $1,
      $2,
      $3,
      $4
  )
  ON CONFLICT (email)
  DO NOTHING
  RETURNING *;

  -- name: GetUserByEmail :one
  SELECT * FROM users WHERE email = $1;

  -- name: GetUserByGoogleUID :one
  SELECT * FROM users WHERE google_uid = $1;
  ```

### Task 3: Regenerate database models
- [x] Run `sqlc generate` to update Go models
- [x] Verify `internal/database/models.go` includes `GoogleUID sql.NullString` field
- [x] Verify `internal/database/users.sql.go` includes updated `CreateUserParams` struct

## WebSocket Connection Logic

### Task 4: Update User struct in websockets.go
- [x] Add `GoogleUID string` field to User struct:
  ```go
  type User struct {
      ID        uuid.UUID `json:"id"`
      Email     string    `json:"email"`
      FirstName string    `json:"first_name"`
      LastName  string    `json:"last_name"`
      GoogleUID string    `json:"google_uid"`  // Add this
  }
  ```

### Task 5: Create error handling structures
- [x] Add error types and structures to `websockets_custom_events.go`:
  ```go
  type ConnectionError struct {
      Type    string `json:"type"`
      Message string `json:"message"`
      Code    int    `json:"code"`
  }

  const (
      ErrorGoogleUIDMismatch = "google_uid_mismatch"
      ErrorInvalidGoogleUID  = "invalid_google_uid"
      ErrorUserCreation      = "user_creation_failed"
      ErrorDatabaseError     = "database_error"
  )
  ```

### Task 6: Create error sending helper function
- [x] Add helper function to `websockets_custom_events.go`:
  ```go
  func sendError(c *websocket.Conn, errorType, message string, code int) error {
      errorResponse := map[string]interface{}{
          "event": "connection_error",
          "data": ConnectionError{
              Type:    errorType,
              Message: message,
              Code:    code,
          },
      }
      
      payload, _ := json.Marshal(errorResponse)
      return c.Write(context.Background(), websocket.MessageText, payload)
  }
  ```

### Task 7: Update WSOnConnect function
- [x] Replace the entire `WSOnConnect` function in `websockets_custom_events.go`:
  ```go
  func (cfg *config) WSOnConnect(ctx context.Context, c *websocket.Conn, SID uuid.UUID, data []byte) error {
      start := time.Now()
      defer func() {
          metrics.WebSocketEventDuration.WithLabelValues("connect").Observe(time.Since(start).Seconds())
      }()

      var connectionData struct {
          Data User `json:"data"`
      }
      err := json.Unmarshal(data, &connectionData)
      if err != nil {
          return sendError(c, "invalid_data", "Invalid connection data", 400)
      }

      // Validate Google UID
      if connectionData.Data.GoogleUID == "" {
          return sendError(c, ErrorInvalidGoogleUID, "Google UID is required", 400)
      }

      // Try to create user (will return existing user if conflict)
      user, err := cfg.DB.CreateUser(ctx, database.CreateUserParams{
          Email:     connectionData.Data.Email,
          FirstName: connectionData.Data.FirstName,
          LastName:  connectionData.Data.LastName,
          GoogleUID: sql.NullString{
              String: connectionData.Data.GoogleUID,
              Valid:  true,
          },
      })
      
      if err != nil {
          return sendError(c, ErrorUserCreation, "Failed to create user", 500)
      }

      // Verify Google UID matches (for existing users)
      if user.GoogleUID.Valid && user.GoogleUID.String != connectionData.Data.GoogleUID {
          return sendError(c, ErrorGoogleUIDMismatch, "Google UID does not match", 403)
      }

      // Success - continue with normal connection flow
      cfg.WSClientManager.AddClient(&Client{
          SID:  SID,
          Conn: c,
          User: user,
      })

      tasks, err := cfg.DB.GetActiveTaskByUUIDWithTiming(ctx, user.ID)
      if err != nil {
          return sendError(c, ErrorDatabaseError, "Failed to load tasks", 500)
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
  ```

## Testing

### Task 8: Test database migration
- [x] Run `goose up` and verify `google_uid` column is added
- [x] Run `goose down` and verify column is removed
- [x] Run `goose up` again to restore the column

### Task 9: Test WebSocket connection with Google UID
- [x] Start server and verify it starts without errors
- [x] Test connection with valid Google UID:
  ```json
  {
    "event": "connect",
    "data": {
      "id": "28c07fc5-2732-47c0-b305-92982fbddcef",
      "email": "test@example.com",
      "first_name": "Test",
      "last_name": "User",
      "google_uid": "1234567890abcdef"
    }
  }
  ```
- [x] Verify user is created in database with correct Google UID

### Task 10: Test error scenarios
- [x] Test connection without Google UID (should get `invalid_google_uid` error)
- [x] Test connection with mismatched Google UID (should get `google_uid_mismatch` error)
- [x] Test connection with existing user and correct Google UID (should succeed)

### Task 11: Verify error responses
- [x] Check that error responses have correct structure:
  ```json
  {
    "event": "connection_error",
    "data": {
      "type": "google_uid_mismatch",
      "message": "Google UID does not match",
      "code": 403
    }
  }
  ```

## Frontend Integration Notes

### Task 12: Document frontend changes needed
- [x] Update desktop/mobile apps to send Google UID in connect event
- [x] Add error handling for connection_error events
- [x] Test error scenarios in frontend apps

**Frontend error handling example:**
```javascript
socket.onmessage = (event) => {
    const msg = JSON.parse(event.data);
    
    if (msg.event === "connection_error") {
        switch (msg.data.type) {
            case "google_uid_mismatch":
                showError("Security Error", "Please sign out and sign in again");
                break;
            case "invalid_google_uid":
                showError("Authentication Error", "Please sign in with Google");
                break;
            case "user_creation_failed":
                showError("Server Error", "Please try again later");
                break;
        }
        return;
    }
    
    // Handle other events normally
}
```

## Notes
- All changes are server-side only
- Database migration is required before testing
- Google UID verification adds security layer
- Error responses are structured for frontend handling
- Existing functionality remains unchanged
- Test each task before moving to the next
