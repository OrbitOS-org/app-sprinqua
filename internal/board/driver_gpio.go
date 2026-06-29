package board

import (
	"fmt"

	"github.com/OrbitOS-org/sdk-go/v26/client"
)

// gpioDriver is the RelayDriver for boards wired directly to Raspberry Pi
// GPIO pins.
type gpioDriver struct {
	gpio  *client.GpioManager
	board *Board
}

// NewGPIODriver builds the RelayDriver for a GPIO-wired board.
func NewGPIODriver(gpio *client.GpioManager, b *Board) RelayDriver {
	return &gpioDriver{gpio: gpio, board: b}
}

func (d *gpioDriver) SetChannel(channel int, on bool) error {
	pin := d.board.PinByChannel(channel)
	if pin == nil {
		return fmt.Errorf("no pin for channel %d", channel)
	}
	var level client.GpioLevel
	if d.board.ActiveLow {
		if on {
			level = client.GPIO_LEVEL_LOW
		} else {
			level = client.GPIO_LEVEL_HIGH
		}
	} else {
		if on {
			level = client.GPIO_LEVEL_HIGH
		} else {
			level = client.GPIO_LEVEL_LOW
		}
	}
	// SetLevel opens the GPIO line via gpiocdev.RequestLine(chip, offset,
	// AsOutput(val)) with the correct initial level in one atomic call — no
	// separate SetDirection call is needed, and calling one first would
	// always initialise to LOW (AsOutput(0)), which would briefly activate
	// relays on ActiveLow boards.
	return d.gpio.SetLevel(pin, level)
}
