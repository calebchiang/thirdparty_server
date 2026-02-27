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
	"mediator": `You are a calm, fair mediator settling disputes.
In the "reasoning" field, use balanced, neutral, and thoughtful language.
Focus on clarity, fairness, and constructive analysis.`,

	"judge": `You are Judge Judy — direct, no-nonsense, and authoritative.
In the "reasoning" field, be sharp, decisive, and blunt.
Deliver your verdict confidently with strong courtroom energy.`,

	"comedic": `You are a witty, dramatic comedic judge.
In the "reasoning" field, use playful humor, light sarcasm, and entertaining flair.
Be funny and expressive, but still clearly decide a winner. In the last line of the "reasoning" put a funny joke.`,
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
	WinnerName           string `json:"winner_name"`
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

- The FIRST person to speak in the transcript is ALWAYS PERSON A (%s).
- The SECOND person is PERSON B (%s).

STANDARD RULES:
- In the "reasoning" field, ALWAYS refer to them using their actual names (%s and %s).
- In the "winner_name" field, you MUST return ONLY:
  - "%s"
  - "%s"
  - OR "tie"
- You must spell the name EXACTLY as written above.

IMPORTANT RULE (PAY ATTENTION):
- If ANY confirmed instance of lying, deception, dishonesty, manipulation, gaslighting, betrayal, or intentional harm appears in the transcript, that person MUST lose.
- Harmful behavior OVERRIDES tone, politeness, communication style, or emotional delivery.
- Example: If %s lied to %s, then %s is the winner.
- The only exception is if the other person exhibited behavior that is clearly more harmful.

CONVERSATION HEALTH SCORING:
You must score the conversation using these 5 categories from 1–10:

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
  "winner_name": "%s" | "%s" | "tie",
  "reasoning": "2-3 sentence explanation",
  "respect": 1-10,
  "empathy": 1-10,
  "accountability": 1-10,
  "emotional_regulation": 1-10,
  "manipulation_toxicity": 1-10
}

Do NOT include any extra text outside the JSON.`,
		systemPrompt, // 1

		argument.PersonAName, // 2
		argument.PersonBName, // 3

		argument.PersonAName, // 4
		argument.PersonBName, // 5

		argument.PersonAName, // 6
		argument.PersonBName, // 7

		argument.PersonAName, // 8
		argument.PersonBName, // 9

		argument.PersonBName, // 10 (liar in example)
		argument.PersonAName, // 11 (lied to)
		argument.PersonAName, // 12 (winner in example)

		argument.PersonAName, // 13 (JSON option A)
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

	result, err := parseJSONResponse(fullResponse, argument)
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

func parseJSONResponse(response string, argument models.Argument) (*JudgmentResult, error) {

	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")
	response = strings.TrimSpace(response)

	var parsed aiJSONResponse
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		return nil, fmt.Errorf("failed to parse AI JSON response: %v\nRaw response: %s", err, response)
	}

	var mappedWinner string

	switch strings.TrimSpace(strings.ToLower(parsed.WinnerName)) {
	case strings.ToLower(argument.PersonAName):
		mappedWinner = "person_a"
	case strings.ToLower(argument.PersonBName):
		mappedWinner = "person_b"
	case "tie":
		mappedWinner = "tie"
	default:
		return nil, fmt.Errorf("invalid winner_name returned: %s", parsed.WinnerName)
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

	return &JudgmentResult{
		Winner:               mappedWinner,
		Reasoning:            parsed.Reasoning,
		FullResponse:         response,
		Respect:              parsed.Respect,
		Empathy:              parsed.Empathy,
		Accountability:       parsed.Accountability,
		EmotionalRegulation:  parsed.EmotionalRegulation,
		ManipulationToxicity: parsed.ManipulationToxicity,
	}, nil
}

func ProcessJudgment(argumentID uint) {

	fmt.Println("Starting judgment for argument:", argumentID)

	var argument models.Argument
	if err := database.DB.First(&argument, argumentID).Error; err != nil {
		fmt.Println("Failed to load argument:", err)
		return
	}

	fmt.Println("Persona from DB:", argument.Persona)

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
