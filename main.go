package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dinopy/taskbar2_server/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
)

type config struct {
	DB                *database.Queries
	DBPool            *sql.DB
	PORT              string
	WSCfg             WebSocketCfg
	WSClientManager   ClientManager
	Metrics           *prometheus.Registry
	ScheduleService   *ScheduleService
	DispatcherService *DispatcherService
}

type WebSocketCfg struct {
	pingInterval time.Duration
	pingTimeout  time.Duration
}

func (cfg *config) HelloApiHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Hello from API!"})
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Could not load the .env file. Err: %v", err)
	}

	DB_URL := os.Getenv("DB_URL")
	if DB_URL == "" {
		log.Fatalf("Could not find/load database URL")
	}

	PORT := os.Getenv("PORT")
	if PORT == "" {
		log.Fatal("Could not load PORT env")
	}

	db, err := sql.Open("postgres", DB_URL)
	if err != nil {
		log.Fatalf("Could not connect to DB. Err: %v", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	dbQuery := database.New(db)

	cfg := config{
		DB:     dbQuery,
		DBPool: db,
		PORT:   PORT,
		WSCfg: WebSocketCfg{
			pingInterval: 5 * time.Second,
			pingTimeout:  60 * time.Second,
		},
		WSClientManager: *NewClientManager(),
		Metrics:         prometheus.NewRegistry(),
	}

	// Initialize services after config is created
	scheduleService := NewScheduleService(dbQuery, 60, func(userID uuid.UUID, task database.Task) error {
		// Send task creation event via WebSocket to user's connected clients
		log.Printf("Main: Broadcasting new_task_created event for task %s to user %s", task.ID, userID)
		cfg.WSClientManager.BroadcastToSameUser(context.Background(), "new_task_created", userID, task)
		return nil
	})
	dispatcherService := NewDispatcherService(dbQuery, 100, func(userID uuid.UUID, notification database.Notification) error {
		// Send notification via WebSocket to user's connected clients
		cfg.WSClientManager.BroadcastToSameUser(context.Background(), "notification_created", userID, notification)
		return nil
	})
	cleanupService := NewCleanupService(dbQuery)

	// Update config with services
	cfg.ScheduleService = scheduleService
	cfg.DispatcherService = dispatcherService

	// set up router
	mux := http.NewServeMux()

	// Frontend test, we won't need one but will be used for development.
	mux.Handle("/", http.FileServer(http.Dir("./static")))

	// APIs, I'd like to add some in the future.
	mux.HandleFunc("/api/hello", cfg.HelloApiHandler)

	// Prometheus metrics endpoint
	mux.Handle("/metrics", promhttp.Handler())

	// Websocket endpoint
	mux.HandleFunc("/ws/taskbar", cfg.WebSocketsHandler)

	srv := &http.Server{
		Addr:         ":" + cfg.PORT,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	location, _ := time.LoadLocation("Europe/Bucharest")
	cron := cron.New(cron.WithLocation(location))
	cron.AddFunc("59 23 * * *", cfg.WSOnMidnightTaskRefresh)
	cron.AddFunc("@every 1m", cfg.DispatchDueNotifications)

	// Add planner and dispatcher loops
	cron.AddFunc("@every 1m", func() {
		ctx := context.Background()
		if err := cfg.ScheduleService.Tick(ctx); err != nil {
			log.Printf("ScheduleService tick failed: %v", err)
		}
	})

	cron.AddFunc("@every 1m", func() {
		ctx := context.Background()
		if err := cfg.DispatcherService.Tick(ctx); err != nil {
			log.Printf("DispatcherService tick failed: %v", err)
		}
	})

	// Add daily cleanup job (runs at 3 AM)
	cron.AddFunc("0 3 * * *", func() {
		ctx := context.Background()
		if err := cleanupService.CleanupOldOccurrences(ctx); err != nil {
			log.Printf("CleanupService cleanup failed: %v", err)
		}
	})

	cron.Start()

	log.Println("Serving on http://localhost:" + cfg.PORT + "...")
	log.Fatal(srv.ListenAndServe())
}
