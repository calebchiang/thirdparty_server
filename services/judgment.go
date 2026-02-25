package services

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/calebchiang/thirdparty_server/database"
	"github.com/calebchiang/thirdparty_server/models"
	openai "github.com/sashabaranov/go-openai"
)

var personaPrompts = map[string]string{
	"mediator": `You are a fair mediator settling disputes.
Give 2-3 sentences about each person's argument - what they got right or wrong.
Then declare the winner clearly.`,

	"judge": `You are Judge Judy - direct and no-nonsense.
Give 2-3 punchy sentences about each person's case. Call out BS when you see it.
Then declare the winner. One catchphrase allowed.`,

	"comedic": `You are a witty comedic judge.
Give 2-3 funny sentences roasting each person's argument.
Then declare the winner with a good punchline.`,
}

type JudgmentResult struct {
	Winner       string // person_a | person_b | tie
	Reasoning    string
	FullResponse string
}

func GenerateJudgment(argument models.Argument) (*JudgmentResult, error) {

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY not set")
	}

	client := openai.NewClient(apiKey)

	systemPrompt, ok := personaPrompts[argument.Persona]
	if !ok {
		systemPrompt = personaPrompts["mediator"]
	}

	systemMessage := fmt.Sprintf(`%s

Settling: %s vs %s

RULES:
1. 2-3 sentences about %s's argument
2. 2-3 sentences about %s's argument
3. End with: VERDICT: [NAME] or VERDICT: TIE
4. Keep total under 150 words.`,
		systemPrompt,
		argument.PersonAName,
		argument.PersonBName,
		argument.PersonAName,
		argument.PersonBName,
	)

	userMessage := fmt.Sprintf(`Transcript:

%s

Give your verdict with 2-3 sentences per person.`,
		argument.Transcription,
	)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       openai.GPT4oMini,
			Temperature: 0.8,
			MaxTokens:   250,
			Messages: []openai.ChatCompletionMessage{
				{Role: openai.ChatMessageRoleSystem, Content: systemMessage},
				{Role: openai.ChatMessageRoleUser, Content: userMessage},
			},
		},
	)

	if err != nil {
		return nil, err
	}

	fullResponse := resp.Choices[0].Message.Content

	winner, reasoning := parseVerdict(
		fullResponse,
		argument.PersonAName,
		argument.PersonBName,
	)

	return &JudgmentResult{
		Winner:       winner,
		Reasoning:    reasoning,
		FullResponse: fullResponse,
	}, nil
}

func parseVerdict(response, personAName, personBName string) (string, string) {

	re := regexp.MustCompile(`(?i)VERDICT:\s*(.+?)(?:\n|$)`)
	matches := re.FindStringSubmatch(response)

	verdictText := ""
	if len(matches) > 1 {
		verdictText = strings.ToLower(strings.TrimSpace(matches[1]))
	}

	winner := "tie"

	if strings.Contains(verdictText, "tie") || strings.Contains(verdictText, "draw") {
		winner = "tie"
	} else if strings.Contains(verdictText, strings.ToLower(personAName)) ||
		strings.Contains(verdictText, "person a") ||
		strings.Contains(verdictText, "first person") {
		winner = "person_a"
	} else if strings.Contains(verdictText, strings.ToLower(personBName)) ||
		strings.Contains(verdictText, "person b") ||
		strings.Contains(verdictText, "second person") {
		winner = "person_b"
	}

	reasoning := regexp.MustCompile(`(?i)VERDICT:.+$`).ReplaceAllString(response, "")
	reasoning = strings.TrimSpace(reasoning)

	return winner, reasoning
}

func ProcessJudgment(argumentID uint) {

	fmt.Println("Starting judgment for argument:", argumentID)

	var argument models.Argument
	if err := database.DB.First(&argument, argumentID).Error; err != nil {
		fmt.Println("Failed to load argument:", err)
		return
	}

	if argument.Status == "complete" {
		fmt.Println("Already completed:", argumentID)
		return
	}

	result, err := GenerateJudgment(argument)
	if err != nil {
		fmt.Println("GenerateJudgment failed:", err)
		database.DB.Model(&argument).Update("status", "failed")
		return
	}

	fmt.Println("Judgment generated successfully.")

	judgment := models.Judgment{
		ArgumentID:   argument.ID,
		Winner:       result.Winner,
		Reasoning:    result.Reasoning,
		FullResponse: result.FullResponse,
	}

	if err := database.DB.Create(&judgment).Error; err != nil {
		fmt.Println("Failed to save judgment:", err)
		database.DB.Model(&argument).Update("status", "failed")
		return
	}

	database.DB.Model(&argument).Update("status", "complete")

	fmt.Println("Judgment saved and argument marked complete:", argumentID)
}
