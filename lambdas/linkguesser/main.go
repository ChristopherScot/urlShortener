package main

import (
	"context"
	"log"
	"strings"

	ai "github.com/ChristopherScot/urlShortener/lambdas/linkguesser/internal"
	"github.com/ChristopherScot/urlShortener/shared/models"
	"github.com/ChristopherScot/urlShortener/shared/util"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func handler(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	tableName := util.MustGetEnv("DYNAMODB_TABLE")

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		log.Printf("failed to load AWS config: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}

	client := dynamodb.NewFromConfig(cfg)
	out, err := client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		log.Printf("failed to scan DynamoDB: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}

	links := make([]models.Link, 0, len(out.Items))
	for _, item := range out.Items {
		targetURL := getStringAttr(item, "TargetURL")
		if targetURL == "" {
			continue // skip if no TargetURL
		}
		links = append(links, models.Link{
			TargetURL:   targetURL,
			Alias:       getStringAttr(item, "Alias"),
			Description: getStringAttr(item, "Description"),
			Creator:     getStringAttr(item, "Creator"),
		})
	}

	if len(links) == 0 {
		return events.APIGatewayProxyResponse{
			StatusCode: 404,
			Body:       "No links found",
		}, nil
	}

	fullPath := request.QueryStringParameters["path"]
	// If a path contains characters after the first slash we want to match on the
	// beginning of the path and then append the rest of the path to the target URL.
	pathPrefix := ""
	pathSuffix := ""
	if fullPath != "" {
		firstSlashIndex := strings.Index(fullPath, "/")
		if firstSlashIndex == -1 {
			// No slash found, treat the entire fullPath as the prefix
			pathPrefix = fullPath
			pathSuffix = ""
		} else {
			// Split the path into prefix and suffix
			pathPrefix = fullPath[:firstSlashIndex]
			pathSuffix = fullPath[firstSlashIndex:]
		}
	}

	target := getTargetIfExists(links, pathPrefix) + pathSuffix

	if target == "" {
		target = ai.GetBestGuess(links, fullPath) + pathSuffix
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 302,
		Headers: map[string]string{
			"Location": target,
		},
	}, nil
}

func main() {
	lambda.Start(handler)
}

func getStringAttr(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key]; ok {
		if s, ok := v.(*types.AttributeValueMemberS); ok {
			return s.Value
		}
	}
	return ""
}

func getTargetIfExists(links []models.Link, prefix string) string {
	for _, link := range links {
		if link.Alias == "go/"+prefix {
			return link.TargetURL
		}
	}
	return ""
}
