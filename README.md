# intervals-calendar

A lightweight Docker container that generates an ICS calendar feed from your [Intervals.icu](https://intervals.icu) data. Subscribe to it from Apple Calendar, Google Calendar, Outlook, or any calendar app that supports ICS URLs.

## What it does

- Fetches your planned workouts (events) and completed activities from the Intervals.icu API
- **Pairs them together** — when a completed activity is linked to a planned workout, they merge into a single calendar entry showing both actual stats and the coach's plan
- Unpaired planned workouts appear as all-day events
- Unpaired completed activities appear with their actual start/end times and stats
- Serves a live `.ics` feed that auto-refreshes on a configurable schedule

## Calendar entry examples

**Paired (completed + planned):**
```
🚴 Z2 Endurance
--- COMPLETED ---
Type: Ride
Duration: 2h51m
Distance: 92.3km / 57.4mi
Elevation: 726m
Avg Power: 277W (NP)
Avg HR: 146bpm
TSS: 160

--- PLANNED ---
Workout: Z2 Endurance
Planned Duration: 3h00m

Cold Recovery - Building Back
Longer Z2 ride. Keep it easy...
```

**Unpaired planned workout:** Shows as an all-day event with the workout description and target duration.

**Unpaired activity:** Shows with actual time block and ride stats.

## Quick start

```bash
git clone https://github.com/youruser/intervals-calendar.git
cd intervals-calendar
```

Create a `.env` file:

```env
INTERVALS_API_KEY=your_api_key_here
INTERVALS_ATHLETE_ID=i12345
```

Run with Docker Compose:

```bash
docker compose up -d
```

Subscribe in your calendar app:

```
http://your-server-ip:8491/calendar.ics
```

## Getting your Intervals.icu API key

1. Log in to [intervals.icu](https://intervals.icu)
2. Go to **Settings** (gear icon)
3. Scroll to **Developer Settings**
4. Generate an API key
5. Your athlete ID is shown on the settings page (starts with `i`)

## Configuration

All configuration is via environment variables (set in `.env` or `docker-compose.yml`):

| Variable | Default | Description |
|---|---|---|
| `INTERVALS_API_KEY` | *required* | Your Intervals.icu API key |
| `INTERVALS_ATHLETE_ID` | *required* | Your athlete ID (e.g. `i12345`) |
| `PORT` | `8491` | HTTP port for the calendar feed |
| `TZ` | `America/Denver` | Timezone for date calculations |
| `PAST_DAYS` | `90` | How many days back to fetch activities |
| `FUTURE_DAYS` | `60` | How many days ahead to fetch planned events |
| `ACTIVE_REFRESH_MINUTES` | `10` | Refresh interval during active hours |
| `QUIET_REFRESH_MINUTES` | `30` | Refresh interval during quiet hours |
| `ACTIVE_START_HOUR` | `6` | Start of active refresh window (24h) |
| `ACTIVE_END_HOUR` | `22` | End of active refresh window (24h) |

## How pairing works

Intervals.icu links completed activities to planned workouts via the `paired_event_id` field on the activity. When the calendar is generated:

1. Activities with a `paired_event_id` are matched to their planned event
2. The merged entry uses the **activity's actual start/end time** and stats
3. The **planned workout description** is appended below the stats
4. Neither the standalone activity nor the standalone event appears — just the merged entry

Activities without a paired event (e.g. unplanned rides) and events without a paired activity (e.g. future plans, rest days) appear as standalone entries.

## Architecture

```
main.go        — HTTP server, config, refresh loop
intervals.go   — Intervals.icu API client (activities + events)
ical.go        — ICS calendar generation with pairing logic
```

Zero dependencies beyond the Go standard library. The Docker image is built on Alpine and weighs ~15MB.

## Endpoints

| Path | Description |
|---|---|
| `/calendar.ics` | The ICS calendar feed |
| `/health` | Health check (`ok` or `not ready`) |

## License

MIT
