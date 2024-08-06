package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"gopkg.in/yaml.v2"
)

type Config struct {
	AWS struct {
		Region  string `yaml:"region"`
		Profile string `yaml:"profile"`
	} `yaml:"aws"`
	Lambda struct {
		FunctionName string `yaml:"function_name"`
	} `yaml:"lambda"`
}

type LambdaEvent struct {
	Name string `json:"name"`
}

func loadConfig(filename string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(filename)
	if err != nil {
		return cfg, fmt.Errorf("error reading config file: %v", err)
	}

	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return cfg, fmt.Errorf("error parsing config file: %v", err)
	}

	return cfg, nil
}

func main() {
	// Load configuration
	cfg, err := loadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Parse command-line arguments
	name := flag.String("name", "", "Name to pass to the Lambda function")
	flag.Parse()
	if *name == "" {
		log.Fatal("Name is required. Use -name flag to provide a name.")
	}

	// Load AWS configuration
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.AWS.Region),
		config.WithSharedConfigProfile(cfg.AWS.Profile),
	)
	if err != nil {
		log.Fatalf("Unable to load SDK config: %v", err)
	}

	// Create Lambda client
	client := lambda.NewFromConfig(awsCfg)

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
		FunctionName: aws.String(cfg.Lambda.FunctionName),
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
