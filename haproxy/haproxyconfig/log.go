package haproxyconfig

import (
	"strings"

	log "github.com/sirupsen/logrus"
)

func haproxyLog(l string) {
	if len(l) == 0 {
		return
	}

	f := log.Errorf
	defer func() {
		f("haproxy: %s", strings.TrimSpace(l))
	}()

	if l[0] != '[' {
		return
	}

	end := strings.IndexByte(l, ']')
	if end == -1 {
		return
	}

	switch l[1:end] {
	case "WARNING":
		f = log.Warnf
	case "ALERT":
		f = log.Errorf
	default:
		return
	}

	l = l[end+1:]
}
