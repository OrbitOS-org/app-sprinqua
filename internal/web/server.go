package web

import (
	"context"
	"embed"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/OrbitOS-org/sdk-go/v26/client"
	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"sprinqua/internal/board"
	"sprinqua/internal/config"
	"sprinqua/internal/history"
	"sprinqua/internal/i18n"
	"sprinqua/internal/mqtt"
	"sprinqua/internal/scheduler"
	"sprinqua/internal/weather"
	"sprinqua/internal/zone"
)

const logTag = "web"

//go:embed templates
var templateFS embed.FS

var funcMap = template.FuncMap{
	"divInt": func(a, b int) int {
		if b == 0 {
			return 0
		}
		return a / b
	},
	"sub": func(a, b int) int { return a - b },
	"slice": func(vals ...int) []int { return vals },
	"hasDay": func(days []int, d int) bool {
		for _, day := range days {
			if day == d {
				return true
			}
		}
		return false
	},
	// zoneColor returns the persistent hex color for a zone by its index in the zones slice.
	"zoneColor": func(idx int) string {
		return zoneColors[idx%len(zoneColors)]
	},
}

// basePage is embedded in every template data struct to provide S (strings), Lang, and TimeFormat.
type basePage struct {
	S          map[string]string
	Lang       string
	TimeFormat string // "24h" | "12h"
	ActiveTab  string // "zones" | "history" | "schedule" | "setup"
	Version    string
}

// Server holds all dependencies for the HTTP layer.
type Server struct {
	dataDir        string
	cfg            *config.Config
	board          *board.Board
	engine         *zone.Engine
	sched          *scheduler.Scheduler
	hist           *history.Store
	gpio           *client.GpioManager
	system         *client.SystemManager
	appHub         *client.AppHubManager
	mqttClient     *mqtt.Client
	version        string
	tmpl           *template.Template
	testMu     sync.Mutex
	testCancel context.CancelFunc // non-nil while a relay test is active
	etoMu          sync.Mutex
	etoCalculating bool
}

func New(
	dataDir string,
	cfg *config.Config,
	b *board.Board,
	eng *zone.Engine,
	sched *scheduler.Scheduler,
	hist *history.Store,
	c *client.Client,
	version string,
) (*Server, error) {
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	srv := &Server{
		dataDir:     dataDir,
		cfg:         cfg,
		board:       b,
		engine:      eng,
		sched:       sched,
		hist:        hist,
		gpio:        c.GpioManager,
		system:      c.SystemManager,
		appHub:      c.AppHubManager,
		version:     version,
		tmpl:        tmpl,
	}

	mc := mqtt.New()
	mc.OnModeChange = srv.applyModeChange
	srv.mqttClient = mc

	if cfg.SetupDone && eng != nil {
		mc.Connect(cfg.MQTT, cfg.Zones, eng, hist)
	}
	if cfg.MQTT.IsPassive() || cfg.WinterMode {
		sched.SetPaused(true)
	}

	return srv, nil
}

// applyModeChange is called by the MQTT client when HA sends a mode command.
func (s *Server) applyModeChange(mode string) {
	s.cfg.MQTT.Mode = mode
	s.sched.SetPaused(s.cfg.MQTT.IsPassive())
	if err := s.cfg.Save(s.dataDir); err != nil {
		logger.Errorf(logTag, "save config after MQTT mode change: %v", err)
	}
	logger.Infof(logTag, "mode changed to %q via MQTT", mode)
}

