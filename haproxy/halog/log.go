package halog

import (
	"bufio"
	"io"
	"os/exec"
	"strings"

	log "github.com/sirupsen/logrus"
)

func New(prefix string, r io.Reader) {
	scan := bufio.NewScanner(r)
	go func() {
		for scan.Scan() {
			haproxyLog(prefix, scan.Text())
		}
	}()
}

func Cmd(prefix string, cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	New(prefix, stdout)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	New(prefix, stderr)
	return nil
}

func haproxyLog(prefix, l string) {
	if len(l) == 0 {
		return
	}

	f := log.Errorf
	defer func() {
		f("%s: %s", prefix, strings.TrimSpace(l))
	}()

	if l[0] != '[' {
		return
	}

	end := strings.IndexByte(l, ']')
	if end == -1 {
		return
	}

	switch l[1:end] {
	case "NOTICE":
		f = log.Infof
	case "WARNING":
		f = log.Warnf
	case "ALERT":
		f = log.Errorf
	default:
		return
	}

	l = l[end+1:]
}
