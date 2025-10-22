package main

import "strings"

// detectFrequency parses RRULE string to determine frequency
func detectFrequency(rrule string) string {
	if strings.Contains(rrule, "FREQ=MINUTELY") {
		return "minutely"
	} else if strings.Contains(rrule, "FREQ=HOURLY") {
		return "hourly"
	} else if strings.Contains(rrule, "FREQ=DAILY") {
		return "daily"
	} else if strings.Contains(rrule, "FREQ=WEEKLY") {
		return "weekly"
	} else if strings.Contains(rrule, "FREQ=MONTHLY") {
		return "monthly"
	}
	return "other"
}
