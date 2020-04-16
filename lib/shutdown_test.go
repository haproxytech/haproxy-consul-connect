package lib

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test_Shutdown(t *testing.T) {
	sd := NewShutdown()
	expectedDuration := time.Duration(500 * time.Millisecond)
	start := time.Now()
	sd.Add(1)
	go func() {
		time.Sleep(expectedDuration)
		sd.Done()
	}()
	sd.Wait()
	assert.GreaterOrEqual(t, time.Since(start).Milliseconds(), expectedDuration.Milliseconds())

	// Shutting down
	start = time.Now()
	go func() {
		time.Sleep(expectedDuration)
		sd.Shutdown("Kill waiting tasks")
	}()
	<-sd.Stop
	assert.GreaterOrEqual(t, time.Since(start).Milliseconds(), expectedDuration.Milliseconds())

}
