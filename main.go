package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
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
	ActiveRefreshMinutes int
	QuietRefreshMinutes  int
	ActiveStartHour      int
	ActiveEndHour        int
}

func loadConfig() Config {
	return Config{
		APIKey:               mustEnv("INTERVALS_API_KEY"),
		AthleteID:            mustEnv("INTERVALS_ATHLETE_ID"),
		Port:                 envOrDefault("PORT", "8491"),
		Timezone:             envOrDefault("TZ", "America/Denver"),
		PastDays:             envIntOrDefault("PAST_DAYS", 90),
		FutureDays:           envIntOrDefault("FUTURE_DAYS", 60),
		ActiveRefreshMinutes: envIntOrDefault("ACTIVE_REFRESH_MINUTES", 10),
		QuietRefreshMinutes:  envIntOrDefault("QUIET_REFRESH_MINUTES", 30),
		ActiveStartHour:      envIntOrDefault("ACTIVE_START_HOUR", 6),
		ActiveEndHour:        envIntOrDefault("ACTIVE_END_HOUR", 22),
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

// CalendarCache holds the generated ICS data.
type CalendarCache struct {
	mu   sync.RWMutex
	data []byte
}

func (c *CalendarCache) Get() []byte {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.data
}

func (c *CalendarCache) Set(data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data = data
}

func refresh(client *IntervalsClient, cache *CalendarCache, cfg Config) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		log.Printf("WARNING: Could not load timezone %q, using UTC: %v", cfg.Timezone, err)
		loc = time.UTC
	}

	now := time.Now().In(loc)
	pastDate := now.AddDate(0, 0, -cfg.PastDays).Format("2006-01-02")
	futureDate := now.AddDate(0, 0, cfg.FutureDays).Format("2006-01-02")
	today := now.Format("2006-01-02")

	log.Printf("Refreshing: activities %s to %s, events %s to %s",
		pastDate, today, pastDate, futureDate)

	activities, err := client.FetchActivities(pastDate, today)
	if err != nil {
		log.Printf("ERROR fetching activities: %v", err)
		return
	}

	// Fetch events for past AND future — past events show the planned workout
	// alongside the completed activity (what you were supposed to do vs what you did)
	events, err := client.FetchEvents(pastDate, futureDate)
	if err != nil {
		log.Printf("ERROR fetching events: %v", err)
		return
	}

	ics := GenerateICS(activities, events, cfg)
	cache.Set(ics)

	log.Printf("Refreshed: %d activities, %d events, %d bytes ICS",
		len(activities), len(events), len(ics))
}

// refreshInterval returns the appropriate refresh interval based on the current hour.
func refreshInterval(cfg Config, tz *time.Location) time.Duration {
	hour := time.Now().In(tz).Hour()
	if hour >= cfg.ActiveStartHour && hour < cfg.ActiveEndHour {
		return time.Duration(cfg.ActiveRefreshMinutes) * time.Minute
	}
	return time.Duration(cfg.QuietRefreshMinutes) * time.Minute
}

func startRefreshLoop(client *IntervalsClient, cache *CalendarCache, cfg Config) {
	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		loc = time.UTC
	}

	go func() {
		for {
			interval := refreshInterval(cfg, loc)
			log.Printf("Next refresh in %v", interval)
			time.Sleep(interval)
			refresh(client, cache, cfg)
		}
	}()
}

// getOutboundIP returns the preferred outbound IP of this machine.
func getOutboundIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return ""
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String()
}

func main() {
	cfg := loadConfig()
	client := NewIntervalsClient(cfg.APIKey, cfg.AthleteID)
	cache := &CalendarCache{}

	// Initial fetch before serving
	log.Println("Starting initial data fetch...")
	refresh(client, cache, cfg)
	log.Println("Initial fetch complete")

	// Start background refresh loop
	startRefreshLoop(client, cache, cfg)

	// HTTP handlers
	http.HandleFunc("/calendar.ics", func(w http.ResponseWriter, r *http.Request) {
		data := cache.Get()
		if len(data) == 0 {
			http.Error(w, "Calendar data not yet available", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Content-Disposition", `inline; filename="intervals.ics"`)
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Write(data)
	})

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		data := cache.Get()
		if len(data) == 0 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "not ready")
			return
		}
		fmt.Fprint(w, "ok")
	})

	addr := ":" + cfg.Port
	log.Printf("Listening on %s", addr)
	if ip := getOutboundIP(); ip != "" {
		log.Printf("Subscribe to: http://%s:%s/calendar.ics", ip, cfg.Port)
	} else {
		log.Printf("Subscribe to: http://<your-ip>:%s/calendar.ics", cfg.Port)
	}
	log.Printf("Refresh schedule: %dm (active %d:00-%d:00) / %dm (quiet)",
		cfg.ActiveRefreshMinutes, cfg.ActiveStartHour, cfg.ActiveEndHour, cfg.QuietRefreshMinutes)

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
