// Package m702 provides r/w access to registers of M702 unidrive motors.
package m702

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/goburrow/modbus"
)

const (
	enable32bits = 0x4000 // enables the 32b mode of M702 modbus interface
	nregs        = 2      // number of 16b registers to read/write
)

// Parameter is a menu parameter in the M702 unidrive manual.
type Parameter struct {
	Index  [2]int
	Title  string
	DefVal string
	RW     bool
	Data   [4]byte
}

// MBReg returns the (32b) modbus register value corresponding to this parameter.
func (p *Parameter) MBReg() uint16 {
	return uint16(p.Index[0]*100 + p.Index[1] - 1 + enable32bits)
}

func (p Parameter) String() string {
	return fmt.Sprintf("%02d.%03d", p.Index[0], p.Index[1])
}

// NewParameter creates a parameter from its modbus register.
func NewParameter(reg uint16) Parameter {
	return Parameter{
		Index: [2]int{int(reg / 100), int(reg%100) + 1},
	}
}

// NewParameterFromMenu creates a parameter from a menu.index string.
func NewParameterFromMenu(menu string) (Parameter, error) {
	var err error
	var p Parameter

	toks := strings.Split(menu, ".")
	m, err := strconv.Atoi(toks[0])
	if err != nil {
		return p, err
	}
	if m > 162 {
		return p, fmt.Errorf("m702: invalid menu value (%d>162) [pr=%s]", m, menu)
	}

	i, err := strconv.Atoi(toks[1])
	if err != nil {
		return p, err
	}
	if i >= 100 {
		return p, fmt.Errorf("m702: invalid index value (%d>=100) [pr=%s]", i, menu)
	}

	p.Index = [2]int{m, i}

	return p, err
}

// Motor represents a M702 unidrive motor.
type Motor struct {
	Addr string
	c    modbus.Client
}

// New returns a new M702 motor.
func New(addr string) Motor {
	return Motor{
		Addr: addr,
		c:    modbus.TCPClient(addr),
	}
}

// ReadParam reads parameter p's value from the motor.
func (m *Motor) ReadParam(p *Parameter) error {
	o, err := m.c.ReadHoldingRegisters(p.MBReg(), nregs)
	if err != nil {
		return err
	}
	copy(p.Data[:], o)
	return err
}

// WriteParam writes parameter p's value to the motor.
func (m *Motor) WriteParam(p Parameter) error {
	o, err := m.c.WriteMultipleRegisters(p.MBReg(), nregs, p.Data[:])
	if err != nil {
		return err
	}
	if o[1] != nregs {
		return fmt.Errorf(
			"m702: invalid write at Pr-%v. expected %d, got %d",
			p, nregs, o[1],
		)
	}
	return err
}
