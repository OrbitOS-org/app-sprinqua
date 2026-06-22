package scheduler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"sprinqua/internal/adjustment"
	"sprinqua/internal/config"
	"sprinqua/internal/history"
	"sprinqua/internal/weather"
	"sprinqua/internal/zone"
)

const logTag = "scheduler"

var (
	// ErrScheduleNotFound is returned by RunNow when no schedule has the given ID.
	ErrScheduleNotFound = errors.New("schedule not found")
	// ErrScheduleRunning is returned by RunNow when some program — this one or
	// any other — is already mid-run. Only one program may run at a time,
	// since they share the same relay hardware and history records.
	ErrScheduleRunning = errors.New("a program is already running")
	// ErrEngineNotReady is returned by RunNow before hardware setup is complete.
	ErrEngineNotReady = errors.New("engine not ready")
	// ErrScheduleNotRunning is returned by StopRun when this program isn't
	// the one currently mid-run.
	ErrScheduleNotRunning = errors.New("schedule not running")
)

type Scheduler struct {
	mu        sync.Mutex
	cfg       *config.Config
	engine    *zone.Engine
	hist      *history.Store
	lastRun   map[int]time.Time
	runningID int                // ID of the schedule currently mid-run, or 0 if none
	cancel    context.CancelFunc // cancels the current run; nil if none is running
	done      chan struct{}      // closed when the current run's goroutine finishes
	stopCh    chan struct{}
	paused    bool
}

func New(cfg *config.Config, eng *zone.Engine) *Scheduler {
	return &Scheduler{
		cfg:     cfg,
		engine:  eng,
		lastRun: make(map[int]time.Time),
		stopCh:  make(chan struct{}),
	}
}

// IsRunning reports whether the given schedule is the one currently mid-run,
// so the UI can reflect this across page reloads/navigation.
func (s *Scheduler) IsRunning(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runningID == id
}

// RunningID returns the ID of the program currently mid-run, if any. Only one
// program can run at a time, so the UI can grey out "Run now" on every other
// program while this one is active.
func (s *Scheduler) RunningID() (id int, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.runningID, s.runningID != 0
}

// StopRun cancels the in-progress run, but only if it's this schedule: the
// zone it currently has on is turned off immediately and any remaining zones
// in its sequence are skipped. It waits (briefly, with a safety timeout) for
// the run's goroutine to actually finish turning the zone off before
// returning, so callers that immediately re-render the UI see the final
// "stopped" state right away instead of a stale "still running" flash that
// only self-corrects on the next poll. Returns ErrScheduleNotRunning if this
// program isn't the one currently mid-run. Not to be confused with Stop(),
// which shuts down the scheduler loop entirely.
func (s *Scheduler) StopRun(id int) error {
	s.mu.Lock()
	if s.runningID != id || s.cancel == nil {
		s.mu.Unlock()
		return ErrScheduleNotRunning
	}
	cancel := s.cancel
	done := s.done
	s.mu.Unlock()

	cancel()
	if done != nil {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			logger.Warnf(logTag, "schedule %d: StopRun timed out waiting for run to finish", id)
		}
	}
	return nil
}

// startRun records id as the (sole) running schedule and returns a context
// that's canceled when StopRun(id) is called. Caller must hold s.mu.
func (s *Scheduler) startRun(id int) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	s.runningID = id
	s.cancel = cancel
	s.done = make(chan struct{})
	return ctx
}

func (s *Scheduler) finishRun(id int) {
	s.mu.Lock()
	if s.runningID == id {
		s.runningID = 0
		s.cancel = nil
		if s.done != nil {
			close(s.done)
			s.done = nil
		}
	}
	s.mu.Unlock()
}

// RunNow starts a program's zone sequence immediately, ignoring its enabled
// flag, days, start time, and any Smart Watering adjustment — each zone runs
// for exactly its configured duration. Intended for a manual "Run now" /
// "Play" action so the user can test a program without waiting for its
// scheduled time. Returns ErrScheduleRunning if any program (this one or a
// different one) is already mid-run — only one program runs at a time.
func (s *Scheduler) RunNow(id int) error {
	s.mu.Lock()
	if s.engine == nil {
		s.mu.Unlock()
		return ErrEngineNotReady
	}
	if s.runningID != 0 {
		s.mu.Unlock()
		return ErrScheduleRunning
	}
	var sched config.Schedule
	found := false
	for _, sc := range s.cfg.Schedules {
		if sc.ID == id {
			sched = sc
			found = true
			break
		}
	}
	if !found {
		s.mu.Unlock()
		return ErrScheduleNotFound
	}
	ctx := s.startRun(id)
	eng := s.engine
	hist := s.hist
	s.mu.Unlock()

	logger.Infof(logTag, "schedule %d: manual run-now", sched.ID)
	go func() {
		defer s.finishRun(sched.ID)
		runZoneSteps(ctx, eng, sched, hist, 1.0, history.Manual)
	}()
	return nil
}

