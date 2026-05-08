package web

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/OrbitOS-org/sdk-go/v26/client"
	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"sprinkl/internal/board"
	"sprinkl/internal/config"
	"sprinkl/internal/i18n"
	"sprinkl/internal/zone"
)

// ── Language switcher ─────────────────────────────────────────────────────────

func (s *Server) handleLang(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("code")
	if code != "en" && code != "pt" {
		http.Error(w, "unsupported language", http.StatusBadRequest)
		return
	}
	setLangCookie(w, code)
	ref := r.Referer()
	if ref == "" {
		ref = "/"
	}
	http.Redirect(w, r, ref, http.StatusFound)
}

// ── Root ─────────────────────────────────────────────────────────────────────

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if s.cfg.SetupDone {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
		return
	}
	http.Redirect(w, r, "/setup", http.StatusFound)
}

// ── Setup Wizard ──────────────────────────────────────────────────────────────

type step1Data struct {
	basePage
	Boards []*board.Board
}

type step2Data struct {
	basePage
	Board *board.Board
	Zones []config.Zone
}

type step3Data struct {
	basePage
	Board *board.Board
	Zones []config.Zone
}

type step4Data struct {
	basePage
	MQTT config.MQTTConfig
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	s.render(w, "wizard", step1Data{basePage: s.page(r), Boards: board.All})
}

// initBoardPins configures every channel on the board as OUTPUT and ensures
// all relays start in the OFF state. Called once when a board is selected.
func (s *Server) initBoardPins(b *board.Board) {
	offLevel := client.GPIO_LEVEL_LOW
	if b.ActiveLow {
		offLevel = client.GPIO_LEVEL_HIGH
	}
	for _, ch := range b.Pins {
		_ = s.gpio.SetLevel(ch.Pin, offLevel)
		if err := s.gpio.SetDirection(ch.Pin, client.GPIO_DIR_OUT); err != nil {
			logger.Warnf(logTag, "init ch%d direction: %v", ch.Number, err)
		}
		if err := s.gpio.SetLevel(ch.Pin, offLevel); err != nil {
			logger.Warnf(logTag, "init ch%d off: %v", ch.Number, err)
		}
	}
	logger.Infof(logTag, "board %q: %d pins initialized as OUTPUT/OFF", b.ID, len(b.Pins))
}

func (s *Server) handleSetupStep1(w http.ResponseWriter, r *http.Request) {
	boardID := r.FormValue("board_id")
	b := board.Find(boardID)
	if b == nil {
		http.Error(w, "invalid board", http.StatusBadRequest)
		return
	}
	s.cfg.Board = boardID

	zones := make([]config.Zone, b.Channels)
	for i := range zones {
		zones[i] = config.Zone{
			ID:      i + 1,
			Name:    fmt.Sprintf("Zone %d", i+1),
			Channel: i + 1,
			Type:    "sprinkler",
			MaxSecs: 30 * 60,
			Enabled: true,
		}
	}
	s.cfg.Zones = zones

	s.render(w, "step2", step2Data{basePage: s.page(r), Board: b, Zones: zones})
}

func (s *Server) handleSetupStep2(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	b := board.Find(s.cfg.Board)
	if b == nil {
		http.Error(w, "board not configured", http.StatusBadRequest)
		return
	}

	for i := range s.cfg.Zones {
		id := s.cfg.Zones[i].ID
		name := r.FormValue(fmt.Sprintf("zone_%d_name", id))
		if name == "" {
			name = fmt.Sprintf("Zone %d", id)
		}
		zoneType := r.FormValue(fmt.Sprintf("zone_%d_type", id))
		if zoneType == "" {
			zoneType = "sprinkler"
		}
		maxMins, _ := strconv.Atoi(r.FormValue(fmt.Sprintf("zone_%d_max", id)))
		if maxMins <= 0 {
			maxMins = 30
		}
		s.cfg.Zones[i].Name = name
		s.cfg.Zones[i].Type = zoneType
		s.cfg.Zones[i].MaxSecs = maxMins * 60
	}

	s.initBoardPins(b)
	s.render(w, "step3", step3Data{basePage: s.page(r), Board: b, Zones: s.cfg.Zones})
}

