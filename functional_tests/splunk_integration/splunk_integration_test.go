// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build splunk_integration

package splunk_integration

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/signalfx/splunk-otel-collector-chart/functional_tests/internal"
)

const (
	testDir   = "testdata"
	valuesDir = "values"

	// Splunk Search Query related constants
	eventSearchQueryString  = "| search "
	metricSearchQueryString = "| mpreview "

	SplunkHECPort = 8088
)

func Test_Functions(t *testing.T) {
	//testRecallYaml := "./testdata/test_recall.yaml"
	clientset := createK8sClient(t)

	splunkYaml := "./testdata/k8s-splunk.yml"
	deploySplunk(t, splunkYaml)
	internal.CheckPodsReady(t, clientset, internal.Namespace, "app=splunk", 3*time.Minute)
	splunkIpAddr := getPodIpAddress(t, "splunk")
	err := os.Setenv("CI_SPLUNK_HOST", splunkIpAddr)
	require.NoError(t, err)
	fmt.Printf("Splunk Pod IP Address: %s\n", splunkIpAddr)
	fmt.Println(" Host Endpoint - get env: ", os.Getenv("CI_SPLUNK_HOST"))

	//testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	//require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	//config, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	//require.NoError(t, err)
	//clientset, err := kubernetes.NewForConfig(config)
	//require.NoError(t, err)

	connectorValuesYaml := "sck_otel_values_no_comments.yaml.tmpl"
	deploySockConnector(t, connectorValuesYaml)

	logGeneratorsYamlFile := "./testdata/test_setup.yaml"
	deployLogGenerators(t, logGeneratorsYamlFile)
	fmt.Println("Waiting for log generators jobs to complete")
	namespaceWithTestingJobs := [3]string{"ns-w-exclude", "ns-w-index", "ns-wo-index"}
	for _, ns := range namespaceWithTestingJobs {
		waitForJobsToComplete(t, clientset, ns)
	}
	// wait for 60 seconds for the logs to be ingested
	//time.Sleep(120 * time.Second)

	t.Run("verify log ingestion by using annotations", testVerifyLogsIngestionUsingAnnotations)
	t.Run("custom metadata fields annotations", testVerifyCustomMetadataFieldsAnnotations)
	t.Run("metric namespace annotations", testVerifyMetricNamespaceAnnotations)
}

func testVerifyLogsIngestionUsingAnnotations(t *testing.T) {
	tests := []struct {
		name               string
		label              string
		index              string
		expectedNoOfEvents int
	}{
		{"no annotations for namespace and pod", "pod-wo-index-wo-ns-index", "ci_events", 45},
		{"pod annotation only", "pod-w-index-wo-ns-index", "pod-anno", 100},
		{"namespace annotation only", "pod-wo-index-w-ns-index", "ns-anno", 15},
		{"pod and namespace annotation", "pod-w-index-w-ns-index", "pod-anno", 10},
		{"exclude namespace annotation", "pod-w-index-w-ns-exclude", "*", 0},
		{"exclude pod annotation", "pod-wo-index-w-exclude-w-ns-index", "*", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Printf("Test: %s - %s", tt.name, tt.label)
			searchQuery := eventSearchQueryString + "index=" + tt.index + " k8s.pod.labels.app::" + tt.label
			startTime := "-1h@h"
			events := CheckEventsFromSplunk(searchQuery, startTime)
			fmt.Println(" =========>  Events received: ", len(events))
			assert.Equal(t, len(events), tt.expectedNoOfEvents)
		})
	}
}

func testVerifyCustomMetadataFieldsAnnotations(t *testing.T) {
	tests := []struct {
		name               string
		label              string
		index              string
		value              string
		expectedNoOfEvents int
	}{
		{"custom metadata 1", "pod-w-index-wo-ns-index", "pod-anno", "pod-value-2", 100},
		{"custom metadata 2", "pod-w-index-w-ns-index", "pod-anno", "pod-value-1", 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fmt.Printf("Testing custom metadata annotation label=%s value=%s expected=%d event(s)", tt.label, tt.value, tt.expectedNoOfEvents)
			searchQuery := eventSearchQueryString + "index=" + tt.index + " k8s.pod.labels.app::" + tt.label + " customField::" + tt.value
			startTime := "-1h@h"
			events := CheckEventsFromSplunk(searchQuery, startTime)
			fmt.Println(" =========>  Events received: ", len(events))
			assert.Equal(t, len(events), tt.expectedNoOfEvents)
		})
	}
}

func testVerifyMetricIndexAndSourcetypeAnnotations(t *testing.T) {
	t.Run("metrics sent to metricIndex", func(t *testing.T) {
		fmt.Println("Test that metrics are being sent to 'test_metrics' index, as defined by splunk.com/metricsIndex annotation added during setup")
		index := "test_metrics"
		sourcetype := "sourcetype-anno"
		searchQuery := metricSearchQueryString + "index=" + index + " filter=\"sourcetype=" + sourcetype + "\""
		startTime := "-1h@h"
		events := CheckEventsFromSplunk(searchQuery, startTime)
		fmt.Println(" =========>  Events received: ", len(events))
		assert.Greater(t, len(events), 1)
	})
}

