package main

import (
	"fmt"
	"strings"
	"time"
)

// sportEmoji returns an emoji for the activity/event type.
func sportEmoji(actType string) string {
	t := strings.ToLower(actType)
	switch {
	case strings.Contains(t, "ride") || strings.Contains(t, "cycling"):
		return "\U0001F6B4" // 🚴
	case strings.Contains(t, "run"):
		return "\U0001F3C3" // 🏃
	case strings.Contains(t, "swim"):
		return "\U0001F3CA" // 🏊
	case strings.Contains(t, "walk") || strings.Contains(t, "hike"):
		return "\U0001F6B6" // 🚶
	case strings.Contains(t, "weight") || strings.Contains(t, "strength"):
		return "\U0001F3CB" // 🏋
	case strings.Contains(t, "yoga"):
		return "\U0001F9D8" // 🧘
	case strings.Contains(t, "ski"):
		return "\U000026F7" // ⛷
	case strings.Contains(t, "row"):
		return "\U0001F6A3" // 🚣
	default:
		return "" // No emoji
	}
}

// escapeICS escapes text for ICS fields (backslash, semicolon, comma, newlines).
func escapeICS(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\r\n", "\\n")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\n")
	return s
}

// foldLine folds a long ICS line per RFC 5545 (max 75 octets per line).
func foldLine(line string) string {
	if len(line) <= 75 {
		return line
	}
	var b strings.Builder
	for i := 0; i < len(line); {
		end := i + 75
		if i > 0 {
			end = i + 74 // account for leading space on continuation lines
		}
		if end > len(line) {
			end = len(line)
		}
		if i == 0 {
			b.WriteString(line[i:end])
		} else {
			b.WriteString("\r\n ")
			b.WriteString(line[i:end])
		}
		i = end
	}
	return b.String()
}

// formatDuration formats seconds as a human-readable duration.
func formatDuration(seconds int) string {
	h := seconds / 3600
	m := (seconds % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

// formatDistance formats meters as km or miles with one decimal.
func formatDistance(meters float64) string {
	km := meters / 1000.0
	mi := km * 0.621371
	return fmt.Sprintf("%.1fkm / %.1fmi", km, mi)
}

// parseLocalTime parses an Intervals.icu local datetime string.
func parseLocalTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}

	for _, layout := range []string{
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			hasTime := strings.Contains(s, "T") && !strings.HasSuffix(s, "T00:00:00")
			return t, hasTime
		}
	}
	return time.Time{}, false
}

// icsTimestamp formats a time as ICS DTSTART/DTEND with timezone.
func icsTimestamp(t time.Time) string {
	return t.Format("20060102T150405")
}

// activityStats builds the stats lines for a completed activity.
func activityStats(a Activity) []string {
	var stats []string
	if a.Type != "" {
		stats = append(stats, fmt.Sprintf("Type: %s", a.Type))
	}
	if a.MovingTime > 0 {
		stats = append(stats, fmt.Sprintf("Duration: %s", formatDuration(a.MovingTime)))
	}
	if a.Distance > 0 {
		stats = append(stats, fmt.Sprintf("Distance: %s", formatDistance(a.Distance)))
	}
	if a.TotalElevationGain > 0 {
		stats = append(stats, fmt.Sprintf("Elevation: %.0fm", a.TotalElevationGain))
	}
	if a.ICUWeightedAvgW > 0 {
		stats = append(stats, fmt.Sprintf("Avg Power: %dW (NP)", a.ICUWeightedAvgW))
	}
	if a.AverageHeartrate > 0 {
		stats = append(stats, fmt.Sprintf("Avg HR: %dbpm", a.AverageHeartrate))
	}
	if a.ICUTrainingLoad > 0 {
		stats = append(stats, fmt.Sprintf("TSS: %d", a.ICUTrainingLoad))
	}
	if a.Calories > 0 {
		stats = append(stats, fmt.Sprintf("Calories: %d", a.Calories))
	}
	if a.Trainer {
		stats = append(stats, "Indoor/Trainer")
	}
	return stats
}

