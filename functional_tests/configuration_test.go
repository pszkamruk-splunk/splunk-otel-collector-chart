// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build configuration

package functional_tests

import (
	"bytes"
	"context"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"text/template"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	hecReceiverPort        = 8090
	hecMetricsReceiverPort = 8091
	apiPort                = 8881
	testDir                = "testdata"
	valuesDir              = "values"
)

var globalSinks *sinks

var setupRun = sync.Once{}

type sinks struct {
	logsConsumer       *consumertest.LogsSink
	hecMetricsConsumer *consumertest.MetricsSink
	//logsObjectsConsumer               *consumertest.LogsSink
	//agentMetricsConsumer              *consumertest.MetricsSink
	//k8sclusterReceiverMetricsConsumer *consumertest.MetricsSink
	//tracesConsumer                    *consumertest.TracesSink
}

func setupOnce(t *testing.T) *sinks {
	setupRun.Do(func() {
		// create an API server
		internal.CreateApiServer(t, apiPort)
		// set ingest pipelines
		logs, metrics := setupHEC(t)
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
		deployChartsAndApps(t)
	})
	return globalSinks
}

func deployChartsAndApps(t *testing.T) {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	kubeTestEnv, setKubeTestEnv := os.LookupEnv("KUBE_TEST_ENV")
	require.True(t, setKubeTestEnv, "the environment variable KUBE_TEST_ENV must be set")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	//extensionsClient, err := clientset.NewForConfig(kubeConfig)
	//require.NoError(t, err)
	//dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	//require.NoError(t, err)

	chartPath := filepath.Join("..", "helm-charts", "splunk-otel-collector")
	chart, err := loader.Load(chartPath)
	require.NoError(t, err)

	var valuesBytes []byte
	//switch kubeTestEnv {
	//case autopilotTestKubeEnv:
	//	valuesBytes, err = os.ReadFile(filepath.Join(testDir, valuesDir, "autopilot_test_values.yaml.tmpl"))
	//case aksTestKubeEnv:
	//	valuesBytes, err = os.ReadFile(filepath.Join(testDir, valuesDir, "aks_test_values.yaml.tmpl"))
	//default:
	valuesBytes, err = os.ReadFile(filepath.Join(testDir, valuesDir, "test_values_logs_and_metrics_enabled_disabled.yaml.tmpl"))
	//}

	require.NoError(t, err)

	hostEp := hostEndpoint(t)
	if len(hostEp) == 0 {
		require.Fail(t, "Host endpoint not found")
	}

	replacements := struct {
		LogHecEndpoint    string
		MetricHecEndpoint string
		KubeTestEnv       string
	}{
		fmt.Sprintf("http://%s:%d", hostEp, hecReceiverPort),
		fmt.Sprintf("http://%s:%d/services/collector", hostEp, hecMetricsReceiverPort),
		kubeTestEnv,
	}
	tmpl, err := template.New("").Parse(string(valuesBytes))
	require.NoError(t, err)
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, replacements)
	require.NoError(t, err)
	var values map[string]interface{}
	err = yaml.Unmarshal(buf.Bytes(), &values)
	require.NoError(t, err)
	t.Log("vales ---------------------")
	t.Log(values)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(kube.GetConfig(testKubeConfig, "", "default"), "default", os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
		t.Logf(format+"\n", v...)
	}); err != nil {
		require.NoError(t, err)
	}
	install := action.NewInstall(actionConfig)
	install.Namespace = "default"
	install.ReleaseName = "sock"
	_, err = install.Run(chart, values)
	if err != nil {
		t.Logf("error reported during helm install: %v\n", err)
		retryUpgrade := action.NewUpgrade(actionConfig)
		retryUpgrade.Namespace = "default"
		retryUpgrade.Install = true
		_, err = retryUpgrade.Run("sock", chart, values)
		require.NoError(t, err)
	}

	waitForAllDeploymentsToStart(t, client)
	t.Log("Deployments started")

	t.Cleanup(func() {
		if os.Getenv("SKIP_TEARDOWN") == "true" {
			t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
			return
		}
		t.Log("Cleaning up cluster")
		teardown(t)

	})

}
func teardown(t *testing.T) {
	t.Log("Running teardown")
}

