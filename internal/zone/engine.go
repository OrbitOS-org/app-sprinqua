package zone

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"sprinqua/internal/board"
	"sprinqua/internal/config"
	"sprinqua/internal/history"
)

const logTag = "zone"

// State is the public read-only snapshot of a zone.
type State struct {
	ID        int
	Name      string
	Channel   int
	Type      string
	Active    bool
	StartedAt time.Time
	MaxSecs   int
	PulseSecs int // effective pulse duration in seconds (for dashboard button)
}

func (s State) SecondsActive() int {
	if !s.Active {
		return 0
	}
	return int(time.Since(s.StartedAt).Seconds())
}

func (s State) FormatActive() string {
	secs := s.SecondsActive()
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	return fmt.Sprintf("%dm %ds", secs/60, secs%60)
}

func (s State) TypeIcon() string {
	switch s.Type {
	case "sprinkler":
		return "💦"
	case "mist":
		return "🌫️"
	default:
		return "🌱"
	}
}

type entry struct {
	cfg       config.Zone
	active    bool
	startedAt time.Time
	cancel    context.CancelFunc
}

// Engine manages all zones and their relay state, via whichever RelayDriver
// the board's ChannelManager opens (GPIO or I2C).
type Engine struct {
	mu            sync.Mutex
	chMgr         *board.ChannelManager
	driver        board.RelayDriver // opened in Init()
	board         *board.Board
	zones         map[int]*entry
	exclusive     bool // when true, activating a zone turns off all others first
	hist          *history.Store
	OnStateChange func(zoneID int, on bool)
}

// SetHistory wires the history store so the engine can close a zone's history
// entry itself whenever IT decides to turn a zone off — exclusive-mode
// preemption (TurnOn cutting off the previously active zone) and the safety
// timer auto-off both happen with no external caller around to do it,
// otherwise leaving that zone stuck "active" in the history view.
func (e *Engine) SetHistory(h *history.Store) {
	e.mu.Lock()
	e.hist = h
	e.mu.Unlock()
}

func New(chMgr *board.ChannelManager, b *board.Board, zones []config.Zone, exclusive bool) *Engine {
	e := &Engine{
		chMgr:     chMgr,
		board:     b,
		zones:     make(map[int]*entry),
		exclusive: exclusive,
	}
	for _, z := range zones {
		if z.Enabled {
			zc := z
			e.zones[z.ID] = &entry{cfg: zc}
		}
	}
	return e
}

// Init opens the board's relay driver, then sets all relays to OFF.
func (e *Engine) Init() {
	drv, err := e.chMgr.Open(e.board)
	if err != nil {
		logger.Errorf(logTag, "open relay driver for board %q: %v", e.board.ID, err)
	} else {
		e.driver = drv
	}
	for _, en := range e.zones {
		if err := e.setChannel(en.cfg.Channel, false); err != nil {
			logger.Warnf(logTag, "zone %d init OFF: %v", en.cfg.ID, err)
		}
	}
}

// setChannel switches one relay channel on or off via the board's driver.
func (e *Engine) setChannel(channel int, on bool) error {
	if e.driver == nil {
		return fmt.Errorf("relay driver not ready")
	}
	return e.driver.SetChannel(channel, on)
}

// turnOffLocked turns off a zone without acquiring the mutex (must be held by caller).
func (e *Engine) turnOffLocked(id int) error {
	en, ok := e.zones[id]
	if !ok {
		return fmt.Errorf("zone %d not found", id)
	}
	if en.cancel != nil {
		en.cancel()
		en.cancel = nil
	}
	if err := e.setChannel(en.cfg.Channel, false); err != nil {
		return fmt.Errorf("zone %d OFF: %w", id, err)
	}
	en.active = false
	logger.Infof(logTag, "zone %d OFF", id)
	if e.hist != nil {
		e.hist.Stop(id)
	}
	if e.OnStateChange != nil {
		cb := e.OnStateChange
		go cb(id, false)
	}
	return nil
}

