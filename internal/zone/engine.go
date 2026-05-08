package zone

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/OrbitOS-org/sdk-go/v26/client"
	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"sprinkl/internal/board"
	"sprinkl/internal/config"
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

// Engine manages all zones and their relay GPIO state.
type Engine struct {
	mu    sync.Mutex
	gpio  *client.GpioManager
	board *board.Board
	zones map[int]*entry
}

func New(gpio *client.GpioManager, b *board.Board, zones []config.Zone) *Engine {
	e := &Engine{
		gpio:  gpio,
		board: b,
		zones: make(map[int]*entry),
	}
	for _, z := range zones {
		if z.Enabled {
			zc := z
			e.zones[z.ID] = &entry{cfg: zc}
		}
	}
	return e
}

// Init sets all relay pins as OUTPUT and ensures they start OFF.
func (e *Engine) Init() {
	for _, en := range e.zones {
		pin := e.board.PinByChannel(en.cfg.Channel)
		if pin == nil {
			logger.Warnf(logTag, "zone %d: no pin for channel %d", en.cfg.ID, en.cfg.Channel)
			continue
		}
		// Pre-set OFF before enabling output so the driver uses it as initial level.
		_ = e.relayWrite(pin, false)
		if err := e.gpio.SetDirection(pin, client.GPIO_DIR_OUT); err != nil {
			logger.Warnf(logTag, "zone %d SetDirection: %v", en.cfg.ID, err)
		}
		if err := e.relayWrite(pin, false); err != nil {
			logger.Warnf(logTag, "zone %d init OFF: %v", en.cfg.ID, err)
		}
	}
}

// relayWrite drives the relay pin, honouring board ActiveLow logic.
func (e *Engine) relayWrite(pin *client.GpioPin, on bool) error {
	var level client.GpioLevel
	if e.board.ActiveLow {
		if on {
			level = client.GPIO_LEVEL_LOW
		} else {
			level = client.GPIO_LEVEL_HIGH
		}
	} else {
		if on {
			level = client.GPIO_LEVEL_HIGH
		} else {
			level = client.GPIO_LEVEL_LOW
		}
	}
	return e.gpio.SetLevel(pin, level)
}

func (e *Engine) TurnOn(id int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	en, ok := e.zones[id]
	if !ok {
		return fmt.Errorf("zone %d not found", id)
	}

	// Cancel any previous safety timer.
	if en.cancel != nil {
		en.cancel()
		en.cancel = nil
	}

	pin := e.board.PinByChannel(en.cfg.Channel)
	if err := e.relayWrite(pin, true); err != nil {
		return fmt.Errorf("zone %d ON: %w", id, err)
	}
	en.active = true
	en.startedAt = time.Now()

	maxSecs := en.cfg.MaxSecs
	if maxSecs <= 0 {
		maxSecs = 30 * 60 // default 30 min
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

	en, ok := e.zones[id]
	if !ok {
		return fmt.Errorf("zone %d not found", id)
	}
	if en.cancel != nil {
		en.cancel()
		en.cancel = nil
	}
	pin := e.board.PinByChannel(en.cfg.Channel)
	if err := e.relayWrite(pin, false); err != nil {
		return fmt.Errorf("zone %d OFF: %w", id, err)
	}
	en.active = false
	logger.Infof(logTag, "zone %d OFF", id)
	return nil
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

// TestChannel pulses a raw GPIO channel (used during the setup wizard).
// It bypasses the zone map and drives the pin directly.
func (e *Engine) TestChannel(channel, secs int) error {
	pin := e.board.PinByChannel(channel)
	if pin == nil {
		return fmt.Errorf("channel %d not found in board", channel)
	}
	if err := e.relayWrite(pin, true); err != nil {
		return err
	}
	go func() {
		time.Sleep(time.Duration(secs) * time.Second)
		_ = e.relayWrite(pin, false)
	}()
	logger.Infof(logTag, "test channel %d for %ds", channel, secs)
	return nil
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
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
