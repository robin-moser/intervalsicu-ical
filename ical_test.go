package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDistanceAndPace(t *testing.T) {
	for _, tc := range []struct {
		name     string
		seconds  int
		meters   float64
		distance string
		pace     string
	}{
		{"whole minutes", 1500, 5000, "5.0km", "5:00 min/km"},
		{"round seconds", 1553, 5000, "5.0km", "5:11 min/km"},
		{"round up to minute", 1799, 5000, "5.0km", "6:00 min/km"},
		{"fractional distance", 1500, 5123.4, "5.1km", "4:53 min/km"},
		{"missing duration", 0, 5000, "5.0km", ""},
		{"negative duration", -1, 5000, "5.0km", ""},
		{"missing distance", 1500, 0, "", ""},
		{"negative distance", 1500, -1, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{Timezone: time.UTC}
			activity := Activity{ID: "a1", Name: "Run", StartDate: "2026-07-15T09:00:00Z", MovingTime: tc.seconds, ElapsedTime: 7200, Distance: tc.meters}
			event := Event{ID: 1, Name: "Run", StartDateLocal: "2026-07-15T09:00:00", MovingTime: tc.seconds, Distance: tc.meters, Category: "WORKOUT"}
			race := event
			race.Category = "RACE_A"
			for _, calendar := range []struct {
				name string
				data []byte
			}{
				{"activities", GenerateWorkoutsICS([]Activity{activity}, nil, cfg)},
				{"plans", GenerateWorkoutsICS(nil, []Event{event}, cfg)},
				{"races", GenerateRacesICS([]Event{race}, cfg)},
			} {
				t.Run(calendar.name, func(t *testing.T) {
					ics := strings.ReplaceAll(string(calendar.data), "\r\n ", "")
					if strings.Contains(ics, " / ") {
						t.Errorf("unexpected miles in calendar:\n%s", ics)
					}
					if tc.distance == "" {
						if strings.Contains(ics, "Distance:") {
							t.Errorf("unexpected distance in calendar:\n%s", ics)
						}
					} else if !strings.Contains(ics, "Distance: "+tc.distance) {
						t.Errorf("missing distance %s in calendar:\n%s", tc.distance, ics)
					}
					if tc.pace == "" {
						if strings.Contains(ics, "Pace:") {
							t.Errorf("unexpected pace in calendar:\n%s", ics)
						}
					} else if !strings.Contains(ics, "Distance: "+tc.distance+"\\nPace: "+tc.pace) {
						t.Errorf("missing pace directly below distance in calendar:\n%s", ics)
					}
				})
			}
		})
	}
}

func TestActivityPrefersAbsoluteStart(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, data, start, end string
	}{
		{"first repeated hour", `{"start_date":"2026-10-25T00:30:00Z","start_date_local":"2026-10-25T02:30:00"}`, "20261025T003000Z", "20261025T013000Z"},
		{"second repeated hour", `{"start_date":"2026-10-25T01:30:00Z","start_date_local":"2026-10-25T02:30:00"}`, "20261025T013000Z", "20261025T023000Z"},
		{"travel", `{"start_date":"2026-07-15T16:00:00Z","start_date_local":"2026-07-15T09:00:00"}`, "20260715T160000Z", "20260715T170000Z"},
		{"midnight", `{"start_date":"2026-07-14T22:00:00Z","start_date_local":"2026-07-15T00:00:00"}`, "20260714T220000Z", "20260714T230000Z"},
		{"invalid absolute time falls back", `{"start_date":"invalid","start_date_local":"2026-07-15T09:00:00"}`, "20260715T070000Z", "20260715T080000Z"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			activity := Activity{ID: "a1", Name: "Run", ElapsedTime: 3600}
			if err := json.Unmarshal([]byte(tc.data), &activity); err != nil {
				t.Fatal(err)
			}
			ics := string(GenerateWorkoutsICS([]Activity{activity}, nil, Config{Timezone: loc}))
			for _, line := range []string{"DTSTART:" + tc.start, "DTEND:" + tc.end} {
				if !strings.Contains(ics, "\r\n"+line+"\r\n") {
					t.Errorf("missing %s in calendar:\n%s", line, ics)
				}
			}
		})
	}
}

