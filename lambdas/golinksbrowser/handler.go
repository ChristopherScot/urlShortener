package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"net/url"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	lambdasdk "github.com/aws/aws-sdk-go-v2/service/lambda"
)

//go:embed index.go.html
var indexHTML string

type CRUDLinkInput struct {
	FunctionName string `json:"function_name"`
	Action       string `json:"action"` // "create", "read", "update", or "delete"
	Alias        string `json:"alias,omitempty"`
	TargetURL    string `json:"target_url,omitempty"`
	Token        string `json:"token"`
	Description  string `json:"description,omitempty"`
	Creator      string `json:"creator,omitempty"`
}

type Link struct {
	Alias       string `json:"alias"`
	TargetURL   string `json:"target_url"`
	Description string `json:"description,omitempty"`
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	linksCrudLambda := os.Getenv("LINKS_CRUD_LAMBDA")
	token := os.Getenv("TOKEN")

	// Handle form submission to add, update, or delete a URL
	if req.RequestContext.HTTP.Method == "POST" {
		action := "create" // default
		var values url.Values

		body := req.Body
		if req.IsBase64Encoded {
			decodedBody, err := base64.StdEncoding.DecodeString(req.Body)
			if err != nil {
				return events.APIGatewayProxyResponse{
					StatusCode: 400,
					Body:       "Failed to decode request body: " + err.Error(),
				}, nil
			}
			body = string(decodedBody)
		}
		values, err := url.ParseQuery(body)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 400,
				Body:       "Failed to parse request body: " + err.Error(),
			}, nil
		}
		if v := values.Get("action"); v != "" {
			action = v
		}

		// In the UI I prepend the alias with "go/" for the create action
		// so we want to make sure the real value gets this as well
		// but if we do it on other actions we're going to keep adding "go/"
		// to things that already have that prefix.
		alias := values.Get("alias")
		if action == "create" {
			alias = "go/" + alias
		}

		CRUDLinkInput := CRUDLinkInput{
			FunctionName: linksCrudLambda,
			Action:       action,
			Alias:        alias,
			TargetURL:    values.Get("target_url"),
			Description:  values.Get("description"),
			Token:        token,
		}

		_, err = crudLink(ctx, CRUDLinkInput)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Body:       fmt.Sprintf("Failed to %s link: %v", action, err),
			}, nil
		}

		// Redirect back to the main page after the operation
		return events.APIGatewayProxyResponse{
			StatusCode: 302,
			Headers: map[string]string{
				"Location": "/",
			},
		}, nil
	}

	// Handle GET request to display the webpage
	links, err := crudLink(ctx, CRUDLinkInput{
		FunctionName: linksCrudLambda,
		Action:       "read",
		Token:        token,
	})

	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Failed to fetch links from Lambda: " + err.Error(),
		}, nil
	}

	tmpl, err := template.New("index.go.html").Parse(indexHTML)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Failed to load template: " + err.Error(),
		}, nil
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, struct{ Links []Link }{Links: links})
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Failed to render template: " + err.Error(),
		}, nil
	}

	// Return the HTML response
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
		Body: buf.String(),
	}, nil
}

func crudLink(ctx context.Context, i CRUDLinkInput) ([]Link, error) {
	links := []Link{}

	payload := map[string]string{
		"action":      i.Action,
		"alias":       i.Alias,
		"target_url":  i.TargetURL,
		"description": i.Description,
		"token":       i.Token,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return links, fmt.Errorf("failed to marshal payload to JSON: %w", err)
	}

	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return links, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	client := lambdasdk.NewFromConfig(cfg)

	resp, err := client.Invoke(ctx, &lambdasdk.InvokeInput{
		FunctionName: aws.String(i.FunctionName),
		Payload:      payloadBytes,
	})
	if err != nil {
		return links, fmt.Errorf("failed to invoke Lambda: %w", err)
	}
	if resp.StatusCode != 200 {
		return links, fmt.Errorf("unexpected Lambda response: %d", resp.StatusCode)
	}

	// Unmarshal only if links are present in the response
	var lambdaResp struct {
		Links []Link `json:"links"`
		Error string `json:"error,omitempty"`
	}
	if err := json.Unmarshal(resp.Payload, &lambdaResp); err != nil {
		return links, fmt.Errorf("failed to unmarshal Lambda response: %w", err)
	}
	if lambdaResp.Error != "" {
		return links, fmt.Errorf("lambda error: %s", lambdaResp.Error)
	}
	if lambdaResp.Links != nil {
		links = lambdaResp.Links
	}

	return links, nil
}
