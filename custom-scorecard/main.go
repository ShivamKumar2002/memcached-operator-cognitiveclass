package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"slices"
	"time"

	cachev1beta1 "memcached-operator/api/v1beta1"

	scapiv1alpha3 "github.com/operator-framework/api/pkg/apis/scorecard/v1alpha3"
	apimanifests "github.com/operator-framework/api/pkg/manifests"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	// The location the operator's bundle will be mounted
	PodBundleRoot = "/bundle"
	// our first test
	CustomTest1Name = "hello-world"

	// Basic test to standup a Memcached instance
	CustomTest2Name = "basic-standup"
)

func main() {
	entrypoint := os.Args[1:]
	if len(entrypoint) == 0 {
		log.Fatal("Test name argument is required")
	}

	// Read the pod's untar'd bundle from a well-known path.
	cfg, err := apimanifests.GetBundleFromDir(PodBundleRoot)
	if err != nil {
		log.Fatal(err.Error())
	}

	var result scapiv1alpha3.TestStatus
	// Names of the custom tests which would be passed in the `operator-sdk` command.
	switch entrypoint[0] {
	case CustomTest1Name:
		result = HelloWorld(cfg)

	case CustomTest2Name:
		result = BasicMemcachedStandup(cfg)

	default:
		result = scapiv1alpha3.TestStatus{
			Results: []scapiv1alpha3.TestResult{
				{
					State:  scapiv1alpha3.FailState,
					Errors: []string{"invalid test"},
				},
			},
		}
	}

	// Convert scapiv1alpha3.TestResult to json.
	prettyJSON, err := json.MarshalIndent(result, "", "    ")
	if err != nil {
		log.Fatal("Failed to generate json", err)
	}

	fmt.Printf("%s\n", string(prettyJSON))
}

func HelloWorld(bundle *apimanifests.Bundle) scapiv1alpha3.TestStatus {
	r := scapiv1alpha3.TestStatus{
		Results: []scapiv1alpha3.TestResult{
			{
				Name:  CustomTest1Name,
				State: scapiv1alpha3.PassState,
				Log:   "Hello world!",
			},
		},
	}
	almExamples := bundle.CSV.GetAnnotations()["alm-examples"]
	if almExamples == "" {
		fmt.Println("no alm-examples in the bundle CSV")
	}
	return r
}

func BasicMemcachedStandup(bundle *apimanifests.Bundle) scapiv1alpha3.TestStatus {
	result := scapiv1alpha3.TestStatus{
		Results: []scapiv1alpha3.TestResult{
			{
				Name:  CustomTest2Name,
				State: scapiv1alpha3.ErrorState,
			},
		},
	}

	almExamples := bundle.CSV.GetAnnotations()["alm-examples"]
	if almExamples == "" {
		fmt.Println("no alm-examples in the bundle CSV")
	}

	// construct a Scheme that contains our custom type so the k8s client knows how to talk to it
	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(cachev1beta1.AddToScheme(scheme))

	cfg, err := config.GetConfig()
	if err != nil {
		result.Results[0].Errors = append(result.Results[0].Errors, err.Error())
		return result
	}

	kubeClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		result.Results[0].Errors = append(result.Results[0].Errors, err.Error())
		return result
	}

	// create a Memcached instance with 3 nodes
	testMemcached := &cachev1beta1.Memcached{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-memcached",
			Namespace: "default",
		},
		Spec: cachev1beta1.MemcachedSpec{
			Size:             3,
			DisableEvictions: false,
		},
	}
	if err := kubeClient.Create(context.TODO(), testMemcached); err != nil {
		result.Results[0].Errors = append(result.Results[0].Errors, err.Error())
		return result
	}

	defer func() {
		// cleanup
		if err := kubeClient.Delete(context.TODO(), testMemcached); err != nil {
			result.Results[0].Errors = append(result.Results[0].Errors, err.Error())
		}
	}()

	// wait for the Memcached instance to be ready
	startTime := time.Now()
	for {
		updatedMemcached := &cachev1beta1.Memcached{}
		if err := kubeClient.Get(context.TODO(), types.NamespacedName{Name: testMemcached.Name, Namespace: testMemcached.Namespace}, updatedMemcached); err != nil {
			result.Results[0].Errors = append(result.Results[0].Errors, err.Error())
			return result
		}

		if updatedMemcached.Status.Nodes != nil && len(updatedMemcached.Status.Nodes) == int(testMemcached.Spec.Size) {
			break
		}

		if time.Since(startTime) > 2*time.Minute {
			result.Results[0].Errors = append(result.Results[0].Errors, "timed out waiting for Memcached instance to be ready")
			result.Results[0].State = scapiv1alpha3.FailState
			return result
		}

		time.Sleep(10 * time.Second)
	}

	// fetch all the Pods
	pods := &corev1.PodList{}
	if err := kubeClient.List(context.TODO(), pods, client.InNamespace(testMemcached.Namespace), client.MatchingLabels(map[string]string{"app": "memcached", "memcached_cr": testMemcached.Name})); err != nil {
		result.Results[0].Errors = append(result.Results[0].Errors, err.Error())
		return result
	}

	// wait for all pods to be ready
	startTime = time.Now()
	for {
		allReady := true
		for _, pod := range pods.Items {
			if pod.Status.Phase != corev1.PodRunning {
				allReady = false
				break
			}
			for _, cond := range pod.Status.Conditions {
				if cond.Type == corev1.PodReady && cond.Status != corev1.ConditionTrue {
					allReady = false
					break
				}
			}
		}

		if allReady {
			break
		}

		if time.Since(startTime) > 2*time.Minute {
			result.Results[0].Errors = append(result.Results[0].Errors, "timed out waiting for pods to be ready")
			result.Results[0].State = scapiv1alpha3.FailState
			return result
		}

		time.Sleep(10 * time.Second)

		// re-fetch pods
		if err := kubeClient.List(context.TODO(), pods, client.InNamespace(testMemcached.Namespace), client.MatchingLabels(map[string]string{"app": "memcached", "memcached_cr": testMemcached.Name})); err != nil {
			result.Results[0].Errors = append(result.Results[0].Errors, err.Error())
			return result
		}
	}

	updatedMemcached := &cachev1beta1.Memcached{}
	if err := kubeClient.Get(context.TODO(), types.NamespacedName{Name: testMemcached.Name, Namespace: testMemcached.Namespace}, updatedMemcached); err != nil {
		result.Results[0].Errors = append(result.Results[0].Errors, err.Error())
		return result
	}

	var isFail bool
	// check if created pods match status.nodes in the Memcached object
	for _, pod := range pods.Items {
		if !slices.Contains(updatedMemcached.Status.Nodes, pod.Name) {
			result.Results[0].Log += fmt.Sprintf("Pod %s not found in status.Nodes for Memcached %s\n", pod.Name, testMemcached.Name)
			isFail = true
		}
	}

	if isFail {
		result.Results[0].State = scapiv1alpha3.FailState
	} else {
		result.Results[0].State = scapiv1alpha3.PassState
	}

	return result
}