func testVerifyMetricNamespaceAnnotations(t *testing.T) {
	namespace := "default"
	defaultIndex := "ci_metrics"
	defaultSourcetype := "httpevent"
	annotationIndex := "test_metrics"
	annotationSourcetype := "annotation_sourcetype"

	client := createK8sClient(t)

	tests := []struct {
		name                      string
		annotationIndexValue      string
		annotationSourcetypeValue string
	}{
		{"default index and default sourcetype", "", ""},
		{"annotation index and default sourcetype", annotationIndex, ""},
		{"default index and annotation sourcetype", "", annotationSourcetype},
		{"annotation index and annotation sourcetype", annotationIndex, annotationSourcetype},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addNamespaceAnnotation(t, client, namespace, tt.annotationIndexValue, tt.annotationSourcetypeValue)
			time.Sleep(20 * time.Second)

			index := defaultIndex
			if tt.annotationIndexValue != "" {
				index = tt.annotationIndexValue
			}
			sourcetype := defaultSourcetype
			if tt.annotationSourcetypeValue != "" {
				sourcetype = tt.annotationSourcetypeValue
			}
			searchQuery := metricSearchQueryString + "index=" + index + " filter=\"sourcetype=" + sourcetype + "\" | search \"k8s.namespace.name\"=" + namespace
			fmt.Println("Search Query: ", searchQuery)
			startTime := "-15s@s"
			events := CheckEventsFromSplunk(searchQuery, startTime)
			fmt.Println(" =========>  Events received: ", len(events))
			assert.Greater(t, len(events), 1)

			removeAllNamespaceAnnotations(t, client, namespace)
		})
	}
}

func createK8sClient(t *testing.T) *kubernetes.Clientset {
	kubeConfig := getKubeConfig(t)
	client, err := kubernetes.NewForConfig(kubeConfig)
	require.NoError(t, err)
	return client
}

func createDynamicK8sClient(t *testing.T) *dynamic.DynamicClient {
	kubeConfig := getKubeConfig(t)
	client, err := dynamic.NewForConfig(kubeConfig)
	require.NoError(t, err)
	return client
}

func getKubeConfig(t *testing.T) *rest.Config {
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	return kubeConfig
}

func removeAllNamespaceAnnotations(t *testing.T, clientset *kubernetes.Clientset, namespace_name string) {
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace_name, metav1.GetOptions{})
	require.NoError(t, err)
	ns.Annotations = make(map[string]string)

	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
	require.NoError(t, err)
	fmt.Printf("All annotations removed from namespace_name %s\n", namespace_name)
}

func addNamespaceAnnotation(t *testing.T, clientset *kubernetes.Clientset, namespace_name string, annotationIndex string, annotationSourcetype string) {
	ns, err := clientset.CoreV1().Namespaces().Get(context.TODO(), namespace_name, metav1.GetOptions{})
	require.NoError(t, err)
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	if annotationIndex != "" {
		ns.Annotations["splunk.com/metricsIndex"] = annotationIndex
	}
	if annotationSourcetype != "" {
		ns.Annotations["splunk.com/sourcetype"] = annotationSourcetype
	}

	_, err = clientset.CoreV1().Namespaces().Update(context.TODO(), ns, metav1.UpdateOptions{})
	require.NoError(t, err)
	fmt.Printf("Annotation added to namespace_name %s\n", namespace_name)
}

func deploySplunk(t *testing.T, yaml_file_name string) {
	// Deploy Splunk
	fmt.Println("Deploying Splunk")
	installK8sWorkloadResources(t, yaml_file_name)
	clientset := createK8sClient(t)
	fmt.Println("waiting for splunk pods to be ready")
	internal.CheckPodsReady(t, clientset, "default", "app=splunk", 3*time.Minute)
	fmt.Println("done")
}

func deploySockConnector(t *testing.T, valuesFileName string) {
	// Deploy Splunk Connector
	fmt.Println("Deploying SOCK Connector")
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")

	fmt.Println(" Host Endpoint: ", os.Getenv("CI_SPLUNK_HOST"))
	//fmt.Println(" Host Endpoint: ", getPodIpAddress(t, "splunk"))
	fmt.Println(" Splunk HEC : ", os.Getenv("CI_SPLUNK_HEC_TOKEN"))
	replacements := map[string]interface{}{
		"SplunkHecEndpoint": fmt.Sprintf("https://%s:%d/services/collector", os.Getenv("CI_SPLUNK_HOST"), SplunkHECPort),
		"SplunkHecToken":    os.Getenv("CI_SPLUNK_HEC_TOKEN"),
	}

	valuesFile, err := filepath.Abs(filepath.Join(testDir, valuesDir, valuesFileName))
	require.NoError(t, err)
	internal.ChartInstallOrUpgrade(t, testKubeConfig, valuesFile, replacements)

	//t.Cleanup(func() {
	//	if os.Getenv("SKIP_TEARDOWN") == "true" {
	//		t.Log("Skipping teardown as SKIP_TEARDOWN is set to true")
	//		return
	//	}
	//	t.Log("Cleaning up cluster")
	//	teardown(t)
	//
	//})

}

