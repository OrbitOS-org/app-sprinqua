package weather

import (
	"time"

	"sprinqua/internal/config"
)

// DailyRain holds actual precipitation for one calendar day.
type DailyRain struct {
	Date   time.Time
	RainMM float64
}

// RainDelayResult describes whether a post-rain irrigation pause is active.
type RainDelayResult struct {
	Active   bool
	DaysLeft int
	RainDate time.Time
}

// EvalRainDelay checks past daily rain against the delay rules without calling the API.
// rain must be sorted by date ascending; only days strictly before now's calendar day count.
// clearedRainDate (if set) marks a specific past rain event the user already dismissed via
// the "clear" action — it only suppresses that event, not any later rain that arrives after it.
func EvalRainDelay(rain []DailyRain, threshold float64, delayDays int, clearedRainDate string, now time.Time) RainDelayResult {
	if delayDays <= 0 {
		return RainDelayResult{}
	}
	today := truncateDay(now)

	var latestRain time.Time
	found := false
	for _, d := range rain {
		day := truncateDay(d.Date)
		if !day.Before(today) {
			continue
		}
		if d.RainMM >= threshold {
			if !found || day.After(latestRain) {
				latestRain = day
				found = true
			}
		}
	}
	if !found {
		return RainDelayResult{}
	}

	if clearedRainDate != "" {
		if cleared, err := time.ParseInLocation("2006-01-02", clearedRainDate, now.Location()); err == nil {
			if !latestRain.After(truncateDay(cleared)) {
				return RainDelayResult{}
			}
		}
	}

	daysSince := int(today.Sub(latestRain).Hours() / 24)
	if daysSince <= delayDays {
		return RainDelayResult{
			Active:   true,
			DaysLeft: delayDays - daysSince,
			RainDate: latestRain,
		}
	}
	return RainDelayResult{}
}

// RainDelayStatus fetches recent daily rain and evaluates the configured delay.
func RainDelayStatus(sw config.SmartWateringConfig, now time.Time) (RainDelayResult, error) {
	if !sw.Enabled || !sw.SkipEnabled || sw.RainDelayDays <= 0 || sw.Lat == 0 {
		return RainDelayResult{}, nil
	}
	fetchDays := sw.RainDelayDays + 2
	if fetchDays < 3 {
		fetchDays = 3
	}
	rain, err := FetchDailyRain(sw.Lat, sw.Lon, fetchDays)
	if err != nil {
		return RainDelayResult{}, err
	}
	return EvalRainDelay(rain, sw.EffectiveThreshold(), sw.RainDelayDays, sw.RainDelayClearedRainDate, now), nil
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
