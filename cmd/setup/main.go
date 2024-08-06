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
	roleName      = "lambda-execution-role"
	awsProfile    = "personal"
)

func main() {
	// Check if LAMBDA_EXECUTION_ROLE_ARN exists
	roleARN := os.Getenv("LAMBDA_EXECUTION_ROLE_ARN")
	if roleARN == "" {
		var err error
		roleARN, err = getOrCreateLambdaExecutionRole()
		if err != nil {
			log.Fatalf("Failed to get or create Lambda execution role: %v", err)
		}
		os.Setenv("LAMBDA_EXECUTION_ROLE_ARN", roleARN)
	}

	// Create ECR repository
	if err := createECRRepository(); err != nil {
		log.Printf("Error creating ECR repository: %v", err)
	} else {
		fmt.Println("ECR repository created successfully")
	}

	// Get AWS Account ID
	awsAccountID, err := getAWSAccountID()
	if err != nil {
		log.Fatalf("Error getting AWS Account ID: %v", err)
	}

	// Create Lambda function with a container image
	if err := createLambdaFunction(roleARN, awsAccountID); err != nil {
		log.Printf("Error creating Lambda function: %v", err)
	} else {
		fmt.Println("Lambda function created successfully")
	}
}

func getOrCreateLambdaExecutionRole() (string, error) {
	// Try to get the role first
	getRoleCmd := exec.Command("aws", "iam", "get-role",
		"--role-name", roleName,
		"--profile", "personal")

	output, err := getRoleCmd.CombinedOutput()
	if err == nil {
		var roleResponse struct {
			Role struct {
				Arn string `json:"Arn"`
			} `json:"Role"`
		}
		if err := json.Unmarshal(output, &roleResponse); err != nil {
			return "", fmt.Errorf("error parsing role response: %v", err)
		}
		fmt.Println("Lambda execution role already exists")
		return roleResponse.Role.Arn, nil
	}

	// If the role doesn't exist, create it
	createRoleCmd := exec.Command("aws", "iam", "create-role",
		"--role-name", roleName,
		"--assume-role-policy-document", `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`,
		"--profile", "personal")

	output, err = createRoleCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error creating IAM role: %v\n%s", err, output)
	}

	var roleResponse struct {
		Role struct {
			Arn string `json:"Arn"`
		} `json:"Role"`
	}
	if err := json.Unmarshal(output, &roleResponse); err != nil {
		return "", fmt.Errorf("error parsing role creation response: %v", err)
	}

	// Attach AWSLambdaBasicExecutionRole policy
	attachPolicyCmd := exec.Command("aws", "iam", "attach-role-policy",
		"--role-name", roleName,
		"--policy-arn", "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole",
		"--profile", "personal")

	output, err = attachPolicyCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error attaching policy to role: %v\n%s", err, output)
	}

	fmt.Println("Lambda execution role created successfully")
	return roleResponse.Role.Arn, nil
}

func createECRRepository() error {
	createECRCmd := exec.Command("aws", "ecr", "create-repository",
		"--repository-name", ecrRepository,
		"--profile", "personal",
		"--region", region)

	output, err := createECRCmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "RepositoryAlreadyExistsException") {
			fmt.Println("ECR repository already exists")
			return nil
		}
		return fmt.Errorf("error creating ECR repository: %v\n%s", err, output)
	}
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
func createLambdaFunction(roleARN string, awsAccountID string) error {
	imageUri := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", awsAccountID, region, ecrRepository)

	createLambdaCmd := exec.Command("aws", "lambda", "create-function",
		"--function-name", functionName,
		"--package-type", "Image",
		"--code", fmt.Sprintf("ImageUri=%s", imageUri),
		"--role", roleARN,
		"--profile", awsProfile,
		"--region", region)

	output, err := createLambdaCmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(output), "ResourceConflictException") {
			fmt.Println("Lambda function already exists")
			return nil
		}
		return fmt.Errorf("error creating Lambda function: %v\n%s", err, output)
	}

	fmt.Println("Lambda function created successfully")
	return nil
}
