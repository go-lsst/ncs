package ncs

import (
	"time"

	"golang.org/x/net/context"

	"github.com/gdamore/mangos"
	"github.com/gdamore/mangos/protocol/bus"
	"github.com/gdamore/mangos/transport/ipc"
	"github.com/gdamore/mangos/transport/tcp"
	"github.com/gonuts/logger"
)

type Base struct {
	*logger.Logger
	ticker *time.Ticker
	bus    mangos.Socket
}

func NewBase(name string) *Base {
	return &Base{
		Logger: logger.New(name),
		ticker: time.NewTicker(1 * time.Second),
	}
}

func (b *Base) Tick() <-chan time.Time {
	return b.ticker.C
}

func (b *Base) Boot(ctx context.Context) error {
	sock, err := bus.NewSocket()
	if err != nil {
		return err
	}
	b.bus = sock

	b.bus.AddTransport(ipc.NewTransport())
	b.bus.AddTransport(tcp.NewTransport())

	err = b.bus.Listen("tcp://127.0.0.1:0")
	if err != nil {
		return err
	}

	err = b.bus.Dial(BusAddr)
	if err != nil {
		return err
	}

	return err
}

func (b *Base) Shutdown(ctx context.Context) error {
	err := b.bus.Close()
	if err != nil {
		b.Errorf("error closing connection to system-bus: %v\n", err)
		return err
	}

	return err
}

// Send sends data on the system bus
func (b *Base) Send(data []byte) error {
	msg := append([]byte("name="+b.Name()+"; "), data...)
	return b.bus.Send(msg)
}
