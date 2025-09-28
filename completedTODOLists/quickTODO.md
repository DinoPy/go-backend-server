# Quick TODO - Server Optimizations

## Database Connection Pooling

### Task 1: Add connection pool configuration to main.go
- [x] Import `time` package if not already imported
- [x] After `sql.Open("postgres", DB_URL)` add connection pool settings:
  ```go
  db.SetMaxOpenConns(25)
  db.SetMaxIdleConns(5)
  db.SetConnMaxLifetime(5 * time.Minute)
  ```
- [x] Test that server still starts and connects properly

### Task 2: Add connection pool to config struct
- [x] Add `DBPool *sql.DB` field to config struct
- [x] Initialize `DBPool` in main.go with the configured database connection
- [x] Update database queries to use `DBPool` instead of direct connection

## Database Indexes

### Task 3: Create database migration for indexes
- [x] Create new migration file: `sql/schema/8_indexes.sql`
- [x] Add the following indexes:
  ```sql
  -- +goose Up
  CREATE INDEX idx_tasks_user_id_active ON tasks(user_id, is_completed) WHERE is_completed = FALSE;
  CREATE INDEX idx_tasks_user_id_completed ON tasks(user_id, completed_at) WHERE is_completed = TRUE;
  CREATE INDEX idx_tasks_category ON tasks(category);
  CREATE INDEX idx_tasks_tags ON tasks USING GIN(tags);
  
  -- +goose Down
  DROP INDEX IF EXISTS idx_tasks_user_id_active;
  DROP INDEX IF EXISTS idx_tasks_user_id_completed;
  DROP INDEX IF EXISTS idx_tasks_category;
  DROP INDEX IF EXISTS idx_tasks_tags;
  ```
- [x] Run migration to apply indexes

## Prometheus Implementation

### Task 4: Add Prometheus dependencies
- [x] Add to go.mod:
  ```go
  require (
      github.com/prometheus/client_golang v1.19.0
  )
  ```
- [x] Run `go mod tidy`

### Task 5: Create Prometheus metrics
- [x] Create new file: `internal/metrics/metrics.go`
- [x] Define metrics:
  ```go
  package metrics

  import (
      "github.com/prometheus/client_golang/prometheus"
      "github.com/prometheus/client_golang/prometheus/promauto"
  )

  var (
      WebSocketConnections = promauto.NewGaugeVec(
          prometheus.GaugeOpts{
              Name: "websocket_connections_total",
              Help: "Number of active WebSocket connections",
          },
          []string{"user_id"},
      )

      DatabaseQueryDuration = promauto.NewHistogramVec(
          prometheus.HistogramOpts{
              Name: "db_query_duration_seconds",
              Help: "Database query duration",
          },
          []string{"query_type"},
      )

      WebSocketEventDuration = promauto.NewHistogramVec(
          prometheus.HistogramOpts{
              Name: "websocket_event_duration_seconds",
              Help: "WebSocket event processing duration",
          },
          []string{"event_type"},
      )
  )
  ```

### Task 6: Add Prometheus to config
- [x] Add to config struct:
  ```go
  type config struct {
      DB              *database.Queries
      DBPool          *sql.DB
      PORT            string
      WSCfg           WebSocketCfg
      WSClientManager ClientManager
      Metrics         *prometheus.Registry
  }
  ```
- [x] Initialize metrics in main.go:
  ```go
  cfg := config{
      // ... existing fields
      Metrics: prometheus.NewRegistry(),
  }
  ```

### Task 7: Add metrics endpoint
- [x] Add Prometheus metrics endpoint to main.go:
  ```go
  import (
      "github.com/prometheus/client_golang/prometheus/promhttp"
  )
  
  // Add to mux
  mux.Handle("/metrics", promhttp.Handler())
  ```

### Task 8: Instrument WebSocket events
- [x] Add metrics to WebSocket event handlers:
  ```go
  import "github.com/dinopy/taskbar2_server/internal/metrics"
  
  // In each event handler, add:
  start := time.Now()
  defer func() {
      metrics.WebSocketEventDuration.WithLabelValues("connect").Observe(time.Since(start).Seconds())
  }()
  ```

### Task 9: Instrument database queries
- [x] Add metrics to database query wrappers:
  ```go
  func (q *Queries) GetActiveTaskByUUIDWithTiming(ctx context.Context, userID uuid.UUID) ([]Task, error) {
      start := time.Now()
      defer func() {
          metrics.DatabaseQueryDuration.WithLabelValues("get_active_tasks").Observe(time.Since(start).Seconds())
      }()
      return q.GetActiveTaskByUUID(ctx, userID)
  }
  ```

### Task 10: Track WebSocket connections
- [x] Add metrics to client manager:
  ```go
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
  ```

## Testing

### Task 11: Test all changes
- [x] Start server and verify it connects to database
- [x] Test WebSocket connection from frontend
- [x] Verify metrics are available at `/metrics` endpoint
- [x] Check logs for timing information
- [x] Verify database indexes are created
- [x] Test that all existing functionality still works

## Notes
- All changes are server-side only
- No impact on desktop/mobile apps
- Changes are backward compatible
- Test each task before moving to the next
- Keep existing functionality intact
