package downloader

import (
	"MediaNinja/core/events"
	"context"
	"fmt"
	"time"
)

func downloadContext(c ClientInterface) context.Context {
	if v, ok := c.(interface{ DownloadContext() context.Context }); ok {
		return v.DownloadContext()
	}
	return context.Background()
}
func report(c ClientInterface, e events.Event) {
	e.Time = time.Now()
	if v, ok := c.(interface{ Report(events.Event) }); ok {
		v.Report(e)
	}
}
func waitRetry(c ClientInterface, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-downloadContext(c).Done():
		return downloadContext(c).Err()
	case <-timer.C:
		return nil
	}
}

func logf(c ClientInterface, format string, args ...any) {
	report(c, events.Event{Type: "log", Message: fmt.Sprintf(format, args...)})
}
