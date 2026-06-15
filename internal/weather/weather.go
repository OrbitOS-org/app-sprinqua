package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	forecastURL = "https://api.open-meteo.com/v1/forecast?latitude=%g&longitude=%g&daily=precipitation_sum,temperature_2m_min&forecast_days=1&timezone=auto"
	yesterdayURL = "https://api.open-meteo.com/v1/forecast?latitude=%g&longitude=%g&daily=precipitation_sum,temperature_2m_max,temperature_2m_min,relative_humidity_2m_mean,et0_fao_evapotranspiration&past_days=1&forecast_days=0&timezone=auto"
	archiveURL  = "https://archive-api.open-meteo.com/v1/archive?latitude=%g&longitude=%g&start_date=%s&end_date=%s&daily=et0_fao_evapotranspiration&timezone=auto"
)

// Result holds today's forecast, used by the rain and frost skip checks.
type Result struct {
	RainMM    float64
	TempMinC  float64
	FetchedAt time.Time
}

// DailyData holds yesterday's weather actuals for adjustment calculations.
type DailyData struct {
	RainMM      float64
	TempMaxC    float64
	TempMinC    float64
	HumidityPct float64
	EToMM       float64
	FetchedAt   time.Time
}

var (
	mu          sync.Mutex
	todayCache  map[string]*Result
	yesterCache map[string]*DailyData
)

func init() {
	todayCache  = make(map[string]*Result)
	yesterCache = make(map[string]*DailyData)
}

// FetchToday returns today's precipitation forecast in mm for the given coordinates.
// Results are cached for 1 hour.
func FetchToday(lat, lon float64) (*Result, error) {
	key := fmt.Sprintf("%.4f,%.4f", lat, lon)

	mu.Lock()
	if r, ok := todayCache[key]; ok && time.Since(r.FetchedAt) < time.Hour {
		mu.Unlock()
		return r, nil
	}
	mu.Unlock()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf(forecastURL, lat, lon))
	if err != nil {
		return nil, fmt.Errorf("open-meteo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo: status %d", resp.StatusCode)
	}

	var body struct {
		Daily struct {
			PrecipitationSum []float64 `json:"precipitation_sum"`
			TempMin          []float64 `json:"temperature_2m_min"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("open-meteo: decode: %w", err)
	}
	if len(body.Daily.PrecipitationSum) == 0 {
		return nil, fmt.Errorf("open-meteo: no data")
	}

	r := &Result{
		RainMM:    body.Daily.PrecipitationSum[0],
		TempMinC:  safeIdx(body.Daily.TempMin, 0),
		FetchedAt: time.Now(),
	}
	mu.Lock()
	todayCache[key] = r
	mu.Unlock()
	return r, nil
}

// FetchYesterday returns yesterday's weather actuals for Zimmerman/ETo calculations.
// Results are cached for 1 hour (data doesn't change once the day is done).
func FetchYesterday(lat, lon float64) (*DailyData, error) {
	key := fmt.Sprintf("%.4f,%.4f", lat, lon)

	mu.Lock()
	if d, ok := yesterCache[key]; ok && time.Since(d.FetchedAt) < time.Hour {
		mu.Unlock()
		return d, nil
	}
	mu.Unlock()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf(yesterdayURL, lat, lon))
	if err != nil {
		return nil, fmt.Errorf("open-meteo yesterday: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo yesterday: status %d", resp.StatusCode)
	}

	var body struct {
		Daily struct {
			PrecipitationSum      []float64 `json:"precipitation_sum"`
			TempMax               []float64 `json:"temperature_2m_max"`
			TempMin               []float64 `json:"temperature_2m_min"`
			HumidityMean          []float64 `json:"relative_humidity_2m_mean"`
			ETo                   []float64 `json:"et0_fao_evapotranspiration"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("open-meteo yesterday: decode: %w", err)
	}

	// past_days=1 + forecast_days=0 returns exactly 1 row (yesterday)
	if len(body.Daily.PrecipitationSum) == 0 {
		return nil, fmt.Errorf("open-meteo yesterday: no data")
	}

	d := &DailyData{
		RainMM:      safeIdx(body.Daily.PrecipitationSum, 0),
		TempMaxC:    safeIdx(body.Daily.TempMax, 0),
		TempMinC:    safeIdx(body.Daily.TempMin, 0),
		HumidityPct: safeIdx(body.Daily.HumidityMean, 0),
		EToMM:       safeIdx(body.Daily.ETo, 0),
		FetchedAt:   time.Now(),
	}
	mu.Lock()
	yesterCache[key] = d
	mu.Unlock()
	return d, nil
}

// FetchEToBaseline fetches 12 months of historical ETo from the Open-Meteo archive
// and returns the mean daily ETo in mm/day.
func FetchEToBaseline(lat, lon float64) (float64, error) {
	end := time.Now().AddDate(0, 0, -1)
	start := end.AddDate(-1, 0, 0)

	url := fmt.Sprintf(archiveURL, lat, lon,
		start.Format("2006-01-02"),
		end.Format("2006-01-02"),
	)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, fmt.Errorf("open-meteo archive: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("open-meteo archive: status %d", resp.StatusCode)
	}

	var body struct {
		Daily struct {
			ETo []float64 `json:"et0_fao_evapotranspiration"`
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0, fmt.Errorf("open-meteo archive: decode: %w", err)
	}
	if len(body.Daily.ETo) == 0 {
		return 0, fmt.Errorf("open-meteo archive: no ETo data")
	}

	var sum float64
	var count int
	for _, v := range body.Daily.ETo {
		if v > 0 {
			sum += v
			count++
		}
	}
	if count == 0 {
		return 0, fmt.Errorf("open-meteo archive: all ETo values are zero")
	}
	return sum / float64(count), nil
}

func safeIdx(s []float64, i int) float64 {
	if i < len(s) {
		return s[i]
	}
	return 0
}
