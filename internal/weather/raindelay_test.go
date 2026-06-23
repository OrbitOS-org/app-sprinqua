package weather

import (
	"testing"
	"time"
)

func TestEvalRainDelay(t *testing.T) {
	loc := time.Local
	threshold := 5.0
	delayDays := 2

	monday := time.Date(2026, 6, 15, 0, 0, 0, 0, loc)
	rain := []DailyRain{{Date: monday, RainMM: 8.0}}

	t.Run("active day after rain", func(t *testing.T) {
		tuesday := time.Date(2026, 6, 16, 12, 0, 0, 0, loc)
		res := EvalRainDelay(rain, threshold, delayDays, "", tuesday)
		if !res.Active {
			t.Fatal("expected active delay on Tuesday")
		}
		if res.DaysLeft != 1 {
			t.Fatalf("daysLeft = %d, want 1", res.DaysLeft)
		}
	})

	t.Run("last delay day", func(t *testing.T) {
		wednesday := time.Date(2026, 6, 17, 8, 0, 0, 0, loc)
		res := EvalRainDelay(rain, threshold, delayDays, "", wednesday)
		if !res.Active {
			t.Fatal("expected active delay on Wednesday")
		}
		if res.DaysLeft != 0 {
			t.Fatalf("daysLeft = %d, want 0", res.DaysLeft)
		}
	})

	t.Run("delay expired", func(t *testing.T) {
		thursday := time.Date(2026, 6, 18, 8, 0, 0, 0, loc)
		res := EvalRainDelay(rain, threshold, delayDays, "", thursday)
		if res.Active {
			t.Fatal("expected no delay on Thursday")
		}
	})

	t.Run("below threshold ignored", func(t *testing.T) {
		light := []DailyRain{{Date: monday, RainMM: 2.0}}
		tuesday := time.Date(2026, 6, 16, 12, 0, 0, 0, loc)
		res := EvalRainDelay(light, threshold, delayDays, "", tuesday)
		if res.Active {
			t.Fatal("expected no delay when rain below threshold")
		}
	})

	t.Run("cleared rain date suppresses that event", func(t *testing.T) {
		tuesday := time.Date(2026, 6, 16, 12, 0, 0, 0, loc)
		// User dismissed Monday's rain event specifically.
		res := EvalRainDelay(rain, threshold, delayDays, "2026-06-15", tuesday)
		if res.Active {
			t.Fatal("expected cleared delay")
		}
	})

	t.Run("clearing an old event does not block a newer rain event", func(t *testing.T) {
		wednesday := time.Date(2026, 6, 17, 0, 0, 0, 0, loc)
		rainTwoEvents := []DailyRain{
			{Date: monday, RainMM: 8.0},    // dismissed by the user
			{Date: wednesday, RainMM: 12.0}, // new, independent rain after the clear
		}
		thursday := time.Date(2026, 6, 18, 12, 0, 0, 0, loc)
		res := EvalRainDelay(rainTwoEvents, threshold, delayDays, "2026-06-15", thursday)
		if !res.Active {
			t.Fatal("expected a fresh delay triggered by Wednesday's rain, even though Monday's was cleared")
		}
		if !res.RainDate.Equal(wednesday) {
			t.Fatalf("rainDate = %v, want Wednesday's rain event", res.RainDate)
		}
	})

	t.Run("zero delay days off", func(t *testing.T) {
		tuesday := time.Date(2026, 6, 16, 12, 0, 0, 0, loc)
		res := EvalRainDelay(rain, threshold, 0, "", tuesday)
		if res.Active {
			t.Fatal("expected off when delay days is 0")
		}
	})
}
