package main

import (
	"context"
	"log"
	"time"

	"github.com/dinopy/taskbar2_server/internal/database"
)

type CleanupService struct {
	queries *database.Queries
}

func NewCleanupService(queries *database.Queries) *CleanupService {
	return &CleanupService{
		queries: queries,
	}
}

func (s *CleanupService) CleanupOldOccurrences(ctx context.Context) error {
	start := time.Now()
	log.Printf("CleanupService: Starting occurrence cleanup (deleting occurrences older than 14 days)")

	err := s.queries.DeleteOldOccurrences(ctx)
	if err != nil {
		log.Printf("CleanupService: Failed to delete old occurrences: %v", err)
		return err
	}

	log.Printf("CleanupService: Completed occurrence cleanup in %v", time.Since(start))
	return nil
}
