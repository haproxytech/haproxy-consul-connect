package halog

import (
	"bufio"
	"io"
	"strings"

	log "github.com/sirupsen/logrus"
)

func New(r io.Reader) {
	scan := bufio.NewScanner(r)
	go func() {
		for scan.Scan() {
			haproxyLog(scan.Text())
		}
	}()
}

func haproxyLog(l string) {
	if len(l) == 0 {
		return
	}

	f := log.Errorf
	defer func() {
		f("%s: %s", "haproxy", strings.TrimSpace(l))
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
