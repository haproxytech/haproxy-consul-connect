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

// getVersion Launch Help from program path and Find Version
// to capture the output and retrieve version information
func getVersion(path string) (string, error) {
	cmd := exec.Command(path, "-v")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed executing %s: %s", path, err.Error())
	}
	re := regexp.MustCompile("\\d+(\\.\\d+)+")
	return string(re.Find(out)), nil
}

// CheckEnvironment Verifies that all dependencies are correct
func CheckEnvironment(dataplaneapiBin, haproxyBin string) error {
	var err error
	wg := &sync.WaitGroup{}
	wg.Add(2)
	ensureVersion := func(path, minVer string) {
		defer wg.Done()
		currVer, e := getVersion(path)
		if e != nil {
			err = e
		}
		res, e := compareVersion(currVer, minVer)
		if e != nil {
			err = e
		}
		if res < 0 {
			err = fmt.Errorf("%s version must be > %s, but is: %s", path, minVer, currVer)
		}
	}
	go ensureVersion(haproxyBin, "2.0")
	go ensureVersion(dataplaneapiBin, "1.2")

	wg.Wait()
	if err != nil {
		return err
	}
	return nil
}

// compareVersion compares two semver versions.
// If v1 > v2 returns 1, if v1 < v2 returns -1, if equal returns 0.
// If major versions are not the same, returns -1.
// If an error occurs, returns -1 and error.
func compareVersion(v1, v2 string) (int, error) {
	a := strings.Split(v1, ".")
	b := strings.Split(v2, ".")

	if len(a) < 2 {
		return -1, fmt.Errorf("%s arg is not a version string", v1)
	}
	if len(b) < 2 {
		return -1, fmt.Errorf("%s arg is not a version string", v2)
	}

	if len(a) != len(b) {
		switch {
		case len(a) > len(b):
			for i := len(b); len(b) < len(a); i++ {
				b = append(b, " ")
			}
			break
		case len(a) < len(b):
			for i := len(a); len(a) < len(b); i++ {
				a = append(a, " ")
			}
			break
		}
	}

	var res int

	for i, s := range a {
		var ai, bi int
		fmt.Sscanf(s, "%d", &ai)
		fmt.Sscanf(b[i], "%d", &bi)

		if i == 0 {
			//major versions should be the same
			if ai != bi {
				res = -1
				break
			}
			continue
		}
		if ai > bi {
			res = 1
			break
		}
		if ai < bi {
			res = -1
			break
		}
	}
	return res, nil
}