// Start registers with the OrbitOS App Hub and begins serving HTTP.
func (s *Server) Start(addr, hubRoute string) error {
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	if err := s.appHub.RegisterWebUI(addr, hubRoute); err != nil {
		logger.Warnf(logTag, "AppHub register: %v (continuing without hub registration)", err)
	} else {
		logger.Infof(logTag, "registered with AppHub at route %s", hubRoute)
	}

	logger.Infof(logTag, "HTTP server listening on %s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", s.handleRoot)

	// Language switcher
	mux.HandleFunc("GET /lang/{code}", s.handleLang)

	// Settings + wizard
	mux.HandleFunc("GET /setup", s.handleSetup)
	mux.HandleFunc("POST /setup/save", s.handleSettingsSave)
	mux.HandleFunc("POST /setup/zones", s.handleZonesSave)
	mux.HandleFunc("POST /setup/reset", s.handleSetupReset)
	mux.HandleFunc("GET /setup/wizard", s.handleSetupWizard)
	mux.HandleFunc("GET /setup/channels", s.handleSetupChannels)
	mux.HandleFunc("POST /setup/step/1", s.handleSetupStep1)
	mux.HandleFunc("POST /setup/step/2", s.handleSetupStep2)
	mux.HandleFunc("POST /setup/test/{channel}", s.handleSetupTest)
	mux.HandleFunc("POST /setup/step/3", s.handleSetupStep3)

	// Dashboard
	mux.HandleFunc("GET /dashboard", s.handleDashboard)

	// Zone API
	mux.HandleFunc("GET /api/zones/fragment", s.handleZonesFragment)
	mux.HandleFunc("POST /api/zones/{id}/on", s.handleZoneOn)
	mux.HandleFunc("POST /api/zones/{id}/off", s.handleZoneOff)
	mux.HandleFunc("POST /api/zones/{id}/pulse", s.handleZonePulse)

	// Smart Watering
	mux.HandleFunc("GET /api/weather", s.handleWeatherStatus)
	mux.HandleFunc("POST /api/eto-baseline/recalculate", s.handleEToBaselineRecalculate)

	// History
	mux.HandleFunc("GET /history", s.handleHistory)

	// Schedule
	mux.HandleFunc("GET /schedule", s.handleSchedule)
	mux.HandleFunc("GET /schedule/new", s.handleScheduleNew)
	mux.HandleFunc("POST /schedule", s.handleScheduleCreate)
	mux.HandleFunc("GET /schedule/{id}/edit", s.handleScheduleEdit)
	mux.HandleFunc("POST /schedule/{id}", s.handleScheduleUpdate)
	mux.HandleFunc("POST /schedule/{id}/toggle", s.handleScheduleToggle)
	mux.HandleFunc("POST /schedule/{id}/delete", s.handleScheduleDelete)
}

// lang detects the active language for a request.
func (s *Server) lang(r *http.Request) string {
	var cookie string
	if c, err := r.Cookie("sprinqua_lang"); err == nil {
		cookie = c.Value
	}
	return i18n.Detect(r.Header.Get("Accept-Language"), cookie)
}

// page builds a basePage for the detected language.
func (s *Server) page(r *http.Request) basePage {
	l := s.lang(r)
	tf := s.cfg.TimeFormat
	if tf == "" {
		tf = "24h"
	}
	var tab string
	switch {
	case strings.HasPrefix(r.URL.Path, "/history"):
		tab = "history"
	case strings.HasPrefix(r.URL.Path, "/schedule"):
		tab = "schedule"
	case strings.HasPrefix(r.URL.Path, "/setup"):
		tab = "setup"
	default:
		tab = "zones"
	}
	return basePage{S: i18n.Strings(l), Lang: l, TimeFormat: tf, ActiveTab: tab, Version: s.version}
}

// setLangCookie writes the language preference cookie.
func setLangCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "sprinqua_lang",
		Value:    lang,
		Path:     "/",
		MaxAge:   int((365 * 24 * time.Hour).Seconds()),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, data); err != nil {
		logger.Errorf(logTag, "render %s: %v", name, err)
		http.Error(w, "render error", http.StatusInternalServerError)
	}
}

// triggerEToBaseline fetches the ETo baseline in a background goroutine and
// persists the result to config. Safe to call multiple times — ignores if already running.
func (s *Server) triggerEToBaseline() {
	s.etoMu.Lock()
	if s.etoCalculating {
		s.etoMu.Unlock()
		return
	}
	s.etoCalculating = true
	s.etoMu.Unlock()

	go func() {
		defer func() {
			s.etoMu.Lock()
			s.etoCalculating = false
			s.etoMu.Unlock()
		}()

		sw := s.cfg.SmartWatering
		baseline, err := weather.FetchEToBaseline(sw.Lat, sw.Lon)
		if err != nil {
			logger.Warnf(logTag, "ETo baseline fetch failed: %v", err)
			return
		}

		s.cfg.SmartWatering.EToBaseline = baseline
		s.cfg.SmartWatering.EToBaselineCalculatedAt = time.Now().Format("2006-01-02")
		if err := s.cfg.Save(s.dataDir); err != nil {
			logger.Errorf(logTag, "save config after ETo baseline: %v", err)
			return
		}
		logger.Infof(logTag, "ETo baseline calculated: %.3f mm/day", baseline)
	}()
}

// handleEToBaselineRecalculate resets the stored baseline and triggers a fresh calculation.
func (s *Server) handleEToBaselineRecalculate(w http.ResponseWriter, r *http.Request) {
	s.cfg.SmartWatering.EToBaseline = 0
	s.cfg.SmartWatering.EToBaselineCalculatedAt = ""
	if err := s.cfg.Save(s.dataDir); err != nil {
		logger.Errorf(logTag, "save config on ETo reset: %v", err)
		http.Error(w, "save error", http.StatusInternalServerError)
		return
	}
	go s.triggerEToBaseline()
	http.Redirect(w, r, "/setup", http.StatusSeeOther)
}
