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
	SKU         string
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
		SKU:         "KS0212",
		Channels:    4,
		ActiveLow:   false,
		Pins: []Channel{
			{Number: 1, Pin: &client.GpioPin{Name: "GPIO4"}},
			{Number: 2, Pin: &client.GpioPin{Name: "GPIO22"}},
			{Number: 3, Pin: &client.GpioPin{Name: "GPIO6"}},
			{Number: 4, Pin: &client.GpioPin{Name: "GPIO26"}},
		},
	},
	{
		ID:          "waveshare-8ch",
		Name:        "Waveshare RPi 8-Channel Relay",
		Description: "8-channel relay HAT for Raspberry Pi via direct GPIO.",
		SKU:         "15423",
		Channels:    8,
		ActiveLow:   false,
		Pins: []Channel{
			{Number: 1, Pin: &client.GpioPin{Name: "GPIO5"}},
			{Number: 2, Pin: &client.GpioPin{Name: "GPIO6"}},
			{Number: 3, Pin: &client.GpioPin{Name: "GPIO13"}},
			{Number: 4, Pin: &client.GpioPin{Name: "GPIO16"}},
			{Number: 5, Pin: &client.GpioPin{Name: "GPIO19"}},
			{Number: 6, Pin: &client.GpioPin{Name: "GPIO20"}},
			{Number: 7, Pin: &client.GpioPin{Name: "GPIO21"}},
			{Number: 8, Pin: &client.GpioPin{Name: "GPIO26"}},
		},
	},
	{
		ID:          "waveshare-pi0-6ch",
		Name:        "Waveshare RPi Zero 6-ch Relay",
		Description: "6-channel Industrial Relay Module for Raspberry Pi Zero.",
		SKU:         "20863",
		Channels:    6,
		ActiveLow:   false,
		Pins: []Channel{
			{Number: 1, Pin: &client.GpioPin{Name: "GPIO5"}},
			{Number: 2, Pin: &client.GpioPin{Name: "GPIO6"}},
			{Number: 3, Pin: &client.GpioPin{Name: "GPIO13"}},
			{Number: 4, Pin: &client.GpioPin{Name: "GPIO16"}},
			{Number: 5, Pin: &client.GpioPin{Name: "GPIO19"}},
			{Number: 6, Pin: &client.GpioPin{Name: "GPIO20"}},
		},
	},
	{
		ID:          "waveshare-3ch",
		Name:        "Waveshare RPi 3-Channel Relay",
		Description: "3-channel relay HAT for Raspberry Pi via direct GPIO.",
		SKU:         "11638",
		Channels:    3,
		ActiveLow:   false,
		Pins: []Channel{
			{Number: 1, Pin: &client.GpioPin{Name: "GPIO26"}},
			{Number: 2, Pin: &client.GpioPin{Name: "GPIO20"}},
			{Number: 3, Pin: &client.GpioPin{Name: "GPIO21"}},
		},
	},
	{
		ID:          "seengreat-3ch",
		Name:        "Seengreat 3-CH Relay HAT",
		Description: "3-channel relay expansion board for Raspberry Pi via direct GPIO.",
		SKU:         "250509",
		Channels:    3,
		ActiveLow:   false,
		Pins: []Channel{
			{Number: 1, Pin: &client.GpioPin{Name: "GPIO26"}},
			{Number: 2, Pin: &client.GpioPin{Name: "GPIO19"}},
			{Number: 3, Pin: &client.GpioPin{Name: "GPIO13"}},
		},
	},
	{
		ID:          "seengreat-4ch",
		Name:        "Seengreat 4-CH Relay HAT",
		Description: "4-channel relay expansion board for Raspberry Pi via direct GPIO.",
		SKU:         "220741",
		Channels:    4,
		ActiveLow:   false,
		Pins: []Channel{
			{Number: 1, Pin: &client.GpioPin{Name: "GPIO26"}},
			{Number: 2, Pin: &client.GpioPin{Name: "GPIO19"}},
			{Number: 3, Pin: &client.GpioPin{Name: "GPIO13"}},
			{Number: 4, Pin: &client.GpioPin{Name: "GPIO6"}},
		},
	},
	{
		ID:          "seengreat-8ch",
		Name:        "Seengreat 8-CH Relay Board",
		Description: "8-channel optocoupler-isolated relay expansion board for Raspberry Pi, 5–12V wide voltage input.",
		SKU:         "260115",
		Channels:    8,
		ActiveLow:   false,
		Pins: []Channel{
			{Number: 1, Pin: &client.GpioPin{Name: "GPIO6"}},
			{Number: 2, Pin: &client.GpioPin{Name: "GPIO13"}},
			{Number: 3, Pin: &client.GpioPin{Name: "GPIO19"}},
			{Number: 4, Pin: &client.GpioPin{Name: "GPIO26"}},
			{Number: 5, Pin: &client.GpioPin{Name: "GPIO12"}},
			{Number: 6, Pin: &client.GpioPin{Name: "GPIO16"}},
			{Number: 7, Pin: &client.GpioPin{Name: "GPIO20"}},
			{Number: 8, Pin: &client.GpioPin{Name: "GPIO21"}},
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
