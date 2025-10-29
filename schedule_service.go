package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/dinopy/taskbar2_server/internal/database"
	"github.com/google/uuid"
	"github.com/teambition/rrule-go"
)

type ScheduleService struct {
	queries         *database.Queries
	horizonDays     int
	sendTaskCreated func(userID uuid.UUID, task database.Task) error
}

func NewScheduleService(queries *database.Queries, horizonDays int, sendTaskCreated func(userID uuid.UUID, task database.Task) error) *ScheduleService {
	return &ScheduleService{
		queries:         queries,
		horizonDays:     horizonDays,
		sendTaskCreated: sendTaskCreated,
	}
}

func (s *ScheduleService) Tick(ctx context.Context) error {
	now := time.Now()
	horizon := now.AddDate(0, 0, s.horizonDays)

	log.Printf("ScheduleService: Starting planner tick at %s, horizon: %s", now.Format(time.RFC3339), horizon.Format(time.RFC3339))

	schedules, err := s.queries.GetActiveSchedules(ctx)
	if err != nil {
		log.Printf("ScheduleService: Failed to get active schedules: %v", err)
		return err
	}

	log.Printf("ScheduleService: Found %d active schedules", len(schedules))

	for _, sch := range schedules {
		if err := s.processSchedule(ctx, sch, now, horizon); err != nil {
			log.Printf("ScheduleService: Failed to process schedule %s: %v", sch.ID, err)
			// Continue processing other schedules
			continue
		}
	}

	log.Printf("ScheduleService: Completed planner tick")
	return nil
}

