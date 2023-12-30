package editor

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jcelliott/lumber"
	openai "github.com/sashabaranov/go-openai"

	"github.com/brendanjerwin/simple_wiki/common"
)

//go:embed system_prompt.txt
var systemPrompt string

//go:embed quality_prompt.txt
var qualityPrompt string

// Interaction represents the result of an edit operation.
type InteractionID string

func generateInteractionID() (InteractionID, error) {
	randBytes := make([]byte, 8)
	_, err := rand.Read(randBytes)
	if err != nil {
		return "", err
	}
	randNum := binary.BigEndian.Uint64(randBytes)
	interactionID := time.Now().UnixNano() + int64(randNum)

	hash := sha256.Sum256([]byte(strconv.FormatInt(interactionID, 10)))
	return InteractionID(fmt.Sprintf("%x", hash)), nil
}

type Interaction struct {
	client                    *openai.Client
	logger                    *lumber.ConsoleLogger
	userCommand               string
	pageContent               string
	assistantResponses        []string
	PageIdentifier            common.PageIdentifier
	PageAccessedTimestamp     time.Time
	LastResponse              *Response
	InteractionID             InteractionID
	memory                    Memory
	LastResponseQualityRounds int
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

func (m Memory) ToFrontmatterMap() map[string]interface{} {
	memoryMap := make(map[string]interface{})
	memoryMap["facts"] = m.Facts
	return memoryMap
}

type Response struct {
	NewContent     string `json:"new_content,omitempty"`
	ResponseToUser string `json:"response_to_user,omitempty"`
	Memory         Memory `json:"memory"`
}

func newInteraction(client *openai.Client, logger *lumber.ConsoleLogger, userCommand string, memory Memory, content string, pageIdentifier common.PageIdentifier, pageAccessedTimestamp time.Time) (*Interaction, error) {
	interaction := &Interaction{
		client:                client,
		userCommand:           userCommand,
		memory:                memory,
		pageContent:           content,
		logger:                logger,
		PageIdentifier:        pageIdentifier,
		PageAccessedTimestamp: pageAccessedTimestamp,
	}

	id, err := generateInteractionID()
	if err != nil {
		return nil, err
	}
	interaction.InteractionID = id
	interaction.freezeToRam()
	return interaction, nil
}

func (i *Interaction) clone(userCommand string, memory Memory) (*Interaction, error) {
	return newInteraction(i.client, i.logger, userCommand, memory, i.pageContent, i.PageIdentifier, i.PageAccessedTimestamp)
}

var (
	globalRAM                   = make(map[InteractionID]*Interaction)
	globalRAMInsertionTimestamp = make(map[InteractionID]int64)
	globalRAMMutex              = &sync.RWMutex{}
)

func (i *Interaction) freezeToRam() error {
	purgeOldRAM()

	globalRAMMutex.Lock()
	defer globalRAMMutex.Unlock()

	globalRAM[i.InteractionID] = i
	globalRAMInsertionTimestamp[i.InteractionID] = time.Now().UnixNano()

	return nil
}

func purgeOldRAM() {
	globalRAMMutex.Lock()
	defer globalRAMMutex.Unlock()

	// Check all the current ids and remove any that are older than 15 minutes
	for id, timestamp := range globalRAMInsertionTimestamp {
		if time.Since(time.Unix(0, timestamp)) > 15*time.Minute {
			delete(globalRAM, id)
			delete(globalRAMInsertionTimestamp, id)
		}
	}
}

func RestoreInteractionFromRAM(id InteractionID) (*Interaction, error) {
	purgeOldRAM()

	globalRAMMutex.RLock()
	defer globalRAMMutex.RUnlock()

	if _, ok := globalRAM[id]; !ok {
		return &Interaction{}, errors.New("interaction not found")
	}
	return globalRAM[id], nil
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
		jsonOutput = strings.TrimPrefix(jsonOutput, "```")
		jsonOutput = strings.TrimPrefix(jsonOutput, "json")
		jsonOutput = strings.TrimSuffix(jsonOutput, "```")

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

func (i Interaction) Respond(userStatement string) (*Interaction, error) {
	return i.internalRespond(userStatement, false, 0, false, 0)
}

// internalRespond processes a user statement and returns an updated Interaction.
func (i Interaction) internalRespond(userStatement string, doQualityControl bool, qualityRoundCounter int, isRetry bool, retryCounter int) (*Interaction, error) {
	if !isRetry {
		retryCounter = 0
	} else {
		if retryCounter > 5 {
			return nil, errors.New("too many retries")
		}
		retryCounter++
	}

	if !doQualityControl {
		qualityRoundCounter = 0
	} else {
		if qualityRoundCounter > 5 {
			return nil, errors.New("too many quality control rounds")
		}
		qualityRoundCounter++
	}

	if userStatement == "" {
		return nil, errors.New("user statement cannot be empty")
	}

	userRequest := UserRequest{
		User:        userStatement,
		PageContent: i.pageContent,
		Memory:      i.memory,
	}
	userRequestJSON, err := json.Marshal(userRequest)
	if err != nil {
		return nil, err
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
			Role:    openai.ChatMessageRoleSystem,
			Content: qualityPrompt,
		})
	}

	i.logger.Info("Sending OpenAI Request")
	response, err := i.client.CreateChatCompletion(context.Background(), openai.ChatCompletionRequest{
		Model:       openai.GPT4TurboPreview,
		Messages:    messages,
		Temperature: 1.0,
		Stop:        []string{"[[FIN]]"},
	})
	i.logger.Info("Got OpenAI Response")
	if err != nil {
		i.logger.Error("Error from OpenAI: %v", err)
		return nil, err
	}
	if len(response.Choices) == 0 {
		return nil, errors.New("no completion choices")
	}
	choice := response.Choices[0]
	if choice.FinishReason != openai.FinishReasonStop {
		return nil, errors.New("finish reason not stop")
	}
	if choice.Message.Role != openai.ChatMessageRoleAssistant {
		return nil, errors.New("response not from assistant")
	}
	latestResponseMessage := choice.Message.Content
	i.logger.Debug("Latest response message: %v", latestResponseMessage)

	if containsResponseMarker(latestResponseMessage) {
		i.logger.Info("Got OpenAI Response")
		response, err := responseFromAssistantResponse(latestResponseMessage)
		if err != nil {
			return nil, err
		}
		nextInteraction, err := i.clone(userStatement, response.Memory)
		if err != nil {
			return nil, err
		}
		nextInteraction.assistantResponses = append(i.assistantResponses, latestResponseMessage)
		nextInteraction.LastResponse = &response

		// // If there are open questions, we need to ask them
		// if len(response.Memory.OpenQuestions) > 0 {
		// 	i.logger.Info("Open questions: %v", response.Memory.OpenQuestions)
		// 	return nextInteraction, nil
		// }

		//otherwise its time to start quality control
		i.logger.Info("Starting quality control round %v", qualityRoundCounter+1)
		return nextInteraction.internalRespond(userStatement, true, qualityRoundCounter, false, 0)
	} else if containsAllGoodMarker(latestResponseMessage) {
		//got all good, no more quality control is needed so take the last assistant response before this as the final response
		i.logger.Info("Got all good marker")
		response, err := responseFromAssistantResponse(i.assistantResponses[len(i.assistantResponses)-1])
		if err != nil {
			return nil, err
		}
		nextInteraction, err := i.clone(userStatement, response.Memory)
		if err != nil {
			return nil, err
		}
		nextInteraction.LastResponse = &response
		nextInteraction.LastResponseQualityRounds = qualityRoundCounter + 1
		return nextInteraction, nil
	} else {
		i.logger.Error("Invalid response from assistant, no response marker. Retry: %v", retryCounter+1)
		return i.internalRespond(i.userCommand, false, 0, true, retryCounter)
	}
}