func deployLogGenerators(t *testing.T, configFileName string) {
	// Deploy log generators
	fmt.Println("Deploying log generators")
	installK8sWorkloadResources(t, configFileName)
}

func installK8sWorkloadResources(t *testing.T, configFileName string) {
	//testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	//require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	//kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	//require.NoError(t, err)
	//client, err := dynamic.NewForConfig(kubeConfig)
	//require.NoError(t, err)
	client := createDynamicK8sClient(t)

	file, err := os.Open(configFileName)
	if err != nil {
		fmt.Printf("Failed to open file: %v", err)
	}
	defer file.Close()

	// Create a YAML decoder
	decoder := yaml.NewDecoder(file)

	// Iterate through all the documents
	for {
		var values map[string]interface{}
		err := decoder.Decode(&values)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			fmt.Printf("Failed to decode YAML: %v", err)
		}
		fmt.Printf("\n\n\nVALUES: %v\n", values)

		// -----
		// Convert the map to an Unstructured object
		obj := &unstructured.Unstructured{Object: values}
		if obj.GetKind() == "Namespace" {

			// Define the GVR (Group-Version-Resource) for Namespace
			gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}

			_, err = client.Resource(gvr).Create(context.Background(), obj, v1.CreateOptions{})
			if err != nil {
				_, err2 := client.Resource(gvr).Update(context.Background(), obj, v1.UpdateOptions{})
				assert.NoError(t, err2)
			}
		}

		if obj.GetKind() == "Job" {
			// Define the GVR (Group-Version-Resource) for Jobs
			gvr := schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"}

			// Apply the object to the cluster in the specified namespace
			namespace := obj.GetNamespace()
			fmt.Println("obj", obj)
			fmt.Println("NAMESPACE: ", namespace)
			_, err = client.Resource(gvr).Namespace(namespace).Create(context.TODO(), obj, v1.CreateOptions{})
			if err != nil {
				_, err2 := client.Resource(gvr).Namespace(namespace).Update(context.TODO(), obj, v1.UpdateOptions{})
				assert.NoError(t, err2)
			}

			fmt.Println("Job resource applied successfully!")
		}
		if obj.GetKind() == "ConfigMap" {
			// Define the GVR (Group-Version-Resource) for Jobs
			gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"}

			// Apply the object to the cluster in the specified namespace
			namespace := obj.GetNamespace()
			fmt.Println("obj", obj)
			fmt.Println("NAMESPACE: ", namespace)
			_, err = client.Resource(gvr).Namespace(namespace).Create(context.TODO(), obj, v1.CreateOptions{})
			if err != nil {
				_, err2 := client.Resource(gvr).Namespace(namespace).Update(context.TODO(), obj, v1.UpdateOptions{})
				assert.NoError(t, err2)
			}

			fmt.Println("ConfigMap resource applied successfully!")
		}
		if obj.GetKind() == "Pod" {
			// Define the GVR (Group-Version-Resource) for Jobs
			gvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}

			// Apply the object to the cluster in the specified namespace
			namespace := obj.GetNamespace()
			fmt.Println("obj", obj)
			fmt.Println("NAMESPACE: ", namespace)
			_, err = client.Resource(gvr).Namespace(namespace).Create(context.TODO(), obj, v1.CreateOptions{})
			fmt.Println("err", err)
			if err != nil {
				_, err2 := client.Resource(gvr).Update(context.TODO(), obj, v1.UpdateOptions{})
				assert.NoError(t, err2)
			}

			fmt.Println("ConfigMap resource applied successfully!")
		}
	}

	fmt.Println("Resource applied successfully!")
}

func getPodIpAddress(t *testing.T, podName string) string {
	//testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	//require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	//kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	//require.NoError(t, err)
	//clientset, err := kubernetes.NewForConfig(kubeConfig)
	//require.NoError(t, err)
	clientset := createK8sClient(t)

	pod, err := clientset.CoreV1().Pods("default").Get(context.TODO(), podName, metav1.GetOptions{})
	require.NoError(t, err)

	fmt.Printf("Pod IP Address: %s\n", pod.Status.PodIP)
	return pod.Status.PodIP
}

func waitForJobsToComplete(t *testing.T, clientset *kubernetes.Clientset, namespace string) {
	jobs, err := clientset.BatchV1().Jobs(namespace).List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err)
	for _, job := range jobs.Items {
		err = wait.PollImmediate(5*time.Second, 10*time.Minute, func() (bool, error) {
			job, err := clientset.BatchV1().Jobs(namespace).Get(context.TODO(), job.Name, metav1.GetOptions{})
			require.NoError(t, err)
			if job.Status.Succeeded > 0 {
				return true, nil
			}
			return false, nil
		})
		require.NoError(t, err)
		fmt.Printf("Job %s completed successfully\n", job.Name)
	}
}
