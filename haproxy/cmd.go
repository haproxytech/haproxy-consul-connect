package haproxy

import (
	"os/exec"
	"sync/atomic"
	"syscall"

	"github.com/criteo/haproxy-consul-connect/haproxy/halog"
	"github.com/criteo/haproxy-consul-connect/lib"
	log "github.com/sirupsen/logrus"
)

func runCommand(sd *lib.Shutdown, stopSig syscall.Signal, path string, args ...string) (*exec.Cmd, error) {
	cmd := exec.Command(path, args...)
	halog.Cmd("haproxy", cmd)

	sd.Add(1)
	err := cmd.Start()
	if err != nil {
		sd.Done()
		return nil, err
	}
	exited := uint32(0)
	go func() {
		defer sd.Done()
		err := cmd.Wait()
		atomic.StoreUint32(&exited, 1)
		if err != nil {
			log.Errorf("%s exited with error: %s", path, err)
			sd.Shutdown()
		}
	}()
	go func() {
		<-sd.Stop
		if atomic.LoadUint32(&exited) > 0 {
			return
		}
		log.Infof("killing %s with sig %d", path, stopSig)
		syscall.Kill(cmd.Process.Pid, stopSig)
	}()

	return cmd, nil
}
