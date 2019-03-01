package spoe

import (
	"bufio"
	"fmt"
	"net"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/pkg/errors"
)

type connState int

const (
	connStateInit connState = iota
	connStateProcessing
)

const (
	version      = "2.0"
	maxFrameSize = 16380
)

type conn struct {
	net.Conn
	state connState

	handler      Handler
	buff         *bufio.ReadWriter
	maxFrameSize int
}

type Handler func(args []Message) ([]Action, error)

type Agent struct {
	Handler Handler

	maxFrameSize int

	acks chan frame
}

func New(h Handler) *Agent {
	a := &Agent{
		Handler: h,
		acks:    make(chan frame),
	}
	return a
}

func (a *Agent) ListenAndServe(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return errors.Wrap(err, "spoe")
	}

	return a.Serve(lis)
}

func (a *Agent) Serve(lis net.Listener) error {
	log.Infof("spoe: listening on %s", lis.Addr().String())

	go func() {
		for {
			c, err := lis.Accept()
			if err != nil {
				log.Errorf("spoe: %s", err)
				continue
			}

			log.Debugf("spoe: connection from %s", c.RemoteAddr())

			go func() {
				c := &conn{
					Conn:    c,
					state:   connStateInit,
					handler: a.Handler,
					buff:    bufio.NewReadWriter(bufio.NewReader(c), bufio.NewWriter(c)),
				}
				err := c.run(a)
				if err != nil {
					log.Errorf("spoe: error handling connection: %s", err)
				}
			}()
		}
	}()

	return nil
}

func (c *conn) run(a *Agent) error {
	defer c.Close()

	done := make(chan struct{})
	defer close(done)

	frame, err := decodeFrame(c, make([]byte, maxFrameSize))
	if err != nil {
		return err
	}

	if frame.ftype != frameTypeHaproxyHello {
		return fmt.Errorf("unexpected frame type %x when initializing connection", frame.ftype)
	}

	frame, healcheck, err := c.handleHello(frame)
	if err != nil {
		return err
	}

	err = encodeFrame(c.buff, frame)
	if err != nil {
		return err
	}
	err = c.buff.Flush()
	if err != nil {
		return err
	}
	if healcheck {
		return nil
	}

	pool := &sync.Pool{
		New: func() interface{} {
			return make([]byte, c.maxFrameSize)
		},
	}

	// run reply loop
	go func() {
		for {
			select {
			case <-done:
				return
			case frame := <-a.acks:
				err = encodeFrame(c.buff, frame)
				if err != nil {
					log.Errorf("spoe: %s", err)
					continue
				}
				err = c.buff.Flush()
				if err != nil {
					log.Errorf("spoe: %s", err)
					continue
				}
				pool.Put(frame.originalBuffer)
			}
		}
	}()

	for {
		frame, err := decodeFrame(c, pool.Get().([]byte))
		if err != nil {
			return err
		}

		switch frame.ftype {

		case frameTypeHaproxyNotify:
			go func() {
				frame, err = c.handleNotify(frame)
				if err != nil {
					log.Errorf("spoe: %s", err)
					return
				}

				a.acks <- frame
			}()

		case frameTypeHaproxyDiscon:
			err := c.handleDisconnect(frame)
			if err != nil {
				log.Errorf("spoe: %s", err)
			}
			return nil

		default:
			log.Errorf("spoe: frame type %x is not handled", frame.ftype)
		}
	}
}
