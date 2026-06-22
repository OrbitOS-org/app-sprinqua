package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const filename = "config.json"

type Config struct {
	SetupDone     bool       `json:"setup_done"`
	Board         string     `json:"board"`
	Zones         []Zone     `json:"zones"`
	MQTT          MQTTConfig `json:"mqtt"`
	Schedules     []Schedule `json:"schedules"`
	TimeFormat    string     `json:"time_format"` // "24h" | "12h"
	SmartWatering SmartWateringConfig `json:"smart_watering,omitempty"`
	WinterMode    bool                `json:"winter_mode,omitempty"`
}

// IsExclusiveMode reports whether at most one zone may be active at a time.
// Always true — no longer user-configurable, since running more than one
// zone at once isn't a supported/sane setup for this hardware.
func (c *Config) IsExclusiveMode() bool {
	return true
}

type Schedule struct {
	ID            int           `json:"id"`
	Name          string        `json:"name,omitempty"`
	Zones         []ProgramZone `json:"zones"`
	Days          []int         `json:"days"`       // 0=Sun … 6=Sat (Go time.Weekday)
	StartTime     string        `json:"start_time"` // "HH:MM"
	Enabled       bool          `json:"enabled"`
	SmartWatering bool          `json:"smart_watering,omitempty"`
}

// ProgramZone is one step in a multi-zone program: a zone and how long it
// runs for, in the order zones are listed within the Schedule.
type ProgramZone struct {
	ZoneID  int `json:"zone_id"`
	DurMins int `json:"dur_mins"`
}

// TotalMins returns the combined duration of every zone step in the program.
func (s Schedule) TotalMins() int {
	total := 0
	for _, z := range s.Zones {
		total += z.DurMins
	}
	return total
}

