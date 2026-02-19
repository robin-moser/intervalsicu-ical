package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const baseURL = "https://intervals.icu/api/v1"

// Activity represents a completed workout from Intervals.icu.
type Activity struct {
	ID                 string  `json:"id"`
	StartDateLocal     string  `json:"start_date_local"`
	Type               string  `json:"type"`
	Name               string  `json:"name"`
	Description        string  `json:"description"`
	MovingTime         int     `json:"moving_time"`
	ElapsedTime        int     `json:"elapsed_time"`
	Distance           float64 `json:"distance"`
	TotalElevationGain float64 `json:"total_elevation_gain"`
	ICUTrainingLoad    int     `json:"icu_training_load"`
	ICUWeightedAvgW    int     `json:"icu_weighted_avg_watts"`
	AverageHeartrate   int     `json:"average_heartrate"`
	AverageSpeed       float64 `json:"average_speed"`
	Calories           int     `json:"calories"`
	Trainer            bool    `json:"trainer"`
	PairedEventID      *int    `json:"paired_event_id"`
}

// Event represents a planned workout/event from Intervals.icu.
type Event struct {
	ID             int    `json:"id"`
	StartDateLocal string `json:"start_date_local"`
	EndDateLocal   string `json:"end_date_local"`
	Type           string `json:"type"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Category       string `json:"category"`
	MovingTime     int    `json:"moving_time"`
	Distance       float64 `json:"distance"`
	Indoor         bool   `json:"indoor"`
	Color          string `json:"color"`
}

// Wellness represents daily fitness/fatigue metrics from Intervals.icu.
type Wellness struct {
	ID         string  `json:"id"`
	CTL        float64 `json:"ctl"`
	ATL        float64 `json:"atl"`
	TSB        float64 `json:"tsb"`
	RampRate   float64 `json:"rampRate"`
	CTLRamp    float64 `json:"ctlRamp"`
	Weight     float64 `json:"weight"`
	RestingHR  float64 `json:"restingHR"`
	HRV        float64 `json:"hrv"`
	SleepScore float64 `json:"sleepScore"`
	SleepSecs  float64 `json:"sleepSecs"`
}

// IntervalsClient fetches data from the Intervals.icu API.
type IntervalsClient struct {
	apiKey    string
	athleteID string
	client    *http.Client
}

func NewIntervalsClient(apiKey, athleteID string) *IntervalsClient {
	return &IntervalsClient{
		apiKey:    apiKey,
		athleteID: athleteID,
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *IntervalsClient) doRequest(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.SetBasicAuth("API_KEY", c.apiKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

// FetchActivities returns completed activities within the date range.
func (c *IntervalsClient) FetchActivities(oldest, newest string) ([]Activity, error) {
	url := fmt.Sprintf("%s/athlete/%s/activities?oldest=%s&newest=%s",
		baseURL, c.athleteID, oldest, newest)

	data, err := c.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("fetching activities: %w", err)
	}

	var activities []Activity
	if err := json.Unmarshal(data, &activities); err != nil {
		return nil, fmt.Errorf("parsing activities: %w", err)
	}
	return activities, nil
}

// FetchEvents returns planned events within the date range.
func (c *IntervalsClient) FetchEvents(oldest, newest string) ([]Event, error) {
	url := fmt.Sprintf("%s/athlete/%s/events?oldest=%s&newest=%s",
		baseURL, c.athleteID, oldest, newest)

	data, err := c.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("fetching events: %w", err)
	}

	var events []Event
	if err := json.Unmarshal(data, &events); err != nil {
		return nil, fmt.Errorf("parsing events: %w", err)
	}
	return events, nil
}

// FetchWellness returns daily wellness/fitness metrics within the date range.
func (c *IntervalsClient) FetchWellness(oldest, newest string) ([]Wellness, error) {
	url := fmt.Sprintf("%s/athlete/%s/wellness?oldest=%s&newest=%s",
		baseURL, c.athleteID, oldest, newest)

	data, err := c.doRequest(url)
	if err != nil {
		return nil, fmt.Errorf("fetching wellness: %w", err)
	}

	var wellness []Wellness
	if err := json.Unmarshal(data, &wellness); err != nil {
		return nil, fmt.Errorf("parsing wellness: %w", err)
	}
	return wellness, nil
}
