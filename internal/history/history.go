package history

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"

	"sprinqua/internal/config"
)

type Trigger string

const (
	Manual   Trigger = "manual"
	Schedule Trigger = "schedule"
	Pulse    Trigger = "pulse"
	MQTT     Trigger = "mqtt"

	maxEntries = 500
)

type Entry struct {
	ID        int        `json:"id"`
	ZoneID    int        `json:"zone_id"`
	ZoneName  string     `json:"zone_name"`
	Channel   int        `json:"channel"`
	Trigger   Trigger    `json:"trigger"`
	StartedAt time.Time  `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	DurSecs   int        `json:"dur_secs,omitempty"`
	Skipped   bool       `json:"skipped,omitempty"`
}

type Store struct {
	mu      sync.Mutex
	dataDir string
	cfg     *config.Config
	entries []Entry
	nextID  int
}

func New(dataDir string, cfg *config.Config) (*Store, error) {
	s := &Store{dataDir: dataDir, cfg: cfg}
	if err := s.load(); err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(filepath.Join(s.dataDir, "history.json"))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, &s.entries); err != nil {
		return err
	}
	for _, e := range s.entries {
		if e.ID >= s.nextID {
			s.nextID = e.ID + 1
		}
	}
	return nil
}

func (s *Store) save() {
	data, _ := json.MarshalIndent(s.entries, "", "  ")
	_ = os.WriteFile(filepath.Join(s.dataDir, "history.json"), data, 0644)
}

// Start records the beginning of a zone activation.
// If a previous activation for the same zone is still open, it is closed first.
func (s *Store) Start(zoneID int, trig Trigger) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()

	// Close any dangling open entry for this zone.
	for i := len(s.entries) - 1; i >= 0; i-- {
		if s.entries[i].ZoneID == zoneID && s.entries[i].EndedAt == nil {
			s.entries[i].EndedAt = &now
			s.entries[i].DurSecs = int(now.Sub(s.entries[i].StartedAt).Seconds())
			break
		}
	}

	var name string
	var ch int
	for _, z := range s.cfg.Zones {
		if z.ID == zoneID {
			name = z.Name
			ch = z.Channel
			break
		}
	}

	s.entries = append(s.entries, Entry{
		ID:        s.nextID,
		ZoneID:    zoneID,
		ZoneName:  name,
		Channel:   ch,
		Trigger:   trig,
		StartedAt: now,
	})
	s.nextID++
	s.trim()
	s.save()
}

// Skip records a schedule that was skipped (e.g., due to Smart Watering rain forecast).
func (s *Store) Skip(zoneID int, trig Trigger) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	var name string
	var ch int
	for _, z := range s.cfg.Zones {
		if z.ID == zoneID {
			name = z.Name
			ch = z.Channel
			break
		}
	}

	s.entries = append(s.entries, Entry{
		ID:        s.nextID,
		ZoneID:    zoneID,
		ZoneName:  name,
		Channel:   ch,
		Trigger:   trig,
		StartedAt: now,
		EndedAt:   &now,
		Skipped:   true,
	})
	s.nextID++
	s.trim()
	s.save()
}

// Stop records the end of a zone activation.
func (s *Store) Stop(zoneID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for i := len(s.entries) - 1; i >= 0; i-- {
		if s.entries[i].ZoneID == zoneID && s.entries[i].EndedAt == nil {
			s.entries[i].EndedAt = &now
			s.entries[i].DurSecs = int(now.Sub(s.entries[i].StartedAt).Seconds())
			break
		}
	}
	s.save()
}

// Recent returns up to n entries, newest first. Pass n=0 for all.
func (s *Store) Recent(n int) []Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]Entry, len(s.entries))
	copy(result, s.entries)
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	if n > 0 && len(result) > n {
		return result[:n]
	}
	return result
}

// Clear removes all entries and deletes history.json from disk.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = nil
	s.nextID = 0
	_ = os.Remove(filepath.Join(s.dataDir, "history.json"))
}

func (s *Store) trim() {
	if len(s.entries) <= maxEntries {
		return
	}
	remove := len(s.entries) - maxEntries
	out := s.entries[:0]
	removed := 0
	for _, e := range s.entries {
		if removed < remove && e.EndedAt != nil {
			removed++
			continue
		}
		out = append(out, e)
	}
	s.entries = out
}
