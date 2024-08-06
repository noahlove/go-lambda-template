package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

const (
	region        = "us-west-2"
	functionName  = "hello-world-lambda"
	ecrRepository = "hello-world-repo"
	awsProfile    = "personal"
)

func main() {
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

func checkIAMPermissions() error {
	cmd := exec.Command("aws", "iam", "get-user", "--profile", awsProfile)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to get IAM user info: %v\nOutput: %s", err, output)
	}
	fmt.Println("Successfully retrieved IAM user info. You have the necessary permissions.")
	return nil
}

func getAWSAccountID() (string, error) {
	cmd := exec.Command("aws", "sts", "get-caller-identity", "--query", "Account", "--output", "json", "--profile", awsProfile)
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
	cmd := exec.Command("docker", "build", "-t", fmt.Sprintf("%s/%s", ecrRepository, functionName), ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build Docker image: %v", err)
	}
	fmt.Println("Docker image built successfully")
	return nil
}

func authenticateDocker(awsAccountID string) error {
	cmd := exec.Command("aws", "ecr", "get-login-password", "--region", region, "--profile", awsProfile)
	password, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get ECR login password: %v", err)
	}

	ecrURL := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com", awsAccountID, region)
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
		fmt.Sprintf("%s/%s:latest", ecrRepository, functionName),
		fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", awsAccountID, region, ecrRepository))
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
		fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", awsAccountID, region, ecrRepository))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to push Docker image: %v", err)
	}
	fmt.Println("Docker image pushed to ECR successfully")
	return nil
}

func updateLambdaFunction(awsAccountID string) error {
	imageUri := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", awsAccountID, region, ecrRepository)
	updateCodeCmd := exec.Command("aws", "lambda", "update-function-code",
		"--function-name", functionName,
		"--image-uri", imageUri,
		"--profile", awsProfile,
		"--region", region)

	output, err := updateCodeCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to update Lambda function code: %v\nOutput: %s", err, output)
	}

	fmt.Println("Lambda function code updated successfully")
	return nil
}