func (e *Engine) TurnOn(id int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	en, ok := e.zones[id]
	if !ok {
		return fmt.Errorf("zone %d not found", id)
	}

	// Exclusive mode: turn off every other active zone before activating this one.
	if e.exclusive {
		for oid, oen := range e.zones {
			if oid != id && oen.active {
				if err := e.turnOffLocked(oid); err != nil {
					logger.Warnf(logTag, "exclusive off zone %d: %v", oid, err)
				}
			}
		}
	}

	// Cancel any previous safety timer for this zone.
	if en.cancel != nil {
		en.cancel()
		en.cancel = nil
	}

	if err := e.setChannel(en.cfg.Channel, true); err != nil {
		return fmt.Errorf("zone %d ON: %w", id, err)
	}
	en.active = true
	en.startedAt = time.Now()
	if e.OnStateChange != nil {
		cb := e.OnStateChange
		go cb(id, true)
	}

	maxSecs := en.cfg.MaxSecs
	if maxSecs <= 0 {
		maxSecs = 30 * 60
	}

	ctx, cancel := context.WithCancel(context.Background())
	en.cancel = cancel

	go func() {
		select {
		case <-time.After(time.Duration(maxSecs) * time.Second):
			logger.Infof(logTag, "zone %d: safety timer expired", id)
			_ = e.TurnOff(id)
		case <-ctx.Done():
		}
	}()

	logger.Infof(logTag, "zone %d ON (safety timer: %ds)", id, maxSecs)
	return nil
}

func (e *Engine) TurnOff(id int) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.turnOffLocked(id)
}

// Pulse turns a zone ON and auto-offs after secs seconds.
func (e *Engine) Pulse(id, secs int) error {
	if err := e.TurnOn(id); err != nil {
		return err
	}
	go func() {
		time.Sleep(time.Duration(secs) * time.Second)
		_ = e.TurnOff(id)
	}()
	return nil
}

// TestChannel pulses a raw relay channel (used during the setup wizard).
// It bypasses the zone map and drives the channel directly.
func (e *Engine) TestChannel(channel, secs int) error {
	if err := e.setChannel(channel, true); err != nil {
		return err
	}
	go func() {
		time.Sleep(time.Duration(secs) * time.Second)
		_ = e.setChannel(channel, false)
	}()
	logger.Infof(logTag, "test channel %d for %ds", channel, secs)
	return nil
}

// SetZones applies an updated zone configuration list. Zones that became
// disabled are turned off and dropped; newly enabled zones are added and
// initialised to OFF; existing zones get their config (name, type, max
// duration) refreshed in place without affecting their active state.
func (e *Engine) SetZones(zones []config.Zone) {
	e.mu.Lock()
	defer e.mu.Unlock()

	want := make(map[int]config.Zone, len(zones))
	for _, z := range zones {
		if z.Enabled {
			want[z.ID] = z
		}
	}

	for id := range e.zones {
		if _, ok := want[id]; !ok {
			if e.zones[id].active {
				_ = e.turnOffLocked(id)
			}
			delete(e.zones, id)
		}
	}

	for id, z := range want {
		if en, ok := e.zones[id]; ok {
			en.cfg = z
			continue
		}
		e.zones[id] = &entry{cfg: z}
		if err := e.setChannel(z.Channel, false); err != nil {
			logger.Warnf(logTag, "zone %d init OFF: %v", id, err)
		}
	}
}

// States returns a sorted snapshot of all zone states.
func (e *Engine) States() []State {
	e.mu.Lock()
	defer e.mu.Unlock()

	out := make([]State, 0, len(e.zones))
	for _, en := range e.zones {
		out = append(out, State{
			ID:        en.cfg.ID,
			Name:      en.cfg.Name,
			Channel:   en.cfg.Channel,
			Type:      en.cfg.Type,
			Active:    en.active,
			StartedAt: en.startedAt,
			MaxSecs:   en.cfg.MaxSecs,
			PulseSecs: en.cfg.EffectivePulseSecs(),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