// activityEndTime calculates the end time for an activity.
func activityEndTime(startTime time.Time, a Activity) time.Time {
	if a.ElapsedTime > 0 {
		return startTime.Add(time.Duration(a.ElapsedTime) * time.Second)
	}
	if a.MovingTime > 0 {
		return startTime.Add(time.Duration(a.MovingTime) * time.Second)
	}
	return startTime.Add(1 * time.Hour)
}

// hasExcludedKeyword returns true if name contains any of the excluded keywords (case-insensitive).
func hasExcludedKeyword(name string, keywords []string) bool {
	if len(keywords) == 0 {
		return false
	}
	lower := strings.ToLower(name)
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// GenerateMetricsICS generates a calendar with wellness/metrics data only.
func GenerateMetricsICS(wellness []Wellness, cfg Config) []byte {
	var b strings.Builder

	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//Intervals Calendar - Metrics//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString(foldLine("X-WR-CALNAME:Intervals.icu - Metrics") + "\r\n")
	b.WriteString(foldLine("X-WR-TIMEZONE:"+cfg.Timezone) + "\r\n")

	now := time.Now().UTC().Format("20060102T150405Z")

	var prevWeight float64
	for i, w := range wellness {
		wDate, err := time.Parse("2006-01-02", w.ID)
		if err != nil {
			continue
		}

		var parts []string

		// Sleep duration and score together
		if w.SleepSecs > 0 {
			hours := int(w.SleepSecs) / 3600
			mins := (int(w.SleepSecs) % 3600) / 60
			sleepStr := ""
			if hours > 0 && mins > 0 {
				sleepStr = fmt.Sprintf("🛌%dh%dm", hours, mins)
			} else if hours > 0 {
				sleepStr = fmt.Sprintf("🛌%dh", hours)
			} else {
				sleepStr = fmt.Sprintf("🛌%dm", mins)
			}

			if w.SleepScore > 0 {
				sleepStr = fmt.Sprintf("%s (%.0f)", sleepStr, w.SleepScore)
			}
			parts = append(parts, sleepStr)
		} else if w.SleepScore > 0 {
			parts = append(parts, fmt.Sprintf("😴(%.0f)", w.SleepScore))
		}

		if w.RestingHR > 0 {
			parts = append(parts, fmt.Sprintf("❤️%.0f", w.RestingHR))
		}
		if w.HRV > 0 {
			parts = append(parts, fmt.Sprintf("%.0f", w.HRV))
		}

		// Only show weight if it changed from previous day
		if w.Weight > 0 && (i == 0 || w.Weight != prevWeight) {
			parts = append(parts, fmt.Sprintf("⚖️%.1f", w.Weight))
		}
		if w.Weight > 0 {
			prevWeight = w.Weight
		}

		if len(parts) == 0 {
			continue
		}

		summary := strings.Join(parts, " ")

		dateStr := wDate.Format("20060102")
		nextDay := wDate.AddDate(0, 0, 1).Format("20060102")

		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(foldLine(fmt.Sprintf("UID:wellness-%s@intervals-calendar", w.ID)) + "\r\n")
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))
		b.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", dateStr))
		b.WriteString(fmt.Sprintf("DTEND;VALUE=DATE:%s\r\n", nextDay))
		b.WriteString("SEQUENCE:0\r\n")
		b.WriteString(foldLine("SUMMARY:"+escapeICS(summary)) + "\r\n")
		b.WriteString("END:VEVENT\r\n")
	}

	b.WriteString("END:VCALENDAR\r\n")
	return []byte(b.String())
}

