package ncs

import (
	"io"

	"github.com/go-mangos/mangos"
	"github.com/go-mangos/mangos/protocol/bus"
	"github.com/go-mangos/mangos/transport/ipc"
	"github.com/go-mangos/mangos/transport/tcp"
	"golang.org/x/net/context"
)

const (
	// BusAddr is the default rendez-vous point for the system bus
	BusAddr = "tcp://127.0.0.1:40000"
)

// sysbus is the main system bus
type sysbus struct {
	*Base
	sck mangos.Socket
}

func newSysBus() *sysbus {
	return &sysbus{
		Base: NewBase("sysbus"),
	}
}

func (b *sysbus) init() error {
	sck, err := bus.NewSocket()
	if err != nil {
		b.Errorf("error creating a nanomsg socket: %v\n", err)
		return err
	}

	b.sck = sck
	b.sck.AddTransport(ipc.NewTransport())
	b.sck.AddTransport(tcp.NewTransport())

	err = b.sck.Listen(BusAddr)
	if err != nil {
		return err
	}

	return err
}

func (bus *sysbus) Boot(ctx context.Context) error {
	var err error

	go func() {
	loop:
		for {
			select {
			case <-ctx.Done():
				bus.Infof("shutting down system-bus...\n")
				return

			default:
				msg, err := bus.sck.Recv()
				if err != nil {
					if err == io.EOF || err == mangos.ErrClosed {
						bus.Errorf("received EOF: %v\n", err)
						break loop
					}
					bus.Errorf("error receiving from system-bus: %v\n", err)
					continue
				}
				bus.Infof("recv: %v\n", string(msg))
			}
		}
	}()

	return err
}

func (bus *sysbus) Start(ctx context.Context) error {
	var err error
	return err
}

func (bus *sysbus) Stop(ctx context.Context) error {
	var err error
	return err
}

func (bus *sysbus) Shutdown(ctx context.Context) error {
	var err error

	err = bus.sck.Close()
	if err != nil {
		bus.Errorf("error closing system-bus socket: %v\n", err)
		return err
	}

	return err
}

// busNode is a node on the system-bus
type busNode struct {
	sock mangos.Socket
}

func newBusNode(addr string) (busNode, error) {
	var err error
	var node busNode

	sock, err := bus.NewSocket()
	if err != nil {
		return node, err
	}

	node.sock = sock
	node.sock.AddTransport(ipc.NewTransport())
	node.sock.AddTransport(tcp.NewTransport())

	err = node.sock.Listen("tcp://127.0.0.1:0")
	if err != nil {
		return node, err
	}

	err = node.sock.Dial(addr)
	if err != nil {
		return node, err
	}

	return node, err
}

func (bus *busNode) Close() error {
	return bus.sock.Close()
}

func (bus *busNode) Send(m Component, data []byte) error {
	msg := append([]byte("name="+m.Name()+"; "), data...)
	return bus.sock.Send(msg)
}