func TestTimedCalendars(t *testing.T) {
	for _, tc := range []struct {
		name, zone, local, start, end string
		seconds                       int
	}{
		{"Denver winter", "America/Denver", "2026-01-15T09:00:00", "20260115T160000Z", "20260115T170000Z", 3600},
		{"Denver summer", "America/Denver", "2026-07-15T09:00:00", "20260715T150000Z", "20260715T160000Z", 3600},
		{"Berlin winter", "Europe/Berlin", "2026-01-15T09:00:00", "20260115T080000Z", "20260115T090000Z", 3600},
		{"Berlin summer minutes", "Europe/Berlin", "2026-07-15T09:00", "20260715T070000Z", "20260715T080000Z", 3600},
		{"fractional offset and previous UTC day", "Asia/Kathmandu", "2026-07-15T01:00:00", "20260714T191500Z", "20260714T201500Z", 3600},
		{"spring forward", "Europe/Berlin", "2026-03-29T01:30:00", "20260329T003000Z", "20260329T023000Z", 7200},
		{"fall back", "Europe/Berlin", "2026-10-25T00:30:00", "20261024T223000Z", "20261025T023000Z", 14400},
		{"UTC", "UTC", "2026-07-15T09:00:00", "20260715T090000Z", "20260715T100000Z", 3600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			loc, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{Timezone: loc}
			activity := Activity{ID: "a1", Name: "Run", StartDateLocal: tc.local, ElapsedTime: tc.seconds, MovingTime: tc.seconds / 2}
			event := Event{ID: 1, Name: "Run", StartDateLocal: tc.local, MovingTime: tc.seconds, Category: "WORKOUT"}
			race := event
			race.Category = "RACE_A"

			for _, calendar := range []struct {
				name string
				data []byte
			}{
				{"activities", GenerateWorkoutsICS([]Activity{activity}, nil, cfg)},
				{"plans", GenerateWorkoutsICS(nil, []Event{event}, cfg)},
				{"races", GenerateRacesICS([]Event{race}, cfg)},
			} {
				t.Run(calendar.name, func(t *testing.T) {
					ics := string(calendar.data)
					for _, line := range []string{"DTSTART:" + tc.start, "DTEND:" + tc.end} {
						if !strings.Contains(ics, "\r\n"+line+"\r\n") {
							t.Errorf("missing %s in calendar:\n%s", line, ics)
						}
					}
				})
			}
		})
	}
}

func TestAllDayDatesStayLocal(t *testing.T) {
	for _, tc := range []struct {
		zone, date, start, end string
	}{
		{"Europe/Berlin", "2026-03-29", "20260329", "20260330"},
		// Santiago skips midnight on this date; a local-time parse would move it back a day.
		{"America/Santiago", "2026-09-06", "20260906", "20260907"},
	} {
		t.Run(tc.zone, func(t *testing.T) {
			loc, err := time.LoadLocation(tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{Timezone: loc, IncludeAllDayEvents: true}
			for _, local := range []string{tc.date, tc.date + "T00:00:00"} {
				event := Event{ID: 1, Name: "Rest", StartDateLocal: local, Category: "WORKOUT"}
				race := event
				race.Category = "RACE_A"
				for _, data := range [][]byte{
					GenerateWorkoutsICS(nil, []Event{event}, cfg),
					GenerateRacesICS([]Event{race}, cfg),
					GenerateMetricsICS([]Wellness{{ID: tc.date, Weight: 70}}, cfg),
				} {
					ics := string(data)
					for _, line := range []string{"DTSTART;VALUE=DATE:" + tc.start, "DTEND;VALUE=DATE:" + tc.end} {
						if !strings.Contains(ics, "\r\n"+line+"\r\n") {
							t.Errorf("input %s: missing %s in calendar:\n%s", local, line, ics)
						}
					}
				}
			}
		})
	}
}