func (s *ScheduleService) processSchedule(ctx context.Context, sch database.Schedule, now, horizon time.Time) error {
	log.Printf("ScheduleService: Processing schedule %s (%s)", sch.ID, sch.Title)

	// Determine frequency for adaptive horizon
	frequency := "other"
	if sch.Rrule.Valid && sch.Rrule.String != "" {
		frequency = detectFrequency(sch.Rrule.String)
	}

	// Calculate adaptive horizon based on frequency
	var adaptiveHorizon time.Time
	switch frequency {
	case "minutely":
		// For minutely tasks, only look ahead 2 occurrences
		adaptiveHorizon = now.Add(2 * time.Minute)
	case "hourly":
		// For hourly tasks, look ahead 6 hours
		adaptiveHorizon = now.Add(6 * time.Hour)
	case "daily":
		// For daily tasks, look ahead 7 days
		adaptiveHorizon = now.Add(7 * 24 * time.Hour)
	case "weekly", "monthly":
		// For weekly/monthly tasks, use full horizon
		adaptiveHorizon = horizon
	default:
		// For one-off or unknown frequency, use full horizon
		adaptiveHorizon = horizon
	}

	// Use the more restrictive horizon
	if adaptiveHorizon.Before(horizon) {
		horizon = adaptiveHorizon
	}

	log.Printf("ScheduleService: Using adaptive horizon for frequency '%s': %s", frequency, horizon.Format(time.RFC3339))

	// Start from last materialized until, or small backfill window
	start := now.Add(-2 * time.Minute) // backfill window
	if sch.LastMaterializedUntil.Valid && sch.LastMaterializedUntil.Time.After(start) {
		start = sch.LastMaterializedUntil.Time
		log.Printf("ScheduleService: Continuing from last materialized until: %s", start.Format(time.RFC3339))
	}

	// Build iterator
	loc, err := time.LoadLocation(sch.Tz)
	if err != nil {
		log.Printf("ScheduleService: Failed to load timezone %s: %v", sch.Tz, err)
		return err
	}

	seed := time.Date(
		sch.StartLocal.Year(), sch.StartLocal.Month(), sch.StartLocal.Day(),
		sch.StartLocal.Hour(), sch.StartLocal.Minute(), sch.StartLocal.Second(),
		0, loc,
	)

	var itr *rrule.Set
	if sch.Rrule.Valid && sch.Rrule.String != "" {
		// Parse RRULE
		var rruleStr string
		if strings.Contains(sch.Rrule.String, "DTSTART:") {
			// RRULE string already contains DTSTART, use as-is
			rruleStr = sch.Rrule.String
			log.Printf("ScheduleService: Using RRULE string as-is (contains DTSTART): %s", rruleStr)
		} else {
			// Add DTSTART to RRULE string
			rruleStr = "DTSTART:" + seed.Format("20060102T150405Z") + "\n" + sch.Rrule.String
			log.Printf("ScheduleService: Added DTSTART to RRULE string: %s", rruleStr)
		}

		r, err := rrule.StrToRRule(rruleStr)
		if err != nil {
			log.Printf("ScheduleService: Failed to parse RRULE %s: %v", sch.Rrule.String, err)
			return err
		}

		set := rrule.Set{}
		set.RRule(r)

		// Add UNTIL if specified
		if sch.UntilLocal.Valid {
			untilUTC := sch.UntilLocal.Time.In(loc).UTC()
			set.ExDate(untilUTC)
		}

		itr = &set
	} else {
		// One-off: emulate a single occurrence at seed
		set := rrule.Set{}
		set.RDate(seed)
		itr = &set
	}

	// Iterate and materialize occurrences
	const maxOccurrencesPerTick = 100
	occurrenceCount := 0
	for {
		if occurrenceCount >= maxOccurrencesPerTick {
			log.Printf("ScheduleService: Warning - Hit max occurrences limit (%d) for schedule %s", maxOccurrencesPerTick, sch.ID)
			break
		}

		next := s.nextAfterLocal(itr, start, loc, sch)
		if next.IsZero() {
			break
		}
		if next.After(horizon) {
			break
		}

		occursAtUTC := time.Date(next.Year(), next.Month(), next.Day(),
			next.Hour(), next.Minute(), next.Second(), next.Nanosecond(), loc).UTC()

		// Upsert occurrence
		occ, err := s.queries.UpsertOccurrence(ctx, database.UpsertOccurrenceParams{
			ScheduleID: sch.ID,
			OccursAt:   occursAtUTC,
			Rev:        sch.Rev,
		})
		if err != nil {
			log.Printf("ScheduleService: Failed to upsert occurrence: %v", err)
			return err
		}

		occurrenceCount++

		// Create task if kind is 'task'
		if sch.Kind == "task" {
			if err := s.ensureTaskForOccurrence(ctx, sch, occ); err != nil {
				log.Printf("ScheduleService: Failed to ensure task for occurrence: %v", err)
				return err
			}
		}

		// Create notification jobs based on kind
		if sch.Kind == "task" {
			// Tasks: advance notifications (48h, 24h, 12h, 6h, 3h) but NO "due now" notification
			if err := s.createTaskNotificationJobs(ctx, sch, occ); err != nil {
				log.Printf("ScheduleService: Failed to create task notification jobs: %v", err)
				return err
			}
		} else if sch.Kind == "reminder" {
			// Reminders: only notification at exact occurrence time (offset 0)
			if err := s.createReminderNotificationJobs(ctx, sch, occ); err != nil {
				log.Printf("ScheduleService: Failed to create reminder notification jobs: %v", err)
				return err
			}
		}

		start = occursAtUTC // Advance cursor
	}

	// Update last materialized until
	if err := s.queries.SetLastMaterializedUntil(ctx, database.SetLastMaterializedUntilParams{
		ID:                    sch.ID,
		LastMaterializedUntil: sql.NullTime{Time: horizon, Valid: true},
	}); err != nil {
		log.Printf("ScheduleService: Failed to update last materialized until: %v", err)
		return err
	}

	// Deactivate schedules that have completed
	shouldDeactivate := false
	deactivateReason := ""

	if !sch.Rrule.Valid {
		// One-time schedule (no recurrence rule)
		shouldDeactivate = true
		deactivateReason = "one-time schedule completed"
	} else if sch.UntilLocal.Valid && sch.UntilLocal.Time.Before(now) {
		// Recurring schedule that has reached its end date
		shouldDeactivate = true
		deactivateReason = "recurring schedule reached end date"
	}

	if shouldDeactivate {
		if err := s.queries.DeactivateSchedule(ctx, sch.ID); err != nil {
			log.Printf("ScheduleService: Failed to deactivate schedule %s: %v", sch.ID, err)
			return err
		}
		log.Printf("ScheduleService: Deactivated schedule %s (%s) - reason: %s", sch.ID, sch.Title, deactivateReason)
	}

	log.Printf("ScheduleService: Processed %d occurrences for schedule %s", occurrenceCount, sch.ID)
	return nil
}

