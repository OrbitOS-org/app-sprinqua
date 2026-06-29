package board

import "github.com/OrbitOS-org/sdk-go/v26/client"

// ChannelManager opens the right RelayDriver for a board — GPIO or I2C —
// so callers don't need to branch on Board.Kind themselves.
type ChannelManager struct {
	gpio *client.GpioManager
	i2c  *client.I2CManager
}

// NewChannelManager wraps the GPIO and I2C managers behind one entry point.
func NewChannelManager(gpio *client.GpioManager, i2c *client.I2CManager) *ChannelManager {
	return &ChannelManager{gpio: gpio, i2c: i2c}
}

// Open returns a RelayDriver ready to switch b's channels on or off.
func (m *ChannelManager) Open(b *Board) (RelayDriver, error) {
	if b.Kind == KindI2C {
		return OpenI2CDriver(m.i2c, b)
	}
	return NewGPIODriver(m.gpio, b), nil
}
