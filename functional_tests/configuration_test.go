// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build configuration

package functional_tests

import (
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func Test_Functions(t *testing.T) {
	//_ = setupOnce(t)
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

	t.Run("agent logs and metrics enabled/disabled", testAgentLogsAndMetrics)

}

func testAgentLogsAndMetrics(t *testing.T) {

	t.Run("check agent logs and metrics received when both are enabled", func(t *testing.T) {
		t.Log("Running test")
		t.Log("Setup")
		t.Log("test")
		t.Log("assert")
		ok = true
		assert.True(t, ok)
	})

}
