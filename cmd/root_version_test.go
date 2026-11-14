package cmd

import (
	"testing"
	"time"

	"github.com/spf13/viper"
)

func TestGetNewVersionSkipsRecentCheckWithoutBlocking(t *testing.T) {
	oldDetectionTime := viper.Get("core.detection_time")
	t.Cleanup(func() { viper.Set("core.detection_time", oldDetectionTime) })
	viper.Set("core.detection_time", time.Now().Unix())

	done := make(chan struct{})
	go func() {
		GetNewVersion()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("recent version check should return immediately")
	}
}
