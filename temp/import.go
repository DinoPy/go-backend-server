package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/dinopy/taskbar2_server/internal/database"
	"github.com/gocarina/gocsv"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type TaskNoNullable struct {
	ID             uuid.UUID      `csv:"id"`
	Title          string         `csv:"title"`
	Description    string         `csv:"description"`
	CreatedAt      CSVTime        `csv:"created_at"`
	CompletedAt    CSVTime        `csv:"completed_at"`
	Duration       string         `csv:"duration"`
	Category       string         `csv:"category"`
	Tags           CSVStringSlice `csv:"tags"`
	ToggledAt      int64          `csv:"toggled_at"`
	IsActive       CSVBool        `csv:"is_active"`
	IsCompleted    CSVBool        `csv:"is_completed"`
	UserID         uuid.UUID      `csv:"user_id"`
	LastModifiedAt int64          `csv:"last_modified_at"`
}

type CSVTime struct {
	time.Time
}

func (ct *CSVTime) UnmarshalCSV(value string) error {
	layouts := []string{
		"2006-01-02 15:04:05", // full datetime
		"2006-01-02",          // date only
		"1/2/2006 15:04:05",   // New format: 4/11/2024 17:33:55}
	}

	for _, layout := range layouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			ct.Time = t
			return nil
		}
	}

	return fmt.Errorf("invalid time format: %s", value)
}

type CSVBool bool

func (b *CSVBool) UnmarshalCSV(value string) error {
	switch value {
	case "1", "true":
		*b = true
	case "0", "false":
		*b = false
	default:
		return fmt.Errorf("invalid bool: %s", value)
	}
	return nil
}

type CSVStringSlice []string

func (s *CSVStringSlice) UnmarshalCSV(value string) error {
	// Remove surrounding quotes if any
	value = strings.Trim(value, `"`)
	*s = strings.Split(value, ",")
	return nil
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

	dbQuery := database.New(db)
	cwd, err := os.Getwd()
	file, err := os.Open(path.Join(cwd, "temp", "tasks.csv"))
	if err != nil {
		log.Fatalln("Could not load tasks.csv", err)
	}
	defer file.Close()

	var tasks []*TaskNoNullable
	if err := gocsv.UnmarshalFile(file, &tasks); err != nil {
		log.Fatalln("Could not parse tasks.csv", err)
	}

	for _, task := range tasks {
		_, err := dbQuery.CreateTask(context.Background(), database.CreateTaskParams{
			ID:          task.ID,
			Title:       task.Title,
			Description: task.Description,
			CreatedAt:   task.CompletedAt.In(time.UTC),
			CompletedAt: sql.NullTime{
				Time:  task.CompletedAt.In(time.UTC),
				Valid: true,
			},
			Duration: task.Duration,
			Category: task.Category,
			Tags:     task.Tags,
			ToggledAt: sql.NullInt64{
				Int64: task.ToggledAt,
				Valid: true,
			},
			IsActive:       bool(task.IsActive),
			IsCompleted:    bool(task.IsCompleted),
			UserID:         task.UserID,
			LastModifiedAt: task.LastModifiedAt,
		})
		fmt.Println("trying to import task with userid: ", task.UserID, "for user: ", "b1193283-bade-46a3-9c57-67bdf6925697")

		if err != nil {
			log.Fatalf("error: %v", err)
		}

	}

	fmt.Println("total tasks", len(tasks))
}