// GenerateWorkoutsICS generates a calendar with completed activities + planned workouts (non-races).
func GenerateWorkoutsICS(activities []Activity, events []Event, cfg Config) []byte {
	var b strings.Builder

	// Build a set of event IDs that have a completed activity paired to them.
	// These planned events should be suppressed in favour of the actual activity.
	pairedEventIDs := make(map[int]bool)
	for _, a := range activities {
		if a.PairedEventID != nil {
			pairedEventIDs[*a.PairedEventID] = true
		}
	}

	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//Intervals Calendar - Workouts//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString(foldLine("X-WR-CALNAME:Intervals.icu - Workouts") + "\r\n")
	b.WriteString(foldLine("X-WR-TIMEZONE:"+cfg.Timezone) + "\r\n")

	now := time.Now().UTC().Format("20060102T150405Z")

	for _, a := range activities {
		// Skip bare activities with no useful data
		if a.Name == "" && a.MovingTime <= 0 && a.Distance <= 0 {
			continue
		}

		if hasExcludedKeyword(a.Name, cfg.ExcludedKeywords) {
			continue
		}

		startTime, hasTime := parseLocalTime(a.StartDateLocal)
		if !hasTime {
			continue
		}

		endTime := activityEndTime(startTime, a)

		emoji := sportEmoji(a.Type)
		summary := a.Name
		if a.Name == "" {
			summary = a.Type
		}
		if emoji != "" {
			summary = fmt.Sprintf("%s %s", emoji, summary)
		}

		desc := activityStats(a)

		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(foldLine(fmt.Sprintf("UID:activity-%s@intervals-calendar", a.ID)) + "\r\n")
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))
		b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", icsTimestamp(startTime)))
		b.WriteString(fmt.Sprintf("DTEND:%s\r\n", icsTimestamp(endTime)))
		b.WriteString("SEQUENCE:0\r\n")
		b.WriteString(foldLine("SUMMARY:"+escapeICS(summary)) + "\r\n")
		if len(desc) > 0 {
			b.WriteString(foldLine("DESCRIPTION:"+escapeICS(strings.Join(desc, "\n"))) + "\r\n")
		}
		b.WriteString("END:VEVENT\r\n")
	}

	// Add planned workout events (excluding races)
	for _, e := range events {
		// Skip events that have a completed activity paired to them
		if pairedEventIDs[e.ID] {
			continue
		}

		// Skip race events
		if strings.HasPrefix(e.Category, "RACE") {
			continue
		}

		// Skip non-workout categories
		switch e.Category {
		case "SET_EFTP", "FITNESS_DAYS", "SEASON_START", "SET_FITNESS", "TARGET":
			continue
		}

		if hasExcludedKeyword(e.Name, cfg.ExcludedKeywords) {
			continue
		}

		startTime, hasTime := parseLocalTime(e.StartDateLocal)
		if startTime.IsZero() {
			continue
		}

		// Skip all-day events unless IncludeAllDayEvents is true
		if !hasTime && !cfg.IncludeAllDayEvents {
			continue
		}

		emoji := sportEmoji(e.Type)
		summary := e.Name
		if e.Name == "" {
			summary = "Planned " + e.Type
		}
		if emoji != "" {
			summary = fmt.Sprintf("%s %s", emoji, summary)
		}

		var desc []string
		if e.Category != "" && e.Category != "WORKOUT" {
			desc = append(desc, fmt.Sprintf("Category: %s", e.Category))
		}
		if e.Type != "" {
			desc = append(desc, fmt.Sprintf("Type: %s", e.Type))
		}
		if e.MovingTime > 0 {
			desc = append(desc, fmt.Sprintf("Planned Duration: %s", formatDuration(e.MovingTime)))
		}
		if e.Distance > 0 {
			desc = append(desc, fmt.Sprintf("Planned Distance: %s", formatDistance(e.Distance)))
		}
		if e.Description != "" {
			desc = append(desc, "")
			desc = append(desc, e.Description)
		}

		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(foldLine(fmt.Sprintf("UID:event-%d@intervals-calendar", e.ID)) + "\r\n")
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))
		if hasTime {
			duration := time.Duration(e.MovingTime) * time.Second
			if duration <= 0 {
				duration = 1 * time.Hour
			}
			endTime := startTime.Add(duration)
			b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", icsTimestamp(startTime)))
			b.WriteString(fmt.Sprintf("DTEND:%s\r\n", icsTimestamp(endTime)))
		} else {
			// All-day event
			dateStr := startTime.Format("20060102")
			nextDay := startTime.AddDate(0, 0, 1).Format("20060102")
			b.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", dateStr))
			b.WriteString(fmt.Sprintf("DTEND;VALUE=DATE:%s\r\n", nextDay))
		}
		b.WriteString("SEQUENCE:0\r\n")
		b.WriteString(foldLine("SUMMARY:"+escapeICS(summary)) + "\r\n")
		if len(desc) > 0 {
			b.WriteString(foldLine("DESCRIPTION:"+escapeICS(strings.Join(desc, "\n"))) + "\r\n")
		}
		b.WriteString("END:VEVENT\r\n")
	}

	b.WriteString("END:VCALENDAR\r\n")
	return []byte(b.String())
}

