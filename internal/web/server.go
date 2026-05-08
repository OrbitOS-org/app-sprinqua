package web

import (
	"context"
	"embed"
	"html/template"
	"net/http"
	"sync"
	"time"

	"github.com/OrbitOS-org/sdk-go/v26/client"
	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"sprinkl/internal/board"
	"sprinkl/internal/config"
	"sprinkl/internal/i18n"
	"sprinkl/internal/scheduler"
	"sprinkl/internal/zone"
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
}

// basePage is embedded in every template data struct to provide S (strings), Lang, and TimeFormat.
type basePage struct {
	S          map[string]string
	Lang       string
	TimeFormat string // "24h" | "12h"
}

// Server holds all dependencies for the HTTP layer.
type Server struct {
	dataDir     string
	cfg         *config.Config
	board       *board.Board
	engine      *zone.Engine
	sched       *scheduler.Scheduler
	gpio        *client.GpioManager
	system      *client.SystemManager
	appHub      *client.AppHubManager
	tmpl        *template.Template
	testMu      sync.Mutex
	testCancels map[int]context.CancelFunc
}

func New(
	dataDir string,
	cfg *config.Config,
	b *board.Board,
	eng *zone.Engine,
	sched *scheduler.Scheduler,
	c *client.Client,
) (*Server, error) {
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{
		dataDir:     dataDir,
		cfg:         cfg,
		board:       b,
		engine:      eng,
		sched:       sched,
		gpio:        c.GpioManager,
		system:      c.SystemManager,
		appHub:      c.AppHubManager,
		tmpl:        tmpl,
		testCancels: make(map[int]context.CancelFunc),
	}, nil
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

	// Setup wizard
	mux.HandleFunc("GET /setup", s.handleSetup)
	mux.HandleFunc("POST /setup/step/1", s.handleSetupStep1)
	mux.HandleFunc("POST /setup/step/2", s.handleSetupStep2)
	mux.HandleFunc("POST /setup/test/{channel}", s.handleSetupTest)
	mux.HandleFunc("POST /setup/step/3", s.handleSetupStep3)
	mux.HandleFunc("POST /setup/step/4", s.handleSetupStep4)

	// Dashboard
	mux.HandleFunc("GET /dashboard", s.handleDashboard)

	// Zone API
	mux.HandleFunc("GET /api/zones/fragment", s.handleZonesFragment)
	mux.HandleFunc("POST /api/zones/{id}/on", s.handleZoneOn)
	mux.HandleFunc("POST /api/zones/{id}/off", s.handleZoneOff)
	mux.HandleFunc("POST /api/zones/{id}/pulse", s.handleZonePulse)

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
	if c, err := r.Cookie("sprinkl_lang"); err == nil {
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
	return basePage{S: i18n.Strings(l), Lang: l, TimeFormat: tf}
}

// setLangCookie writes the language preference cookie.
func setLangCookie(w http.ResponseWriter, lang string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "sprinkl_lang",
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
