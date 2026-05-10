package mqtt

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	paho "github.com/eclipse/paho.mqtt.golang"
	"github.com/OrbitOS-org/sdk-go/v26/logger"
	"sprinqua/internal/config"
	"sprinqua/internal/history"
	"sprinqua/internal/zone"
)

const logTag = "mqtt"

// Client wraps a paho MQTT connection and manages zone state publishing.
type Client struct {
	mu           sync.Mutex
	pahoC        paho.Client
	cfg          config.MQTTConfig
	zones        []config.Zone
	eng          *zone.Engine
	hist         *history.Store
	OnModeChange func(mode string) // called when HA sends a mode command
}

// New returns an unconnected Client.
func New() *Client {
	return &Client{}
}

// Connect (re)connects using the given config and registers the zone engine callback.
// Safe to call more than once — disconnects the previous connection first.
func (c *Client) Connect(cfg config.MQTTConfig, zones []config.Zone, eng *zone.Engine, hist *history.Store) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.disconnectLocked()

	if !cfg.Enabled || cfg.Broker == "" {
		return
	}

	c.cfg = cfg
	c.zones = zones
	c.eng = eng
	c.hist = hist

	opts := paho.NewClientOptions()
	port := cfg.Port
	if port <= 0 {
		port = 1883
	}
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.Broker, port))
	opts.SetClientID("sprinqua")
	opts.SetAutoReconnect(true)
	opts.SetCleanSession(true)
	if cfg.Username != "" {
		opts.SetUsername(cfg.Username)
		opts.SetPassword(cfg.Password)
	}
	opts.SetOnConnectHandler(func(cl paho.Client) {
		logger.Infof(logTag, "connected to broker %s:%d", cfg.Broker, port)
		c.onConnect(cl)
	})
	opts.SetConnectionLostHandler(func(_ paho.Client, err error) {
		logger.Warnf(logTag, "connection lost: %v", err)
	})

	pc := paho.NewClient(opts)
	if tok := pc.Connect(); tok.Wait() && tok.Error() != nil {
		logger.Warnf(logTag, "initial connect failed: %v (will retry)", tok.Error())
	}
	c.pahoC = pc

	if eng != nil {
		eng.OnStateChange = c.publishState
	}
}

// Disconnect cleanly closes the MQTT connection and removes the engine callback.
func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.disconnectLocked()
}

func (c *Client) disconnectLocked() {
	if c.pahoC != nil && c.pahoC.IsConnected() {
		c.pahoC.Disconnect(500)
	}
	c.pahoC = nil
	if c.eng != nil {
		c.eng.OnStateChange = nil
		c.eng = nil
	}
	c.hist = nil
}

// onConnect publishes discovery configs and current states after (re)connecting.
func (c *Client) onConnect(cl paho.Client) {
	c.mu.Lock()
	zones := c.zones
	eng := c.eng
	prefix := c.cfg.Prefix
	mode := c.cfg.Mode
	if mode == "" {
		mode = "active"
	}
	c.mu.Unlock()

	// Zone switch entities.
	for _, z := range zones {
		c.publishZoneDiscovery(cl, prefix, z)
	}

	// Mode select entity.
	c.publishModeDiscovery(cl, prefix)

	// Current zone states.
	if eng != nil {
		for _, st := range eng.States() {
			payload := "OFF"
			if st.Active {
				payload = "ON"
			}
			cl.Publish(fmt.Sprintf("%s/zone/%d/state", prefix, st.ID), 0, true, payload)
		}
	}

	// Current mode state (publish the HA-visible label).
	cl.Publish(fmt.Sprintf("%s/mode/state", prefix), 0, true, modeLabel(mode))

	// Subscribe to zone commands and mode commands.
	cl.Subscribe(fmt.Sprintf("%s/zone/+/set", prefix), 0, c.handleZoneCommand)
	cl.Subscribe(fmt.Sprintf("%s/mode/set", prefix), 0, c.handleModeCommand)
}

// ── Discovery payloads ────────────────────────────────────────────────────────

type haDevice struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Manufacturer string   `json:"manufacturer"`
	Model        string   `json:"model"`
}

var sprinquaDevice = haDevice{
	Identifiers:  []string{"sprinqua"},
	Name:         "Sprinqua",
	Manufacturer: "Sprinqua",
	Model:        "Irrigation Controller",
}

type haSwitch struct {
	Name         string   `json:"name"`
	UniqueID     string   `json:"unique_id"`
	CommandTopic string   `json:"command_topic"`
	StateTopic   string   `json:"state_topic"`
	PayloadOn    string   `json:"payload_on"`
	PayloadOff   string   `json:"payload_off"`
	Device       haDevice `json:"device"`
}

