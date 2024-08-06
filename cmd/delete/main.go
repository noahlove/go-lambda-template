package main

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/aws/aws-sdk-go/service/lambda"
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
	ECR struct {
		RepositoryName string `yaml:"repository_name"`
	} `yaml:"ecr"`
}

func main() {
	// Read and parse the YAML config
	config := &Config{}
	configFile, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}
	err = yaml.Unmarshal(configFile, config)
	if err != nil {
		log.Fatalf("Error parsing config file: %v", err)
	}

	// Confirm deletion with user
	fmt.Print("Are you sure you want to delete the Lambda function and ECR repository? (y/n): ")
	var confirmation string
	fmt.Scanln(&confirmation)
	if confirmation != "y" && confirmation != "Y" {
		fmt.Println("Deletion cancelled.")
		return
	}

	// Create AWS session
	sess, err := session.NewSessionWithOptions(session.Options{
		Profile: config.AWS.Profile,
		Config: aws.Config{
			Region: aws.String(config.AWS.Region),
		},
	})
	if err != nil {
		log.Fatalf("Error creating AWS session: %v", err)
	}

	// Delete Lambda function
	lambdaClient := lambda.New(sess)
	_, err = lambdaClient.DeleteFunction(&lambda.DeleteFunctionInput{
		FunctionName: aws.String(config.Lambda.FunctionName),
	})
	if err != nil {
		log.Printf("Error deleting Lambda function: %v", err)
	} else {
		fmt.Printf("Lambda function '%s' deleted successfully.\n", config.Lambda.FunctionName)
	}

	// Delete ECR repository
	ecrClient := ecr.New(sess)
	_, err = ecrClient.DeleteRepository(&ecr.DeleteRepositoryInput{
		RepositoryName: aws.String(config.ECR.RepositoryName),
		Force:          aws.Bool(true),
	})
	if err != nil {
		log.Printf("Error deleting ECR repository: %v", err)
	} else {
		fmt.Printf("ECR repository '%s' deleted successfully.\n", config.ECR.RepositoryName)
	}
}
