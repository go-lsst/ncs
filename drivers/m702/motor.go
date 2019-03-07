// Package m702 provides r/w access to registers of M702 unidrive motors.
package m702

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/goburrow/modbus"
)

const (
	enable32bits = 0x4000 // enables the 32b mode of M702 modbus interface
	nregs        = 2      // number of 16b registers to read/write
)

// Parameter is a menu parameter in the M702 unidrive manual.
type Parameter struct {
	Index  [3]int
	Title  string
	DefVal string
	RW     bool
	Data   [4]byte
}

// MBReg returns the (32b) modbus register value corresponding to this parameter.
func (p *Parameter) MBReg() uint16 {
	return uint16(p.Index[1]*100 + p.Index[2] - 1 + enable32bits)
}

func (p Parameter) String() string {
	return fmt.Sprintf("%02d.%02d.%03d", p.Index[0], p.Index[1], p.Index[2])
}

// NewParameter creates a parameter from a [slot.]menu.index string.
func NewParameter(menu string) (Parameter, error) {
	var err error
	var p Parameter

	var (
		slot = 0
		m    = 0
		i    = 0
	)

	toks := strings.Split(menu, ".")
	itoks := make([]int, len(toks))
	for j, tok := range toks {
		v, err := strconv.Atoi(tok)
		if err != nil {
			return p, err
		}
		itoks[j] = v
	}

	switch len(itoks) {
	case 2:
		m = itoks[0]
		i = itoks[1]
	case 3:
		slot = itoks[0]
		m = itoks[1]
		i = itoks[2]
	default:
		return p, fmt.Errorf(
			"m702: invalid menu value (too many/too few dots) [pr=%s]",
			menu,
		)
	}

	if slot > 4 || slot < 0 {
		return p, fmt.Errorf(
			"m702: invalid slot value (%d) [pr=%s]",
			slot,
			menu,
		)
	}

	if m > 162 {
		return p, fmt.Errorf("m702: invalid menu value (%d>162) [pr=%s]", m, menu)
	}

	if i >= 100 {
		return p, fmt.Errorf("m702: invalid index value (%d>=100) [pr=%s]", i, menu)
	}

	p.Index = [3]int{slot, m, i}

	return p, err
}

// Motor represents a M702 unidrive motor.
type Motor struct {
	Addr    string
	Timeout time.Duration
}

// New returns a new M702 motor.
func New(addr string) Motor {
	return Motor{
		Addr:    addr,
		Timeout: 5 * time.Second,
	}
}

func (m *Motor) client(slave byte) *modbus.TCPClientHandler {
	c := modbus.NewTCPClientHandler(m.Addr)
	c.SlaveId = slave
	c.Timeout = m.Timeout
	return c
}

// ReadParam reads parameter p's value from the motor.
func (m *Motor) ReadParam(p *Parameter) error {
	c := m.client(byte(p.Index[0]))
	defer c.Close()

	cli := modbus.NewClient(c)
	o, err := cli.ReadHoldingRegisters(p.MBReg(), nregs)
	if err != nil {
		return err
	}
	copy(p.Data[:], o)
	return err
}

// WriteParam writes parameter p's value to the motor.
func (m *Motor) WriteParam(p Parameter) error {
	c := m.client(byte(p.Index[0]))
	defer c.Close()

	o, err := modbus.NewClient(c).WriteMultipleRegisters(p.MBReg(), nregs, p.Data[:])
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
