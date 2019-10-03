package haproxy_cmd

import (
	"fmt"
	"os/exec"
	"path"
	"sync/atomic"
	"syscall"

	"github.com/criteo/haproxy-consul-connect/haproxy/halog"
	"github.com/criteo/haproxy-consul-connect/lib"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

func runCommand(sd *lib.Shutdown, cmdPath string, args ...string) (*exec.Cmd, error) {
	_, file := path.Split(cmdPath)
	cmd := exec.Command(cmdPath, args...)
	halog.Cmd("haproxy", cmd)

	sd.Add(1)
	err := cmd.Start()
	if err != nil {
		sd.Done()
		return nil, errors.Wrapf(err, "error starting %s", file)
	}
	exited := uint32(0)
	go func() {
		defer sd.Done()
		err := cmd.Wait()
		atomic.StoreUint32(&exited, 1)
		if err != nil {
			log.Errorf("%s exited with error: %s", file, err)
		} else {
			log.Errorf("%s exited", file)
		}
		sd.Shutdown(fmt.Sprintf("%s exited", file))
	}()
	go func() {
		<-sd.Stop
		if atomic.LoadUint32(&exited) > 0 {
			return
		}
		log.Infof("killing %s with sig %d", file, syscall.SIGTERM)
		syscall.Kill(cmd.Process.Pid, syscall.SIGTERM)
	}()

	return cmd, nil
}
