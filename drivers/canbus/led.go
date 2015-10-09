package canbus

import (
	"fmt"
	"time"

	"github.com/go-lsst/ncs"
	"golang.org/x/net/context"
)

type LED struct {
	*ncs.Base
	bus Bus
	dac *DAC

	cid uint8 // channel index on DAC
}

func (led *LED) Boot(ctx context.Context) error {
	var err error
	led.dac = led.bus.DAC()
	return err
}

func (led *LED) Start(ctx context.Context) error {
	return nil
}

func (led *LED) Stop(ctx context.Context) error {
	var err error
	err = led.TurnOff()
	if err != nil {
		return err
	}

	return err
}

func (led *LED) Shutdown(ctx context.Context) error {
	return nil
}

func (led *LED) Tick(ctx context.Context) error {
	led.Debugf("tick...\n")
	var err error

	err = led.TurnOn()
	if err != nil {
		led.Errorf("error turning LED ON: %v\n", err)
		return err
	}

	time.Sleep(500 * time.Millisecond)

	err = led.TurnOff()
	if err != nil {
		led.Errorf("error turning LED OFF: %v\n", err)
		return err
	}
	return err
}

func (led *LED) TurnOn() error {
	return led.write(0x14000)
}

func (led *LED) TurnOff() error {
	return led.write(0x0)
}

func (led *LED) write(value uint32) error {
	var err error
	const subchannel = 0x2
	cmd, err := led.bus.Send(Command{
		Name: Wsdo,
		Data: []byte(fmt.Sprintf("%x,%x,%x,%x,%x",
			led.dac.Node(),
			0x6411,
			led.cid,
			subchannel,
			value,
		)),
	})
	if err != nil {
		return err
	}
	return cmd.Err()
}

func NewLED(name string, bus string) *LED {
	busdev := ncs.System.Device(bus)
	led := &LED{
		Base: ncs.NewBase(name),
		bus:  busdev.(Bus),
		cid:  0x1,
	}
	return led
}
