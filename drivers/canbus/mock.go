package canbus

import (
	"bytes"
	"fmt"
	"io"

	"github.com/go-lsst/ncs"
	"github.com/gonuts/logger"
)

func NewMock(name string, port int, adc *ADC, dac *DAC, devices ...ncs.Device) ncs.Module {
	bus := New(name, port, adc, dac, devices...)
	bus.(*busImpl).conn = newCwrapperMock()
	return bus
}

type cwrapperMock struct {
	quit chan struct{}
	conn chan []byte
}

func newCwrapperMock() *cwrapperMock {
	return &cwrapperMock{
		quit: make(chan struct{}),
		conn: make(chan []byte),
	}
}

func (c *cwrapperMock) init(msg *logger.Logger) error {
	var err error
	go c.run()
	return err
}

func (c *cwrapperMock) run() {
	c.conn <- []byte("TestBench ISO-8859-1\n")
	for _, node := range []int{0x41, 0x42} {
		c.conn <- Command{
			Name: Boot,
			Data: []byte(fmt.Sprintf("%x", node)),
		}.bytes()
	}

	type Node struct {
		id       int
		device   int
		vendor   int
		product  int
		revision int
		serial   string
	}

	for _, node := range []Node{
		{
			id:     0x41,
			device: 262545, vendor: 23, product: 587333634, revision: 65538,
			serial: "c7c80499",
		},
		{
			id:     0x42,
			device: 524689, vendor: 23, product: 587464706, revision: 1,
			serial: "c7c60327",
		},
	} {
		c.conn <- Command{
			Name: Info,
			Data: []byte(fmt.Sprintf(
				"%x,%x,%x,%x,%x,%s",
				node.id,
				node.device,
				node.vendor,
				node.product,
				node.revision,
				node.serial,
			)),
		}.bytes()
	}
}

func (c *cwrapperMock) Read(data []byte) (int, error) {
	buf := <-c.conn
	n := len(buf)
	if n > len(data) {
		n = len(data)
	}
	copy(data[:n], buf[:n])
	return n, nil
}

func (c *cwrapperMock) Write(data []byte) (int, error) {
	var err error
	ocmd := Command{}
	icmd := newCommand(data)
	switch icmd.Name {
	case Boot:
	case Info:
		return 0, nil
	case Quit:
		return 0, io.EOF
	case Rsdo:
		var node int
		var idx int
		var sub int
		_, err = fmt.Fscanf(bytes.NewReader(icmd.Data),
			"%x,%x,%x",
			&node,
			&idx,
			&sub,
		)
		if err != nil {
			return -1, err
		}
		ocmd = Command{
			Name: Rsdo,
			Data: []byte(fmt.Sprintf(
				"%x,%x,%x",
				node,
				0,
				sub*5000,
			)),
		}

	case Wsdo:
		var node int
		var idx int
		_, err = fmt.Fscanf(bytes.NewReader(icmd.Data),
			"%x,%x",
			&node,
			&idx,
		)
		if err != nil {
			return -1, err
		}

		ocmd = Command{
			Name: Wsdo,
			Data: []byte(fmt.Sprintf(
				"%x,%x",
				node,
				0,
			)),
		}
	}
	go func() {
		c.conn <- ocmd.bytes()
	}()
	return len(ocmd.bytes()), nil
}

func (c *cwrapperMock) Close() error {
	go func() {
		c.quit <- struct{}{}
	}()
	return nil
}
