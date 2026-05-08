package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const filename = "config.json"

type Config struct {
	SetupDone bool       `json:"setup_done"`
	Board     string     `json:"board"`
	Zones     []Zone     `json:"zones"`
	MQTT      MQTTConfig `json:"mqtt"`
	Schedules []Schedule `json:"schedules"`
}

type Schedule struct {
	ID        int    `json:"id"`
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

type MQTTConfig struct {
	Enabled  bool   `json:"enabled"`
	Broker   string `json:"broker"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Prefix   string `json:"prefix"`
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