func (s *ScheduleService) ensureTaskForOccurrence(ctx context.Context, sch database.Schedule, occ database.Occurrence) error {
	// Check if task already exists for this occurrence
	_, err := s.queries.GetTaskIDForOccurrence(ctx, occ.ID)
	if err == nil {
		// Task already exists, skip
		return nil
	}

	// Determine if this should have a due date
	var dueAt sql.NullTime
	if isHighFrequency(sch) {
		// High-frequency tasks (minutely/hourly) have no due date
		dueAt = sql.NullTime{Valid: false}
	} else {
		// Normal tasks have due date set to occurrence time
		dueAt = sql.NullTime{Time: occ.OccursAt, Valid: true}
	}

	// Get category from schedule, default to "Life" if not set
	category := "Life"
	if sch.Category.Valid {
		category = sch.Category.String
	}

	// Create new task
	task, err := s.queries.CreateTask(ctx, database.CreateTaskParams{
		ID:                uuid.New(),
		Title:             sch.Title,
		Description:       "", // Could be enhanced later
		CreatedAt:         occ.OccursAt,
		CompletedAt:       sql.NullTime{Valid: false},
		Duration:          "00:00:00",
		Category:          category,
		Tags:              []string{},
		ToggledAt:         sql.NullInt64{Valid: false},
		IsActive:          false,
		IsCompleted:       false,
		UserID:            sch.UserID,
		LastModifiedAt:    time.Now().UnixMilli(),
		Priority:          sql.NullInt32{Valid: false},
		DueAt:             dueAt,
		ShowBeforeDueTime: sch.ShowBeforeMinutes,
	})
	if err != nil {
		return err
	}

	// Link task to occurrence
	err = s.queries.LinkTaskToOccurrence(ctx, database.LinkTaskToOccurrenceParams{
		OccurrenceID: occ.ID,
		TaskID:       task.ID,
	})
	if err != nil {
		return err
	}

	// Send task creation event to user if callback is provided
	if s.sendTaskCreated != nil {
		if err := s.sendTaskCreated(sch.UserID, task); err != nil {
			log.Printf("ScheduleService: Failed to send task created event for task %s: %v", task.ID, err)
			// Don't return error - task creation succeeded, just notification failed
		} else {
			log.Printf("ScheduleService: Sent task created event for task %s to user %s", task.ID, sch.UserID)
		}
	}

	return nil
}

