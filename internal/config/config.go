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
	TimeFormat    string     `json:"time_format"`    // "24h" | "12h"
	ExclusiveMode  *bool               `json:"exclusive_mode,omitempty"` // nil = default true
	SmartWatering  SmartWateringConfig `json:"smart_watering,omitempty"`
}

// IsExclusiveMode returns true when at most one zone may be active at a time.
// Defaults to true for new installs (nil field).
func (c *Config) IsExclusiveMode() bool {
	return c.ExclusiveMode == nil || *c.ExclusiveMode
}

type Schedule struct {
	ID        int    `json:"id"`
	Name      string `json:"name,omitempty"`
	ZoneID    int    `json:"zone_id"`
	Days      []int  `json:"days"`       // 0=Sun … 6=Sat (Go time.Weekday)
	StartTime string `json:"start_time"` // "HH:MM"
	DurMins   int    `json:"dur_mins"`
	Enabled   bool   `json:"enabled"`
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
	Enabled         bool    `json:"enabled"`
	Lat             float64 `json:"lat"`
	Lon             float64 `json:"lon"`
	RainThresholdMM float64 `json:"rain_threshold_mm"` // skip if daily rain >= this; 0 → default 2mm
}

func (s SmartWateringConfig) EffectiveThreshold() float64 {
	if s.RainThresholdMM <= 0 {
		return 2.0
	}
	return s.RainThresholdMM
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

func (c *Config) Save(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, filename), data, 0o644)
}
