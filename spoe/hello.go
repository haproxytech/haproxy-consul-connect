package spoe

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
)

const (
	helloKeyMaxFrameSize      = "max-frame-size"
	helloKeySupportedVersions = "supported-versions"
	helloKeyVersion           = "version"
	helloKeyCapabilities      = "capabilities"
	helloKeyHealthcheck       = "healthcheck"

	capabilityAsync      = "async"
	capabilityPipelining = "pipelining"
)

var (
	helloCapabilities = []string{capabilityAsync, capabilityPipelining}
)

func (c *conn) handleHello(frame frame) (frame, bool, error) {
	data, _, err := decodeKVs(frame.data, -1)
	if err != nil {
		return frame, false, errors.Wrap(err, "hello")
	}

	log.Infof("spoe: hello from %s: %+v", c.Conn.RemoteAddr(), data)

	remoteFrameSize, ok := data[helloKeyMaxFrameSize].(uint)
	if !ok {
		return frame, false, fmt.Errorf("hello: expected %s", helloKeyMaxFrameSize)
	}

	connFrameSize := remoteFrameSize
	if connFrameSize > maxFrameSize {
		connFrameSize = maxFrameSize
	}

	c.maxFrameSize = int(connFrameSize)

	remoteSupportedVersions, ok := data[helloKeySupportedVersions].(string)
	if !ok {
		return frame, false, fmt.Errorf("hello: expected %s", helloKeyVersion)
	}

	versionOK := false
	for _, supportedVersion := range strings.Split(remoteSupportedVersions, ",") {
		remoteVersion, err := parseVersion(supportedVersion)
		if err != nil {
			return frame, false, errors.Wrap(err, "hello")
		}

		if remoteVersion[0] == 2 {
			versionOK = true
		}
	}

	if !versionOK {
		return frame, false, fmt.Errorf("hello: incompatible version %s, need %s", remoteSupportedVersions, version)
	}

	remoteCapabilities, ok := data[helloKeyCapabilities].(string)
	if !ok {
		return frame, false, fmt.Errorf("hello: expected %s", helloKeyCapabilities)
	}

	if !checkCapabilities(remoteCapabilities) {
		return frame, false, fmt.Errorf("hello: expected capabilities %v", helloCapabilities)
	}

	frame.ftype = frameTypeAgentHello
	frame.flags = frameFlagFin

	off := 0
	n, err := encodeKV(frame.data[off:], helloKeyVersion, version)
	if err != nil {
		return frame, false, errors.Wrap(err, "hello")
	}
	off += n

	n, err = encodeKV(frame.data[off:], helloKeyMaxFrameSize, connFrameSize)
	if err != nil {
		return frame, false, errors.Wrap(err, "hello")
	}
	off += n

	n, err = encodeKV(frame.data[off:], helloKeyCapabilities, strings.Join(helloCapabilities, ","))
	if err != nil {
		return frame, false, errors.Wrap(err, "hello")
	}
	off += n

	frame.data = frame.data[:off]

	healthcheck, _ := data[helloKeyHealthcheck].(bool)

	return frame, healthcheck, nil
}

func checkCapabilities(capas string) bool {
	hasAsync := false
	hasPipelining := false

	for _, s := range strings.Split(capas, ",") {
		switch s {
		case capabilityAsync:
			hasAsync = true
		case capabilityPipelining:
			hasPipelining = true
		}
	}

	return hasAsync && hasPipelining
}

func parseVersion(v string) ([]int, error) {
	res := []int{}

	v = strings.TrimSpace(v)
	s, e := 0, 0

	for i := 0; i <= len(v); i++ {
		if i == len(v) || v[i] == '.' {
			n, err := strconv.Atoi(v[s:e])
			if err != nil {
				return nil, errors.Wrap(err, "version parse")
			}
			res = append(res, n)
			s = i + 1
			e = s
			continue
		}
		if v[i] >= '0' && v[i] <= '9' {
			e++
			continue
		}
		return nil, fmt.Errorf("version parse: unexpected char %s", string(v[i]))
	}

	if len(res) == 0 {
		return nil, fmt.Errorf("version parse: expected at least one digit")
	}

	return res, nil
}