// UnmarshalJSON upgrades legacy single-zone schedules (top-level "zone_id" +
// "dur_mins") to the current []ProgramZone shape so old config.json files
// keep loading after the multi-zone program change.
func (s *Schedule) UnmarshalJSON(data []byte) error {
	var raw struct {
		ID            int           `json:"id"`
		Name          string        `json:"name,omitempty"`
		Zones         []ProgramZone `json:"zones"`
		ZoneID        int           `json:"zone_id"`
		DurMins       int           `json:"dur_mins"`
		Days          []int         `json:"days"`
		StartTime     string        `json:"start_time"`
		Enabled       bool          `json:"enabled"`
		SmartWatering bool          `json:"smart_watering,omitempty"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*s = Schedule{
		ID:            raw.ID,
		Name:          raw.Name,
		Zones:         raw.Zones,
		Days:          raw.Days,
		StartTime:     raw.StartTime,
		Enabled:       raw.Enabled,
		SmartWatering: raw.SmartWatering,
	}
	if len(s.Zones) == 0 && raw.ZoneID != 0 {
		s.Zones = []ProgramZone{{ZoneID: raw.ZoneID, DurMins: raw.DurMins}}
	}
	return nil
}

func (c *Config) NextScheduleID() int {
	max := 0
	for _, s := range c.Schedules {
		if s.ID > max {
			max = s.ID
		}
	}
	return max + 1
}

func (c *Config) ZoneMap() map[int]Zone {
	m := make(map[int]Zone, len(c.Zones))
	for _, z := range c.Zones {
		m[z.ID] = z
	}
	return m
}

type Zone struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Channel int    `json:"channel"`
	Type    string `json:"type"`     // drip | sprinkler | mist
	MaxSecs int    `json:"max_secs"` // safety auto-off in seconds
	Enabled bool   `json:"enabled"`
}

type SmartWateringConfig struct {
	Enabled         bool        `json:"enabled"`
	SkipEnabled     bool        `json:"skip_enabled"`      // skip entire session on rain/frost; migrated to true for existing configs
	Lat             float64     `json:"lat"`
	Lon             float64     `json:"lon"`
	RainThresholdMM float64     `json:"rain_threshold_mm"` // skip if daily rain >= this; 0 → default 2mm
	FrostThresholdC float64     `json:"frost_threshold_c,omitempty"` // skip if today's min temp < this; 0 = disabled
	Method          string      `json:"method,omitempty"`  // "" | "manual" | "monthly" | "zimmerman" | "eto"
	ManualPct       float64     `json:"manual_pct,omitempty"`
	MonthlyPct      [12]float64 `json:"monthly_pct"`

	// Zimmerman parameters (metric: °C and mm)
	ZimmBT float64 `json:"zimm_bt,omitempty"` // baseline temperature °C, default 21
	ZimmBH float64 `json:"zimm_bh,omitempty"` // baseline humidity %, default 30
	ZimmBP float64 `json:"zimm_bp,omitempty"` // baseline precipitation mm, default 0
	ZimmWT float64 `json:"zimm_wt,omitempty"` // temperature weight %, default 100
	ZimmWH float64 `json:"zimm_wh,omitempty"` // humidity weight %, default 100
	ZimmWP float64 `json:"zimm_wp,omitempty"` // precipitation weight %, default 100

	// ETo parameters
	Altitude                float64 `json:"altitude,omitempty"`
	EToBaseline             float64 `json:"eto_baseline,omitempty"`              // mean daily ETo mm/day over last 12 months
	EToBaselineCalculatedAt string  `json:"eto_baseline_calculated_at,omitempty"` // ISO date of last calculation
}

// UnmarshalJSON migrates configs created before SkipEnabled was introduced.
// Old configs had skip behavior implicit whenever Enabled was true, so we
// default SkipEnabled=true when the key is absent from the JSON object.
func (s *SmartWateringConfig) UnmarshalJSON(data []byte) error {
	type Alias SmartWateringConfig
	if err := json.Unmarshal(data, (*Alias)(s)); err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil // already parsed above; ignore map error
	}
	if _, present := raw["skip_enabled"]; !present && s.Enabled {
		s.SkipEnabled = true
	}
	return nil
}

func (s SmartWateringConfig) EffectiveThreshold() float64 {
	if s.RainThresholdMM <= 0 {
		return 2.0
	}
	return s.RainThresholdMM
}

// EffectiveMonthlyPct returns the configured percentage for month m (0=Jan),
// defaulting to 100 when the slot is uninitialised.
func (s SmartWateringConfig) EffectiveMonthlyPct(m int) float64 {
	if m < 0 || m > 11 {
		return 100
	}
	if s.MonthlyPct[m] == 0 {
		return 100
	}
	return s.MonthlyPct[m]
}

// Zimmerman defaults — returns the stored value or the standard default when 0.
func (s SmartWateringConfig) EffectiveZimmBT() float64 { return effective(s.ZimmBT, 21) }
func (s SmartWateringConfig) EffectiveZimmBH() float64 { return effective(s.ZimmBH, 30) }
func (s SmartWateringConfig) EffectiveZimmWT() float64 { return effective(s.ZimmWT, 100) }
func (s SmartWateringConfig) EffectiveZimmWH() float64 { return effective(s.ZimmWH, 100) }
func (s SmartWateringConfig) EffectiveZimmWP() float64 { return effective(s.ZimmWP, 100) }

func effective(v, def float64) float64 {
	if v == 0 {
		return def
	}
	return v
}

type MQTTConfig struct {
	Enabled  bool   `json:"enabled"`
	Mode     string `json:"mode,omitempty"` // "active" (default) | "passive"
	Broker   string `json:"broker"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Prefix   string `json:"prefix"`
}

// IsPassive returns true when HA has full control and internal schedules are paused.
func (m MQTTConfig) IsPassive() bool {
	return m.Enabled && m.Mode == "passive"
}

func Load(dataDir string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, filename))
	if os.IsNotExist(err) {
		return &Config{}, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	return &cfg, json.Unmarshal(data, &cfg)
}

// Save writes the config atomically: it writes to a temp file first, then
// renames it over the real file. This prevents a corrupted config.json if
// the process is killed mid-write (common on Raspberry Pi with SD cards).
func (c *Config) Save(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dataDir, filename+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dataDir, filename))
}
