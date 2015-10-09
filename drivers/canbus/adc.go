package canbus

import (
	"github.com/go-lsst/ncs"
)

const (
	adcVoltsPerBit  = 0.3125 * 1e-3 // in Volts
	waterFreezeTemp = 273.15
)

type ADC struct {
	*ncs.Base
	node   int
	serial string
	tx     int
	bus    Bus
}

func (adc *ADC) Node() int {
	return adc.node
}

func (adc *ADC) Serial() string {
	return adc.serial
}

func (adc *ADC) Tx() int {
	return adc.tx
}

func (*ADC) Volts(adc int) float64 {
	return float64(adc) * adcVoltsPerBit
}

func (adc *ADC) init() error {
	var err error
	return err
}

func NewADC(name, serial string, tx int) *ADC {
	return &ADC{
		Base:   ncs.NewBase(name),
		serial: serial,
		tx:     tx,
	}
}
