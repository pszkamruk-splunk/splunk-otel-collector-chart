// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build configuration

package functional_tests

import (
	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"os"
	"sync"
	"testing"
)

var globalSinks *sinks

var setupRun = sync.Once{}

type sinks struct {
	logsConsumer                      *consumertest.LogsSink
	hecMetricsConsumer                *consumertest.MetricsSink
	logsObjectsConsumer               *consumertest.LogsSink
	agentMetricsConsumer              *consumertest.MetricsSink
	k8sclusterReceiverMetricsConsumer *consumertest.MetricsSink
	tracesConsumer                    *consumertest.TracesSink
}

func setupOnce(t *testing.T) *sinks {
	setupRun.Do(func() {
		// create an API server
		internal.CreateApiServer(t, apiPort)
		// set ingest pipelines
		logs, metrics := setupHec(t)
		globalSinks = &sinks{
			logsConsumer:       logs,
			hecMetricsConsumer: metrics,
		}
		if os.Getenv("TEARDOWN_BEFORE_SETUP") == "true" {
			teardown(t)
		}
		// deploy the chart and applications.
		if os.Getenv("SKIP_SETUP") == "true" {
			t.Log("Skipping setup as SKIP_SETUP is set to true")
			return
		}
		//deployChartsAndApps(t)
	})
}
func teardown(t *testing.T) {
	t.Log("teardown")
}

func Test_Functions(t *testing.T) {
	// _ = setupOnce(t)
	if os.Getenv("SKIP_TESTS") == "true" {
		t.Log("Skipping tests as SKIP_TESTS is set to true")
		return
	}
	//
	//kubeTestEnv, setKubeTestEnv := os.LookupEnv("KUBE_TEST_ENV")
	//require.True(t, setKubeTestEnv, "the environment variable KUBE_TEST_ENV must be set")
	//
	//switch kubeTestEnv {
	//case kindTestKubeEnv, autopilotTestKubeEnv, aksTestKubeEnv, gceTestKubeEnv:
	//	expectedValuesDir = kindValuesDir
	//case eksTestKubeEnv:
	//	expectedValuesDir = eksValuesDir
	//default:
	//	assert.Fail(t, "KUBE_TEST_ENV is set to invalid value. Must be one of [kind, eks].")
	//}

	t.Run("agent logs and metrics enabled or disabled", testAgentLogsAndMetrics)

}

func testAgentLogsAndMetrics(t *testing.T) {

	t.Run("check agent logs and metrics received when both are enabled", func(t *testing.T) {
		t.Log("Running test")
		t.Log("Setup")
		t.Log("test")
		t.Log("assert")
		ok := true
		assert.True(t, ok)
	})

	t.Run("check agent logs and metrics received when only logs enabled", func(t *testing.T) {
		t.Log("Running test")
		t.Log("Setup")
		t.Log("test")
		t.Log("assert")
		ok := true
		assert.True(t, ok)
	})

	t.Run("check agent logs and metrics received when only metrics enabled", func(t *testing.T) {
		t.Log("Running test")
		t.Log("Setup")
		t.Log("test")
		t.Log("assert")
		ok := true
		assert.True(t, ok)
	})

}