func (s *ScheduleService) createNotificationJobs(ctx context.Context, sch database.Schedule, occ database.Occurrence) error {
	// Calculate effective offsets (notify_offsets_min \ muted_offsets_min)
	offsets := s.effectiveOffsets(sch.NotifyOffsetsMin, sch.MutedOffsetsMin)
	now := time.Now()

	// Always add a notification at the exact reminder time (offset 0) unless it's muted
	mutedMap := make(map[int32]bool)
	for _, x := range sch.MutedOffsetsMin {
		mutedMap[x] = true
	}

	if !mutedMap[0] {
		offsets = append([]int{0}, offsets...)
		log.Printf("ScheduleService: Added offset 0 (exact reminder time) for schedule %s", sch.ID)
	} else {
		log.Printf("ScheduleService: Offset 0 is muted for schedule %s, skipping exact reminder notification", sch.ID)
	}

	for _, off := range offsets {
		sendAt := occ.OccursAt.Add(-time.Duration(off) * time.Minute)

		// Skip notifications that would be sent in the past
		if sendAt.Before(now) {
			log.Printf("ScheduleService: Skipping notification for schedule %s, offset %d minutes (would send at %v, now is %v)", sch.ID, off, sendAt, now)
			continue
		}

		// Skip notifications where offset is greater than time until due
		timeUntilDue := occ.OccursAt.Sub(now)
		offsetDuration := time.Duration(off) * time.Minute
		if offsetDuration > timeUntilDue {
			log.Printf("ScheduleService: Skipping notification offset %d min (due in %v) for schedule %s", off, timeUntilDue, sch.ID)
			continue
		}

		log.Printf("ScheduleService: Creating notification job for schedule %s, offset %d minutes (will send at %v)", sch.ID, off, sendAt)

		// Create payload
		payload := map[string]interface{}{
			"schedule_id":    sch.ID.String(),
			"occurrence_id":  occ.ID.String(),
			"offset_minutes": off,
			"title":          sch.Title,
			"kind":           sch.Kind,
		}
		payloadJSON, _ := json.Marshal(payload)

		// Upsert notification job
		err := s.queries.UpsertNotificationJob(ctx, database.UpsertNotificationJobParams{
			UserID:        sch.UserID,
			ScheduleID:    uuid.NullUUID{UUID: sch.ID, Valid: true},
			OccurrenceID:  occ.ID,
			OffsetMinutes: int32(off),
			PlannedSendAt: sendAt,
			Payload:       payloadJSON,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// createTaskNotificationJobs creates advance notifications for tasks (48h, 24h, 12h, 6h, 3h) but NOT at creation time
func (s *ScheduleService) createTaskNotificationJobs(ctx context.Context, sch database.Schedule, occ database.Occurrence) error {
	// Calculate effective offsets (notify_offsets_min \ muted_offsets_min) but exclude offset 0
	offsets := s.effectiveOffsets(sch.NotifyOffsetsMin, sch.MutedOffsetsMin)
	now := time.Now()

	// Filter out offset 0 (exact reminder time) for tasks
	var taskOffsets []int
	for _, off := range offsets {
		if off != 0 {
			taskOffsets = append(taskOffsets, off)
		}
	}

	log.Printf("ScheduleService: Creating task notification jobs for schedule %s with offsets: %v (excluding offset 0)", sch.ID, taskOffsets)

	for _, off := range taskOffsets {
		sendAt := occ.OccursAt.Add(-time.Duration(off) * time.Minute)

		// Skip notifications that would be sent in the past
		if sendAt.Before(now) {
			log.Printf("ScheduleService: Skipping task notification for schedule %s, offset %d minutes (would send at %v, now is %v)", sch.ID, off, sendAt, now)
			continue
		}

		// Skip notifications where offset is greater than time until due
		timeUntilDue := occ.OccursAt.Sub(now)
		offsetDuration := time.Duration(off) * time.Minute
		if offsetDuration > timeUntilDue {
			log.Printf("ScheduleService: Skipping task notification offset %d min (due in %v) for schedule %s", off, timeUntilDue, sch.ID)
			continue
		}

		log.Printf("ScheduleService: Creating task notification job for schedule %s, offset %d minutes (will send at %v)", sch.ID, off, sendAt)

		// Create payload
		payload := map[string]interface{}{
			"schedule_id":    sch.ID.String(),
			"occurrence_id":  occ.ID.String(),
			"offset_minutes": off,
			"title":          sch.Title,
			"kind":           sch.Kind,
		}
		payloadJSON, _ := json.Marshal(payload)

		// Upsert notification job
		err := s.queries.UpsertNotificationJob(ctx, database.UpsertNotificationJobParams{
			UserID:        sch.UserID,
			ScheduleID:    uuid.NullUUID{UUID: sch.ID, Valid: true},
			OccurrenceID:  occ.ID,
			OffsetMinutes: int32(off),
			PlannedSendAt: sendAt,
			Payload:       payloadJSON,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

// createReminderNotificationJobs creates only the exact-time notification for reminders (offset 0)
func (s *ScheduleService) createReminderNotificationJobs(ctx context.Context, sch database.Schedule, occ database.Occurrence) error {
	now := time.Now()

	// Check if offset 0 is muted
	mutedMap := make(map[int32]bool)
	for _, x := range sch.MutedOffsetsMin {
		mutedMap[x] = true
	}

	if mutedMap[0] {
		log.Printf("ScheduleService: Offset 0 is muted for reminder schedule %s, skipping notification", sch.ID)
		return nil
	}

	// Only create notification at exact occurrence time (offset 0)
	sendAt := occ.OccursAt

	// Skip notifications that would be sent in the past
	if sendAt.Before(now) {
		log.Printf("ScheduleService: Skipping reminder notification for schedule %s (would send at %v, now is %v)", sch.ID, sendAt, now)
		return nil
	}

	log.Printf("ScheduleService: Creating reminder notification job for schedule %s at exact time %v", sch.ID, sendAt)

	// Create payload
	payload := map[string]interface{}{
		"schedule_id":    sch.ID.String(),
		"occurrence_id":  occ.ID.String(),
		"offset_minutes": 0,
		"title":          sch.Title,
		"kind":           sch.Kind,
	}
	payloadJSON, _ := json.Marshal(payload)

	// Upsert notification job
	err := s.queries.UpsertNotificationJob(ctx, database.UpsertNotificationJobParams{
		UserID:        sch.UserID,
		ScheduleID:    uuid.NullUUID{UUID: sch.ID, Valid: true},
		OccurrenceID:  occ.ID,
		OffsetMinutes: 0,
		PlannedSendAt: sendAt,
		Payload:       payloadJSON,
	})
	if err != nil {
		return err
	}

	return nil
}

// Helper: calculate effective offsets (all minus muted)
func (s *ScheduleService) effectiveOffsets(all, muted []int32) []int {
	mutedMap := make(map[int32]bool)
	for _, x := range muted {
		mutedMap[x] = true
	}

	var result []int
	for _, x := range all {
		if !mutedMap[x] {
			result = append(result, int(x))
		}
	}
	return result
}

// isHighFrequency determines if a schedule should create tasks without due dates
func isHighFrequency(sch database.Schedule) bool {
	if !sch.Rrule.Valid {
		return false
	}
	freq := detectFrequency(sch.Rrule.String)
	return freq == "minutely" || freq == "hourly"
}

// nextAfterLocal emulates NextAfter with local-time awareness
func (s *ScheduleService) nextAfterLocal(set *rrule.Set, start time.Time, loc *time.Location, sch database.Schedule) time.Time {
	// Convert start to local time for comparison
	startLocal := start.In(loc)

	// Use iterator to get only the next occurrence
	iter := set.Iterator()

	// Find the next occurrence after start
	maxIterations := 1000 // Safety limit to prevent infinite loops
	iterationCount := 0

	for {
		occurrence, hasMore := iter()
		if !hasMore {
			break
		}

		iterationCount++
		if iterationCount > maxIterations {
			log.Printf("ScheduleService: Warning - exceeded %d iterations looking for next occurrence for schedule %s", maxIterations, sch.ID)
			break
		}

		if occurrence.After(startLocal) {
			log.Printf("ScheduleService: Next occurrence for schedule %s at %v (found after %d iterations)", sch.ID, occurrence, iterationCount)
			return occurrence
		}
	}

	return time.Time{}
}
