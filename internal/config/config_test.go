package config

import (
	"encoding/json"
	"testing"
)

func TestScheduleUnmarshalLegacySingleZone(t *testing.T) {
	data := []byte(`{
		"id": 1,
		"name": "Morning",
		"zone_id": 2,
		"dur_mins": 15,
		"days": [1,2,3],
		"start_time": "06:00",
		"enabled": true
	}`)
	var sc Schedule
	if err := json.Unmarshal(data, &sc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(sc.Zones) != 1 || sc.Zones[0].ZoneID != 2 || sc.Zones[0].DurMins != 15 {
		t.Fatalf("expected one synthesized zone {2,15}, got %+v", sc.Zones)
	}
	if sc.TotalMins() != 15 {
		t.Fatalf("expected TotalMins=15, got %d", sc.TotalMins())
	}
}

func TestScheduleUnmarshalMultiZoneRoundTrip(t *testing.T) {
	sc := Schedule{
		ID:        1,
		Name:      "Morning",
		Zones:     []ProgramZone{{ZoneID: 1, DurMins: 5, SoakAfterMins: 3}, {ZoneID: 2, DurMins: 10}},
		Days:      []int{1, 2, 3},
		StartTime: "06:00",
		Enabled:   true,
	}
	data, err := json.Marshal(sc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got Schedule
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Zones) != 2 || got.Zones[0] != sc.Zones[0] || got.Zones[1] != sc.Zones[1] {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", got.Zones, sc.Zones)
	}
	if got.TotalMins() != 15 {
		t.Fatalf("expected TotalMins=15, got %d", got.TotalMins())
	}
	if got.TotalRunMins() != 18 {
		t.Fatalf("expected TotalRunMins=18, got %d", got.TotalRunMins())
	}
}

func TestTotalRunMinsSoakOnlyBetweenSteps(t *testing.T) {
	sc := Schedule{
		Zones: []ProgramZone{
			{ZoneID: 1, DurMins: 10, SoakAfterMins: 5},
			{ZoneID: 2, DurMins: 8, SoakAfterMins: 99}, // last step soak ignored
		},
	}
	if sc.TotalRunMins() != 23 {
		t.Fatalf("expected TotalRunMins=23, got %d", sc.TotalRunMins())
	}
}
