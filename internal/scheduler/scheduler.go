package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"sprinqua/internal/config"
	"sprinqua/internal/history"
	"sprinqua/internal/weather"
	"sprinqua/internal/zone"
)

const logTag = "scheduler"

type Scheduler struct {
	mu      sync.Mutex
	cfg     *config.Config
	engine  *zone.Engine
	hist    *history.Store
	lastRun map[int]time.Time
	stopCh  chan struct{}
	paused  bool
}

func New(cfg *config.Config, eng *zone.Engine) *Scheduler {
	return &Scheduler{
		cfg:     cfg,
		engine:  eng,
		lastRun: make(map[int]time.Time),
		stopCh:  make(chan struct{}),
	}
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
		if !sched.Enabled || sched.DurMins <= 0 {
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
		s.lastRun[sched.ID] = minute
		eng := s.engine
		hist := s.hist
		sw := s.cfg.SmartWatering
		go runSchedule(eng, sched, hist, sw)
	}
}

func runSchedule(eng *zone.Engine, sched config.Schedule, hist *history.Store, sw config.SmartWateringConfig) {
	if sw.Enabled && sw.Lat != 0 {
		res, err := weather.FetchToday(sw.Lat, sw.Lon)
		if err != nil {
			logger.Warnf(logTag, "schedule %d: weather fetch failed: %v", sched.ID, err)
		} else if res.RainMM >= sw.EffectiveThreshold() {
			logger.Infof(logTag, "schedule %d skipped: rain %.1fmm >= threshold %.1fmm", sched.ID, res.RainMM, sw.EffectiveThreshold())
			if hist != nil {
				hist.Skip(sched.ZoneID, history.Schedule)
			}
			return
		}
	}

	logger.Infof(logTag, "schedule %d zone %d ON for %dmin", sched.ID, sched.ZoneID, sched.DurMins)
	if err := eng.TurnOn(sched.ZoneID); err != nil {
		logger.Warnf(logTag, "schedule %d zone %d ON: %v", sched.ID, sched.ZoneID, err)
		return
	}
	if hist != nil {
		hist.Start(sched.ZoneID, history.Schedule)
	}
	time.Sleep(time.Duration(sched.DurMins) * time.Minute)
	if err := eng.TurnOff(sched.ZoneID); err != nil {
		logger.Warnf(logTag, "schedule %d zone %d OFF: %v", sched.ID, sched.ZoneID, err)
	}
	if hist != nil {
		hist.Stop(sched.ZoneID)
	}
	logger.Infof(logTag, "schedule %d complete", sched.ID)
}

// NextRunFor returns the next scheduled run time for the given schedule.
func NextRunFor(sched config.Schedule) *time.Time {
	if !sched.Enabled || sched.DurMins <= 0 || len(sched.Days) == 0 {
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

func dayInList(days []int, day int) bool {
	for _, d := range days {
		if d == day {
			return true
		}
	}
	return false
}
