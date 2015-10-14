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
	Component
	//Release() error
	//Parent() Device
	//Driver() Driver
}

// Driver is responsible for initializing devices.
type Driver interface {
	Component
	Devices() []Device
}

type Component interface {
	Name() string
}

type Module interface {
	Component
	Boot(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Shutdown(ctx context.Context) error
}

type Ticker interface {
	Component
	Tick(ctx context.Context) error
}
