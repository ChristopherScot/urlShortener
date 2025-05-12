package main

import (
	"context"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Input struct {
	Alias       string `json:"alias"`
	TargetURL   string `json:"target_url"`
	Token       string `json:"token"`
	Description string `json:"description,omitempty"`
	Creator     string `json:"creator,omitempty"`
}

func handler(ctx context.Context, input Input) (string, error) {
	// Load environment variables
	bucket := mustGetEnv("BUCKET_NAME")
	tableName := mustGetEnv("DYNAMODB_TABLE")

	if input.Alias == "" || input.TargetURL == "" {
		return "", fmt.Errorf("missing required fields")
	}

	if input.Token != mustGetEnv("TOKEN") {
		return "", fmt.Errorf("invalid token")
	}

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to load AWS configuration: %v", err)
	}

	// Initialize S3 and DynamoDB clients
	s3Client := s3.NewFromConfig(cfg)
	dynamoClient := dynamodb.NewFromConfig(cfg)

	// Write to S3
	_, err = s3Client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:                  aws.String(bucket),
		Key:                     aws.String(input.Alias),
		WebsiteRedirectLocation: aws.String(input.TargetURL),
		ContentType:             aws.String("text/html"),
		Metadata: map[string]string{
			"description": input.Description,
			"creator":     input.Creator,
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to create redirect in S3: %v", err)
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item: map[string]types.AttributeValue{
			"Alias":       &types.AttributeValueMemberS{Value: input.Alias},
			"TargetURL":   &types.AttributeValueMemberS{Value: input.TargetURL},
			"Description": &types.AttributeValueMemberS{Value: input.Description},
			"Creator":     &types.AttributeValueMemberS{Value: input.Creator},
		},
	})
	if err != nil {
		return "", fmt.Errorf("failed to write to DynamoDB: %v", err)
	}

	// Generate redirect URL
	url := fmt.Sprintf("http://%s.s3-website-%s.amazonaws.com/go/%s", bucket, cfg.Region, input.Alias)
	fmt.Printf("Successfully created redirect: %s\n", url)

	return url, nil
}

func main() {
	lambda.Start(handler)
}

func mustGetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic(fmt.Sprintf("environment variable %s is required but not set", key))
	}
	return value
}
