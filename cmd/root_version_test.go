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

func TestShouldCheckForUpdate(t *testing.T) {
	now := time.Now().Unix()
	cases := []struct {
		name    string
		lastNow int64
		now     int64
		want    bool
	}{
		{"从未检测过", 0, now, true},
		{"刚检测过", now, now + 1, false},
		{"不足 24h", now - checkInterval + 1, now, false},
		{"正好 24h", now - checkInterval, now, true},
		{"超过 24h", now - checkInterval - 1, now, true},
		{"时钟回拨", now + 100, now, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldCheckForUpdate(tc.lastNow, tc.now); got != tc.want {
				t.Errorf("shouldCheckForUpdate(%d, %d) = %v, want %v", tc.lastNow, tc.now, got, tc.want)
			}
		})
	}
}
