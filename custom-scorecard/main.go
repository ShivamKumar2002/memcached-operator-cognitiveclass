package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	scapiv1alpha3 "github.com/operator-framework/api/pkg/apis/scorecard/v1alpha3"
	apimanifests "github.com/operator-framework/api/pkg/manifests"
)

const (
	// The location the operator's bundle will be mounted
	PodBundleRoot = "/bundle"
	// our first test
	CustomTest1Name = "hello-world"
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
