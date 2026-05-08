package board

import "github.com/OrbitOS-org/sdk-go/v26/client"

// Channel maps a relay channel number to a GPIO pin.
type Channel struct {
	Number int
	Pin    *client.GpioPin
}

// Board describes a supported relay board.
type Board struct {
	ID          string
	Name        string
	Description string
	Channels    int
	ActiveLow   bool // LOW signal activates the relay
	Pins        []Channel
}

// PinByChannel returns the GPIO pin for the given 1-based channel number.
func (b *Board) PinByChannel(ch int) *client.GpioPin {
	for _, c := range b.Pins {
		if c.Number == ch {
			return c.Pin
		}
	}
	return nil
}

// All is the registry of supported relay boards.
var All = []*Board{
	{
		ID:          "keyestudio-4ch",
		Name:        "Keyestudio RPI 4-Channel Relay",
		Description: "4-channel relay board for Raspberry Pi via direct GPIO.",
		Channels:    4,
		ActiveLow:   false,
		Pins: []Channel{
			{Number: 1, Pin: &client.GpioPin{Name: "GPIO4"}},
			{Number: 2, Pin: &client.GpioPin{Name: "GPIO22"}},
			{Number: 3, Pin: &client.GpioPin{Name: "GPIO6"}},
			{Number: 4, Pin: &client.GpioPin{Name: "GPIO26"}},
		},
	},
}

// Find returns the board with the given ID, or nil.
func Find(id string) *Board {
	for _, b := range All {
		if b.ID == id {
			return b
		}
	}
	return nil
}
