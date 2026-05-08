package web

import (
	"context"
	"fmt"
	"sort"
	"net/http"
	"strconv"
	"time"

	"github.com/OrbitOS-org/sdk-go/v26/client"
	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"sprinkl/internal/board"
	"sprinkl/internal/config"
	"sprinkl/internal/i18n"
	"sprinkl/internal/scheduler"
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
	MQTT       config.MQTTConfig
	TimeFormat string
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
	tf := s.cfg.TimeFormat
	if tf == "" {
		tf = "24h"
	}
	s.render(w, "step4", step4Data{
		basePage:   s.page(r),
		MQTT:       config.MQTTConfig{Port: 1883, Prefix: "sprinkl"},
		TimeFormat: tf,
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

	tf := r.FormValue("time_format")
	if tf != "12h" {
		tf = "24h"
	}
	s.cfg.TimeFormat = tf

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
		if s.sched != nil {
			s.sched.SetEngine(eng)
		}
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

// ── Schedule ──────────────────────────────────────────────────────────────────

var zoneColors = []string{
	"#10b981", // emerald
	"#3b82f6", // blue
	"#8b5cf6", // violet
	"#f59e0b", // amber
	"#f43f5e", // rose
	"#0ea5e9", // sky
	"#f97316", // orange
	"#14b8a6", // teal
}

type legendEntry struct {
	Name  string
	Color string
}

type chartBar struct {
	Label     string
	Color     string
	LeftPct   string
	WidthPct  string
	TopPx     int
	StartTime string
	DurMins   int
}

type chartDay struct {
	Bars     []chartBar
	HeightPx int
}

type schedulePageData struct {
	basePage
	Schedules  []scheduleView
	Chart      []chartDay
	ZoneLegend []legendEntry
}

type scheduleView struct {
	config.Schedule
	ZoneName    string
	NextRun     string
	DisplayTime string // StartTime formatted per TimeFormat
}

type scheduleFormData struct {
	basePage
	Schedule  config.Schedule
	Zones     []config.Zone
	IsNew     bool
	DaysError bool
}

func (s *Server) buildSchedulePage(r *http.Request) schedulePageData {
	zmap := s.cfg.ZoneMap()
	pg := s.page(r)
	use12h := s.cfg.TimeFormat == "12h"

	// Stable color per zone (ordered by zone list).
	zoneColor := make(map[int]string, len(s.cfg.Zones))
	for i, z := range s.cfg.Zones {
		zoneColor[z.ID] = zoneColors[i%len(zoneColors)]
	}

	// Schedule card views.
	views := make([]scheduleView, len(s.cfg.Schedules))
	for i, sc := range s.cfg.Schedules {
		next := scheduler.NextRunFor(sc)
		nextStr := pg.S["sched_no_next"]
		if next != nil {
			if use12h {
				nextStr = next.Format("Mon 3:04 PM")
			} else {
				nextStr = next.Format("Mon 15:04")
			}
		}
		views[i] = scheduleView{
			Schedule:    sc,
			ZoneName:    zmap[sc.ZoneID].Name,
			NextRun:     nextStr,
			DisplayTime: formatStartTime(sc.StartTime, use12h),
		}
	}

	// Weekly chart: collect raw bars per day, then pack into lanes.
	// minBarPct is the minimum rendered width (% of 24h) so short schedules
	// are always visible. Lane packing uses VISUAL positions so bars never
	// overlap on screen even when the minimum width expands them past where
	// the next schedule begins.
	const minBarPct = 1.5 // ≈ 21 min visual minimum; ~5px on a 350px container

	type rawBar struct {
		name     string
		color    string
		leftPct  float64 // start position on 0–100% axis
		rawWidth float64 // actual duration as % (before min applied)
		dispTime string
		durMins  int
	}
	dayRaw := [7][]rawBar{}
	legendSeen := make(map[int]bool)
	var legend []legendEntry

	for _, sc := range s.cfg.Schedules {
		if !sc.Enabled || sc.DurMins <= 0 {
			continue
		}
		t, err := time.Parse("15:04", sc.StartTime)
		if err != nil {
			continue
		}
		startMin := t.Hour()*60 + t.Minute()
		leftPct := float64(startMin) / (24 * 60) * 100
		rawWidth := float64(sc.DurMins) / (24 * 60) * 100
		color := zoneColor[sc.ZoneID]
		name := zmap[sc.ZoneID].Name
		if !legendSeen[sc.ZoneID] {
			legendSeen[sc.ZoneID] = true
		}
		rb := rawBar{
			name:     name,
			color:    color,
			leftPct:  leftPct,
			rawWidth: rawWidth,
			dispTime: formatStartTime(sc.StartTime, use12h),
			durMins:  sc.DurMins,
		}
		for _, d := range sc.Days {
			if d >= 0 && d <= 6 {
				dayRaw[d] = append(dayRaw[d], rb)
			}
		}
	}

	// Lane packing using visual end positions to prevent on-screen overlap.
	chart := make([]chartDay, 7)
	for d := range chart {
		bars := dayRaw[d]
		sort.Slice(bars, func(i, j int) bool { return bars[i].leftPct < bars[j].leftPct })

		laneVisualEnds := []float64{} // visual right edge (%) of last bar in each lane
		for _, rb := range bars {
			w := rb.rawWidth
			if w < minBarPct {
				w = minBarPct
			}
			if rb.leftPct+w > 100 {
				w = 100 - rb.leftPct
			}
			visualEnd := rb.leftPct + w

			lane := -1
			for i, end := range laneVisualEnds {
				if rb.leftPct >= end {
					lane = i
					break
				}
			}
			if lane == -1 {
				lane = len(laneVisualEnds)
				laneVisualEnds = append(laneVisualEnds, 0)
			}
			laneVisualEnds[lane] = visualEnd

			chart[d].Bars = append(chart[d].Bars, chartBar{
				Label:     rb.name,
				Color:     rb.color,
				LeftPct:   fmt.Sprintf("%.2f", rb.leftPct),
				WidthPct:  fmt.Sprintf("%.2f", w),
				TopPx:     lane * 22,
				StartTime: rb.dispTime,
				DurMins:   rb.durMins,
			})
		}
		if n := len(laneVisualEnds); n == 0 {
			chart[d].HeightPx = 20
		} else {
			chart[d].HeightPx = n*22 + 4
		}
	}

	// Build legend ordered by zone list.
	for _, z := range s.cfg.Zones {
		if legendSeen[z.ID] {
			legend = append(legend, legendEntry{Name: zmap[z.ID].Name, Color: zoneColor[z.ID]})
		}
	}

	return schedulePageData{
		basePage:   pg,
		Schedules:  views,
		Chart:      chart,
		ZoneLegend: legend,
	}
}

// formatStartTime converts a stored "HH:MM" string to display format.
func formatStartTime(hhmm string, use12h bool) string {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return hhmm
	}
	if use12h {
		return t.Format("3:04 PM")
	}
	return t.Format("15:04")
}

func (s *Server) handleSchedule(w http.ResponseWriter, r *http.Request) {
	s.render(w, "schedule", s.buildSchedulePage(r))
}

func (s *Server) handleScheduleNew(w http.ResponseWriter, r *http.Request) {
	s.render(w, "schedule_form", scheduleFormData{
		basePage:  s.page(r),
		Schedule:  config.Schedule{Enabled: true, StartTime: "08:00"},
		Zones:     s.cfg.Zones,
		IsNew:     true,
		DaysError: r.URL.Query().Get("err") == "days",
	})
}

func (s *Server) handleScheduleEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	for _, sc := range s.cfg.Schedules {
		if sc.ID == id {
			s.render(w, "schedule_form", scheduleFormData{
				basePage:  s.page(r),
				Schedule:  sc,
				Zones:     s.cfg.Zones,
				IsNew:     false,
				DaysError: r.URL.Query().Get("err") == "days",
			})
			return
		}
	}
	http.NotFound(w, r)
}

func (s *Server) handleScheduleCreate(w http.ResponseWriter, r *http.Request) {
	sc := s.parseScheduleForm(r)
	if len(sc.Days) == 0 {
		http.Redirect(w, r, "/schedule/new?err=days", http.StatusFound)
		return
	}
	sc.ID = s.cfg.NextScheduleID()
	s.cfg.Schedules = append(s.cfg.Schedules, sc)
	if err := s.cfg.Save(s.dataDir); err != nil {
		logger.Errorf(logTag, "save schedule: %v", err)
	}
	http.Redirect(w, r, "/schedule", http.StatusFound)
}

func (s *Server) handleScheduleUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sc := s.parseScheduleForm(r)
	if len(sc.Days) == 0 {
		http.Redirect(w, r, fmt.Sprintf("/schedule/%d/edit?err=days", id), http.StatusFound)
		return
	}
	sc.ID = id
	for i, existing := range s.cfg.Schedules {
		if existing.ID == id {
			s.cfg.Schedules[i] = sc
			break
		}
	}
	if err := s.cfg.Save(s.dataDir); err != nil {
		logger.Errorf(logTag, "save schedule: %v", err)
	}
	http.Redirect(w, r, "/schedule", http.StatusFound)
}

