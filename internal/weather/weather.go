package weather

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const apiURL = "https://api.open-meteo.com/v1/forecast?latitude=%g&longitude=%g&daily=precipitation_sum&forecast_days=1&timezone=auto"

type Result struct {
	RainMM float64
	FetchedAt time.Time
}

var (
	mu    sync.Mutex
	cache map[string]*Result
)

func init() {
	cache = make(map[string]*Result)
}

// FetchToday returns today's precipitation forecast in mm for the given coordinates.
// Results are cached for 1 hour.
func FetchToday(lat, lon float64) (*Result, error) {
	key := fmt.Sprintf("%.4f,%.4f", lat, lon)

	mu.Lock()
	if r, ok := cache[key]; ok && time.Since(r.FetchedAt) < time.Hour {
		mu.Unlock()
		return r, nil
	}
	mu.Unlock()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf(apiURL, lat, lon))
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
		} `json:"daily"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("open-meteo: decode: %w", err)
	}
	if len(body.Daily.PrecipitationSum) == 0 {
		return nil, fmt.Errorf("open-meteo: no data")
	}

	r := &Result{RainMM: body.Daily.PrecipitationSum[0], FetchedAt: time.Now()}
	mu.Lock()
	cache[key] = r
	mu.Unlock()
	return r, nil
}
