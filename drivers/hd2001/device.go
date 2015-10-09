package hd2001

import (
	"bytes"
	"fmt"

	"github.com/go-lsst/ncs"
	"github.com/go-lsst/ncs/drivers/canbus"
	"golang.org/x/net/context"
)

type Device struct {
	*ncs.Base
	bus canbus.Bus
	adc *canbus.ADC

	// offsets of sensors in ADC-range
	offsets struct {
		H float64 // hygrometry
		P float64 // pressure
		T float64 // temperature
	}

	// channel indices on ADC
	chans struct {
		H uint8 // hygrometry
		P uint8 // pressure
		T uint8 // temperature
	}
}

func (dev *Device) Boot(ctx context.Context) error {
	return dev.Base.Boot(ctx)
}

func (dev *Device) Start(ctx context.Context) error {
	dev.adc = dev.bus.ADC()
	return nil
}

func (dev *Device) Stop(ctx context.Context) error {
	var err error
	return err
}

func (dev *Device) Shutdown(ctx context.Context) error {
	return dev.Base.Shutdown(ctx)
}

func (dev *Device) Tick(ctx context.Context) error {
	dev.Debugf("tick...\n")
	var err error

	temp, err := dev.Temperature()
	if err != nil {
		dev.Errorf("error reading temperature: %v\n", err)
		return err
	}
	dev.Debugf("temperature=%v C\n", temp)

	hygro, err := dev.Hygrometry()
	if err != nil {
		dev.Errorf("error reading hygrometry: %v\n", err)
		return err
	}
	dev.Debugf("hygro=%v%%\n", hygro)

	press, err := dev.Pressure()
	if err != nil {
		dev.Errorf("error reading pressure: %v\n", err)
		return err
	}
	dev.Debugf("press=%v mbar\n", press)

	err = dev.Send([]byte(fmt.Sprintf(
		"hygro=%v%%; press=%vmbar; temp=%vC",
		hygro,
		press,
		temp,
	)))
	if err != nil {
		dev.Errorf("error sending sensor data: %v\n", err)
		return err
	}

	return err
}

func New(name string, bus string) *Device {
	busdev := ncs.System.Device(bus)
	dev := &Device{
		Base: ncs.NewBase(name),
		bus:  busdev.(canbus.Bus),
	}
	dev.offsets.H = 0.0
	dev.offsets.P = 600.0
	dev.offsets.T = -20.0

	dev.chans.H = 0x3
	dev.chans.P = 0x2
	dev.chans.T = 0x4
	return dev
}

func (dev *Device) Hygrometry() (float64, error) {
	volts, err := dev.readVolts("hygrometry", dev.chans.H)
	if err != nil {
		return 0, err
	}
	hygro := (volts * 10) + dev.offsets.H
	return hygro, err
}

func (dev *Device) Pressure() (float64, error) {
	volts, err := dev.readVolts("pressure", dev.chans.P)
	if err != nil {
		return 0, err
	}
	p := (volts * 50) + dev.offsets.P
	return p, err
}

func (dev *Device) Temperature() (float64, error) {
	volts, err := dev.readVolts("temperature", dev.chans.T)
	if err != nil {
		return 0, err
	}
	temp := (volts * 10) + dev.offsets.T
	return temp, err
}

func (dev *Device) readVolts(name string, id uint8) (float64, error) {
	const nretries = 3
	for i := 0; i < nretries; i++ {
		var err error
		cmd, err := dev.bus.Send(canbus.Command{
			Name: canbus.Rsdo,
			Data: []byte(fmt.Sprintf("%x,%x,%x", dev.adc.Node(), 0x6401, id)),
		})
		if err != nil {
			return 0, err
		}
		if cmd.Err() != nil {
			return 0, cmd.Err()
		}

		volts := 0.0

		switch cmd.Name {
		case canbus.Rsdo:
			node := 0
			ecode := 0
			adc := 0
			if bytes.Count(cmd.Data, []byte(",")) < 2 {
				dev.Debugf("got invalid command: %v\n", cmd)
				continue
			}
			_, err = fmt.Fscanf(
				bytes.NewReader(cmd.Data),
				"%x,%x,%x",
				&node,
				&ecode,
				&adc,
			)
			if err != nil {
				dev.Errorf("error decoding %s: %v (cmd=%v)\n", name, err, cmd)
				return 0, err
			}
			if ecode != 0 {
				dev.Errorf("canbus error: %v\n", ecode)
				return 0, canbus.Error{ecode}
			}
			if node != dev.adc.Node() {
				return 0, fmt.Errorf(
					"error cross-talk: got node=%d, want=%d",
					node,
					dev.adc.Node(),
				)
			}
			volts = dev.adc.Volts(adc)
		default:
			return 0, fmt.Errorf(
				"unexpected canbus command: %q (cmd=%v)",
				cmd.Name,
				cmd,
			)
		}
		return volts, err
	}
	return 0, fmt.Errorf("could not read volts from ADC")
}
