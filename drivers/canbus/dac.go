package canbus

import (
	"github.com/go-lsst/ncs"
)

type DAC struct {
	*ncs.Base
	node   int
	serial string
	bus    Bus
}

func (dac *DAC) Node() int {
	return dac.node
}

func (dac *DAC) Serial() string {
	return dac.serial
}

func (dac *DAC) init() error {
	var err error
	return err
}

func NewDAC(name, serial string) *DAC {
	return &DAC{
		Base:   ncs.NewBase(name),
		serial: serial,
	}
}
