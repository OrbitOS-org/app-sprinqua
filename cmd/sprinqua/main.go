package main

import (
	_ "embed"
	"flag"
	"os"

	"github.com/OrbitOS-org/sdk-go/v26/client"
	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"github.com/OrbitOS-org/sdk-go/v26/metadata"
	"sprinqua/internal/board"
	"sprinqua/internal/config"
	"sprinqua/internal/history"
	"sprinqua/internal/scheduler"
	"sprinqua/internal/web"
	"sprinqua/internal/zone"
)

const logTag = "main"

//go:embed metadata.json
var metadataJSON []byte

var appManifest = metadata.MustParseAppManifestJSON(metadataJSON)

func main() {
	host := flag.String("host", "192.168.1.51", "Device IP (used when not running on-device)")
	addr := flag.String("addr", ":8083", "HTTP listen address")
	dataDir := flag.String("data", ".", "Data directory for config and state")
	flag.Parse()

	meta := metadata.Build(appManifest)
	logger.Init(meta.Name, "INFO", true)
	logger.Infof(logTag, "Starting %s v%s", meta.Name, meta.Version)
	appManifest.PrintInfo()

	// Connect to Gravity RT (UDS on-device, TCP+mTLS from laptop).
	c, err := client.NewClientAuto(*host)
	if err != nil {
		logger.Fatalf(logTag, "connect to Gravity RT: %v", err)
		os.Exit(1)
	}
	defer c.Close()

	hwModel, _ := c.SystemManager.GetHardwareModel()
	logger.Infof(logTag, "hardware: %s", hwModel)

	// Load persisted config.
	cfg, err := config.Load(*dataDir)
	if err != nil {
		logger.Fatalf(logTag, "load config: %v", err)
		os.Exit(1)
	}

	// History store — load existing records, ignore first-run error.
	hist, err := history.New(*dataDir, cfg)
	if err != nil {
		logger.Fatalf(logTag, "load history: %v", err)
		os.Exit(1)
	}

	// Scheduler is always created so the wizard can inject the engine later.
	sched := scheduler.New(cfg, nil)
	sched.SetHistory(hist)

	var b *board.Board
	var eng *zone.Engine

	if cfg.SetupDone {
		b = board.Find(cfg.Board)
		if b == nil {
			// Never fatal-exit here: under the OrbitOS launcher (or any
			// supervisor that auto-restarts on crash) this becomes an
			// infinite crash loop with no way to reach the web UI to fix
			// it. Degrade instead — serve with no engine, same as the
			// "setup not done" state below, so the user can still open
			// Settings and re-run the wizard.
			logger.Errorf(logTag, "configured board %q not found in registry — re-run the setup wizard from Settings", cfg.Board)
		} else {
			chMgr := board.NewChannelManager(c.GpioManager, c.I2CManager)
			eng = zone.New(chMgr, b, cfg.Zones, cfg.IsExclusiveMode())
			eng.Init()
			eng.SetHistory(hist)
			sched.SetEngine(eng)
			logger.Infof(logTag, "zone engine ready (%d zones, board: %s)", len(cfg.Zones), b.Name)
		}
	} else {
		logger.Infof(logTag, "setup not complete — serving wizard")
	}

	sched.Start()
	logger.Infof(logTag, "scheduler started")

	// Build and start HTTP server.
	srv, err := web.New(*dataDir, cfg, b, eng, sched, hist, c, hwModel, meta.Version)
	if err != nil {
		logger.Fatalf(logTag, "create web server: %v", err)
		os.Exit(1)
	}

	if err := srv.Start(*addr, "/sprinqua"); err != nil {
		logger.Fatalf(logTag, "HTTP server: %v", err)
		os.Exit(1)
	}
}
