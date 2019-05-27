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

	engineID string
}

type Handler func(args []Message) ([]Action, error)

type Agent struct {
	Handler Handler

	maxFrameSize int

	acksLock sync.Mutex
	acks     map[string]chan frame
	acksWG   map[string]*sync.WaitGroup
}

func New(h Handler) *Agent {
	a := &Agent{
		Handler: h,
		acks:    make(map[string]chan frame),
		acksWG:  make(map[string]*sync.WaitGroup),
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
}

func (c *conn) run(a *Agent) error {
	defer c.Close()

	done := make(chan struct{})
	defer close(done)

	myframe, err := decodeFrame(c, make([]byte, maxFrameSize))
	if err != nil {
		return err
	}

	if myframe.ftype != frameTypeHaproxyHello {
		return fmt.Errorf("unexpected frame type %x when initializing connection", myframe.ftype)
	}

	myframe, healcheck, err := c.handleHello(myframe)
	if err != nil {
		return err
	}

	err = encodeFrame(c.buff, myframe)
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

	a.acksLock.Lock()
	if _, ok := a.acks[c.engineID]; !ok {
		a.acks[c.engineID] = make(chan frame)
		wg := &sync.WaitGroup{}
		wg.Add(1)
		a.acksWG[c.engineID] = wg

		go func() {
			// wait until there is not more connection for this engine-id
			// before deleting it
			wg.Wait()

			a.acksLock.Lock()
			delete(a.acksWG, c.engineID)
			delete(a.acks, c.engineID)
			a.acksLock.Unlock()
		}()
	} else {
		a.acksWG[c.engineID].Add(1)
	}
	// signal that this connection is done using the engine
	defer a.acksWG[c.engineID].Done()

	acks := a.acks[c.engineID]
	a.acksLock.Unlock()

	// run reply loop
	go func() {
		for {
			select {
			case <-done:
				return
			case myframe := <-acks:
				err = encodeFrame(c.buff, myframe)
				if err != nil {
					log.Errorf("spoe: %s", err)
					continue
				}
				err = c.buff.Flush()
				if err != nil {
					log.Errorf("spoe: %s", err)
					continue
				}
				pool.Put(myframe.originalBuffer)
			}
		}
	}()

	for {
		myframe, err := decodeFrame(c, pool.Get().([]byte))
		if err != nil {
			return err
		}

		switch myframe.ftype {

		case frameTypeHaproxyNotify:
			go func() {
				myframe, err = c.handleNotify(myframe)
				if err != nil {
					log.Errorf("spoe: %s", err)
					return
				}

				acks <- myframe
			}()

		case frameTypeHaproxyDiscon:
			err := c.handleDisconnect(myframe)
			if err != nil {
				log.Errorf("spoe: %s", err)
			}
			return nil

		default:
			log.Errorf("spoe: frame type %x is not handled", myframe.ftype)
		}
	}
}
