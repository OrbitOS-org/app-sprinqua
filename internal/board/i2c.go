package board

import "github.com/OrbitOS-org/sdk-go/v26/client"

// RelayDriver switches a single relay channel on or off, with whatever
// bus/pins/address it needs already bound. Implemented by each board kind
// (see driver_gpio.go, driver_ep0099.go) — callers go through
// ChannelManager.Open instead of constructing one directly.
type RelayDriver interface {
	SetChannel(channel int, on bool) error
}

// OpenI2CDriver opens the I2C bus configured for board b (standard
// 100kHz/7-bit-address settings) and returns a RelayDriver ready to switch
// its channels.
func OpenI2CDriver(mgr *client.I2CManager, b *Board) (RelayDriver, error) {
	bus, err := mgr.Open(b.I2CBus, 100000, false, false)
	if err != nil {
		return nil, err
	}
	return b.I2CNewDriver(bus), nil
}
