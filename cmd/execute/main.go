package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
)

const (
	region       = "us-west-2"
	functionName = "hello-world-lambda"
	awsProfile   = "personal"
)

type LambdaEvent struct {
	Name string `json:"name"`
}

func main() {
	// Parse command-line arguments
	name := flag.String("name", "", "Name to pass to the Lambda function")
	flag.Parse()

	if *name == "" {
		log.Fatal("Name is required. Use -name flag to provide a name.")
	}

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithSharedConfigProfile(awsProfile),
	)
	if err != nil {
		log.Fatalf("Unable to load SDK config: %v", err)
	}

	// Create Lambda client
	client := lambda.NewFromConfig(cfg)

	// Prepare the Lambda event
	event := LambdaEvent{
		Name: *name,
	}

	// Convert event to JSON
	payload, err := json.Marshal(event)
	if err != nil {
		log.Fatalf("Error marshaling Lambda event: %v", err)
	}

	// Invoke Lambda function
	result, err := client.Invoke(context.TODO(), &lambda.InvokeInput{
		FunctionName: aws.String(functionName),
		Payload:      payload,
	})
	if err != nil {
		log.Fatalf("Error invoking Lambda function: %v", err)
	}

	// Print the Lambda function response
	fmt.Println("Lambda function response:")
	fmt.Println(string(result.Payload))

	// Check if there was a function error
	if result.FunctionError != nil {
		fmt.Printf("Lambda function error: %s\n", *result.FunctionError)
		log.Fatal("Lambda function returned an error")
	}
}
