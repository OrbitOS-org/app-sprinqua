package main

import (
	_ "embed"
	"flag"
	"os"

	"github.com/OrbitOS-org/sdk-go/v26/client"
	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"github.com/OrbitOS-org/sdk-go/v26/metadata"
	"sprinkl/internal/board"
	"sprinkl/internal/config"
	"sprinkl/internal/web"
	"sprinkl/internal/zone"
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

	// If setup is already done, initialize board and zone engine immediately.
	var b *board.Board
	var eng *zone.Engine

	if cfg.SetupDone {
		b = board.Find(cfg.Board)
		if b == nil {
			logger.Fatalf(logTag, "configured board %q not found in registry", cfg.Board)
			os.Exit(1)
		}
		eng = zone.New(c.GpioManager, b, cfg.Zones)
		eng.Init()
		logger.Infof(logTag, "zone engine ready (%d zones, board: %s)", len(cfg.Zones), b.Name)
	} else {
		logger.Infof(logTag, "setup not complete — serving wizard")
	}

	// Build and start HTTP server.
	srv, err := web.New(*dataDir, cfg, b, eng, c)
	if err != nil {
		logger.Fatalf(logTag, "create web server: %v", err)
		os.Exit(1)
	}

	if err := srv.Start(*addr, "/sprinkl"); err != nil {
		logger.Fatalf(logTag, "HTTP server: %v", err)
		os.Exit(1)
	}
}
