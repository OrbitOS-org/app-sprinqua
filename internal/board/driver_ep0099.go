package board

import "github.com/OrbitOS-org/sdk-go/v26/client"

// I2C relay command bytes for the 52Pi EP-0099, confirmed against:
//
//	i2cset -y <bus> 0x10 <channel> 0xFF   # relay on
//	i2cset -y <bus> 0x10 <channel> 0x00   # relay off
//
// The channel number doubles as the device register — no separate bitmask
// or channel→register table is needed for this board. Other I2C boards may
// use a completely different protocol (e.g. a single bitmask register) —
// each gets its own driver type and file, referenced from its registry
// entry's I2CNewDriver field, rather than sharing this one.
const (
	ep0099RelayOn  byte = 0xFF
	ep0099RelayOff byte = 0x00
)

// ep0099BaseAddr is the EP-0099's default I2C address — not user-configurable
// anywhere in Sprinqua, so it lives here rather than in the board registry.
const ep0099BaseAddr uint32 = 0x10

// ep0099ChannelsPerBoard is fixed by the hardware: up to 4 EP-0099 boards can
// be stacked on the same bus at consecutive addresses (0x10, 0x11, 0x12,
// 0x13), each exposing 4 relays. Channels 1-4 map to the first board, 5-8 to
// the second, and so on.
const ep0099ChannelsPerBoard = 4

// ep0099Driver is a RelayDriver bound to a stack of EP-0099s starting at
// ep0099BaseAddr.
type ep0099Driver struct {
	bus *client.I2CBus
}

// NewEP0099Driver builds the RelayDriver for one or more stacked EP-0099s.
func NewEP0099Driver(bus *client.I2CBus) RelayDriver {
	return &ep0099Driver{bus: bus}
}

func (d *ep0099Driver) SetChannel(channel int, on bool) error {
	addr, register := ep0099Target(channel)
	val := ep0099RelayOff
	if on {
		val = ep0099RelayOn
	}
	_, err := d.bus.Transfer(addr, []byte{register, val}, 0, 0)
	return err
}

// ep0099Target maps a 1-based channel number to the I2C address of the
// specific board in the stack that owns it, and that board's own register
// for the channel (always 1-4, regardless of stack position).
func ep0099Target(channel int) (addr uint32, register byte) {
	boardOffset := (channel - 1) / ep0099ChannelsPerBoard
	localChannel := (channel-1)%ep0099ChannelsPerBoard + 1
	return ep0099BaseAddr + uint32(boardOffset), byte(localChannel)
}
