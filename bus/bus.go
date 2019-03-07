package bus

import (
	"io"

	"github.com/go-lsst/ncs"
	"golang.org/x/net/context"
	"nanomsg.org/go-mangos"
	"nanomsg.org/go-mangos/protocol/bus"
	"nanomsg.org/go-mangos/transport/ipc"
	"nanomsg.org/go-mangos/transport/tcp"
)

const (
	// LogBus is the default rendez-vous point for the logging bus
	LogBus = "tcp://127.0.0.1:40001"

	// StatusBus is the default rendez-vous point for the status bus
	StatusBus = "tcp://127.0.0.1:40002"

	// CmdBus is the default reendez-vous point for the command bus
	CmdBus = "tcp://127.0.0.1:40003"
)

// sysbus is the main system bus
type sysbus struct {
	*ncs.Base
	addr string
	sck  mangos.Socket
}

func New(name, addr string) ncs.Module {
	return &sysbus{
		Base: ncs.NewBase(name),
		addr: addr,
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

	err = b.sck.Listen(b.addr)
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
