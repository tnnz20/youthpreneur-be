package model

import "time"

// optionalString returns nil for an empty string so optional fields serialize as
// JSON null.
func optionalString(value string) *string {
	if value == "" {
		return nil
	}

	return &value
}

// formatOptionalDate renders an optional date as `YYYY-MM-DD` or JSON null.
func formatOptionalDate(date *time.Time) *string {
	if date == nil {
		return nil
	}

	formatted := date.Format("2006-01-02")
	return &formatted
}
