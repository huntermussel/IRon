package calendar

import (
	"fmt"
	"time"
)

// Helper to format current time for mock responses if needed
func now() string {
	return time.Now().Format(time.RFC3339)
}

func listCalendarEvents(start string) string {
	// In a real implementation, this would use a Calendar API.
	// For now, we return a mock response.
	// Check if start date is valid (basic check)
	if _, err := time.Parse("2006-01-02", start); err != nil {
		return fmt.Sprintf("error: invalid start_date format '%s', expected YYYY-MM-DD", start)
	}
	return fmt.Sprintf("[MOCK] list_calendar_events: found 1 event on %s\n- 10:00 AM: Team Sync", start)
}

func createCalendarEvent(title, start, end string) string {
	// In a real implementation, this would use a Calendar API.
	// For now, we return a mock response.
	return fmt.Sprintf("[MOCK] create_calendar_event: created '%s' from %s to %s", title, start, end)
}
