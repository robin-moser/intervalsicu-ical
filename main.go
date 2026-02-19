package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration from environment variables.
type Config struct {
	APIKey               string
	AthleteID            string
	Port                 string
	Timezone             string
	PastDays             int
	FutureDays           int
	ExcludedKeywords     []string
	IncludeAllDayEvents  bool
	IncludeFutureRaces   bool
}

func loadConfig() Config {
	return Config{
		APIKey:              mustEnv("INTERVALS_API_KEY"),
		AthleteID:           mustEnv("INTERVALS_ATHLETE_ID"),
		Port:                envOrDefault("PORT", "8491"),
		Timezone:            envOrDefault("TZ", "America/Denver"),
		PastDays:            envIntOrDefault("PAST_DAYS", 90),
		FutureDays:          envIntOrDefault("FUTURE_DAYS", 60),
		ExcludedKeywords:    envKeywords("EXCLUDED_KEYWORDS"),
		IncludeAllDayEvents: envBoolOrDefault("INCLUDE_ALL_DAY_EVENTS", true),
		IncludeFutureRaces:  envBoolOrDefault("INCLUDE_FUTURE_RACES", true),
	}
}

func mustEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("Required environment variable %s is not set", key)
	}
	return v
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func envBoolOrDefault(key string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch v {
	case "true", "1", "yes":
		return true
	case "false", "0", "no":
		return false
	}
	return def
}

// envKeywords parses a comma-separated list of keywords from an env var.
// Returns nil (no filtering) if the var is unset or blank.
func envKeywords(key string) []string {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	var out []string
	for _, kw := range strings.Split(v, ",") {
		kw = strings.TrimSpace(kw)
		if kw != "" {
			out = append(out, strings.ToLower(kw))
		}
	}
	return out
}

func main() {
	cfg := loadConfig()
	client := NewIntervalsClient(cfg.APIKey, cfg.AthleteID)

	// Route pattern: /<athlete-id>/<calendar-type>.ics
	baseRoute := "/" + cfg.AthleteID

	// Metrics calendar (wellness data only)
	http.HandleFunc(baseRoute+"/metrics.ics", func(w http.ResponseWriter, r *http.Request) {
		handleMetricsCalendar(w, r, client, cfg)
	})

	// Workouts calendar (completed activities + planned workouts, no races)
	http.HandleFunc(baseRoute+"/workouts.ics", func(w http.ResponseWriter, r *http.Request) {
		handleWorkoutsCalendar(w, r, client, cfg)
	})

	// Races calendar (race events only, extended 1 year)
	http.HandleFunc(baseRoute+"/races.ics", func(w http.ResponseWriter, r *http.Request) {
		handleRacesCalendar(w, r, client, cfg)
	})

	// Health check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok")
	})

	addr := ":" + cfg.Port
	log.Printf("Listening on %s", addr)
	log.Printf("Metrics:  http://<your-ip>:%s%s/metrics.ics", cfg.Port, baseRoute)
	log.Printf("Workouts: http://<your-ip>:%s%s/workouts.ics", cfg.Port, baseRoute)
	log.Printf("Races:    http://<your-ip>:%s%s/races.ics", cfg.Port, baseRoute)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func handleMetricsCalendar(w http.ResponseWriter, r *http.Request, client *IntervalsClient, cfg Config) {
	loc, _ := time.LoadLocation(cfg.Timezone)
	if loc == nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)
	pastDate := now.AddDate(0, 0, -cfg.PastDays).Format("2006-01-02")
	today := now.Format("2006-01-02")

	wellness, err := client.FetchWellness(pastDate, today)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch wellness data: %v", err), http.StatusInternalServerError)
		return
	}

	ics := GenerateMetricsICS(wellness, cfg)

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="metrics.ics"`)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(ics)
}

func handleWorkoutsCalendar(w http.ResponseWriter, r *http.Request, client *IntervalsClient, cfg Config) {
	loc, _ := time.LoadLocation(cfg.Timezone)
	if loc == nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)
	pastDate := now.AddDate(0, 0, -cfg.PastDays).Format("2006-01-02")
	futureDate := now.AddDate(0, 0, cfg.FutureDays).Format("2006-01-02")
	today := now.Format("2006-01-02")

	// Fetch completed activities
	activities, err := client.FetchActivities(pastDate, today)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch activities: %v", err), http.StatusInternalServerError)
		return
	}

	// Fetch planned workouts (non-race events)
	events, err := client.FetchEvents(pastDate, futureDate)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch events: %v", err), http.StatusInternalServerError)
		return
	}

	ics := GenerateWorkoutsICS(activities, events, cfg)

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="workouts.ics"`)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(ics)
}

func handleRacesCalendar(w http.ResponseWriter, r *http.Request, client *IntervalsClient, cfg Config) {
	loc, _ := time.LoadLocation(cfg.Timezone)
	if loc == nil {
		loc = time.UTC
	}

	now := time.Now().In(loc)
	pastDate := now.AddDate(0, 0, -cfg.PastDays).Format("2006-01-02")
	farFutureDate := now.AddDate(1, 0, 0).Format("2006-01-02")

	// Fetch all events in extended range
	events, err := client.FetchEvents(pastDate, farFutureDate)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch events: %v", err), http.StatusInternalServerError)
		return
	}

	ics := GenerateRacesICS(events, cfg)

	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Content-Disposition", `inline; filename="races.ics"`)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Write(ics)
}
