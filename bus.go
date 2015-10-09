package ncs

import (
	"io"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/bus"
	"github.com/gdamore/mangos/transport/ipc"
	"github.com/gdamore/mangos/transport/tcp"
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
