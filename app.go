package ncs

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"golang.org/x/net/context"
)

type App struct {
	*Base
	ctx     context.Context
	sysbus  *sysbus
	modules []Module
}

func New(name string, modules ...Module) (*App, error) {
	for _, module := range modules {
		dev := module.(Device)
		if System.Device(module.Name()) == dev {
			continue
		}
		System.Register(dev)
	}

	return &App{
		Base:    NewBase(name),
		ctx:     context.Background(),
		sysbus:  newSysBus(),
		modules: modules,
	}, nil
}

func (app *App) AddModule(m Module) {
	app.modules = append(app.modules, m)
}

func (app *App) Run() error {
	var err error

	// initialize system-bus
	err = app.sysbus.init()
	if err != nil {
		app.Errorf("error initializing system-bus: %v\n", err)
		return err
	}

	app.modules = append([]Module{app.sysbus}, app.modules...)

	ctx, cancel := context.WithCancel(app.ctx)
	defer cancel()

	sigch := make(chan os.Signal)
	signal.Notify(sigch, os.Interrupt, os.Kill)

	err = app.sysBoot(ctx)
	if err != nil {
		app.Errorf("boot-error: %v\n", err)
		cancel()
		return err
	}

	err = app.sysStart(ctx)
	if err != nil {
		app.Errorf("start-error: %v\n", err)
		cancel()
		return err
	}

	tick := time.NewTicker(1 * time.Second)

	quit := make(chan struct{})
	go func() {
		for {
			select {
			case <-sigch:
				app.Infof("stopping app...\n")
				tick.Stop()
				quit <- struct{}{}
				return
			}
		}
	}()

loop:
	for {
		select {

		case <-tick.C:
			app.Debugf("tick...\n")
			err = app.sysTick(ctx)
			if err != nil {
				app.Errorf("tick error: %v\n", err)
				cancel()
				break loop
			}

		case <-quit:
			break loop

		case <-ctx.Done():
			app.Infof("ctx.done!!\n")
			return ctx.Err()
		}
	}

	err = app.sysStop(ctx)
	if err != nil {
		cancel()
		return err
	}

	err = app.sysShutdown(ctx)
	if err != nil {
		cancel()
		return err
	}

	return err
}

func (app *App) visit(node Node) error {
	type named struct {
		Name string
		Lvl  int
	}
	var nodes []named
	var visit func(node Node, lvl int)
	visit = func(node Node, lvl int) {
		nodes = append(nodes, named{node.Name, lvl})
		for _, node := range node.Nodes {
			visit(node, lvl+1)
		}
	}
	visit(node, 0)
	for _, n := range nodes {
		fmt.Printf("%s--> %s\n", strings.Repeat("  ", n.Lvl), n.Name)
	}

	return nil
}

type modFunc func(m Module, ctx context.Context) error

func (app *App) doSys(ctx context.Context, f modFunc) error {

	errc := make(chan error, len(app.modules))
	for _, m := range app.modules {
		go func(m Module) {
			errc <- f(m, ctx)
		}(m)
	}
	for range app.modules {
		err := <-errc
		if err != nil {
			return err
		}
	}
	return nil
}

func (app *App) sysBoot(ctx context.Context) error {
	return app.doSys(ctx, modFunc(Module.Boot))
}

func (app *App) sysStart(ctx context.Context) error {
	return app.doSys(ctx, modFunc(Module.Start))
}

func (app *App) sysStop(ctx context.Context) error {
	return app.doSys(ctx, modFunc(Module.Stop))
}

func (app *App) sysShutdown(ctx context.Context) error {
	return app.doSys(ctx, modFunc(Module.Shutdown))
}

func (app *App) sysTick(ctx context.Context) error {

	errc := make(chan error, len(app.modules))
	for _, m := range app.modules {
		go func(m Module) {
			tick, ok := m.(Ticker)
			if !ok {
				errc <- nil
				app.Debugf(
					"module [%s] does not implement fwk.Ticker\n",
					m.Name(),
				)
				return
			}
			err := tick.Tick(ctx)
			if err != nil {
				app.Errorf("tick error from %q: %v\n", m.Name(), err)
			}
			errc <- err
		}(m)
	}

	for range app.modules {
		err := <-errc
		if err != nil {
			return err
		}
	}

	return nil
}
