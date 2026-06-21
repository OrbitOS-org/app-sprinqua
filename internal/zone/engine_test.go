package zone

import (
	"context"
	"testing"

	gpiov26 "github.com/OrbitOS-org/sdk-go/v26/api/gpio_service/v26"
	"github.com/OrbitOS-org/sdk-go/v26/client"
	"google.golang.org/grpc"
	"sprinqua/internal/board"
	"sprinqua/internal/config"
	"sprinqua/internal/history"
)

// fakeGpioClient implements gpiov26.GpioServiceClient without touching real
// hardware, so TurnOn/TurnOff exercise the real relay-write code path.
type fakeGpioClient struct {
	gpiov26.GpioServiceClient
}

func (f *fakeGpioClient) SetGPIOLevel(ctx context.Context, in *gpiov26.GpioLevelRequest, opts ...grpc.CallOption) (*gpiov26.GpioLevelResponse, error) {
	return &gpiov26.GpioLevelResponse{}, nil
}

func TestExclusiveModeTurnOnClosesPreviousZoneHistory(t *testing.T) {
	zones := []config.Zone{
		{ID: 1, Name: "Lawn", Channel: 1, Type: "sprinkler", MaxSecs: 1800, Enabled: true},
		{ID: 2, Name: "Flowerbed", Channel: 2, Type: "drip", MaxSecs: 1800, Enabled: true},
	}
	b := board.Find("waveshare-3ch")
	if b == nil {
		t.Fatal("waveshare-3ch board not found in registry")
	}
	gpio := client.NewGpioManager(&fakeGpioClient{}, context.Background())
	eng := New(gpio, b, zones, true) // exclusive mode, matches the reported bug

	cfg := &config.Config{Zones: zones}
	hist, err := history.New(t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	eng.SetHistory(hist)

	// Simulate the manual dashboard flow: click ON for zone 1...
	if err := eng.TurnOn(1); err != nil {
		t.Fatalf("TurnOn(1): %v", err)
	}
	hist.Start(1, history.Manual)

	// ...then click ON for zone 2 instead of Stop. Exclusive mode makes the
	// engine turn zone 1 off internally as a side effect.
	if err := eng.TurnOn(2); err != nil {
		t.Fatalf("TurnOn(2): %v", err)
	}
	hist.Start(2, history.Manual)

	states := eng.States()
	if zoneActive(states, 1) {
		t.Fatalf("expected zone 1 to be off after exclusive cutoff, states=%+v", states)
	}
	if !zoneActive(states, 2) {
		t.Fatalf("expected zone 2 to be on, states=%+v", states)
	}

	// The bug: zone 1's history entry must be closed (EndedAt set), not left
	// dangling "active" forever just because exclusive mode cut it off
	// instead of an explicit Stop.
	entries := hist.Recent(0)
	var zone1Entry, zone2Entry *history.Entry
	for i := range entries {
		switch entries[i].ZoneID {
		case 1:
			zone1Entry = &entries[i]
		case 2:
			zone2Entry = &entries[i]
		}
	}
	if zone1Entry == nil {
		t.Fatal("expected a history entry for zone 1")
	}
	if zone1Entry.EndedAt == nil {
		t.Fatal("expected zone 1's history entry to be closed (EndedAt set) after exclusive cutoff, but it's still open")
	}
	if zone2Entry == nil || zone2Entry.EndedAt != nil {
		t.Fatalf("expected zone 2's history entry to still be open, got %+v", zone2Entry)
	}
}

func zoneActive(states []State, id int) bool {
	for _, st := range states {
		if st.ID == id {
			return st.Active
		}
	}
	return false
}
