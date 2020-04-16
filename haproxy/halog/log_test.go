package halog

import (
	"fmt"
	"testing"

	"github.com/bmizerany/assert"
	"github.com/sirupsen/logrus"
)

type fakeHook struct {
	lastMessage map[logrus.Level]string
}

// newFakeHook build a fake hook to retrive last error messages
func newFakeHook() *fakeHook {
	h := fakeHook{}
	h.lastMessage = make(map[logrus.Level]string, 4)
	return &h
}

func (*fakeHook) Levels() []logrus.Level {
	return logrus.AllLevels
}

func (h *fakeHook) Fire(e *logrus.Entry) error {
	h.lastMessage[e.Level] = e.Message
	return nil
}

func ensureLogIsPresent(t *testing.T, hook *fakeHook, expectedLevel logrus.Level, prefix, msg string) {
	haproxyLog("haproxy", fmt.Sprintf("%s%s", prefix, msg))
	assert.Equal(t, fmt.Sprintf("haproxy: %s", msg), hook.lastMessage[expectedLevel])
}

func Test_log(t *testing.T) {
	log := logrus.StandardLogger()
	hook := newFakeHook()
	log.AddHook(hook)
	// Test the parsing that should fail being parsed
	ensureLogIsPresent(t, hook, logrus.ErrorLevel, "", "lol")
	ensureLogIsPresent(t, hook, logrus.ErrorLevel, "", "[lol")
	ensureLogIsPresent(t, hook, logrus.ErrorLevel, "", "lol]")
	ensureLogIsPresent(t, hook, logrus.ErrorLevel, "", "[]")

	// while crap is well formatted, no recognized as valid type
	ensureLogIsPresent(t, hook, logrus.ErrorLevel, "", "[CRAP] Unknown kind of error")

	ensureLogIsPresent(t, hook, logrus.InfoLevel, "[NOTICE]", "yup")
	ensureLogIsPresent(t, hook, logrus.InfoLevel, "[NOTICE]", "")

	ensureLogIsPresent(t, hook, logrus.WarnLevel, "[WARNING]", "yup")
	ensureLogIsPresent(t, hook, logrus.ErrorLevel, "[ALERT]", "yup")
}