func (s *Server) handleScheduleToggle(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	for i := range s.cfg.Schedules {
		if s.cfg.Schedules[i].ID == id {
			s.cfg.Schedules[i].Enabled = !s.cfg.Schedules[i].Enabled
			break
		}
	}
	if err := s.cfg.Save(s.dataDir); err != nil {
		logger.Errorf(logTag, "save schedule: %v", err)
	}
	s.render(w, "schedule_list", s.buildSchedulePage(r))
}

func (s *Server) handleScheduleDelete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.Atoi(r.PathValue("id"))
	kept := s.cfg.Schedules[:0]
	for _, sc := range s.cfg.Schedules {
		if sc.ID != id {
			kept = append(kept, sc)
		}
	}
	s.cfg.Schedules = kept
	if err := s.cfg.Save(s.dataDir); err != nil {
		logger.Errorf(logTag, "save schedule: %v", err)
	}
	s.render(w, "schedule_list", s.buildSchedulePage(r))
}

func (s *Server) parseScheduleForm(r *http.Request) config.Schedule {
	if err := r.ParseForm(); err != nil {
		return config.Schedule{}
	}
	zoneID, _ := strconv.Atoi(r.FormValue("zone_id"))
	var days []int
	for _, d := range r.Form["days"] {
		if n, err := strconv.Atoi(d); err == nil && n >= 0 && n <= 6 {
			days = append(days, n)
		}
	}
	startTime := r.FormValue("start_time")
	if startTime == "" {
		startTime = "08:00"
	}
	durMins, _ := strconv.Atoi(r.FormValue("dur_mins"))
	if durMins <= 0 {
		durMins = 10
	}
	return config.Schedule{
		ZoneID:    zoneID,
		Days:      days,
		StartTime: startTime,
		DurMins:   durMins,
		Enabled:   r.FormValue("enabled") == "1",
	}
}
