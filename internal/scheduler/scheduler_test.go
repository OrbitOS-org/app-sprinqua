package scheduler

import (
	"context"
	"testing"
	"time"

	gpiov26 "github.com/OrbitOS-org/sdk-go/v26/api/gpio_service/v26"
	"github.com/OrbitOS-org/sdk-go/v26/client"
	"google.golang.org/grpc"
	"sprinqua/internal/board"
	"sprinqua/internal/config"
	"sprinqua/internal/history"
	"sprinqua/internal/zone"
)

// fakeGpioClient implements gpiov26.GpioServiceClient without touching real
// hardware, so TurnOn/TurnOff exercise the real engine code path in tests.
type fakeGpioClient struct {
	gpiov26.GpioServiceClient
}

func (f *fakeGpioClient) SetGPIOLevel(ctx context.Context, in *gpiov26.GpioLevelRequest, opts ...grpc.CallOption) (*gpiov26.GpioLevelResponse, error) {
	return &gpiov26.GpioLevelResponse{}, nil
}

func newTestEngine(t *testing.T, zones []config.Zone) *zone.Engine {
	t.Helper()
	b := board.Find("waveshare-3ch")
	if b == nil {
		t.Fatal("waveshare-3ch board not found in registry")
	}
	gpio := client.NewGpioManager(&fakeGpioClient{}, context.Background())
	return zone.New(gpio, b, zones, false)
}

func waitUntilNotRunning(t *testing.T, s *Scheduler, id int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !s.IsRunning(id) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("schedule %d still running after %v", id, timeout)
}

func waitUntilZoneActive(t *testing.T, eng *zone.Engine, zoneID int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if zoneActive(eng.States(), zoneID) {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("zone %d never became active after %v", zoneID, timeout)
}

func TestRunNowThenStopRunTurnsOffActiveZoneAndSkipsRest(t *testing.T) {
	zones := []config.Zone{
		{ID: 1, Name: "Lawn", Channel: 1, Type: "sprinkler", MaxSecs: 1800, Enabled: true},
		{ID: 2, Name: "Flowerbed", Channel: 2, Type: "drip", MaxSecs: 1800, Enabled: true},
	}
	eng := newTestEngine(t, zones)
	cfg := &config.Config{
		Zones: zones,
		Schedules: []config.Schedule{
			{ID: 1, Name: "Morning", Enabled: true, Days: []int{0, 1, 2, 3, 4, 5, 6}, StartTime: "06:00",
				Zones: []config.ProgramZone{{ZoneID: 1, DurMins: 10}, {ZoneID: 2, DurMins: 10}}},
		},
	}
	hist, err := history.New(t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, eng)
	s.SetHistory(hist)

	if s.IsRunning(1) {
		t.Fatal("expected not running before RunNow")
	}

	if err := s.RunNow(1); err != nil {
		t.Fatalf("RunNow: %v", err)
	}
	if !s.IsRunning(1) {
		t.Fatal("expected running immediately after RunNow")
	}

	// Calling RunNow again while mid-run must be rejected.
	if err := s.RunNow(1); err != ErrScheduleRunning {
		t.Fatalf("expected ErrScheduleRunning on second RunNow, got %v", err)
	}

	waitUntilZoneActive(t, eng, 1, time.Second)

	if err := s.StopRun(1); err != nil {
		t.Fatalf("StopRun: %v", err)
	}

	// StopRun must wait for the actual turn-off before returning, so the
	// caller can re-render the UI immediately with the final state — no
	// stale "still running" flash that only self-corrects on the next poll.
	if s.IsRunning(1) {
		t.Fatal("expected IsRunning(1) to already be false right after StopRun returns")
	}
	if _, ok := s.RunningID(); ok {
		t.Fatal("expected no program to be running right after StopRun returns")
	}

	states := eng.States()
	if zoneActive(states, 1) {
		t.Fatalf("expected zone 1 to be turned off after StopRun, states=%+v", states)
	}
	if zoneActive(states, 2) {
		t.Fatalf("expected zone 2 to never start after StopRun, states=%+v", states)
	}

	// Stopping again once it's no longer running must report not-running.
	if err := s.StopRun(1); err != ErrScheduleNotRunning {
		t.Fatalf("expected ErrScheduleNotRunning, got %v", err)
	}
}

func TestRunNowRejectsADifferentProgramWhileOneIsRunning(t *testing.T) {
	zones := []config.Zone{
		{ID: 1, Name: "Lawn", Channel: 1, Type: "sprinkler", MaxSecs: 1800, Enabled: true},
		{ID: 2, Name: "Flowerbed", Channel: 2, Type: "drip", MaxSecs: 1800, Enabled: true},
	}
	eng := newTestEngine(t, zones)
	cfg := &config.Config{
		Zones: zones,
		Schedules: []config.Schedule{
			{ID: 1, Name: "Morning", Enabled: true, Days: []int{0, 1, 2, 3, 4, 5, 6}, StartTime: "06:00",
				Zones: []config.ProgramZone{{ZoneID: 1, DurMins: 10}}},
			{ID: 2, Name: "Evening", Enabled: true, Days: []int{0, 1, 2, 3, 4, 5, 6}, StartTime: "18:00",
				Zones: []config.ProgramZone{{ZoneID: 2, DurMins: 10}}},
		},
	}
	hist, err := history.New(t.TempDir(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	s := New(cfg, eng)
	s.SetHistory(hist)

	if err := s.RunNow(1); err != nil {
		t.Fatalf("RunNow(1): %v", err)
	}
	waitUntilZoneActive(t, eng, 1, time.Second)

	// Starting a *different* program while program 1 is running must be
	// rejected too — only one program may run at a time.
	if err := s.RunNow(2); err != ErrScheduleRunning {
		t.Fatalf("expected ErrScheduleRunning when starting program 2 while program 1 runs, got %v", err)
	}
	if id, ok := s.RunningID(); !ok || id != 1 {
		t.Fatalf("expected RunningID()=1, got id=%d ok=%v", id, ok)
	}
	if s.IsRunning(2) {
		t.Fatal("program 2 must not be marked running")
	}

	// Stop must only affect the program that's actually running.
	if err := s.StopRun(2); err != ErrScheduleNotRunning {
		t.Fatalf("expected ErrScheduleNotRunning when stopping a program that isn't running, got %v", err)
	}
	if err := s.StopRun(1); err != nil {
		t.Fatalf("StopRun(1): %v", err)
	}
	waitUntilNotRunning(t, s, 1, time.Second)

	// Once free, program 2 should be runnable.
	if err := s.RunNow(2); err != nil {
		t.Fatalf("RunNow(2) after program 1 stopped: %v", err)
	}
	if !s.IsRunning(2) {
		t.Fatal("expected program 2 to be running")
	}
	// Clean up: program 2's zone step sleeps for real minutes, so stop it
	// rather than waiting for it to finish naturally.
	if err := s.StopRun(2); err != nil {
		t.Fatalf("StopRun(2) cleanup: %v", err)
	}
	waitUntilNotRunning(t, s, 2, time.Second)
}

func zoneActive(states []zone.State, id int) bool {
	for _, st := range states {
		if st.ID == id {
			return st.Active
		}
	}
	return false
}
