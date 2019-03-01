package haproxyconfig

import (
	"bufio"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aestek/haproxy-connect/lib"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

type command struct {
	bin      string
	sockPath string
	cfgPath  string

	wg   sync.WaitGroup
	lock sync.Mutex
	sd   *lib.Shutdown

	cmds []*exec.Cmd

	Stopped chan struct{}
}

func newCommand(bin, cfgPath, sockPath string, sd *lib.Shutdown) *command {
	return &command{
		bin:      bin,
		sockPath: sockPath,
		cfgPath:  cfgPath,
		sd:       sd,
		Stopped:  make(chan struct{}),
	}
}

func (c *command) Start() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	cmd := exec.Command(c.bin, "-f", c.cfgPath)
	setupCommand(cmd)

	err := c.start(cmd)
	go func() {
		c.wg.Wait()
		close(c.Stopped)
	}()
	return err
}

func (c *command) Stop() error {
	log.Info("haproxy: stopping gracefully...")

	c.lock.Lock()
	defer c.lock.Unlock()

	for _, cmd := range c.cmds {
		log.Debugf("haproxy: sending USR1 to %d", cmd.Process.Pid)
		errors.Wrap(syscall.Kill(cmd.Process.Pid, syscall.SIGUSR1), "haproxy command graceful shutdown")
	}
	<-c.Stopped
	return nil
}

func (c *command) Reload(newCfgPath string) error {
	log.Info("haproxy: reloading...")
	defer log.Info("haproxy: reloaded")
	c.lock.Lock()
	defer c.lock.Unlock()

	newCmd := exec.Command(c.bin, "-f", newCfgPath, "-x", c.sockPath, "-sf", strconv.Itoa(c.cmds[0].Process.Pid))
	setupCommand(newCmd)

	return c.start(newCmd)
}

func (c *command) start(cmd *exec.Cmd) error {
	c.cmds = append(c.cmds, cmd)

	log.Infof("starting haproxy (%v)...", strings.Join(cmd.Args, " "))
	defer log.Info("haproxy: started")

	c.wg.Add(1)
	err := errors.Wrap(cmd.Start(), "haproxy command start")
	if err != nil {
		return err
	}

	go c.wait(cmd)

	for {
		select {
		case <-c.sd.Stop:
			return nil
		default:
		}
		if err := c.ping(cmd); err != nil {
			log.Debugf("haproxy: start ping failed: %s", err)
			time.Sleep(time.Second)
			continue
		}
		break
	}

	// TODO: find a bette way to ensure haproxy is ready
	time.Sleep(3 * time.Second)

	return nil
}

func (c *command) wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	if err != nil {
		log.Errorf("haproxy: process exited with err: %s", err)
	} else {
		log.Info("haproxy: process exited successfully")
	}
	c.wg.Done()
	c.lock.Lock()
	defer c.lock.Unlock()
	for i, cm := range c.cmds {
		if cm == cmd {
			c.cmds[i] = c.cmds[len(c.cmds)-1]
			c.cmds = c.cmds[:len(c.cmds)-1]
			break
		}
	}
}

func (c *command) ping(cmd *exec.Cmd) error {
	conn, err := net.Dial("unix", c.sockPath)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	_, err = conn.Write([]byte("show info\n"))
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}

	buf := make([]byte, 1024)
	_, err = conn.Read(buf)
	if err != nil {
		return err
	}

	return nil
}

func setupCommand(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}

	go func() {
		reader := bufio.NewScanner(stdout)
		for reader.Scan() {
			haproxyLog(reader.Text())
		}
	}()

	go func() {
		reader := bufio.NewScanner(stderr)
		for reader.Scan() {
			haproxyLog(reader.Text())
		}
	}()
}
