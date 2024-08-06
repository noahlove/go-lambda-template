package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

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

var config Config

func main() {
	if err := loadConfig("config.yaml"); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := checkIAMPermissions(); err != nil {
		log.Fatalf("IAM permission check failed: %v", err)
	}

	awsAccountID, err := getAWSAccountID()
	if err != nil {
		log.Fatalf("Error getting AWS Account ID: %v", err)
	}

	if err := buildDockerImage(); err != nil {
		log.Fatalf("Error building Docker image: %v", err)
	}

	if err := authenticateDocker(awsAccountID); err != nil {
		log.Fatalf("Error authenticating Docker: %v", err)
	}

	if err := tagDockerImage(awsAccountID); err != nil {
		log.Fatalf("Error tagging Docker image: %v", err)
	}

	if err := pushDockerImage(awsAccountID); err != nil {
		log.Fatalf("Error pushing Docker image: %v", err)
	}

	if err := updateLambdaFunction(awsAccountID); err != nil {
		log.Fatalf("Error updating Lambda function: %v", err)
	}

	fmt.Println("Deployment completed successfully")
}

func loadConfig(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading config file: %v", err)
	}

	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return fmt.Errorf("error parsing config file: %v", err)
	}

	return nil
}

func checkIAMPermissions() error {
	cmd := exec.Command("aws", "iam", "get-user", "--profile", config.AWS.Profile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get IAM user info: %v\nOutput: %s", err, output)
	}
	fmt.Println("Successfully retrieved IAM user info. You have the necessary permissions.")
	return nil
}

func getAWSAccountID() (string, error) {
	cmd := exec.Command("aws", "sts", "get-caller-identity", "--query", "Account", "--output", "json", "--profile", config.AWS.Profile)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get AWS Account ID: %v", err)
	}

	var accountID string
	if err := json.Unmarshal(output, &accountID); err != nil {
		return "", fmt.Errorf("failed to parse AWS Account ID: %v", err)
	}

	return strings.Trim(accountID, "\""), nil
}

func buildDockerImage() error {
	cmd := exec.Command("docker", "build", "-t", fmt.Sprintf("%s/%s", config.ECR.RepositoryName, config.Lambda.FunctionName), ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build Docker image: %v", err)
	}
	fmt.Println("Docker image built successfully")
	return nil
}

func authenticateDocker(awsAccountID string) error {
	cmd := exec.Command("aws", "ecr", "get-login-password", "--region", config.AWS.Region, "--profile", config.AWS.Profile)
	password, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get ECR login password: %v", err)
	}

	ecrURL := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", awsAccountID, config.AWS.Region)
	loginCmd := exec.Command("docker", "login", "--username", "AWS", "--password-stdin", ecrURL)
	loginCmd.Stdin = strings.NewReader(string(password))
	loginCmd.Stdout = os.Stdout
	loginCmd.Stderr = os.Stderr
	if err := loginCmd.Run(); err != nil {
		return fmt.Errorf("failed to login to ECR: %v", err)
	}
	fmt.Println("Successfully authenticated Docker with ECR")
	return nil
}

func tagDockerImage(awsAccountID string) error {
	cmd := exec.Command("docker", "tag",
		fmt.Sprintf("%s/%s:latest", config.ECR.RepositoryName, config.Lambda.FunctionName),
		fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", awsAccountID, config.AWS.Region, config.ECR.RepositoryName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to tag Docker image: %v", err)
	}
	fmt.Println("Docker image tagged successfully")
	return nil
}

func pushDockerImage(awsAccountID string) error {
	cmd := exec.Command("docker", "push",
		fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", awsAccountID, config.AWS.Region, config.ECR.RepositoryName))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push Docker image: %v", err)
	}
	fmt.Println("Docker image pushed to ECR successfully")
	return nil
}

func updateLambdaFunction(awsAccountID string) error {
	imageUri := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", awsAccountID, config.AWS.Region, config.ECR.RepositoryName)
	updateCodeCmd := exec.Command("aws", "lambda", "update-function-code",
		"--function-name", config.Lambda.FunctionName,
		"--image-uri", imageUri,
		"--profile", config.AWS.Profile,
		"--region", config.AWS.Region)

	output, err := updateCodeCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update Lambda function code: %v\nOutput: %s", err, output)
	}

	fmt.Println("Lambda function code updated successfully")
	return nil
}
