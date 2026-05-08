# Orbit OS SDK API Reference
**Version:** SDK API 26 — v26.0.1 (latest)  
**Language:** Go — v26.0501.2330  
**Source:** https://www.orbit-os.org/api-reference.html

---

## Table of Contents

- [Auth Manager](#auth-manager)
- [System Manager](#system-manager)
- [Package Manager](#package-manager)
- [Ethernet Manager](#ethernet-manager)
- [WiFi Manager](#wifi-manager)
- [GPIO Manager](#gpio-manager)
- [PWM Manager](#pwm-manager)
- [UART Manager](#uart-manager)
- [I2C Manager](#i2c-manager)
- [SPI Manager](#spi-manager)
- [Camera Manager](#camera-manager)
- [AI Manager](#ai-manager)
- [Firewall Manager](#firewall-manager)
- [VPN Manager](#vpn-manager)
- [App Hub Manager](#app-hub-manager)
- [Event Manager](#event-manager)
- [Development Manager](#development-manager)
- [Update Manager](#update-manager)
- [Power Manager](#power-manager)
- [Types](#types)

---

## Auth Manager

Authentication and session management.

### Login
Authenticates a user and returns a session token.

```go
func (m *AuthManager) Login(username, password string) (token string, expiresAt int64, err error)
```

| Param | Type | Notes |
|---|---|---|
| username | string | |
| password | string | |

**Returns:** `token string`, `expiresAt int64` (Unix timestamp), `error`

```go
token, expiresAt, err := client.AuthManager.Login(username, password)
```

---

### Logout
Invalidates a session token.

```go
func (m *AuthManager) Logout(token string) error
```

```go
err := client.AuthManager.Logout(token)
```

---

## System Manager

Device identity, hardware info, OS metadata and live metrics.  
**Required permission:** `SystemService/*`

| Method | Signature | Description |
|---|---|---|
| GetApiVersion | `() (version int64, revision int64, err error)` | Returns API version and revision |
| GetApiVersionInfo | `() (string, error)` | Returns API version as formatted string |
| GetDeviceName | `() (string, error)` | Returns configured device name |
| GetSocModel | `() (string, error)` | Returns SoC model identifier |
| GetSocVendor | `() (string, error)` | Returns SoC vendor name |
| GetBoardModel | `() (string, error)` | Returns board model identifier |
| GetBoardVendor | `() (string, error)` | Returns board vendor name |
| GetHardwareVersion | `() (string, error)` | Returns hardware revision string |
| GetHardwareModel | `() (string, error)` | Returns hardware model identifier |
| GetSystemUuid | `() (string, error)` | Returns system UUID |
| GetBoardSerial | `() (string, error)` | Returns board serial number |
| GetCpuSerial | `() (string, error)` | Returns CPU serial number |
| GetMachineId | `() (string, error)` | Returns machine ID |
| GetArchitecture | `() (string, error)` | Returns CPU architecture (e.g. "arm64", "amd64") |
| GetTotalRAM | `() (uint64, error)` | Returns total RAM in bytes |
| GetCpuModel | `() (string, error)` | Returns CPU model string |
| GetCpuCores | `() (int64, error)` | Returns number of physical CPU cores |
| GetCpuThreads | `() (int64, error)` | Returns number of logical CPU threads |
| GetCpuMinMhz | `() (float64, error)` | Returns minimum CPU frequency in MHz |
| GetCpuMaxMhz | `() (float64, error)` | Returns maximum CPU frequency in MHz |
| GetOsName | `() (string, error)` | Returns OS name |
| GetOsVersion | `() (string, error)` | Returns OS version string |
| GetKernelVersion | `() (string, error)` | Returns kernel version string |
| GetDistro | `() (string, error)` | Returns Linux distribution name |
| GetDistroVersion | `() (string, error)` | Returns distribution version string |
| GetRuntimeVersion | `() (string, error)` | Returns Orbit runtime version |
| GetOsRevision | `() (string, error)` | Returns logical OS revision string |
| GetBuildVersion | `() (string, error)` | Alias for GetOsRevision |
| GetRuntimeBuildDate | `() (string, error)` | Returns runtime image build date |
| GetBuildDate | `() (string, error)` | Alias for GetRuntimeBuildDate |

### GetMetrics
Returns live CPU usage, RAM and storage stats.

```go
func (s *SystemManager) GetMetrics() (*MetricsInfoResponse, error)

metrics, err := client.SystemManager.GetMetrics()
```

### Attach
Attaches the SDK client to the running Gravity system service.

```go
func (s *SystemManager) Attach() (bool, error)

ok, err := client.SystemManager.Attach()
```

### Developer / SSH / Reboot / App Trust controls

| Method | Description |
|---|---|
| `EnableDevMode() (bool, error)` | Enables developer mode |
| `DisableDevMode() (bool, error)` | Disables developer mode |
| `IsDevModeEnabled() (bool, error)` | Reports whether dev mode is enabled |
| `EnableSSHServer() (bool, error)` | Enables SSH server |
| `DisableSSHServer() (bool, error)` | Disables SSH server |
| `IsSSHServerEnabled() (bool, error)` | Reports whether SSH is enabled |
| `EnableRebootOnFailure() (bool, error)` | Enables auto reboot on failure |
| `DisableRebootOnFailure() (bool, error)` | Disables auto reboot on failure |
| `IsRebootOnFailureEnabled() (bool, error)` | Reports whether auto reboot is enabled |
| `AllowUntrustedApps() (bool, error)` | Allows installation from untrusted sources |
| `DisallowUntrustedApps() (bool, error)` | Blocks installation from untrusted sources |
| `IsUntrustedAppsAllowed() (bool, error)` | Reports whether untrusted apps are allowed |

---

## Package Manager

Application package installation and management.  
**Required permission:** `PackageService/*`

### ListInstalledPackages
```go
func (p *PackageManager) ListInstalledPackages() ([]*InstalledPackage, error)

pkgs, err := client.PackageManager.ListInstalledPackages()
```

### InstallPackage
Uploads and installs an `.orb` package from a local path.

```go
func (p *PackageManager) InstallPackage(ctx context.Context, orbPath string) error

err := client.PackageManager.InstallPackage(ctx, "/path/to/app.orb")
```

### RemovePackage
Uninstalls a package by ID.

```go
func (p *PackageManager) RemovePackage(ctx context.Context, packageID string) error

err := client.PackageManager.RemovePackage(ctx, packageID)
```

---

## Ethernet Manager

Ethernet interface configuration and status.  
**Required permission:** `EthernetService/*`

### ListEthernetInterfaces
```go
func (e *EthernetManager) ListEthernetInterfaces() ([]*EthernetLinkProperties, error)

ifaces, err := client.EthernetManager.ListEthernetInterfaces()
```

### IsEthernetConnected
```go
func (e *EthernetManager) IsEthernetConnected(interfaceName string) (bool, error)

ok, err := client.EthernetManager.IsEthernetConnected("eth0")
```

### GetEthernetLinkProperties
Returns link properties (MAC, state, MTU, IP config) for an interface.

```go
func (e *EthernetManager) GetEthernetLinkProperties(interfaceName string) (*EthernetLinkProperties, error)

props, err := client.EthernetManager.GetEthernetLinkProperties("eth0")
```

### SetEthernetConfig
Applies static IP or DHCP settings to an interface.

```go
func (e *EthernetManager) SetEthernetConfig(
    interfaceName string, enable, dhcpEnable bool,
    ipv4Address, ipv4Gateway string, ipv4Dns []string,
) (bool, error)
```

| Param | Type | Notes |
|---|---|---|
| interfaceName | string | |
| enable | bool | |
| dhcpEnable | bool | |
| ipv4Address | string | CIDR notation — ignored when dhcpEnable is true |
| ipv4Gateway | string | |
| ipv4Dns | []string | |

```go
ok, err := client.EthernetManager.SetEthernetConfig(
    "eth0", true, false,
    "192.168.1.10/24", "192.168.1.1", []string{"8.8.8.8"},
)
```

### EnableEthernet / DisableEthernet
```go
func (e *EthernetManager) EnableEthernet(interfaceName string) (bool, error)
func (e *EthernetManager) DisableEthernet(interfaceName string) (bool, error)

ok, err := client.EthernetManager.EnableEthernet("eth0")
ok, err := client.EthernetManager.DisableEthernet("eth0")
```

---

## WiFi Manager

Wi-Fi interface management — client mode and access point.  
**Required permission:** `WiFiService/*`

### General

```go
// List all Wi-Fi interfaces
func (w *WiFiManager) ListInterfaces() ([]*WiFiLinkProperties, error)

// Get link properties for an interface
func (w *WiFiManager) GetLinkProperties(ifname string) (*WiFiLinkProperties, error)

// Check if interface is connected
func (w *WiFiManager) IsConnected(ifname string) (bool, error)

// Get current operating mode
func (w *WiFiManager) GetMode(ifname string) (WiFiMode, error)

// Switch to client mode
func (w *WiFiManager) SetModeClient(ifname string) (bool, error)
```

### Client Mode

#### SetClientConfig
```go
func (w *WiFiManager) SetClientConfig(
    ifname, ssid, password, security string,
    dhcpEnable bool,
    ipv4Address, ipv4Gateway string,
    ipv4Dns []string,
) (bool, error)
```

`security` values: `"none"` | `"wpa2"` | `"wpa3"` | `"wpa2-wpa3"`

```go
ok, err := client.WiFiManager.SetClientConfig(
    "wlan0", "MySSID", "pass", "wpa2",
    true, "", "", nil,
)
```

#### GetClientProperties / Connect / Disconnect
```go
func (w *WiFiManager) GetClientProperties(ifname string) (*ClientProperties, error)
func (w *WiFiManager) Connect(ifname string) (bool, error)
func (w *WiFiManager) Disconnect(ifname string) (bool, error)
```

### Access Point Mode

#### StartAP
```go
func (w *WiFiManager) StartAP(ifname, ssid, password, band string, channel int32) (bool, error)
```
`band`: `"2.4GHz"` or `"5GHz"`. `channel = 0` means auto.

```go
ok, err := client.WiFiManager.StartAP("wlan0", "MyAP", "pass", "5GHz", 0)
```

#### StopAP / GetAPProperties
```go
func (w *WiFiManager) StopAP(ifname string) (bool, error)
func (w *WiFiManager) GetAPProperties(ifname string) (*APProperties, error)
```

### Scan
```go
func (w *WiFiManager) Scan(ifname string, forceRescan bool) ([]*ScannedNetwork, error)

nets, err := client.WiFiManager.Scan("wlan0", true)
```
`forceRescan = true` triggers a new hardware scan (~3 s).

---

## GPIO Manager

General-purpose I/O pin control.  
**Required permission:** `GpioService/*`

```go
// List all GPIO pins
func (m *GpioManager) ListPins() ([]*GpioPin, error)

// Get direction of a pin
func (m *GpioManager) GetDirection(pin *GpioPin) (GpioDirection, error)

// Set pin direction: GPIO_DIR_IN | GPIO_DIR_OUT
func (m *GpioManager) SetDirection(pin *GpioPin, dir GpioDirection) error

// Read logic level
func (m *GpioManager) GetLevel(pin *GpioPin) (GpioLevel, error)

// Set logic level (output pins only): GPIO_LEVEL_LOW | GPIO_LEVEL_HIGH
func (m *GpioManager) SetLevel(pin *GpioPin, level GpioLevel) error
```

```go
pins, err := client.GpioManager.ListPins()
err := client.GpioManager.SetDirection(pin, GPIO_DIR_OUT)
err := client.GpioManager.SetLevel(pin, GPIO_LEVEL_HIGH)
```

---

## PWM Manager

PWM channel configuration and control.  
**Required permission:** `PwmService/*`

```go
// List available channels
func (m *PwmManager) ListChannels() ([]*PwmChannel, error)

// Read channel configuration
func (m *PwmManager) GetProperties(ch *PwmChannel) (*PwmProperties, error)

// Configure and start PWM output (dutyCycle: 0.0–1.0)
func (m *PwmManager) SetPwm(ch *PwmChannel, dutyCycle, frequencyHz float64) error

// Disable PWM output
func (m *PwmManager) StopPwm(ch *PwmChannel) error
```

```go
err := client.PwmManager.SetPwm(ch, 0.5, 1000.0)
```

---

## UART Manager

Serial port (UART) communication.  
**Required permission:** `UartService/*`

`Open` returns a `*UartPort` handle — all port operations are called on the handle.

### UartManager

```go
// List available UART port names
func (m *UartManager) ListPorts() ([]string, error)

// Open and configure a port
func (m *UartManager) Open(cfg UartConfig) (*UartPort, error)
```

```go
port, err := client.UartManager.Open(UartConfig{
    Port:     "ttyS0",
    Baudrate: 115200,
    DataBits: 8,
    Parity:   UartParityNone,
    StopBits: UartStopBits1,
})
```

### UartPort

```go
// Close the port
func (p *UartPort) Close() error

// Get current config
func (p *UartPort) GetConfig() (*UartConfig, error)

// Write bytes; returns bytes written
func (p *UartPort) Write(data []byte) (int, error)

// Stream incoming bytes via callback (blocks until ctx cancelled)
func (p *UartPort) Listen(ctx context.Context, maxChunkSize int, onChunk func([]byte)) error

// Stream incoming bytes via channel (channel closed on end)
func (p *UartPort) ListenAsync(ctx context.Context, maxChunkSize int) (<-chan []byte, error)
```

```go
n, err := port.Write([]byte("hello"))

err := port.Listen(ctx, 256, func(b []byte) {
    fmt.Printf("rx: %q\n", b)
})

ch, err := port.ListenAsync(ctx, 256)
for b := range ch {
    fmt.Printf("rx: %q\n", b)
}
```

---

## I2C Manager

I²C bus discovery and data transfer.  
**Required permission:** `I2cService/*`

### I2CManager

```go
// List all I²C bus indices
func (m *I2CManager) ListBuses() ([]uint32, error)

// Configure bus and return a handle
func (m *I2CManager) Open(bus uint32, clockHz uint32, tenBitAddr bool, clockStretching bool) (*I2CBus, error)
```

```go
b, err := client.I2CManager.Open(1, 400000, false, false)
```

### I2CBus

```go
// Probe addresses 0x03–0x77, return those that respond
func (b *I2CBus) Scan() ([]uint32, error)

// Get current bus config
func (b *I2CBus) GetConfig() (I2CConfig, error)

// Write, read, or write-then-read transaction
// Pass nil data for read-only; readLen=0 for write-only
func (b *I2CBus) Transfer(addr uint32, data []byte, readLen uint32, flags uint32) ([]byte, error)
```

```go
data, err := b.Transfer(0x48, []byte{0x00}, 2, 0)
```

---

## SPI Manager

SPI bus configuration and full-duplex transfer.  
**Required permission:** `SpiService/*`

### SpiManager

```go
// List available SPI device names (e.g. "spidev0.0")
func (m *SpiManager) ListDevices() ([]string, error)

// Configure SPI device and return a handle
// mode: 0–3 (CPOL/CPHA)
func (m *SpiManager) Open(bus, cs uint32, maxSpeedHz, bitsPerWord uint32, mode int, lsbFirst bool) (*SpiDevice, error)
```

```go
dev, err := client.SpiManager.Open(0, 0, 1000000, 8, 0, false)
```

### SpiDevice

```go
// Get current device config
func (d *SpiDevice) GetConfig() (*SpiConfig, error)

// Full-duplex transfer; readLength=0 for write-only
func (d *SpiDevice) Transfer(dataOut []byte, readLength uint32) ([]byte, error)
```

```go
rx, err := dev.Transfer(txData, uint32(len(txData)))
```

---

## Camera Manager

V4L2 camera capture and streaming.  
**Required permission:** `CameraService/*`

```go
// List all V4L2 video devices
func (m *CameraManager) ListDevices(ctx context.Context) ([]*CameraDeviceInfo, error)

// Get metadata for a specific camera
func (m *CameraManager) GetDeviceInfo(ctx context.Context, deviceID string) (*CameraDeviceInfo, error)

// Acquire exclusive lock (required before capture/stream)
func (m *CameraManager) LockCamera(ctx context.Context, deviceID, clientID string) error

// Release lock
func (m *CameraManager) UnlockCamera(ctx context.Context, deviceID, clientID string) error

// Capture a single frame; format: "mjpeg" or "yuyv"
func (m *CameraManager) CaptureImage(ctx context.Context, deviceID string, width, height int32, format string) (*CaptureImageResult, error)

// Open a streaming session (read until io.EOF)
func (m *CameraManager) StreamFrames(ctx context.Context, req *StreamFramesRequest) (ServerStream[Frame], error)
```

```go
err := client.CameraManager.LockCamera(ctx, "/dev/video0", "myapp")
img, err := client.CameraManager.CaptureImage(ctx, "/dev/video0", 1280, 720, "mjpeg")

stream, err := client.CameraManager.StreamFrames(ctx, &StreamFramesRequest{
    DeviceId: "/dev/video0", Fps: 30, Width: 1280, Height: 720,
})
for {
    frame, err := stream.Recv()
    if err == io.EOF { break }
}
```

---

## AI Manager

On-device AI model management and inference.  
**Required permission:** `AiService/*`

`LoadModel` / `UploadAndLoadModel` return an `*AIModel` handle — inference and lifecycle operations are called on the handle.

### AIManager

```go
// List metadata for all loaded models
func (m *AIManager) ListModels() ([]*ModelInfo, error)

// Load a model already present on device filesystem
func (m *AIManager) LoadModel(modelID, modelPath string, backend ModelBackend, execution ExecutionMode) (*AIModel, error)

// Stream a local model file to the device then load it
func (m *AIManager) UploadAndLoadModel(modelID, localPath string, backend ModelBackend, execution ExecutionMode) (*AIModel, error)
```

`backend`: `ModelBackend_ONNX` | `ModelBackend_TFLITE`  
`execution`: `ExecutionMode_EXEC_CPU` | `ExecutionMode_EXEC_GPU` | `ExecutionMode_EXEC_HIGH_THREADS`

```go
model, err := client.AIManager.LoadModel(
    "detector", "/models/yolo.onnx",
    ModelBackend_ONNX, ExecutionMode_EXEC_CPU,
)

model, err := client.AIManager.UploadAndLoadModel(
    "detector", "./yolo.onnx",
    ModelBackend_ONNX, ExecutionMode_EXEC_CPU,
)
```

### AIModel

```go
// Free model from inference backend
func (m *AIModel) Unload() error

// Check if loaded + get tensor schema
func (m *AIModel) IsLoaded() (*IsModelLoadedResponse, error)

// Single synchronous forward pass
// inputShape nil = use schema from LoadModel
func (m *AIModel) Infer(ctx context.Context, inputData []byte, inputShape []int32, dtype TensorDataType) (*InferResponse, error)

// Open bidirectional inference stream
func (m *AIModel) StreamInfer(ctx context.Context) (*AIInferStream, error)
```

```go
resp, err := model.Infer(ctx, inputData, []int32{1, 3, 640, 640}, TensorDataType_TENSOR_FLOAT32)
```

### AIInferStream

```go
// Submit inference request
func (s *AIInferStream) Send(inputData []byte, inputShape []int32, dtype TensorDataType) error

// Read next result (io.EOF when server closes)
func (s *AIInferStream) Recv() (*InferResponse, error)

// Signal end of send side
func (s *AIInferStream) Close() error
```

---

## Firewall Manager

Network firewall zone and rule management.  
**Required permission:** `FirewallService/*`

### Zones

```go
func (f *FirewallManager) ListZones() ([]*ZoneRequest, error)
func (f *FirewallManager) AddZone(name string, interfaces []string, inputPolicy, outputPolicy ZonePolicy, masquerade bool) (bool, error)
func (f *FirewallManager) RemoveZone(name string) (bool, error)
```

```go
ok, err := client.FirewallManager.AddZone("lan", []string{"eth0"}, ZonePolicy_ACCEPT, ZonePolicy_ACCEPT, false)
```

### Rules

```go
func (f *FirewallManager) ListRules() ([]*FirewallRule, error)

// protocol: PROTO_ANY | PROTO_TCP | PROTO_UDP | PROTO_ICMP
// srcIP: CIDR or empty for any; destPort: 0 for any
func (f *FirewallManager) AddRule(srcZone, dstZone string, protocol FirewallProtocol, srcIP string, destPort int32, action ZonePolicy, comment string) (bool, error)

func (f *FirewallManager) RemoveRule(id string) (bool, error)
func (f *FirewallManager) FlushRules() (bool, error)      // removes all rules, zones preserved
func (f *FirewallManager) ApplyFirewall() (bool, error)   // commit pending changes
```

```go
ok, err := client.FirewallManager.AddRule("lan", "wan", FirewallProtocol_TCP, "", 443, ZonePolicy_ACCEPT, "allow HTTPS")
ok, err := client.FirewallManager.ApplyFirewall()
```

---

## VPN Manager

VPN profile management and connection control.  
**Required permission:** `VpnService/*`

### Capabilities & Profiles

```go
func (v *VPNManager) GetCapabilities() (*VpnCapabilities, error)
func (v *VPNManager) ListProfiles() ([]*VpnProfile, error)

// Create/update profile; empty ProfileId = server assigns
func (v *VPNManager) ApplyProfile(profile *VpnProfile, connectAfterApply bool) (string, error)

// WireGuard shortcut
func (v *VPNManager) ApplyWireGuard(displayName string, configData []byte, autoConnect bool) (string, error)

// OpenVPN shortcut
func (v *VPNManager) ApplyOpenVPN(displayName string, configData []byte, autoConnect bool) (string, error)

func (v *VPNManager) RemoveProfile(profileID string) (bool, error)
```

### Connection

```go
// Returns sessionID; tunnel comes up asynchronously
func (v *VPNManager) Connect(profileID string) (string, error)

// Empty profileID disconnects the active profile
func (v *VPNManager) Disconnect(profileID string) (bool, error)
```

### Status

```go
func (v *VPNManager) GetStatus() (*Session, string, error)
func (v *VPNManager) ListSessions() ([]*Session, error)
func (v *VPNManager) IsConnected() (bool, error)

// Stream tunnel events; empty profileID = all profiles
func (v *VPNManager) WatchEvents(ctx context.Context, profileID string, handler func(*VPNEvent)) error
```

```go
sessionID, err := client.VPNManager.Connect(profileID)

err := client.VPNManager.WatchEvents(ctx, "", func(e *VPNEvent) {
    fmt.Println(e.GetState())
})
```

---

## App Hub Manager

Service registration and HTTP routing via the Orbit app hub.  
**Required permission:** `AppHubService/*`

### Registration

```go
// Common case: register local HTTP server with TCP health-check
func (m *AppHubManager) RegisterWebUI(addr, route string) error

// Full control registration
func (m *AppHubManager) RegisterService(req *RegisterServiceRequest) error

func (m *AppHubManager) UnregisterService() error
```

```go
err := client.AppHubManager.RegisterWebUI("127.0.0.1:9033", "/myapp")
```

### Discovery

```go
func (m *AppHubManager) GetService(serviceID string) (*Service, error)
func (m *AppHubManager) ListServices() ([]*Service, error)
```

### Routes

```go
func (m *AppHubManager) AddRoute(path string) error
func (m *AppHubManager) RemoveRoute(path string) error
func (m *AppHubManager) GetRoutingTable() ([]*RoutingEntry, error)
```

### Events

```go
func (m *AppHubManager) WatchServices(ctx context.Context, handler func(*ServiceEvent)) error

err := client.AppHubManager.WatchServices(ctx, func(e *ServiceEvent) {
    fmt.Println(e.GetType(), e.GetService().GetServiceId())
})
```

---

## Event Manager

System-wide event bus.  
**Required permission:** `EventService/*`

### Subscribe
Subscribes to system events. Blocks until context is cancelled.

```go
func (e *EventManager) Subscribe(ctx context.Context, handler func(*Event), types ...EventType) error
```

Omit `types` to receive all events. Available event types include:  
`EVENT_APP_INSTALLED`, `EVENT_APP_REMOVED`, `EVENT_APP_STARTED`, etc.

```go
err := client.EventManager.Subscribe(ctx, func(e *Event) {
    fmt.Println(e.Type, e.Payload)
}, client.EVENT_APP_STARTED)
```

---

## Development Manager

Live log streaming from running applications.  
**Required permission:** `DevelopmentService/*`

### SubscribeLogs
Streams log entries. Blocks until context is cancelled.

```go
func (d *DevelopmentManager) SubscribeLogs(ctx context.Context, app string, tag string, level LogLevel, onEntry func(LogEntry)) error
```

`level` values: `LOG_LEVEL_DEBUG` | `LOG_LEVEL_INFO` | `LOG_LEVEL_WARNING` | `LOG_LEVEL_ERROR` | `LOG_LEVEL_FATAL`

### SubscribeLogsAsync
Starts SubscribeLogs in a goroutine with automatic reconnection. Returns a channel.

```go
func (d *DevelopmentManager) SubscribeLogsAsync(ctx context.Context, app string, tag string, level LogLevel) <-chan LogEntry
```

```go
// Empty app/tag = match all
err := client.DevelopmentManager.SubscribeLogs(ctx, "myapp", "", client.LOG_LEVEL_INFO, func(e LogEntry) {
    fmt.Println(e.Timestamp, e.Message)
})

ch := client.DevelopmentManager.SubscribeLogsAsync(ctx, "myapp", "", client.LOG_LEVEL_INFO)
```

---

## Update Manager

OTA firmware update and factory reset.  
**Required permission:** `UpdateService/*`

```go
// Send .orbit image to perform OS update
func (u *UpdateManager) Update(ctx context.Context, orbitPath string) error

// Erase all user data
func (u *UpdateManager) FactoryReset() (bool, error)
```

```go
err := client.UpdateManager.Update(ctx, "/path/to/update.orbit")
ok, err := client.UpdateManager.FactoryReset()
```

---

## Power Manager

Device power control.  
**Required permission:** `PowerService/*`

```go
func (p *PowerManager) Reboot(force bool, reason string) (*PowerResult, error)
func (p *PowerManager) Shutdown(force bool, reason string) (*PowerResult, error)
```

`PowerResult` fields: `{ Success bool, Message string }`

```go
res, err := client.PowerManager.Reboot(false, "update")
res, err := client.PowerManager.Shutdown(false, "maintenance")
```

---

## Types

### GpioPin
| Field | Type | Description |
|---|---|---|
| Name | string | Human-readable pin name (e.g. "GPIO17") |
| Number | int32 | Line offset within the chip |
| ChipNumber | int32 | Chip index (0 = /dev/gpiochip0) |

### GpioDirection
| Value | Int | Description |
|---|---|---|
| GPIO_DIR_OUT | 0 | Output |
| GPIO_DIR_IN | 1 | Input |

### GpioLevel
| Value | Int |
|---|---|
| GPIO_LEVEL_LOW | 0 |
| GPIO_LEVEL_HIGH | 1 |

### UartConfig
| Field | Type | Description |
|---|---|---|
| Port | string | Port name (e.g. "ttyS0", "ttyAMA0") |
| Baudrate | int | e.g. 9600, 115200 |
| DataBits | int | 5, 6, 7 or 8 |
| Parity | UartParity | None / Even / Odd |
| StopBits | UartStopBits | 1 or 2 stop bits |
| FlowControl | UartFlowControl | None / Hardware (RTS/CTS) / Software (XON/XOFF) |

### UartParity
`UartParityNone=0`, `UartParityEven=1`, `UartParityOdd=2`

### UartStopBits
`UartStopBits1=0`, `UartStopBits2=1`

### UartFlowControl
`UartFlowNone=0`, `UartFlowHardware=1`, `UartFlowSoftware=2`

### I2CConfig
| Field | Type |
|---|---|
| Bus | uint32 |
| ClockHz | uint32 |
| TenBitAddr | bool |
| ClockStretching | bool |

### SpiConfig
| Field | Type | Notes |
|---|---|---|
| Bus | uint32 | |
| ChipSelect | uint32 | |
| MaxSpeedHz | uint32 | |
| BitsPerWord | uint32 | |
| Mode | int | 0–3 (CPOL/CPHA) |
| LSBFirst | bool | |

### MetricsInfoResponse
| Field | Type |
|---|---|
| Metrics | *MetricsInfo |
| Error | *ErrorInfo |

### MetricsInfo
| Field | Type | Description |
|---|---|---|
| SysUptime | *MetricsInfo_SysUptime | Uptime and load |
| CpuUsage | float64 | Overall CPU usage (0–100%) |
| CpuCorePercent | []float64 | Per-core CPU usage |
| MemoryUsage | float64 | Memory usage % |
| SocThermal | float64 | SoC temperature (°C) |
| CpuStats | *MetricsInfo_CpuStats | Context-switch and interrupt counters |
| CpuFreq | *MetricsInfo_CpuFreq | CPU frequency info (MHz) |
| VirtualMemory | *MetricsInfo_VirtualMemory | Memory breakdown |
| DiskUsage | map[string]*MetricsInfo_DiskUsage | Per mount point |
| DiskIoCounters | *MetricsInfo_DiskIoCounters | Disk I/O counters |
| Network | *MetricsInfo_Network | Network I/O counters |

### MetricsInfo_SysUptime
`Time string`, `Uptime int64` (seconds), `Users int32`, `LoadAverage []float64` (1/5/15 min)

### MetricsInfo_VirtualMemory
All sizes in bytes: `Total`, `Available`, `Used`, `Free`, `Active`, `Inactive`, `Buffers`, `Cached`, `Shared`, `Slab int64`, `Percent float64`

### MetricsInfo_DiskUsage
`Total`, `Used`, `Free int64`, `Percent float64`

### MetricsInfo_Network
`BytesSent`, `BytesRecv`, `PacketsSent`, `PacketsRecv`, `Errin`, `Errout`, `Dropin`, `Dropout int64`

### EthernetLinkProperties
| Field | Type | Description |
|---|---|---|
| InterfaceName | string | e.g. "eth0" |
| MacAddress | string | |
| State | EthernetState | |
| Mtu | int32 | |
| DhcpEnable | bool | |
| Ipv4Address | string | CIDR (e.g. "192.168.1.10/24") |
| Ipv4Gateway | string | |
| Ipv4Dns | []string | |

### EthernetState
`ETH_UNKNOWN=0`, `ETH_UNMANAGED=10`, `ETH_UNAVAILABLE=20`, `ETH_DISCONNECTED=30`, `ETH_PREPARE=40`, `ETH_CONFIG=50`, `ETH_NEED_AUTH=60`, `ETH_IP_CONFIG=70`, `ETH_IP_CHECK=80`, `ETH_SECONDARIES=90`, `ETH_CONNECTED=100`, `ETH_DEACTIVATING=110`, `ETH_FAILED=120`

### WiFiLinkProperties
| Field | Type |
|---|---|
| InterfaceName | string |
| MacAddress | string |
| Mode | WiFiMode |
| State | WiFiState |
| Mtu | int32 |
| ApProperties | *APProperties |
| ClientProperties | *ClientProperties |

### ClientProperties
`InterfaceName`, `MacAddress`, `Ssid`, `Bssid`, `Security`, `Band string`, `State WiFiState`, `Mtu`, `SignalStrength`, `Channel int32`, `DhcpEnable bool`, `Ipv4Address`, `Ipv4Gateway string`, `Ipv4Dns []string`

### APProperties
`InterfaceName`, `Ssid`, `Security`, `Ipv4Address`, `Band string`, `Channel int32`, `Hidden`, `Active bool`, `ConnectedClients int32`

### ScannedNetwork
`Ssid`, `Bssid`, `Security`, `Band string`, `SignalStrength`, `Channel int32`, `Hidden bool`

### WiFiMode
`WIFI_MODE_UNKNOWN=0`, `WIFI_MODE_AP=1`, `WIFI_MODE_CLIENT=2`, `WIFI_MODE_AP_CLIENT=3`, `WIFI_MODE_DISABLED=4`

### WiFiState
`WIFI_UNKNOWN=0`, `WIFI_UNMANAGED=10`, `WIFI_UNAVAILABLE=20`, `WIFI_DISCONNECTED=30`, `WIFI_PREPARE=40`, `WIFI_CONFIG=50`, `WIFI_NEED_AUTH=60`, `WIFI_IP_CONFIG=70`, `WIFI_IP_CHECK=80`, `WIFI_SECONDARIES=90`, `WIFI_ACTIVATED=100`, `WIFI_DEACTIVATING=110`, `WIFI_FAILED=120`

### ModelBackend
`MODEL_BACKEND_UNSPECIFIED=0`, `ONNX=1`, `TFLITE=2`

### ExecutionMode
`EXEC_CPU=0`, `EXEC_GPU=1`, `EXEC_HIGH_THREADS=2`

### TensorDataType
| Value | Int | Description |
|---|---|---|
| TENSOR_FLOAT32 | 0 | 4-byte IEEE 754, normalised [0,1] |
| TENSOR_UINT8 | 1 | 1-byte unsigned [0,255] |
| TENSOR_INT32 | 2 | 4-byte signed integer |
| TENSOR_INT64 | 3 | 8-byte signed integer |

### AIModel (handle)
`Response *LoadModelResponse` — contains tensor schema from load time.

### LoadModelResponse
`ModelId string`, `Success bool`, `Error ErrorInfo`, `Inputs []*TensorInfo`, `Outputs []*TensorInfo`, `SkippedUpload bool`

### ModelInfo
`ModelId`, `Version string`, `Backend ModelBackend`, `LoadedAtUnix int64`

### IsModelLoadedResponse
`Loaded bool`, `Inputs []*TensorInfo`, `Outputs []*TensorInfo`

### InferResponse
`Success bool`, `Error ErrorInfo`, `OutputData []byte`, `OutputShape []int32`, `NamedOutputs map[string][]byte`, `LatencyUs int64`

### TensorInfo
`Name string`, `Dtype TensorDataType`, `Shape []int32`

### CameraDeviceInfo
`DeviceID`, `Driver`, `Card string`, `SupportedFormats []string` (e.g. "mjpeg", "yuyv"), `Resolutions []string` (e.g. "1280x720")

### CaptureImageResult
`ImageData []byte`, `Format string`, `Timestamp int64` (Unix µs)

### StreamFramesRequest
`DeviceId string`, `Fps int32`, `Width int32`, `Height int32`

### Frame
`Data []byte`, `Timestamp int64` (Unix µs), `Width int32`, `Height int32`, `Format string`

### LogLevel
`LOG_LEVEL_DEBUG=0`, `LOG_LEVEL_INFO=1`, `LOG_LEVEL_WARNING=2`, `LOG_LEVEL_ERROR=3`, `LOG_LEVEL_FATAL=4`

### PwmChannel
`Channel uint32`, `Name string`

### PwmProperties
`Channel PwmChannel`, `Enabled bool`, `DutyCycle float64` (0.0–1.0), `FrequencyHz float64`

### VpnProfile
`ProfileId string` (empty = server assigns), `DisplayName string`, `Provider VpnProvider`, `ConfigData []byte`, `AutoConnect bool`, `SecretRef string`

### VpnProvider
`VPN_PROVIDER_WIREGUARD=1`, `VPN_PROVIDER_OPENVPN=2`, `VPN_PROVIDER_IPSEC=3`, `VPN_PROVIDER_CUSTOM=15`

### VPNEvent
`TsUnixMs int64`, `ProfileId string`, `SessionId string`, `Provider VpnProvider`, `Tunnel TunnelStateChanged`, `ProviderEvent ProviderNotification`

### TunnelStateChanged
`NewState string` (`DOWN` | `CONNECTING` | `UP` | `DEGRADED` | `ERROR`), `Message string`

### RegisterServiceRequest
`Host string`, `Port int32`, `Routes []*Route`, `Health *HealthCheck`, `ExposureMode string`

### FirewallProtocol
`PROTO_ANY=0`, `PROTO_TCP=1`, `PROTO_UDP=2`, `PROTO_ICMP=3`

### ZonePolicy
`POLICY_ACCEPT=0`, `POLICY_DROP=1`, `POLICY_REJECT=2`

### ZoneRequest
`Name string`, `Interfaces []string`, `InputPolicy ZonePolicy`, `OutputPolicy ZonePolicy`, `Masquerade bool`

### FirewallRule
`Id string`, `SrcZone string`, `DstZone string`, `Protocol FirewallProtocol`, `SrcIp string`, `DestPort int32`, `Action ZonePolicy`, `Comment string`
