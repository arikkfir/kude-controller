package main

import (
	"context"
	"github.com/arikkfir/kude-controller/test/harness"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	k8sConfig, _, _ := harness.SetupServer(t)
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.TimeEncoderOfLayout(time.StampMilli),
	}

	metricsHost, err := harness.FindFreeLocalAddr()
	if err != nil {
		t.Fatalf("Failed to allocate a random local address for metrics host: %v", err)
	}
	healthHost, err := harness.FindFreeLocalAddr()
	if err != nil {
		t.Fatalf("Failed to allocate a random local address for health host: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		t.Log("Stopping manager")
		cancel()
	})
	go func() {
		if err := run(k8sConfig, metricsHost, false, healthHost, opts, ctx); err != nil {
			t.Errorf("Failed to run manager: %v", err)
		}
	}()

	// Wait for the manager to start
	time.Sleep(5 * time.Second)

	if //goland:noinspection HttpUrlsUsage
	resp, err := http.Get("http://" + metricsHost + "/metrics"); assert.NoErrorf(t, err, "Failed to get metrics") {
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
	if //goland:noinspection HttpUrlsUsage
	resp, err := http.Get("http://" + healthHost + "/healthz"); assert.NoErrorf(t, err, "Failed to get healthz") {
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}
