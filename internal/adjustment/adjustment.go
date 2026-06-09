package adjustment

import (
	"math"
	"time"

	"sprinqua/internal/config"
	"sprinqua/internal/weather"
)

const (
	minMultiplier = 0.0
	maxMultiplier = 2.5
)

// Calc returns the duration multiplier to apply to a schedule's DurMins.
// data may be nil for methods that don't require weather (manual, monthly).
// The result is clamped to [0.0, 2.5] (0–250%).
// Returns 1.0 (no change) when no method is configured.
func Calc(sw config.SmartWateringConfig, data *weather.DailyData) float64 {
	switch sw.Method {
	case "manual":
		return clamp(sw.ManualPct/100.0, minMultiplier, maxMultiplier)
	case "monthly":
		m := int(time.Now().Month()) - 1 // 0-indexed: Jan=0, Dec=11
		return clamp(sw.EffectiveMonthlyPct(m)/100.0, minMultiplier, maxMultiplier)
	case "zimmerman":
		return zimmerman(sw, data)
	case "eto":
		return eto(sw, data)
	}
	return 1.0
}

// zimmerman implements the Zimmerman ET formula in metric units.
// Returns 1.0 when data is nil (no weather available).
// Watering% = 100 + (T-BT)×7.2×WT/100 + (BH-H)×WH/100 - 7.874×(P-BP)×WP/100
// T and BT in °C, P and BP in mm. Coefficients adapted from the classic °F/inch formula.
// Result clamped to 0–200%.
func zimmerman(sw config.SmartWateringConfig, data *weather.DailyData) float64 {
	if data == nil {
		return 1.0
	}
	avgTempC := (data.TempMaxC + data.TempMinC) / 2.0

	bt := sw.EffectiveZimmBT() // baseline temp °C
	bh := sw.EffectiveZimmBH() // baseline humidity %
	bp := sw.ZimmBP             // baseline precip mm (default 0)
	wt := sw.EffectiveZimmWT() / 100.0
	wh := sw.EffectiveZimmWH() / 100.0
	wp := sw.EffectiveZimmWP() / 100.0

	pct := 100.0 +
		(avgTempC-bt)*7.2*wt +
		(bh-data.HumidityPct)*wh -
		7.874*(data.RainMM-bp)*wp

	return clamp(pct/100.0, 0.0, 2.0)
}

// eto implements the ETo-based adjustment.
// Falls back to Zimmerman when baseline is not yet calculated.
// Watering = max(0, ETo - Rain) / EToBaseline, clamped to [0.0, 2.0].
func eto(sw config.SmartWateringConfig, data *weather.DailyData) float64 {
	if data == nil {
		return 1.0
	}
	if sw.EToBaseline <= 0 {
		// Baseline not calculated yet — fall back to Zimmerman
		return zimmerman(sw, data)
	}
	net := data.EToMM - data.RainMM
	if net < 0 {
		net = 0
	}
	return clamp(net/sw.EToBaseline, 0.0, 2.0)
}

func clamp(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
