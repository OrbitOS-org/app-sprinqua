package scheduler

import (
	"fmt"
	"sync"
	"time"

	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"sprinkl/internal/config"
	"sprinkl/internal/zone"
)

const logTag = "scheduler"

type Scheduler struct {
	mu      sync.Mutex
	cfg     *config.Config
	engine  *zone.Engine
	lastRun map[int]time.Time
	stopCh  chan struct{}
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

	if s.engine == nil {
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
		go runSchedule(eng, sched)
	}
}

func runSchedule(eng *zone.Engine, sched config.Schedule) {
	logger.Infof(logTag, "schedule %d zone %d ON for %dmin", sched.ID, sched.ZoneID, sched.DurMins)
	if err := eng.TurnOn(sched.ZoneID); err != nil {
		logger.Warnf(logTag, "schedule %d zone %d ON: %v", sched.ID, sched.ZoneID, err)
		return
	}
	time.Sleep(time.Duration(sched.DurMins) * time.Minute)
	if err := eng.TurnOff(sched.ZoneID); err != nil {
		logger.Warnf(logTag, "schedule %d zone %d OFF: %v", sched.ID, sched.ZoneID, err)
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