type haSelect struct {
	Name         string   `json:"name"`
	UniqueID     string   `json:"unique_id"`
	CommandTopic string   `json:"command_topic"`
	StateTopic   string   `json:"state_topic"`
	Options      []string `json:"options"`
	Icon         string   `json:"icon"`
	Device       haDevice `json:"device"`
}

func (c *Client) publishZoneDiscovery(cl paho.Client, prefix string, z config.Zone) {
	uid := fmt.Sprintf("sprinqua_zone_%d", z.ID)
	payload, _ := json.Marshal(haSwitch{
		Name:         z.Name,
		UniqueID:     uid,
		CommandTopic: fmt.Sprintf("%s/zone/%d/set", prefix, z.ID),
		StateTopic:   fmt.Sprintf("%s/zone/%d/state", prefix, z.ID),
		PayloadOn:    "ON",
		PayloadOff:   "OFF",
		Device:       sprinquaDevice,
	})
	cl.Publish(fmt.Sprintf("homeassistant/switch/%s/config", uid), 0, true, string(payload))
	logger.Infof(logTag, "published HA discovery for zone %d (%s)", z.ID, z.Name)
}

func (c *Client) publishModeDiscovery(cl paho.Client, prefix string) {
	payload, _ := json.Marshal(haSelect{
		Name:         "Sprinqua Mode",
		UniqueID:     "sprinqua_mode",
		CommandTopic: fmt.Sprintf("%s/mode/set", prefix),
		StateTopic:   fmt.Sprintf("%s/mode/state", prefix),
		Options:      []string{modeLabel("active"), modeLabel("passive")},
		Icon:         "mdi:toggle-switch",
		Device:       sprinquaDevice,
	})
	cl.Publish("homeassistant/select/sprinqua_mode/config", 0, true, string(payload))
	logger.Infof(logTag, "published HA discovery for mode select")
}

// modeLabel converts an internal mode value to the human-readable label sent to HA.
func modeLabel(mode string) string {
	if mode == "passive" {
		return "Home Assistant Managed"
	}
	return "Standalone"
}

// modeFromLabel converts a HA label back to the internal mode value.
func modeFromLabel(label string) string {
	if label == "Home Assistant Managed" {
		return "passive"
	}
	return "active"
}

// ── State publishing ──────────────────────────────────────────────────────────

// publishState is called by the zone engine's OnStateChange callback.
func (c *Client) publishState(zoneID int, on bool) {
	c.mu.Lock()
	cl := c.pahoC
	prefix := c.cfg.Prefix
	c.mu.Unlock()

	if cl == nil || !cl.IsConnected() {
		return
	}
	payload := "OFF"
	if on {
		payload = "ON"
	}
	cl.Publish(fmt.Sprintf("%s/zone/%d/state", prefix, zoneID), 0, true, payload)
}

// PublishModeState publishes the current mode to the state topic using the HA-visible label.
func (c *Client) PublishModeState(mode string) {
	c.mu.Lock()
	cl := c.pahoC
	prefix := c.cfg.Prefix
	c.mu.Unlock()

	if cl == nil || !cl.IsConnected() {
		return
	}
	cl.Publish(fmt.Sprintf("%s/mode/state", prefix), 0, true, modeLabel(mode))
}

// ── Command handlers ──────────────────────────────────────────────────────────

func (c *Client) handleZoneCommand(_ paho.Client, msg paho.Message) {
	// topic: {prefix}/zone/{id}/set
	parts := strings.Split(msg.Topic(), "/")
	if len(parts) < 4 {
		return
	}
	id, err := strconv.Atoi(parts[len(parts)-2])
	if err != nil {
		return
	}

	c.mu.Lock()
	eng := c.eng
	hist := c.hist
	c.mu.Unlock()

	if eng == nil {
		return
	}

	switch strings.ToUpper(strings.TrimSpace(string(msg.Payload()))) {
	case "ON":
		if err := eng.TurnOn(id); err != nil {
			logger.Warnf(logTag, "MQTT TurnOn zone %d: %v", id, err)
		} else if hist != nil {
			hist.Start(id, history.MQTT)
		}
	case "OFF":
		if err := eng.TurnOff(id); err != nil {
			logger.Warnf(logTag, "MQTT TurnOff zone %d: %v", id, err)
		} else if hist != nil {
			hist.Stop(id)
		}
	}
}

func (c *Client) handleModeCommand(_ paho.Client, msg paho.Message) {
	label := strings.TrimSpace(string(msg.Payload()))
	mode := modeFromLabel(label)
	if label != modeLabel("active") && label != modeLabel("passive") {
		logger.Warnf(logTag, "unknown mode command: %q", label)
		return
	}

	c.mu.Lock()
	cb := c.OnModeChange
	c.mu.Unlock()

	logger.Infof(logTag, "mode change via MQTT: %s", mode)

	if cb != nil {
		cb(mode)
	}

	// Confirm new state back to HA immediately.
	c.PublishModeState(mode)
}
