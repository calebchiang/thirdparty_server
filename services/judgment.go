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
	Winner                  string
	Reasoning               string
	FullResponse            string
	Respect                 int
	Empathy                 int
	Accountability          int
	EmotionalRegulation     int
	ManipulationToxicity    int
	ConversationHealthScore int
}

type aiJSONResponse struct {
	Winner               string `json:"winner"`
	Reasoning            string `json:"reasoning"`
	Respect              int    `json:"respect"`
	Empathy              int    `json:"empathy"`
	Accountability       int    `json:"accountability"`
	EmotionalRegulation  int    `json:"emotional_regulation"`
	ManipulationToxicity int    `json:"manipulation_toxicity"`
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
- The FIRST person to speak in the transcript is PERSON A (%s).
- The SECOND person is PERSON B (%s).

IMPORTANT RULES:
- In the "reasoning" field, ALWAYS refer to them using their actual names (%s and %s).
- NEVER say "Person A" or "Person B" in the reasoning.
- In the "winner" field, you MUST return ONLY:
  "person_a", "person_b", or "tie".
- Never return the actual name in the "winner" field.
- Strongly prefer selecting a winner.

HARMFUL BEHAVIOR OVERRIDE RULE:
- If one person clearly exhibits harmful behavior (e.g. lying, cheating, intentional deception, manipulation, gaslighting, betrayal, abuse, or deliberate dishonesty), that person MUST automatically lose the argument.
- This rule overrides communication style, empathy, tone, or politeness.
- The only exception is if the other person exhibited behavior that is clearly more harmful.
- If both parties exhibit harmful behavior of roughly equal severity, you may return "tie".
- Harmful conduct must be prioritized above presentation quality.

CONVERSATION HEALTH SCORING:
You must also score the conversation using these 5 categories from 1–10:

- respect
- empathy
- accountability
- emotional_regulation
- manipulation_toxicity

Scoring rules:
- 10 = extremely healthy behavior
- 1 = extremely unhealthy behavior
- For manipulation_toxicity: 10 = no manipulation/toxicity present, 1 = extreme manipulation/toxicity

Return ONLY valid JSON using this exact structure:

{
  "winner": "person_a" | "person_b" | "tie",
  "reasoning": "2-3 sentence explanation",
  "respect": 1-10,
  "empathy": 1-10,
  "accountability": 1-10,
  "emotional_regulation": 1-10,
  "manipulation_toxicity": 1-10
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
			Temperature: 0.3,
			MaxTokens:   500,
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

	result, err := parseJSONResponse(fullResponse)
	if err != nil {
		return nil, err
	}

	// Calculate final conversation health score (0–100)
	total := result.Respect +
		result.Empathy +
		result.Accountability +
		result.EmotionalRegulation +
		result.ManipulationToxicity

	conversationHealthScore := total * 2

	return &JudgmentResult{
		Winner:                  result.Winner,
		Reasoning:               result.Reasoning,
		FullResponse:            fullResponse,
		Respect:                 result.Respect,
		Empathy:                 result.Empathy,
		Accountability:          result.Accountability,
		EmotionalRegulation:     result.EmotionalRegulation,
		ManipulationToxicity:    result.ManipulationToxicity,
		ConversationHealthScore: conversationHealthScore,
	}, nil
}

func parseJSONResponse(response string) (*aiJSONResponse, error) {

	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var parsed aiJSONResponse
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse AI JSON response: %v\nRaw response: %s", err, response)
	}

	if parsed.Winner != "person_a" && parsed.Winner != "person_b" && parsed.Winner != "tie" {
		return nil, fmt.Errorf("invalid winner value returned: %s", parsed.Winner)
	}

	// Validate score ranges (1–10)
	validateScore := func(value int, field string) error {
		if value < 1 || value > 10 {
			return fmt.Errorf("%s must be between 1 and 10, got %d", field, value)
		}
		return nil
	}

	if err := validateScore(parsed.Respect, "respect"); err != nil {
		return nil, err
	}
	if err := validateScore(parsed.Empathy, "empathy"); err != nil {
		return nil, err
	}
	if err := validateScore(parsed.Accountability, "accountability"); err != nil {
		return nil, err
	}
	if err := validateScore(parsed.EmotionalRegulation, "emotional_regulation"); err != nil {
		return nil, err
	}
	if err := validateScore(parsed.ManipulationToxicity, "manipulation_toxicity"); err != nil {
		return nil, err
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
		ArgumentID:              argument.ID,
		Winner:                  result.Winner,
		Reasoning:               result.Reasoning,
		FullResponse:            result.FullResponse,
		Respect:                 result.Respect,
		Empathy:                 result.Empathy,
		Accountability:          result.Accountability,
		EmotionalRegulation:     result.EmotionalRegulation,
		ManipulationToxicity:    result.ManipulationToxicity,
		ConversationHealthScore: result.ConversationHealthScore,
	}

	if err := database.DB.Create(&judgment).Error; err != nil {
		fmt.Println("Failed to save judgment:", err)
		database.DB.Model(&argument).Update("status", "failed")
		return
	}

	database.DB.Model(&argument).Update("status", "complete")

	fmt.Println("Judgment saved and argument marked complete:", argumentID)
}
