package ncs

import (
	"time"

	"github.com/gonuts/logger"
	"golang.org/x/net/context"
)

type Base struct {
	*logger.Logger
	ticker *time.Ticker
	bus    busNode
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
	bus, err := newBusNode(BusAddr)
	if err != nil {
		return err
	}
	b.bus = bus

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
	return b.bus.Send(b, data)
}
