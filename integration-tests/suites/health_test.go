//go:build integration

package suites

import "testing"

func TestHealth(t *testing.T) {
	if err := testClient.Health(t.Context()); err != nil {
		t.Fatalf("health check failed: %v", err)
	}
}