func (s *Scheduler) SetEngine(eng *zone.Engine) {
	s.mu.Lock()
	s.engine = eng
	s.mu.Unlock()
}

func (s *Scheduler) SetHistory(h *history.Store) {
	s.mu.Lock()
	s.hist = h
	s.mu.Unlock()
}

// SetPaused pauses or resumes the internal scheduler.
// When paused, no scheduled runs fire (HA-managed passive mode).
func (s *Scheduler) SetPaused(v bool) {
	s.mu.Lock()
	s.paused = v
	s.mu.Unlock()
	if v {
		logger.Infof(logTag, "scheduler paused — HA passive mode active")
	} else {
		logger.Infof(logTag, "scheduler resumed — standalone active mode")
	}
}

func (s *Scheduler) Start() {
	go s.loop()
}

func (s *Scheduler) Stop() {
	close(s.stopCh)
}

func (s *Scheduler) loop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	s.tick(time.Now())
	for {
		select {
		case t := <-ticker.C:
			s.tick(t)
		case <-s.stopCh:
			return
		}
	}
}

func (s *Scheduler) tick(now time.Time) {
	minute := now.Truncate(time.Minute)
	weekday := int(now.Weekday())
	hhmm := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.engine == nil || s.paused {
		return
	}

	for _, sched := range s.cfg.Schedules {
		if !sched.Enabled || sched.TotalMins() <= 0 {
			continue
		}
		if !dayInList(sched.Days, weekday) {
			continue
		}
		if sched.StartTime != hhmm {
			continue
		}
		if last, ok := s.lastRun[sched.ID]; ok && !last.Before(minute) {
			continue
		}
		if s.runningID != 0 {
			continue // some program (clock-triggered or manual) is already mid-run
		}
		s.lastRun[sched.ID] = minute
		ctx := s.startRun(sched.ID)
		eng := s.engine
		hist := s.hist
		sw := s.cfg.SmartWatering
		go func(sc config.Schedule) {
			defer s.finishRun(sc.ID)
			runSchedule(ctx, eng, sc, hist, sw)
		}(sched)
	}
}

func runSchedule(ctx context.Context, eng *zone.Engine, sched config.Schedule, hist *history.Store, sw config.SmartWateringConfig) {
	if sw.Enabled && sw.SkipEnabled && sw.Lat != 0 {
		res, err := weather.FetchToday(sw.Lat, sw.Lon)
		if err != nil {
			logger.Warnf(logTag, "schedule %d: weather fetch failed: %v", sched.ID, err)
		} else {
			if res.RainMM >= sw.EffectiveThreshold() {
				logger.Infof(logTag, "schedule %d skipped: rain %.1fmm >= %.1fmm", sched.ID, res.RainMM, sw.EffectiveThreshold())
				skipAll(hist, sched, history.SkipRain)
				return
			}
			if sw.FrostThresholdC > 0 && res.TempMinC < sw.FrostThresholdC {
				logger.Infof(logTag, "schedule %d skipped: min temp %.1f°C < %.1f°C", sched.ID, res.TempMinC, sw.FrostThresholdC)
				skipAll(hist, sched, history.SkipFrost)
				return
			}
		}
	}

	mult := 1.0
	if sched.SmartWatering && sw.Enabled && sw.Method != "" {
		var dailyData *weather.DailyData
		if sw.Method == "zimmerman" || sw.Method == "eto" {
			if d, err := weather.FetchYesterday(sw.Lat, sw.Lon); err != nil {
				logger.Warnf(logTag, "schedule %d: yesterday weather fetch failed: %v", sched.ID, err)
			} else {
				dailyData = d
			}
		}
		mult = adjustment.Calc(sw, dailyData)
		logger.Infof(logTag, "schedule %d: adjustment method=%s mult=%.2f", sched.ID, sw.Method, mult)
	}

	runZoneSteps(ctx, eng, sched, hist, mult, history.Schedule)
}

