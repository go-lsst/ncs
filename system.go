package ncs

import (
	"fmt"
)

var System = systemType{
	name:    "root",
	devices: make([]Device, 0, 2),
	devmap:  make(map[string]Device),
}

type systemType struct {
	name    string
	devices []Device
	devmap  map[string]Device
}

func (sys *systemType) Devices() []Device {
	return sys.devices
}

func (sys *systemType) Name() string {
	return sys.name
}

func (sys *systemType) Register(dev Device) {
	sys.devices = append(sys.devices, dev)
	d, dup := sys.devmap[dev.Name()]
	if dup {
		panic(fmt.Errorf(
			"fwk: duplicate device %q\nold=%#v\nnew=%#v",
			dev.Name(),
			d, dev,
		))
	}
	sys.devmap[dev.Name()] = dev
}

func (sys *systemType) Device(name string) Device {
	return sys.devmap[name]
}
