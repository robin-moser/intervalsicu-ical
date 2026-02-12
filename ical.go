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
		return "\U0001F3C6" // 🏆
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
// Intervals.icu returns formats like "2026-02-11T14:36:52" or "2026-02-11T00:00:00" or "2026-02-11".
func parseLocalTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}

	// Try full datetime first
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

// GenerateICS produces a complete ICS calendar from activities and events.
// When an activity is paired with a planned event (via paired_event_id), they
// are merged into a single calendar entry showing the activity timing/stats
// with the planned workout description appended.
func GenerateICS(activities []Activity, events []Event, cfg Config) []byte {
	var b strings.Builder

	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//Intervals Calendar//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString(foldLine("X-WR-CALNAME:Intervals.icu") + "\r\n")
	b.WriteString(foldLine("X-WR-TIMEZONE:"+cfg.Timezone) + "\r\n")

	now := time.Now().UTC().Format("20060102T150405Z")

	// Build lookup: eventID -> Activity for paired entries
	pairedByEvent := make(map[int]Activity)
	pairedActivityIDs := make(map[string]bool)
	for _, a := range activities {
		if a.PairedEventID != nil {
			pairedByEvent[*a.PairedEventID] = a
			pairedActivityIDs[a.ID] = true
		}
	}

	// Unpaired completed activities (no matching planned event)
	for _, a := range activities {
		if pairedActivityIDs[a.ID] {
			continue // will be emitted as part of the merged event
		}

		// Skip bare activities with no useful data (e.g. minimal Strava imports)
		if a.Name == "" && a.MovingTime <= 0 && a.Distance <= 0 {
			continue
		}

		startTime, hasTime := parseLocalTime(a.StartDateLocal)
		if !hasTime {
			continue
		}

		endTime := activityEndTime(startTime, a)

		emoji := sportEmoji(a.Type)
		summary := fmt.Sprintf("%s %s", emoji, a.Name)
		if a.Name == "" {
			summary = fmt.Sprintf("%s %s", emoji, a.Type)
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

	// Events: merged (paired) or standalone (unpaired)
	for _, e := range events {
		// Skip non-workout categories that don't make sense on a calendar
		switch e.Category {
		case "SET_EFTP", "FITNESS_DAYS", "SEASON_START", "SET_FITNESS", "TARGET":
			continue
		}

		if a, paired := pairedByEvent[e.ID]; paired {
			// --- Merged entry: activity + planned event ---
			startTime, hasTime := parseLocalTime(a.StartDateLocal)
			if !hasTime {
				continue
			}
			endTime := activityEndTime(startTime, a)

			emoji := sportEmoji(a.Type)
			summary := fmt.Sprintf("%s %s", emoji, a.Name)
			if a.Name == "" {
				summary = fmt.Sprintf("%s %s", emoji, a.Type)
			}

			// Activity stats first
			var desc []string
			desc = append(desc, "--- COMPLETED ---")
			desc = append(desc, activityStats(a)...)

			// Then the planned workout details
			desc = append(desc, "")
			desc = append(desc, "--- PLANNED ---")
			if e.Name != "" {
				desc = append(desc, fmt.Sprintf("Workout: %s", e.Name))
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
			b.WriteString(foldLine(fmt.Sprintf("UID:paired-%d-%s@intervals-calendar", e.ID, a.ID)) + "\r\n")
			b.WriteString(fmt.Sprintf("DTSTAMP:%s\r\n", now))
			b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", icsTimestamp(startTime)))
			b.WriteString(fmt.Sprintf("DTEND:%s\r\n", icsTimestamp(endTime)))
			b.WriteString("SEQUENCE:0\r\n")
			b.WriteString(foldLine("SUMMARY:"+escapeICS(summary)) + "\r\n")
			b.WriteString(foldLine("DESCRIPTION:"+escapeICS(strings.Join(desc, "\n"))) + "\r\n")
			b.WriteString("END:VEVENT\r\n")
		} else {
			// --- Standalone planned event (future or unmatched) ---
			startTime, hasTime := parseLocalTime(e.StartDateLocal)
			if startTime.IsZero() {
				continue
			}

			emoji := sportEmoji(e.Type)
			summary := fmt.Sprintf("%s %s", emoji, e.Name)
			if e.Name == "" {
				summary = fmt.Sprintf("%s Planned %s", emoji, e.Type)
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
				// All-day event (no specific time on the plan)
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
	}

	b.WriteString("END:VCALENDAR\r\n")
	return []byte(b.String())
}