// runZoneSteps turns each zone in the program on/off in sequence, applying
// the given duration multiplier and recording history under trigger. If ctx
// is canceled (StopRun) while a zone is active, that zone is turned off
// immediately and any remaining zones in the sequence are skipped.
func runZoneSteps(ctx context.Context, eng *zone.Engine, sched config.Schedule, hist *history.Store, mult float64, trigger history.Trigger) {
	for i, step := range sched.Zones {
		if ctx.Err() != nil {
			logger.Infof(logTag, "schedule %d: stopped, skipping remaining zones", sched.ID)
			return
		}
		dur := int(math.Round(float64(step.DurMins) * mult))
		if dur <= 0 {
			logger.Infof(logTag, "schedule %d zone %d: adjusted duration=0, skipping", sched.ID, step.ZoneID)
			if hist != nil {
				hist.Skip(step.ZoneID, trigger)
			}
		} else {
			logger.Infof(logTag, "schedule %d zone %d ON for %dmin", sched.ID, step.ZoneID, dur)
			if err := eng.TurnOn(step.ZoneID); err != nil {
				logger.Warnf(logTag, "schedule %d zone %d ON: %v", sched.ID, step.ZoneID, err)
				continue
			}
			if hist != nil {
				hist.Start(step.ZoneID, trigger)
			}
			stopped := false
			select {
			case <-time.After(time.Duration(dur) * time.Minute):
			case <-ctx.Done():
				stopped = true
			}
			if err := eng.TurnOff(step.ZoneID); err != nil {
				logger.Warnf(logTag, "schedule %d zone %d OFF: %v", sched.ID, step.ZoneID, err)
			}
			if hist != nil {
				hist.Stop(step.ZoneID)
			}
			if stopped {
				logger.Infof(logTag, "schedule %d: stopped during zone %d, skipping remaining zones", sched.ID, step.ZoneID)
				return
			}
		}

		if i < len(sched.Zones)-1 && step.SoakAfterMins > 0 {
			logger.Infof(logTag, "schedule %d: soak %dmin after zone %d", sched.ID, step.SoakAfterMins, step.ZoneID)
			select {
			case <-time.After(time.Duration(step.SoakAfterMins) * time.Minute):
			case <-ctx.Done():
				logger.Infof(logTag, "schedule %d: stopped during soak after zone %d", sched.ID, step.ZoneID)
				return
			}
		}
	}
	logger.Infof(logTag, "schedule %d complete", sched.ID)
}

// skipAll records a weather-driven skip for every zone in the program.
func skipAll(hist *history.Store, sched config.Schedule, trigger history.Trigger) {
	if hist == nil {
		return
	}
	for _, step := range sched.Zones {
		hist.Skip(step.ZoneID, trigger)
	}
}

// NextRunFor returns the next scheduled run time for the given schedule.
func NextRunFor(sched config.Schedule) *time.Time {
	if !sched.Enabled || sched.TotalMins() <= 0 || len(sched.Days) == 0 {
		return nil
	}
	now := time.Now()
	for d := 0; d <= 7; d++ {
		candidate := now.AddDate(0, 0, d)
		if !dayInList(sched.Days, int(candidate.Weekday())) {
			continue
		}
		var h, m int
		fmt.Sscanf(sched.StartTime, "%d:%d", &h, &m)
		t := time.Date(candidate.Year(), candidate.Month(), candidate.Day(), h, m, 0, 0, candidate.Location())
		if t.After(now) {
			return &t
		}
	}
	return nil
}

// NextRunGlobal returns the enabled schedule with the earliest upcoming run, if any.
func NextRunGlobal(schedules []config.Schedule) (config.Schedule, time.Time, bool) {
	var best config.Schedule
	var bestTime time.Time
	found := false
	for _, sc := range schedules {
		t := NextRunFor(sc)
		if t == nil {
			continue
		}
		if !found || t.Before(bestTime) {
			best = sc
			bestTime = *t
			found = true
		}
	}
	return best, bestTime, found
}

func dayInList(days []int, day int) bool {
	for _, d := range days {
		if d == day {
			return true
		}
	}
	return false
}