func (s *Server) handleSetupTest(w http.ResponseWriter, r *http.Request) {
	ch, err := strconv.Atoi(r.PathValue("channel"))
	if err != nil || ch < 1 {
		http.Error(w, "invalid channel", http.StatusBadRequest)
		return
	}
	b := board.Find(s.cfg.Board)
	if b == nil {
		http.Error(w, "board not configured", http.StatusBadRequest)
		return
	}
	pin := b.PinByChannel(ch)
	if pin == nil {
		http.Error(w, "channel not in board", http.StatusBadRequest)
		return
	}

	onLevel := client.GPIO_LEVEL_HIGH
	offLevel := client.GPIO_LEVEL_LOW
	if b.ActiveLow {
		onLevel = client.GPIO_LEVEL_LOW
		offLevel = client.GPIO_LEVEL_HIGH
	}

	// Cancel any previous test pulse on this channel.
	s.testMu.Lock()
	if prev, ok := s.testCancels[ch]; ok {
		prev()
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.testCancels[ch] = cancel
	s.testMu.Unlock()

	_ = s.gpio.SetLevel(pin, offLevel) // ensure known OFF state first
	if err := s.gpio.SetLevel(pin, onLevel); err != nil {
		logger.Warnf(logTag, "test ch%d ON: %v", ch, err)
	}
	go func() {
		defer cancel()
		select {
		case <-time.After(3 * time.Second):
			if err := s.gpio.SetLevel(pin, offLevel); err != nil {
				logger.Warnf(logTag, "test ch%d OFF: %v", ch, err)
			} else {
				logger.Infof(logTag, "test ch%d OFF", ch)
			}
		case <-ctx.Done():
			_ = s.gpio.SetLevel(pin, offLevel)
		}
	}()

	strs := i18n.Strings(s.lang(r))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<span class="text-emerald-600 text-sm font-medium">%s</span>`,
		fmt.Sprintf(strs["test_activated"], ch))
}

func (s *Server) handleSetupStep3(w http.ResponseWriter, r *http.Request) {
	s.render(w, "step4", step4Data{
		basePage: s.page(r),
		MQTT:     config.MQTTConfig{Port: 1883, Prefix: "sprinkl"},
	})
}

func (s *Server) handleSetupStep4(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}

	if r.FormValue("mqtt_enabled") == "1" {
		port, _ := strconv.Atoi(r.FormValue("mqtt_port"))
		if port <= 0 {
			port = 1883
		}
		prefix := r.FormValue("mqtt_prefix")
		if prefix == "" {
			prefix = "sprinkl"
		}
		s.cfg.MQTT = config.MQTTConfig{
			Enabled:  true,
			Broker:   r.FormValue("mqtt_broker"),
			Port:     port,
			Username: r.FormValue("mqtt_user"),
			Password: r.FormValue("mqtt_pass"),
			Prefix:   prefix,
		}
	}

	s.cfg.SetupDone = true
	if err := s.cfg.Save(s.dataDir); err != nil {
		logger.Errorf(logTag, "save config: %v", err)
		http.Error(w, "failed to save config", http.StatusInternalServerError)
		return
	}

	b := board.Find(s.cfg.Board)
	if b != nil {
		s.board = b
		eng := zone.New(s.gpio, b, s.cfg.Zones)
		eng.Init()
		s.engine = eng
		logger.Infof(logTag, "zone engine initialized with %d zones", len(s.cfg.Zones))
	}

	w.Header().Set("HX-Redirect", "/dashboard")
	w.WriteHeader(http.StatusOK)
}

// ── Dashboard ─────────────────────────────────────────────────────────────────

type dashboardData struct {
	basePage
	HWModel string
	Zones   []zone.State
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	hwModel, _ := s.system.GetHardwareModel()
	var states []zone.State
	if s.engine != nil {
		states = s.engine.States()
	}
	s.render(w, "dashboard", dashboardData{
		basePage: s.page(r),
		HWModel:  hwModel,
		Zones:    states,
	})
}

// ── Zone Fragment (HTMX polling) ──────────────────────────────────────────────

type zonesFragData struct {
	basePage
	Zones []zone.State
}

func (s *Server) handleZonesFragment(w http.ResponseWriter, r *http.Request) {
	var states []zone.State
	if s.engine != nil {
		states = s.engine.States()
	}
	s.render(w, "zones_fragment", zonesFragData{basePage: s.page(r), Zones: states})
}

// ── Zone Controls ─────────────────────────────────────────────────────────────

func (s *Server) handleZoneOn(w http.ResponseWriter, r *http.Request) {
	id, eng := s.zonePrecheck(w, r)
	if eng == nil {
		return
	}
	if err := eng.TurnOn(id); err != nil {
		logger.Errorf(logTag, "zone %d ON: %v", id, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleZoneOff(w http.ResponseWriter, r *http.Request) {
	id, eng := s.zonePrecheck(w, r)
	if eng == nil {
		return
	}
	if err := eng.TurnOff(id); err != nil {
		logger.Errorf(logTag, "zone %d OFF: %v", id, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleZonePulse(w http.ResponseWriter, r *http.Request) {
	id, eng := s.zonePrecheck(w, r)
	if eng == nil {
		return
	}
	secs, _ := strconv.Atoi(r.FormValue("secs"))
	if secs <= 0 {
		secs = 300
	}
	if err := eng.Pulse(id, secs); err != nil {
		logger.Errorf(logTag, "zone %d pulse: %v", id, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) zonePrecheck(w http.ResponseWriter, r *http.Request) (int, *zone.Engine) {
	if s.engine == nil {
		http.Error(w, "engine not ready", http.StatusServiceUnavailable)
		return 0, nil
	}
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		http.Error(w, "invalid zone id", http.StatusBadRequest)
		return 0, nil
	}
	return id, s.engine
}
