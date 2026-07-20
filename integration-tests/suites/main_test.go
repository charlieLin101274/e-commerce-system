//go:build integration

package suites

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/linenxing/e-commerce-system/integration-tests/testkit"
)

var (
	testEnvironment testkit.Environment
	testClient      *testkit.Client
	testScenario    *testkit.Scenario
)

func TestMain(m *testing.M) {
	testEnvironment = testkit.LoadEnvironment()
	readyContext, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	if err := testEnvironment.WaitForAPI(readyContext); err != nil {
		cancel()
		fmt.Fprintf(os.Stderr, "integration test environment is not ready: %v\n", err)
		os.Exit(1)
	}
	cancel()
	testClient = testkit.NewClient(testEnvironment)
	testScenario = testkit.NewScenario(testClient, testEnvironment)
	os.Exit(m.Run())
}
