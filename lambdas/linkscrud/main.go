package main

import (
	"context"
	"fmt"

	"github.com/ChristopherScot/urlShortener/lambdas/linkscrud/models"
	"github.com/ChristopherScot/urlShortener/lambdas/linkscrud/util"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type Input struct {
	Action      string `json:"action"` // "create", "read", "update", or "delete"
	Alias       string `json:"alias,omitempty"`
	TargetURL   string `json:"target_url,omitempty"`
	Token       string `json:"token"`
	Description string `json:"description,omitempty"`
	Creator     string `json:"creator,omitempty"`
}

type Response struct {
	Links []models.Link `json:"links,omitempty"`
	URL   string        `json:"url,omitempty"`
	Error string        `json:"error,omitempty"`
}

func handler(ctx context.Context, input Input) (Response, error) {
	if input.Token != util.MustGetEnv("TOKEN") {
		return Response{Error: "invalid token"}, nil
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return Response{Error: fmt.Sprintf("failed to load AWS configuration: %v", err)}, nil
	}

	switch input.Action {
	case "create":
		return createLink(ctx, cfg, input)
	case "read":
		return getLinks(ctx, cfg)
	case "update":
		return updateLink(ctx, cfg, input)
	case "delete":
		return deleteLink(ctx, cfg, input)

	default:
		return Response{Error: "invalid action"}, nil
	}
}

func getLinks(ctx context.Context, cfg aws.Config) (Response, error) {
	tableName := util.MustGetEnv("DYNAMODB_TABLE")
	dynamoClient := dynamodb.NewFromConfig(cfg)

	result, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		return Response{Error: fmt.Sprintf("failed to fetch links from DynamoDB: %v", err)}, nil
	}

	var links []models.Link
	for _, item := range result.Items {
		links = append(links, models.Link{
			Alias:       getAttr(item, "Alias"),
			TargetURL:   getAttr(item, "TargetURL"),
			Description: getAttr(item, "Description"),
			Creator:     getAttr(item, "Creator"),
		})
	}
	return Response{Links: links}, nil
}

func createLink(ctx context.Context, cfg aws.Config, input Input) (Response, error) {
	bucket := util.MustGetEnv("BUCKET_NAME")
	tableName := util.MustGetEnv("DYNAMODB_TABLE")

	if input.Alias == "" || input.TargetURL == "" {
		return Response{Error: "missing required fields"}, nil
	}

	if input.Alias[:3] != "go/" {
		return Response{Error: "alias must start with 'go/'"}, nil
	}

	// S3
	s3Client := s3.NewFromConfig(cfg)
	_, err := s3Client.PutObject(ctx, &s3.PutObjectInput{
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
		return Response{Error: fmt.Sprintf("failed to create redirect in S3: %v", err)}, nil
	}

	// DynamoDB
	dynamoClient := dynamodb.NewFromConfig(cfg)
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
		return Response{Error: fmt.Sprintf("failed to write to DynamoDB: %v", err)}, nil
	}

	url := fmt.Sprintf("http://%s.s3-website-%s.amazonaws.com/go/%s", bucket, cfg.Region, input.Alias)
	return Response{URL: url}, nil
}

func updateLink(ctx context.Context, cfg aws.Config, input Input) (Response, error) {
	tableName := util.MustGetEnv("DYNAMODB_TABLE")
	if input.Alias == "" {
		return Response{Error: "missing alias for update"}, nil
	}

	updateExpr := "SET TargetURL = :url, Description = :desc, Creator = :creator"
	exprAttrVals := map[string]types.AttributeValue{
		":url":     &types.AttributeValueMemberS{Value: input.TargetURL},
		":desc":    &types.AttributeValueMemberS{Value: input.Description},
		":creator": &types.AttributeValueMemberS{Value: input.Creator},
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	_, err := dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"Alias": &types.AttributeValueMemberS{Value: input.Alias},
		},
		UpdateExpression:          aws.String(updateExpr),
		ExpressionAttributeValues: exprAttrVals,
	})
	if err != nil {
		return Response{Error: fmt.Sprintf("failed to update item: %v", err)}, nil
	}
	return Response{}, nil
}

func deleteLink(ctx context.Context, cfg aws.Config, input Input) (Response, error) {
	tableName := util.MustGetEnv("DYNAMODB_TABLE")
	bucket := util.MustGetEnv("BUCKET_NAME")
	if input.Alias == "" {
		return Response{Error: "missing alias for delete"}, nil
	}

	// Delete from DynamoDB
	dynamoClient := dynamodb.NewFromConfig(cfg)
	_, err := dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"Alias": &types.AttributeValueMemberS{Value: input.Alias},
		},
	})
	if err != nil {
		return Response{Error: fmt.Sprintf("failed to delete from DynamoDB: %v", err)}, nil
	}

	// Delete from S3
	s3Client := s3.NewFromConfig(cfg)
	_, err = s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(input.Alias),
	})
	if err != nil {
		return Response{Error: fmt.Sprintf("failed to delete from S3: %v", err)}, nil
	}

	return Response{}, nil
}

func getAttr(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

func main() {
	lambda.Start(handler)
}
