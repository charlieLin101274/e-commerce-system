package testkit

import (
	"context"
	"fmt"
	"time"
)

func Eventually(ctx context.Context, interval time.Duration, assertion func() (bool, error)) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastErr error
	for {
		matched, err := assertion()
		if matched {
			return nil
		}
		if err != nil {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("eventual assertion timed out: %w: %v", ctx.Err(), lastErr)
		case <-ticker.C:
		}
	}
}
