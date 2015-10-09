package canbus

import (
	"bytes"
	"fmt"
	"io"
	"sync"

	"golang.org/x/net/context"

	"github.com/go-lsst/ncs"
)

// Cmd is a type of command to send/receive on/from the CAN bus.
type Cmd string

// The command types known to the CAN bus.
const (
	Boot Cmd = "boot"
	Info     = "info"
	Rsdo     = "rsdo"
	Wsdo     = "wsdo"
	Sync     = "sync"
	Quit     = "quit"
)

type Error struct {
	Code int
}

func (err Error) Error() string {
	return fmt.Sprintf("canbus: error code=%v", err.Code)
}

// Command is a command sent/received on the CAN bus
type Command struct {
	Name Cmd
	Data []byte
}

func (cmd Command) bytes() []byte {
	o := make([]byte, 0, len(cmd.Name)+1+len(cmd.Data))
	o = append(o, []byte(cmd.Name)...)
	if len(cmd.Data) > 0 {
		o = append(o, sepComma...)
		o = append(o, cmd.Data...)
	}
	o = append(o, '\r', 0, '\n')
	return o
}

func (cmd Command) String() string {
	return fmt.Sprintf("Command{%s,%s}", cmd.Name, string(cmd.Data))
}

func (cmd Command) Err() error {
	node := 0
	ecode := 0
	_, err := fmt.Fscanf(bytes.NewReader(cmd.Data),
		"%x,%x",
		&node,
		&ecode,
	)
	if err != nil {
		return err
	}
	if ecode == 0 {
		return nil
	}
	return Error{ecode}
}

func newCommand(data []byte) Command {
	data = bytes.TrimSpace(data)
	if !bytes.Contains(data, sepComma) {
		return Command{}
	}

	tokens := bytes.SplitN(data, sepComma, 2)
	cmd := Command{
		Name: Cmd(tokens[0]),
		Data: tokens[1],
	}
	return cmd
}

type Bus interface {
	ADC() *ADC
	DAC() *DAC
	Send(cmd Command) (Command, error)
}

type busImpl struct {
	*ncs.Base
	conn  cwrapper
	quit  chan struct{}
	nodes []int

	adc *ADC
	dac *DAC

	devices []ncs.Device

	mux  sync.Mutex
	send chan Command
	recv chan Command
}

func New(name string, port int, adc *ADC, dac *DAC, devices ...ncs.Device) ncs.Module {
	devs := append([]ncs.Device{adc, dac}, devices...)
	bus := &busImpl{
		Base:    ncs.NewBase(name),
		conn:    newCwrapperImpl(port),
		quit:    make(chan struct{}),
		nodes:   make([]int, 0, 2),
		adc:     adc,
		dac:     dac,
		send:    make(chan Command),
		recv:    make(chan Command),
		devices: devs,
	}
	ncs.System.Register(bus)
	for _, dev := range bus.devices {
		ncs.System.Register(dev)
	}

	return bus
}

