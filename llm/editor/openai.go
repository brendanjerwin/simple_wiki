package editor

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"log"
	"strings"

	openai "github.com/sashabaranov/go-openai"

	"github.com/brendanjerwin/simple_wiki/common"
)

//go:embed system_prompt.txt
var systemPrompt string

//go:embed quality_prompt.txt
var qualityPrompt string

// Interaction represents the result of an edit operation.
type Interaction struct {
	client             *openai.Client
	logger             *log.Logger
	userCommand        string
	pageContent        string
	assistantResponses []string
	LastResponse       *Response
	Completed          bool
	memory             Memory
}

type UserRequest struct {
	User        string `json:"user"`
	PageContent string `json:"content"`
	Memory      Memory `json:"memory"`
}

type Memory struct {
	Facts         []string `json:"facts"`
	OpenQuestions []string `json:"openQuestions"`
	OpenGoal      string   `json:"openGoal"`
}

type Response struct {
	NewContent       string `json:"new_content,omitempty"`
	SummaryOfChanges string `json:"summary_of_changes,omitempty"`
	Memory           Memory `json:"memory"`
}

func newInteraction(client *openai.Client, logger *log.Logger, userCommand string, memory Memory, content string) Interaction {
	return Interaction{
		client:      client,
		userCommand: userCommand,
		memory:      memory,
		pageContent: content,
	}
}

func responseFromAssistantResponse(assistantResponseText string) (Response, error) {
	var response Response

	if strings.Contains(assistantResponseText, "[[END OUTPUT]]") {
		start := strings.Index(assistantResponseText, "[[BEGIN OUTPUT]]")
		end := strings.Index(assistantResponseText, "[[END OUTPUT]]")
		if start == -1 || end == -1 {
			return response, errors.New("malformed output markers")
		}
		jsonOutput := assistantResponseText[start+len("[[BEGIN OUTPUT]]") : end]
		jsonOutput = strings.Trim(jsonOutput, "```")
		jsonOutput = strings.TrimPrefix(jsonOutput, "json")

		err := json.Unmarshal([]byte(jsonOutput), &response)
		if err != nil {
			return response, err
		}

		return response, nil
	}

	return response, errors.New("no completed output found")
}

func containsResponseMarker(response string) bool {
	return strings.Contains(response, "[[END OUTPUT]]")
}

func containsAllGoodMarker(response string) bool {
	return strings.Contains(response, "[[ALL GOOD]]")
}

func (i Interaction) Respond(userStatement string) (Interaction, error) {
	return i.internalRespond(userStatement, false, 0)
}

// internalRespond processes a user statement and returns an updated Interaction.
func (i Interaction) internalRespond(userStatement string, doQualityControl bool, qualityRoundCounter int) (Interaction, error) {
	if !doQualityControl {
		qualityRoundCounter = 0
	}

	if i.Completed {
		return Interaction{}, errors.New("cannot respond to completed interaction")
	}
	if userStatement == "" {
		return Interaction{}, errors.New("user statement cannot be empty")
	}
	if qualityRoundCounter > 5 {
		return Interaction{}, errors.New("too many quality rounds")
	}
	if i.LastResponse != nil {
		return Interaction{}, errors.New("cannot respond to completed interaction")
	}

	userRequest := UserRequest{
		User:        userStatement,
		PageContent: i.pageContent,
		Memory:      i.memory,
	}
	userRequestJSON, err := json.Marshal(userRequest)
	if err != nil {
		return Interaction{}, err
	}

	// Initial state is to have the system prompt and the user statement
	// If we are doing quality control, we also need to add the previous assistant responses
	// and the quality control prompt
	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemPrompt,
		},
		{
			Role:    openai.ChatMessageRoleUser,
			Content: string(userRequestJSON),
		},
	}

	if doQualityControl {
		for _, response := range i.assistantResponses {
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: response,
			})
		}

		// _AFTER_ the assistant responses, we add the quality prompt if needed.
		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: qualityPrompt,
		})
	}

	response, err := i.client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model:       openai.GPT4,
		Messages:    messages,
		Temperature: 1.0,
		Stop:        []string{"[[FIN]]"},
	})
	if err != nil {
		return Interaction{}, err
	}
	if len(response.Choices) == 0 {
		return Interaction{}, errors.New("no completion choices")
	}
	choice := response.Choices[0]
	if choice.FinishReason != openai.FinishReasonStop {
		return Interaction{}, errors.New("finish reason not stop")
	}
	if choice.Message.Role != openai.ChatMessageRoleAssistant {
		return Interaction{}, errors.New("response not from assistant")
	}
	latestResponseMessage := choice.Message.Content

	if containsResponseMarker(latestResponseMessage) {
		response, err := responseFromAssistantResponse(latestResponseMessage)
		if err != nil {
			return Interaction{}, err
		}
		nextInteraction := newInteraction(i.client, i.logger, userStatement, response.Memory, i.pageContent)
		nextInteraction.assistantResponses = append(i.assistantResponses, latestResponseMessage)
		nextInteraction.LastResponse = &response

		// If there are open questions, we need to ask them
		if len(response.Memory.OpenQuestions) > 0 {
			nextInteraction.Completed = false
			return nextInteraction, nil
		}

		//otherwise its time to start quality control
		return nextInteraction.internalRespond(userStatement, true, qualityRoundCounter)
	} else if containsAllGoodMarker(latestResponseMessage) {
		//got all good, no more quality control is needed so take the last assistant response before this as the final response
		response, err := responseFromAssistantResponse(i.assistantResponses[len(i.assistantResponses)-1])
		if err != nil {
			return Interaction{}, err
		}
		nextInteraction := newInteraction(i.client, i.logger, userStatement, response.Memory, i.pageContent)
		nextInteraction.LastResponse = &response
		nextInteraction.Completed = true
		return nextInteraction, nil
	} else {
		return Interaction{}, errors.New("invalid response from assistant: no response marker")
	}
}

// OpenAIEditor defines the methods for interacting with OpenAI.
type OpenAIEditor interface {
	PerformEdit(markdown, command string, frontMatter common.FrontMatter) (Interaction, error)
}

// OpenAIEditorImpl is the implementation of OpenAIEditor.
type OpenAIEditorImpl struct {
	client *openai.Client
	logger *log.Logger
}

func NewOpenAIEditor(client *openai.Client, logger *log.Logger) OpenAIEditor {
	return OpenAIEditorImpl{
		client: client,
		logger: logger,
	}
}

func memoryFromFrontMatter(frontMatter common.FrontMatter) (Memory, error) {
	// of note: we only take facts from the front matter, not open questions or open goal

	memory := Memory{}
	mem, ok := frontMatter["llm_memory"]
	if !ok {
		return memory, nil
	}
	facts, ok := mem.(map[string]interface{})["facts"]
	if !ok {
		return memory, nil
	}

	factsSlice, ok := facts.([]interface{})
	if !ok {
		return memory, nil
	}

	for _, fact := range factsSlice {
		factString, ok := fact.(string)
		if ok {
			memory.Facts = append(memory.Facts, factString)
		}
	}

	return memory, nil
}

func (e OpenAIEditorImpl) PerformEdit(markdown, command string, frontMatter common.FrontMatter) (Interaction, error) {
	memory, err := memoryFromFrontMatter(frontMatter)
	if err != nil {
		return Interaction{}, err
	}

	interaction := newInteraction(e.client, e.logger, command, memory, markdown)
	return interaction.Respond(command)
}
