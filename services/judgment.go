package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/calebchiang/thirdparty_server/database"
	"github.com/calebchiang/thirdparty_server/models"
	openai "github.com/sashabaranov/go-openai"
)

var personaPrompts = map[string]string{
	"mediator": `You are a fair mediator settling disputes.
Give balanced, thoughtful analysis.`,

	"judge": `You are Judge Judy - direct and no-nonsense.
Be sharp, decisive, and confident.`,

	"comedic": `You are a witty comedic judge.
Be funny but still clearly decide a winner.`,
}

type JudgmentResult struct {
	Winner       string // person_a | person_b | tie
	Reasoning    string
	FullResponse string
}

type aiJSONResponse struct {
	Winner    string `json:"winner"`
	Reasoning string `json:"reasoning"`
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

You are judging a dispute between two people.

PERSON A = %s
PERSON B = %s

IMPORTANT RULES:
- In the "reasoning" field, ALWAYS refer to them using their actual names (%s and %s).
- NEVER say "Person A" or "Person B" in the reasoning.
- In the "winner" field, you MUST return ONLY:
  "person_a", "person_b", or "tie".
- Never return the actual name in the "winner" field.
- The FIRST person to speak in the transcript is PERSON A (%s).
- The SECOND person is PERSON B (%s).

When deciding the winner, prioritize in this order:

1. Serious breaches of trust (cheating, dishonesty, betrayal).
2. Harm caused to the other person.
3. Escalation and responsibility in the conflict.
4. Logic and clarity of reasoning.
5. Emotional maturity.
6. Accountability for harmful actions.

If one person committed a serious breach of trust, that person must lose unless the other party committed a clearly more severe violation.

Communication skill does NOT override wrongdoing.

Only return "tie" if both parties are truly equal across all factors.

The winner MUST logically match your reasoning.
If your reasoning criticizes someone for serious wrongdoing, that person cannot be the winner.

You MUST respond ONLY in valid JSON using this exact structure:

{
  "winner": "person_a" | "person_b" | "tie",
  "reasoning": "2-3 sentence explanation"
}

Do NOT include any extra text outside the JSON.`,
		systemPrompt,
		argument.PersonAName,
		argument.PersonBName,
		argument.PersonAName,
		argument.PersonBName,
		argument.PersonAName,
		argument.PersonBName,
	)

	userMessage := fmt.Sprintf(`Transcript:

%s

Analyze and return your judgment in JSON format.`,
		argument.Transcription,
	)

	resp, err := client.CreateChatCompletion(
		context.Background(),
		openai.ChatCompletionRequest{
			Model:       openai.GPT4oMini,
			Temperature: 0.7,
			MaxTokens:   300,
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

	// Parse JSON safely
	result, err := parseJSONResponse(fullResponse)
	if err != nil {
		return nil, err
	}

	return &JudgmentResult{
		Winner:       result.Winner,
		Reasoning:    result.Reasoning,
		FullResponse: fullResponse,
	}, nil
}

func parseJSONResponse(response string) (*aiJSONResponse, error) {

	// Sometimes model may wrap JSON in ```json blocks
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var parsed aiJSONResponse
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse AI JSON response: %v\nRaw response: %s", err, response)
	}

	// Validate winner
	if parsed.Winner != "person_a" && parsed.Winner != "person_b" && parsed.Winner != "tie" {
		return nil, fmt.Errorf("invalid winner value returned: %s", parsed.Winner)
	}

	return &parsed, nil
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
