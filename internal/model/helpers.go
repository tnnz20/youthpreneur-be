package model

import "time"

// DateFormat is the ISO 8601 date layout (YYYY-MM-DD) used across request and response dates.
const DateFormat = "2006-01-02"

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

	formatted := date.Format(DateFormat)
	return &formatted
}
