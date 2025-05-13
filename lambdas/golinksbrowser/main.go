package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	"github.com/aws/aws-lambda-go/events"
	lambdago "github.com/aws/aws-lambda-go/lambda" // Alias this import to avoid conflict
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	lambdasdk "github.com/aws/aws-sdk-go-v2/service/lambda" // Alias this import to avoid conflict
)

type GetLinksResponse struct {
	Links []Link `json:"links"`
}

type Link struct {
	Alias       string `json:"alias"`
	TargetURL   string `json:"target_url"`
	Description string `json:"description,omitempty"`
}

func handler(ctx context.Context, req events.APIGatewayV2HTTPRequest) (events.APIGatewayProxyResponse, error) {
	fmt.Printf("Received request with body: %s\n and method %s\n", req.Body, req.RequestContext.HTTP.Method)
	fmt.Println("Headers:", req.Headers)
	if req.RequestContext.HTTP.Method == "POST" {
		// Handle form submission to add a new URL
		createLinkLambda := os.Getenv("CREATE_LINK_LAMBDA")
		token := req.Headers["token"]
		err := createLink(ctx, createLinkLambda, req.Body, req.IsBase64Encoded, token)
		if err != nil {
			return events.APIGatewayProxyResponse{
				StatusCode: 500,
				Body:       "Failed to create link: " + err.Error(),
			}, nil
		}

		// Redirect back to the main page after adding the URL
		return events.APIGatewayProxyResponse{
			StatusCode: 302,
			Headers: map[string]string{
				"Location": "/",
			},
		}, nil
	}

	// Handle GET request to display the webpage
	getLinksLambda := os.Getenv("GET_LINKS_LAMBDA")
	links, err := fetchLinks(ctx, getLinksLambda)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "Failed to fetch links from Lambda: " + err.Error(),
		}, nil
	}

	// Generate the HTML content
	html := `
    <!DOCTYPE html>
    <html lang="en">
    <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <title>URL Shortener</title>
    </head>
    <body>
        <h1>Welcome to the URL Shortener</h1>
        <p>This is a simple webpage served by AWS Lambda.</p>
        <table border="1">
            <tr>
                <th>Alias</th>
                <th>Target URL</th>
                <th>Description</th>
            </tr>
    `

	// Add rows to the table for each link
	for _, link := range links {
		html += fmt.Sprintf(`
            <tr>
                <td>%s</td>
                <td><a href="%s">%s</a></td>
                <td>%s</td>
            </tr>
        `, link.Alias, link.TargetURL, link.TargetURL, link.Description)
	}

	html += `
        </table>
        <h2>Add a New URL</h2>
        <form method="POST" action="/">
            <label for="alias">Alias:</label>
            <input type="text" id="alias" name="alias" required>
            <br>
            <label for="url">Target URL:</label>
            <input type="url" id="target_url" name="target_url" required>
            <br>
            <label for="description">Description:</label>
            <input type="text" id="description" name="description">
            <br>
            <button type="submit">Add URL</button>
        </form>
    </body>
    </html>
    `

	// Return the HTML response
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
		Body: html,
	}, nil
}

func fetchLinks(ctx context.Context, functionName string) ([]Link, error) {
	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Create a Lambda client
	client := lambdasdk.NewFromConfig(cfg)

	// Prepare the payload for invoking the get-links Lambda
	payload := map[string]string{
		"token": os.Getenv("TOKEN"),
	}
	payloadBytes, _ := json.Marshal(payload)

	// Invoke the get-links Lambda
	resp, err := client.Invoke(ctx, &lambdasdk.InvokeInput{
		FunctionName: aws.String(functionName),
		Payload:      payloadBytes,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to invoke Lambda: %w", err)
	}
	fmt.Println("Response from get-links Lambda:", string(resp.Payload))

	// Parse the response into the wrapper struct
	var response GetLinksResponse
	err = json.Unmarshal(resp.Payload, &response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Lambda response: %w", err)
	}

	return response.Links, nil
}
func createLink(ctx context.Context, functionName, requestBody string, isBase64Encoded bool, token string) error {
	// Decode the Base64-encoded body if necessary
	if isBase64Encoded {
		decodedBody, err := base64.StdEncoding.DecodeString(requestBody)
		if err != nil {
			return fmt.Errorf("failed to decode Base64 body: %w", err)
		}
		requestBody = string(decodedBody)
	}

	fmt.Println("Decoded request body:", requestBody)

	// Parse the URL-encoded body
	values, err := url.ParseQuery(requestBody)
	if err != nil {
		return fmt.Errorf("failed to parse request body: %w", err)
	}

	// Convert the parsed values to a JSON object
	payload := map[string]string{
		"alias":       values.Get("alias"),
		"target_url":  values.Get("target_url"),
		"description": values.Get("description"),
		"token":       token,
	}

	payloadBytes, err := json.Marshal(payload)
	fmt.Println("Payload to be sent to Lambda:", string(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to marshal payload to JSON: %w", err)
	}

	// Load the AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	// Create a Lambda client
	client := lambdasdk.NewFromConfig(cfg)

	// Invoke the create-link Lambda
	resp, err := client.Invoke(ctx, &lambdasdk.InvokeInput{
		FunctionName: aws.String(functionName),
		Payload:      payloadBytes,
	})
	if err != nil {
		return fmt.Errorf("failed to invoke Lambda: %w", err)
	}
	fmt.Println("Response from create-link Lambda:", string(resp.Payload))

	return nil
}

func main() {
	lambdago.Start(handler)
}
