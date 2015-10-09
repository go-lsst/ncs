package ncs

import (
	"golang.org/x/net/context"
)

type Node struct {
	Name  string
	Nodes []Node
}

// Device represents a physical device mounted onto some hardware.
type Device interface {
	Name() string
	//Release() error
	//Parent() Device
	//Driver() Driver
}

// Driver is responsible for initializing devices.
type Driver interface {
	Name() string
	Devices() []Device
}

type Module interface {
	Name() string
	Boot(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type Ticker interface {
	Name() string
	Tick(ctx context.Context) error
}
