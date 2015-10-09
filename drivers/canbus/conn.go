package canbus

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"syscall"

	"github.com/gonuts/logger"
)

type cwrapper interface {
	io.Reader
	io.Writer
	io.Closer
	init(msg *logger.Logger) error
}

// cwrapperImpl manages the connection to the C-Wrapper program
type cwrapperImpl struct {
	port int
	lst  net.Listener
	conn net.Conn
	proc *exec.Cmd

	quit chan struct{}
	errc chan error
}

func newCwrapperImpl(port int) *cwrapperImpl {
	return &cwrapperImpl{
		port: port,
		quit: make(chan struct{}),
		errc: make(chan error),
	}
}

func (c *cwrapperImpl) init(msg *logger.Logger) error {
	var err error

	if true {
		go c.startCWrapper(msg)
	} else {
		go func() {
			c.quit <- struct{}{}
		}()
	}

	msg.Infof("... starting tcp server ...\n")
	c.lst, err = net.Listen("tcp", fmt.Sprintf(":%d", c.port))
	if err != nil {
		msg.Errorf("error starting tcp server: %v\n", err)
		return err
	}

	msg.Infof("... waiting for a connection ...\n")
	c.conn, err = c.lst.Accept()
	if err != nil {
		msg.Errorf("error accepting connection: %v\n", err)
		return err
	}

	return err
}

func (c *cwrapperImpl) Read(data []byte) (int, error) {
	if c.conn == nil {
		return 0, io.EOF
	}
	return c.conn.Read(data)
}

func (c *cwrapperImpl) Write(data []byte) (int, error) {
	if c.conn == nil {
		return 0, io.EOF
	}
	return c.conn.Write(data)
}

func (c *cwrapperImpl) Close() error {
	var err error
	c.quit <- struct{}{}
	err = <-c.errc
	return err
}

func (c *cwrapperImpl) startCWrapper(msg *logger.Logger) {
	host, err := c.host(msg)
	if err != nil {
		c.errc <- err
		return
	}

	msg.Infof("Starting c-wrapper on PC-104... (listen for %s:%d)\n", host, c.port)

	cmd := exec.Command(
		"ssh",
		"-X",
		"root@clrlsstemb01.in2p3.fr",
		"startCWrapper --host="+host, fmt.Sprintf("--port=%d", c.port),
	)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "TERM=vt100")
	//cmd.Stdin = os.Stdin
	//cmd.Stdout = os.Stderr
	//cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	msg.Infof("c-wrapper command: %v\n", cmd.Args)

	c.proc = cmd
	err = c.proc.Start()
	if err != nil {
		msg.Errorf("error starting c-wrapper: %v\n", err)
		c.errc <- err
		return
	}

	select {
	case <-c.quit:
		c.errc <- c.proc.Process.Kill()
	}
}

func (c *cwrapperImpl) host(msg *logger.Logger) (string, error) {
	host, err := os.Hostname()
	if err != nil {
		msg.Errorf("could not retrieve hostname: %v\n", err)
		return "", err
	}

	addrs, err := net.LookupIP(host)
	if err != nil {
		msg.Errorf("could not lookup hostname IP: %v\n", err)
		return "", err
	}

	for _, addr := range addrs {
		ipv4 := addr.To4()
		if ipv4 == nil {
			continue
		}
		return ipv4.String(), nil
	}

	msg.Errorf("could not infer host IP")
	return "", fmt.Errorf("could not infer host IP")
}
