package main

// This lambda fetch all of the links from the DynamoDB table and returns them as a JSON response.
import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Input struct {
	Token string `json:"token"`
}

type Link struct {
	Alias       string `json:"alias"`
	TargetURL   string `json:"target_url"`
	Description string `json:"description,omitempty"`
	Creator     string `json:"creator,omitempty"`
}

type Response struct {
	Links []Link `json:"links"`
}

func handler(ctx context.Context, input Input) (Response, error) {
	if input.Token != mustGetEnv("TOKEN") {
		return Response{}, fmt.Errorf("invalid token")
	}

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return Response{}, fmt.Errorf("failed to load AWS configuration: %v", err)
	}

	// Initialize DynamoDB client
	dynamoClient := dynamodb.NewFromConfig(cfg)

	// Fetch all links from DynamoDB
	result, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(mustGetEnv("DYNAMODB_TABLE")),
	})
	if err != nil {
		return Response{}, fmt.Errorf("failed to fetch links from DynamoDB: %v", err)
	}

	// Map DynamoDB items to Link structs
	var links []Link
	for _, item := range result.Items {
		links = append(links, Link{
			Alias:       item["Alias"].(*types.AttributeValueMemberS).Value,
			TargetURL:   item["TargetURL"].(*types.AttributeValueMemberS).Value,
			Description: item["Description"].(*types.AttributeValueMemberS).Value,
			Creator:     item["Creator"].(*types.AttributeValueMemberS).Value,
		})
	}

	return Response{Links: links}, nil
}

func mustGetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("environment variable %s is required but not set", key))
	}
	return value
}

func main() {
	lambda.Start(handler)
}
