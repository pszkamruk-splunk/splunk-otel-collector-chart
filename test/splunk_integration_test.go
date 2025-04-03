// Copyright Splunk Inc.
// SPDX-License-Identifier: Apache-2.0

//go:build splunk_integration

package test

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	_ "k8s.io/client-go/plugin/pkg/client/auth"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"testing"
)

const EVENT_SEARCH_QUERY_STRING = "| search "
const METRIC_SEARCH_QUERY_STRING = "| mpreview "

func deploySplunk(t *testing.T) {
	// Deploy Splunk
	fmt.Println("Deploying Splunk")
	// Load kubeconfig
	//var kubeconfig *string
	//if home := homedir.HomeDir(); home != "" {
	//	kubeconfig = flag.String("kubeconfig", filepath.Join(home, ".kube", "config"), "(optional) absolute path to the kubeconfig file")
	//} else {
	//	kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	//}
	//flag.Parse()
	//
	//// Build the configuration from the kubeconfig file
	//config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	//if err != nil {
	//	fmt.Printf("Failed to build kubeconfig: %v\n", err)
	//	return
	//}

	// Create a new client
	//client, err := kubernetes.NewForConfig(kubeConfig)
	//if err != nil {
	//	fmt.Printf("Failed to create Kubernetes client: %v\n", err)
	//	return
	//}

	// MOJ KOD
	testKubeConfig, setKubeConfig := os.LookupEnv("KUBECONFIG")
	require.True(t, setKubeConfig, "the environment variable KUBECONFIG must be set")
	kubeConfig, err := clientcmd.BuildConfigFromFlags("", testKubeConfig)
	require.NoError(t, err)
	client, err := dynamic.NewForConfig(kubeConfig)
	require.NoError(t, err)
	// MOJ KOD

	// Read the YAML file
	//yamlFile, err := ioutil.ReadFile("test_setup.yaml")
	//if err != nil {
	//	fmt.Printf("Failed to read YAML file: %v\n", err)
	//	return
	//}
	//
	//// Parse the YAML into an unstructured object
	//obj := &unstructured.Unstructured{}
	//decoder := yaml.NewYAMLOrJSONDecoder(yamlFile, 4096)
	//if err := decoder.Decode(&obj); err != nil {
	//	fmt.Printf("Failed to decode YAML: %v\n", err)
	//	return
	//}

	//var valuesBytes []byte
	//valuesBytes, err := os.ReadFile("test_setup.yaml")
	//require.NoError(t, err)
	//fmt.Printf("VALUES BYTES: %v", valuesBytes)
	//replacements := map[string]interface{}{
	//	"LogHecEndpoint":    fmt.Sprintf("v1"),
	//	"MetricHecEndpoint": fmt.Sprintf("v2"),
	//}
	//
	//tmpl, err := template.New("").Parse(string(valuesBytes))
	//require.NoError(t, err)
	//var buf bytes.Buffer
	//err = tmpl.Execute(&buf, replacements)
	//require.NoError(t, err)
	//fmt.Printf("VALUES: %v", buf.String())
	//var values map[string]interface{}
	//err = yaml.Unmarshal(buf.Bytes(), &values)
	//require.NoError(t, err)

	// Create a buffer from the YAML content
	//buf := bytes.NewBufferString(valuesBytes)

	// Split the YAML content into separate documents
	//decoder := yaml.NewDecoder(buf)

	// Open the YAML file
	//file, err := os.Open("test_setup.yaml")
	//file, err := os.Open("test_recall.yaml")
	file, err := os.Open("k8s-splunk.yml")
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

			// Apply the object to the cluster
			//namespace := obj.GetNamespace()
			//fmt.Println("obj", obj)
			//fmt.Println("NAMESPACE: ", namespace)
			//if namespace == "" {
			//	namespace = "default" // Use "default" if no namespace is specified
			//}

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
		// ----
		//tmp := client.AppsV1().Namespaces()

		// Process each YAML document

	}

	//fmt.Println(values)

	//// Apply the object to the cluster
	//ctx := ""
	//client.
	//	_, err = client.Resource("default").Create(ctx, values, metav1.CreateOptions{})
	//if err != nil {
	//	if errors.IsAlreadyExists(err) {
	//		fmt.Println("Resource already exists, updating...")
	//		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
	//			_, updateErr := client.Resource("default").Update(ctx, values, metav1.UpdateOptions{})
	//			return updateErr
	//		})
	//		if retryErr != nil {
	//			fmt.Printf("Failed to update resource: %v\n", retryErr)
	//			return
	//		}
	//	} else {
	//		fmt.Printf("Failed to apply resource: %v\n", err)
	//		return
	//	}
	//}

	fmt.Println("Resource applied successfully!")
}

func Test_Functions(t *testing.T) {
	deploySplunk(t)

	t.Run("verify log ingestion by using annotations", testVerifyLogsIngestionUsingAnnotations)
	t.Run("custom metadata fields annotations", testVerifyCustomMetadataFieldsAnnotations)
	t.Run("metric index annotations", testVerifyMetricIndexAndSourcetypeAnnotations)

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
			searchQuery := EVENT_SEARCH_QUERY_STRING + "index=" + tt.index + " k8s.pod.labels.app::" + tt.label
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
			searchQuery := EVENT_SEARCH_QUERY_STRING + "index=" + tt.index + " k8s.pod.labels.app::" + tt.label + " customField::" + tt.value
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
		searchQuery := METRIC_SEARCH_QUERY_STRING + "index=" + index + " filter=\"sourcetype=" + sourcetype + "\""
		startTime := "-1h@h"
		events := CheckEventsFromSplunk(searchQuery, startTime)
		fmt.Println(" =========>  Events received: ", len(events))
		assert.Greater(t, len(events), 1)
	})
}
