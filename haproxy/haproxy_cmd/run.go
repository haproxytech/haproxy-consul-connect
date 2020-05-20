package haproxy_cmd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/haproxytech/haproxy-consul-connect/haproxy/dataplane"
	"github.com/haproxytech/haproxy-consul-connect/lib"
)

const (
	// DefaultDataplaneBin is the default dataplaneapi program name
	DefaultDataplaneBin = "dataplaneapi"
	// DefaultHAProxyBin is the default HAProxy program name
	DefaultHAProxyBin = "haproxy"
)

type Config struct {
	HAProxyPath             string
	HAProxyConfigPath       string
	DataplanePath           string
	DataplaneTransactionDir string
	DataplaneSock           string
	DataplaneUser           string
	DataplanePass           string
}

func Start(sd *lib.Shutdown, cfg Config) (*dataplane.Dataplane, error) {
	haCmd, err := runCommand(sd,
		cfg.HAProxyPath,
		"-f",
		cfg.HAProxyConfigPath,
	)
	if err != nil {
		return nil, err
	}
	if haCmd.Process == nil {
		return nil, fmt.Errorf("%s was not started", cfg.HAProxyPath)
	}

	cmd, err := runCommand(sd,
		cfg.DataplanePath,
		"--scheme", "unix",
		"--socket-path", cfg.DataplaneSock,
		"--haproxy-bin", cfg.HAProxyPath,
		"--config-file", cfg.HAProxyConfigPath,
		"--reload-cmd", fmt.Sprintf("kill -SIGUSR2 %d", haCmd.Process.Pid),
		"--reload-delay", "1",
		"--userlist", "controller",
		"--transaction-dir", cfg.DataplaneTransactionDir,
	)
	cleanupHAProxy := func() {
		haCmd.Process.Signal(os.Kill)
	}
	if err != nil {
		cleanupHAProxy()
		return nil, err
	}
	if cmd.Process == nil {
		cleanupHAProxy()
		return nil, fmt.Errorf("%s failed to start", cfg.DataplanePath)
	}

	dataplaneClient := dataplane.New(
		"http://unix-sock",
		cfg.DataplaneUser,
		cfg.DataplanePass,
		&http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				Dial: func(proto, addr string) (conn net.Conn, err error) {
					return net.Dial("unix", cfg.DataplaneSock)
				},
			},
		},
	)

	// wait for startup
	for i := time.Duration(0); i < (5*time.Second)/(100*time.Millisecond); i++ {
		select {
		case <-sd.Stop:
			return nil, fmt.Errorf("exited")
		default:
		}

		err = dataplaneClient.Ping()
		if err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		break
	}
	if err != nil {
		return nil, fmt.Errorf("timeout waiting for dataplaneapi: %s", err)
	}

	return dataplaneClient, nil
}

// execAndCapture Launch Help from program path and Find Version
// to capture the output and retrieve version information
func execAndCapture(path string, re *regexp.Regexp) (string, error) {
	cmd := exec.Command(path, "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed executing %s: %s", path, err.Error())
	}
	return string(re.Find([]byte(out))), nil
}

// CheckEnvironment Verifies that all dependencies are correct
func CheckEnvironment(dataplaneapiBin, haproxyBin string) error {
	var err error
	wg := &sync.WaitGroup{}
	wg.Add(2)
	ensureVersion := func(path, rx, version string) {
		defer wg.Done()
		r := regexp.MustCompile(rx)
		v, e := execAndCapture(path, r)
		if e != nil {
			err = e
		} else if strings.Compare(v, "1.2") < 0 {
			err = fmt.Errorf("%s version must be > 1.2, but is: %s", path, v)
		}
	}
	go ensureVersion(haproxyBin, "^HA-Proxy version ([0-9]\\.[0-9]\\.[0-9])", "2.0")
	go ensureVersion(dataplaneapiBin, "^HAProxy Data Plane API v([0-9]\\.[0-9]\\.[0-9])", "1.2")

	wg.Wait()
	if err != nil {
		return err
	}
	return nil
}
