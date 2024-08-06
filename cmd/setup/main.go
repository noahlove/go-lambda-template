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
		RoleName     string `yaml:"role_name"`
	} `yaml:"lambda"`
	ECR struct {
		RepositoryName string `yaml:"repository_name"`
	} `yaml:"ecr"`
}

var config Config

func main() {
	// Load configuration
	if err := loadConfig("config.yaml"); err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

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

	// Build and push Docker image
	if err := buildAndPushDockerImage(awsAccountID); err != nil {
		log.Fatalf("Error building and pushing Docker image: %v", err)
	}

	// Create Lambda function with a container image
	if err := createLambdaFunction(roleARN, awsAccountID); err != nil {
		log.Printf("Error creating Lambda function: %v", err)
	} else {
		fmt.Println("Lambda function created successfully")
	}
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

func getOrCreateLambdaExecutionRole() (string, error) {
	// Try to get the role first
	getRoleCmd := exec.Command("aws", "iam", "get-role",
		"--role-name", config.Lambda.RoleName,
		"--profile", config.AWS.Profile)

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
		"--role-name", config.Lambda.RoleName,
		"--assume-role-policy-document", `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":"lambda.amazonaws.com"},"Action":"sts:AssumeRole"}]}`,
		"--profile", config.AWS.Profile)

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
		"--role-name", config.Lambda.RoleName,
		"--policy-arn", "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole",
		"--profile", config.AWS.Profile)

	output, err = attachPolicyCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("error attaching policy to role: %v\n%s", err, output)
	}

	fmt.Println("Lambda execution role created successfully")
	return roleResponse.Role.Arn, nil
}

func createECRRepository() error {
	createECRCmd := exec.Command("aws", "ecr", "create-repository",
		"--repository-name", config.ECR.RepositoryName,
		"--profile", config.AWS.Profile,
		"--region", config.AWS.Region)

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

func createLambdaFunction(roleARN string, awsAccountID string) error {
	imageUri := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", awsAccountID, config.AWS.Region, config.ECR.RepositoryName)

	createLambdaCmd := exec.Command("aws", "lambda", "create-function",
		"--function-name", config.Lambda.FunctionName,
		"--package-type", "Image",
		"--code", fmt.Sprintf("ImageUri=%s", imageUri),
		"--role", roleARN,
		"--profile", config.AWS.Profile,
		"--region", config.AWS.Region)

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

func buildAndPushDockerImage(awsAccountID string) error {
	// Log in to ECR
	loginCmd := exec.Command("aws", "ecr", "get-login-password",
		"--region", config.AWS.Region,
		"--profile", config.AWS.Profile)
	loginCmd.Stdout = os.Stdout
	loginCmd.Stderr = os.Stderr
	if err := loginCmd.Run(); err != nil {
		return fmt.Errorf("failed to get ECR login: %v", err)
	}

	// Build Docker image
	buildCmd := exec.Command("docker", "build", "-t", config.ECR.RepositoryName, ".")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("failed to build Docker image: %v", err)
	}

	// Tag Docker image
	imageUri := fmt.Sprintf("%s.dkr.ecr.%s.amazonaws.com/%s:latest", awsAccountID, config.AWS.Region, config.ECR.RepositoryName)
	tagCmd := exec.Command("docker", "tag", config.ECR.RepositoryName, imageUri)
	tagCmd.Stdout = os.Stdout
	tagCmd.Stderr = os.Stderr
	if err := tagCmd.Run(); err != nil {
		return fmt.Errorf("failed to tag Docker image: %v", err)
	}

	// Push Docker image to ECR
	pushCmd := exec.Command("docker", "push", imageUri)
	pushCmd.Stdout = os.Stdout
	pushCmd.Stderr = os.Stderr
	if err := pushCmd.Run(); err != nil {
		return fmt.Errorf("failed to push Docker image to ECR: %v", err)
	}

	fmt.Println("Docker image built and pushed successfully")
	return nil
}
