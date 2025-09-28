package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/dinopy/taskbar2_server/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/robfig/cron/v3"
)

type config struct {
	DB              *database.Queries
	DBPool          *sql.DB
	PORT            string
	WSCfg           WebSocketCfg
	WSClientManager ClientManager
	Metrics         *prometheus.Registry
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
	cron.Start()

	log.Println("Serving on http://localhost:" + cfg.PORT + "...")
	log.Fatal(srv.ListenAndServe())
}
