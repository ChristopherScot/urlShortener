package ai

import (
	"context"

	"github.com/ChristopherScot/urlShortener/lambdas/linkscrud/models"

	"github.com/openai/openai-go"
)

func getBestGuess(links []models.Links, input string) string {
	if len(links) == 0 {
		return ""
	}
	if len(links) == 1 {
		return links[0]
	}
	// For simplicity, return the first link as the best guess
	return links[0]
}

func main() {
	client := openai.NewClient() // Key is set to OPENAI_API_KEY
	chatCompletion, err := client.Chat.Completions.New(context.TODO(), openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage("Say this is a test"),
		},
		Model: openai.ChatModelGPT4o,
	})
	if err != nil {
		panic(err.Error())
	}
	println(chatCompletion.Choices[0].Message.Content)
}
