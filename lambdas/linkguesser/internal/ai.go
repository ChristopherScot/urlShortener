package ai

import (
	"context"
	"fmt"

	"github.com/ChristopherScot/urlShortener/shared/models"

	"github.com/openai/openai-go"
)

// GetBestGuess passes the provided links and the user input to the AI model
// and then asks for it to return it's best guess based on the links provided of
// which link the user is most likely referring to.
func GetBestGuess(links []models.Link, input string) string {
	if len(links) == 0 {
		return ""
	}
	if len(links) == 1 {
		return links[0].TargetURL
	}
	// For simplicity, return the first link as the best guess
	client := openai.NewClient() // Key is set to OPENAI_API_KEY
	chatCompletion, err := client.Chat.Completions.New(context.Background(), openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Given the following links, which one is the best guess for the user input? Respond with ONLY the targetURL value."),
			openai.UserMessage("User input: " + input),
			openai.UserMessage("Links: " + formatLinks(links)),
		},
		Model: openai.ChatModelGPT4o,
	})
	if err != nil {
		panic(err.Error())
	}
	return chatCompletion.Choices[0].Message.Content
}

func formatLinks(links []models.Link) string {
	formattedLinks := ""
	for _, link := range links {
		formattedLinks += fmt.Sprintf("TargetURL: %s, Alias: %s, Description: %s, Creator: %s\n",
			link.TargetURL,
			link.Alias,
			link.Description,
			link.Creator,
		)
	}
	return formattedLinks
}