func waitForAllDeploymentsToStart(t *testing.T, client *kubernetes.Clientset) {
	require.Eventually(t, func() bool {
		di, err := client.AppsV1().Deployments("default").List(context.Background(), metav1.ListOptions{})
		require.NoError(t, err)
		for _, d := range di.Items {
			if d.Status.ReadyReplicas != d.Status.Replicas {
				var messages string
				for _, c := range d.Status.Conditions {
					messages += c.Message
					messages += "\n"
				}

				t.Logf("Deployment not ready: %s, %s", d.Name, messages)
				return false
			}
		}
		return true
	}, 5*time.Minute, 10*time.Second)
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

// func testAgentLogsAndMetrics(t *testing.T) {
//
//	t.Run("check agent logs and metrics received when both are enabled", func(t *testing.T) {
//		t.Log("Running test")
//		t.Log("Setup")
//		t.Log("test")
//		t.Log("assert")
//		ok := true
//		assert.True(t, ok)
//	})
//
//	t.Run("check agent logs and metrics received when only logs enabled", func(t *testing.T) {
//		t.Log("Running test")
//		t.Log("Setup")
//		t.Log("test")
//		t.Log("assert")
//		ok := true
//		assert.True(t, ok)
//	})
//
//	t.Run("check agent logs and metrics received when only metrics enabled", func(t *testing.T) {
//		t.Log("Running test")
//		t.Log("Setup")
//		t.Log("test")
//		t.Log("assert")
//		ok := true
//		assert.True(t, ok)
//	})
//
// }
func testAgentLogsAndMetrics(t *testing.T) {

	t.Run("check agent logs and metrics received when both are enabled", func(t *testing.T) {

		// connect to the hec receiver

		hecMetricsConsumer := setupOnce(t).hecMetricsConsumer
		waitForMetrics(t, 5, hecMetricsConsumer)
		agentLogsConsumer := setupOnce(t).logsConsumer

		// verify if events are received
		waitForLogs(t, 5, agentLogsConsumer)
		//waitForMetrics(t, 5, agentMetricsConsumer)
		// wait for 2 mins and check events are still being received
		time.Sleep(20 * time.Second)

		waitForMetrics(t, 5, hecMetricsConsumer)
		t.Log("Running test")
		t.Log("Setup")
		t.Log("test")
		t.Log("assert")
		ok := true
		assert.True(t, ok)
	})
}
func setupHEC(t *testing.T) (*consumertest.LogsSink, *consumertest.MetricsSink) {
	// the splunkhecreceiver does poorly at receiving logs and metrics. Use separate ports for now.
	f := splunkhecreceiver.NewFactory()
	cfg := f.CreateDefaultConfig().(*splunkhecreceiver.Config)
	cfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", hecReceiverPort)

	mCfg := f.CreateDefaultConfig().(*splunkhecreceiver.Config)
	mCfg.Endpoint = fmt.Sprintf("0.0.0.0:%d", hecMetricsReceiverPort)

	lc := new(consumertest.LogsSink)
	mc := new(consumertest.MetricsSink)
	rcvr, err := f.CreateLogsReceiver(context.Background(), receivertest.NewNopSettings(), cfg, lc)
	mrcvr, err := f.CreateMetricsReceiver(context.Background(), receivertest.NewNopSettings(), mCfg, mc)
	require.NoError(t, err)

	require.NoError(t, rcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating logs receiver")
	t.Cleanup(func() {
		assert.NoError(t, rcvr.Shutdown(context.Background()))
	})

	require.NoError(t, mrcvr.Start(context.Background(), componenttest.NewNopHost()))
	require.NoError(t, err, "failed creating metrics receiver")
	t.Cleanup(func() {
		assert.NoError(t, mrcvr.Shutdown(context.Background()))
	})

	return lc, mc
}
