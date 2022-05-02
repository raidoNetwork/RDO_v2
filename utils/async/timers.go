package async

import (
	"context"
	"github.com/sirupsen/logrus"
	"time"
)

var log = logrus.WithField("prefix", "async")

func WithInterval(ctx context.Context, interval time.Duration, callback func()) {
	ticker := time.NewTicker(interval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				log.WithField("function", "").Trace("execute")
				callback()
			}
		}
	}()
}
