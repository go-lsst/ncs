package canbus

import (
	"bytes"
	"fmt"
	"io"

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
	err  error
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
	if cmd.err != nil {
		return cmd.err
	}

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
	cmd.err = Error{ecode}
	return cmd.err
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

// Message is a pair request/reply sent on the CAN bus.
type Message struct {
	Req   Command
	Reply chan Command
}

func Msg(name Cmd, data []byte) *Message {
	return &Message{
		Req: Command{
			Name: name,
			Data: data,
		},
		Reply: make(chan Command),
	}
}

type Bus interface {
	ADC() *ADC
	DAC() *DAC
	Queue() chan<- *Message
}

type busImpl struct {
	*ncs.Base
	conn  cwrapper
	quit  chan struct{}
	nodes []int

	adc *ADC
	dac *DAC

	devices []ncs.Device

	queue chan *Message
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
		devices: devs,
		queue:   make(chan *Message),
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

	msg := Msg(Quit, nil)
	bus.Queue() <- msg
	reply := <-msg.Reply
	err = reply.Err()
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
	bus.Infof("run-loop...\n")

loop:
	for {
		select {
		case msg := <-bus.queue:
			err := bus.handle(msg)
			if err == io.EOF {
				bus.Infof("received io.EOF\n")
				break loop
			}

			if err != nil {
				bus.Errorf("error handling message %v: %v\n", msg, err)
				return
			}

		case <-bus.quit:
			bus.Infof("quit...\n")
			break loop
		}
	}

	close(bus.queue)
	bus.Infof("run-loop... [done]\n")
}

func (bus *busImpl) handle(msg *Message) error {
	icmd := msg.Req
	n, err := bus.conn.Write(icmd.bytes())
	if err != nil {
		bus.Errorf("error sending command %v: %v\n", icmd, err)
		msg.Reply <- Command{
			Name: icmd.Name,
			err:  err,
		}
		return err
	}
	if len(icmd.bytes()) > n {
		bus.Errorf(
			"error sending command %v: number of bytes don't match (want=%d, got=%d)\n",
			icmd,
			len(icmd.bytes()),
			n,
		)
		msg.Reply <- Command{
			Name: icmd.Name,
			err:  io.ErrShortWrite,
		}
		return io.ErrShortWrite
	}

	if icmd.Name == Quit {
		bus.Infof("received 'quit' request...\n")
		msg.Reply <- Command{Name: Quit, err: io.EOF}
		return io.EOF
	}

	const retry = true
	return bus.send(msg, retry)
}

func (bus *busImpl) send(msg *Message, retry bool) error {
	const bufsz = 1024

	// TODO(sbinet) only read back when needed?
	buf := make([]byte, bufsz)
	n, err := bus.conn.Read(buf)
	if err != nil {
		bus.Errorf("error receiving message: %v\n", err)
		msg.Reply <- Command{
			Name: msg.Req.Name,
			err:  err,
		}
		return err
	}

	buf = buf[:n]
	ocmd := newCommand(buf)
	ecode := 0
	_, err = fmt.Fscanf(bytes.NewReader(ocmd.Data),
		"%x",
		&ecode,
	)
	if err != nil {
		return err
	}

	// check if the ecode==-1 (an internal error on the bus similar to EAGAIN
	// (?)) error isn't already fixed and bundled with the correct reply.
	// ie: the reply is bundled with the previous read, so drop the reply with
	// the error and parse the remaining of the reply.
	if ecode == -1 && bytes.Count(ocmd.Data, []byte("\n")) > 0 {
		toks := bytes.Split(ocmd.Data, []byte("\n"))
		ocmd = newCommand(toks[1])
		_, err = fmt.Fscanf(bytes.NewReader(ocmd.Data),
			"%x",
			&ecode,
		)
		if err != nil {
			return err
		}
	}

	// need to synchronize bus
	// FIXME(sbinet) figure out what exactly happens.
	if ecode == -1 && retry {
		retry = false
		return bus.send(msg, retry)
	}
	msg.Reply <- ocmd
	return err
}

// Queue returns the channel where clients can send commands
func (bus *busImpl) Queue() chan<- *Message {
	return bus.queue
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
