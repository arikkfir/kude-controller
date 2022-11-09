package main

import (
	"context"
	"github.com/arikkfir/kude-controller/test/harness"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"math/rand"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"strconv"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	k8sConfig, _, _ := harness.SetupServer(t)
	opts := zap.Options{
		Development: true,
		TimeEncoder: zapcore.TimeEncoderOfLayout(time.StampMilli),
	}

	metricsRandPort := rand.Intn(10) + 8080
	probeRandPort := metricsRandPort + 1
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		t.Log("Stopping manager")
		cancel()
	})
	go func() {
		if err := run(k8sConfig, ":"+strconv.Itoa(metricsRandPort), false, ":"+strconv.Itoa(probeRandPort), opts, ctx); err != nil {
			t.Errorf("Failed to run manager: %v", err)
		}
	}()

	// Wait for the manager to start
	time.Sleep(5 * time.Second)

	if resp, err := http.Get("http://localhost:" + strconv.Itoa(metricsRandPort) + "/metrics"); assert.NoErrorf(t, err, "Failed to get metrics") {
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
	if resp, err := http.Get("http://localhost:" + strconv.Itoa(probeRandPort) + "/healthz"); assert.NoErrorf(t, err, "Failed to get healthz") {
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}