// GenerateRacesICS generates a calendar with race events only.
func GenerateRacesICS(events []Event, cfg Config) []byte {
	var b strings.Builder

	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//Intervals Calendar - Races//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString(foldLine("X-WR-CALNAME:Intervals.icu - Races") + "\r\n")
	b.WriteString(foldLine("X-WR-TIMEZONE:"+cfg.Timezone) + "\r\n")

	now := time.Now().UTC().Format("20060102T150405Z")

	for _, e := range events {
		// Only include race events
		if !strings.HasPrefix(e.Category, "RACE") {
			continue
		}

		if hasExcludedKeyword(e.Name, cfg.ExcludedKeywords) {
			continue
		}

		startTime, hasTime := parseLocalTime(e.StartDateLocal)
		if startTime.IsZero() {
			continue
		}

		// Skip all-day events unless IncludeAllDayEvents is true
		if !hasTime && !cfg.IncludeAllDayEvents {
			continue
		}

		emoji := sportEmoji(e.Type)
		summary := e.Name
		if e.Name == "" {
			summary = "Planned " + e.Type
		}
		if emoji != "" {
			summary = fmt.Sprintf("%s %s", emoji, summary)
		}

		var desc []string
		if e.Category != "" && e.Category != "WORKOUT" {
			desc = append(desc, fmt.Sprintf("Category: %s", e.Category))
		}
		if e.Type != "" {
			desc = append(desc, fmt.Sprintf("Type: %s", e.Type))
		}
		if e.MovingTime > 0 {
			desc = append(desc, fmt.Sprintf("Planned Duration: %s", formatDuration(e.MovingTime)))
		}
		if e.Distance > 0 {
			desc = append(desc, fmt.Sprintf("Planned Distance: %s", formatDistance(e.Distance)))
		}
		if e.Description != "" {
			desc = append(desc, "")
			desc = append(desc, e.Description)
		}

		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(foldLine(fmt.Sprintf("UID:event-%d@intervals-calendar", e.ID)) + "\r\n")
		b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))
		if hasTime {
			duration := time.Duration(e.MovingTime) * time.Second
			if duration <= 0 {
				duration = 1 * time.Hour
			}
			endTime := startTime.Add(duration)
			b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", icsTimestamp(startTime)))
			b.WriteString(fmt.Sprintf("DTEND:%s\r\n", icsTimestamp(endTime)))
		} else {
			// All-day event
			dateStr := startTime.Format("20060102")
			nextDay := startTime.AddDate(0, 0, 1).Format("20060102")
			b.WriteString(fmt.Sprintf("DTSTART;VALUE=DATE:%s\r\n", dateStr))
			b.WriteString(fmt.Sprintf("DTEND;VALUE=DATE:%s\r\n", nextDay))
		}
		b.WriteString("SEQUENCE:0\r\n")
		b.WriteString(foldLine("SUMMARY:"+escapeICS(summary)) + "\r\n")
		if len(desc) > 0 {
			b.WriteString(foldLine("DESCRIPTION:"+escapeICS(strings.Join(desc, "\n"))) + "\r\n")
		}
		b.WriteString("END:VEVENT\r\n")
	}

	b.WriteString("END:VCALENDAR\r\n")
	return []byte(b.String())
}