func (bus *busImpl) Boot(ctx context.Context) error {
	bus.Infof(">>> boot...\n")
	var err error

	err = bus.Base.Boot(ctx)
	if err != nil {
		bus.Errorf("error booting: %v\n", err)
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	err = bus.init()
	if err != nil {
		bus.Errorf("error: %v\n", err)
		return err
	}

	bus.Infof(">>> boot... [done]\n")
	return err
}

func (bus *busImpl) Start(ctx context.Context) error {
	var err error
	return err
}

func (bus *busImpl) Stop(ctx context.Context) error {
	var err error
	bus.Infof("stopping...\n")

	return err
}

func (bus *busImpl) Shutdown(ctx context.Context) error {
	var err error
	bus.Infof("shutdown...\n")

	_, err = bus.Send(Command{Quit, nil})
	if err != nil {
		bus.Errorf("error closing canbus: %v\n", err)
	}

	err = bus.Close()
	if err != nil {
		return err
	}

	return err
}

func (bus *busImpl) init() error {
	var err error

	err = bus.conn.init(bus.Base.Logger)
	if err != nil {
		bus.Errorf("error initializing cwrapper: %v\n", err)
		return err
	}

	const bufsz = 1024
	buf := make([]byte, bufsz)

	// consume welcome message
	n, err := bus.conn.Read(buf)
	if err != nil {
		bus.Errorf("error receiving welcome message: %v\n", err)
		return err
	}
	if n <= 0 {
		bus.Errorf("empty welcome message!\n")
		return io.ErrUnexpectedEOF
	}

	if !bytes.HasPrefix(buf[:n], []byte("TestBench ISO-8859-1")) {
		bus.Errorf("unexpected welcome message: %q\n", string(buf[:n]))
		return io.ErrUnexpectedEOF
	}

	// discover nodes
	for len(bus.nodes) < len(bus.devices) {
		buf = buf[:bufsz]
		n, err := bus.conn.Read(buf)
		if err != nil {
			bus.Errorf("error receiving boot message: %v\n", err)
			return err
		}
		if n <= 0 {
			// nothing was read...
			continue
		}
		buf = buf[:n]
		cmd := newCommand(buf)
		switch cmd.Name {
		case Boot:
			id := 0
			_, err := fmt.Fscanf(bytes.NewReader(cmd.Data), "%x", &id)
			if err != nil {
				bus.Errorf("error decoding node id: %v\n", err)
				return err
			}
			bus.Infof("detected node 0x%x\n", id)
			bus.nodes = append(bus.nodes, id)
		default:
			bus.Errorf("unexpected command name: %q (cmd=%v)\n", cmd.Name, cmd)
			return fmt.Errorf("unexpected command %q", cmd.Name)
		}
	}

	type Node struct {
		id       int
		device   int
		vendor   int
		product  int
		revision int
		serial   string
	}

	nodes := make([]Node, len(bus.nodes))
	// fetch infos about nodes
	for _, id := range bus.nodes {
		buf := []byte(fmt.Sprintf("%s,%x\n", Info, id))
		_, err := bus.conn.Write(buf)
		if err != nil {
			bus.Errorf("error sending info message: %v\n", err)
			return err
		}

		buf = make([]byte, bufsz)
		n, err := bus.conn.Read(buf)
		if err != nil {
			bus.Errorf("error receiving info message: %v\n", err)
			return err
		}
		if n <= 0 {
			// nothing was read...
			continue
		}
		buf = buf[:n]
		cmd := newCommand(buf)
		switch cmd.Name {
		case Info:
			var node Node
			_, err = fmt.Fscanf(
				bytes.NewReader(cmd.Data),
				"%x,%x,%x,%x,%x,%s",
				&node.id,
				&node.device,
				&node.vendor,
				&node.product,
				&node.revision,
				&node.serial,
			)
			if err != nil {
				bus.Errorf("error decoding %v: %v\n", cmd, err)
				return err
			}
			bus.Infof("node=%#v\n", node)
			nodes = append(nodes, node)
			//TODO(sbinet): better/more-general handling
			switch node.serial {
			case bus.adc.serial:
				bus.adc.node = node.id
				bus.adc.bus = bus
			case bus.dac.serial:
				bus.dac.node = node.id
				bus.dac.bus = bus
			}

		default:
			err = fmt.Errorf("unexpected command name: %q (cmd: %v)", cmd.Name, cmd)
			bus.Errorf("error: %v\n", err)
			return err
		}
	}

	bus.Infof("adc=%#v\n", bus.adc)
	bus.Infof("dac=%#v\n", bus.dac)

	err = bus.adc.init()
	if err != nil {
		bus.Errorf("error initializing ADC: %v\n", err)
		return err
	}

	err = bus.dac.init()
	if err != nil {
		bus.Errorf("error initializing DAC: %v\n", err)
		return err
	}

	go bus.run()

	return err
}

func (bus *busImpl) Close() error {
	if bus.conn == nil {
		return nil
	}
	bus.Infof("closing tcp server\n")
	return bus.conn.Close()
}

func (bus *busImpl) run() {
	bus.Infof("handle...\n")

	const bufsz = 1024

loop:
	for {
		select {
		case cmd := <-bus.send:
			n, err := bus.conn.Write(cmd.bytes())
			if err != nil {
				bus.Errorf("error sending command %v: %v\n", cmd, err)
				return
			}

			switch cmd.Name {
			case Quit:
				bus.Infof("received 'quit' request...\n")
				break loop
			}

			// TODO(sbinet) only read back when needed?
			buf := make([]byte, bufsz)
			n, err = bus.conn.Read(buf)
			if err != nil {
				bus.Errorf("error receiving message: %v\n", err)
				break loop
			}
			buf = buf[:n]
			cmd = newCommand(buf)
			bus.recv <- cmd

		case <-bus.quit:
			bus.Infof("quit...\n")
			break loop
		}
	}

	close(bus.send)
	close(bus.recv)
	bus.Infof("handle... [done]\n")
}

// Send sends a command down the bus and returns its reply
func (bus *busImpl) Send(icmd Command) (Command, error) {
	var err error

	bus.mux.Lock()
	defer bus.mux.Unlock()

	bus.send <- icmd
	switch icmd.Name {
	case Quit:
		ocmd := <-bus.recv
		return ocmd, err
	}
	ocmd := <-bus.recv

	if ocmd.Name != icmd.Name {
		return ocmd, fmt.Errorf("unexpected command: %v", ocmd)
	}

	ecode := 0
	_, err = fmt.Fscanf(bytes.NewReader(ocmd.Data),
		"%x",
		&ecode,
	)
	if err != nil {
		return ocmd, err
	}

	// need to synchronize bus
	// FIXME(sbinet) figure out what exactly happens.
	if ecode == -1 {
		buf := make([]byte, 1024)
		n, err := bus.conn.Read(buf)
		if err != nil {
			bus.Errorf("error receiving message: %v\n", err)
			return ocmd, err
		}
		buf = buf[:n]
		cmd := newCommand(buf)
		return cmd, err
	}

	return ocmd, err
}

func (bus *busImpl) ADC() *ADC {
	return bus.adc
}

func (bus *busImpl) DAC() *DAC {
	return bus.dac
}

var (
	sepComma = []byte(",")
)