// OpenAIEditor defines the methods for interacting with OpenAI.
type OpenAIEditor interface {
	PerformEdit(pageIdentifier common.PageIdentifier, command string) (*Interaction, error)
}

// OpenAIEditorImpl is the implementation of OpenAIEditor.
type OpenAIEditorImpl struct {
	client     *openai.Client
	logger     *lumber.ConsoleLogger
	pageReader common.IReadPages
}

func NewOpenAIEditor(client *openai.Client, pageReader common.IReadPages, logger *lumber.ConsoleLogger) OpenAIEditor {
	return OpenAIEditorImpl{
		client:     client,
		logger:     logger,
		pageReader: pageReader,
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

func (e OpenAIEditorImpl) PerformEdit(pageIdentifier common.PageIdentifier, command string) (*Interaction, error) {
	pageAccessedTimestamp := time.Now()
	identifier, markdown, err := e.pageReader.ReadMarkdown(pageIdentifier)
	if err != nil {
		return nil, err
	}
	_, frontMatter, err := e.pageReader.ReadFrontMatter(identifier)
	if err != nil {
		return nil, err
	}
	memory, err := memoryFromFrontMatter(frontMatter)
	if err != nil {
		return nil, err
	}
	interaction, err := newInteraction(e.client, e.logger, command, memory, markdown, identifier, pageAccessedTimestamp)
	if err != nil {
		return nil, err
	}
	return interaction.Respond(command)
}
