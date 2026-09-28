package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
)

type questionOptionView struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type questionPromptView struct {
	Header   string               `json:"header,omitempty"`
	Question string               `json:"question"`
	Options  []questionOptionView `json:"options"`
	Multiple bool                 `json:"multiple,omitempty"`
	Custom   bool                 `json:"custom"`
	Default  string               `json:"default,omitempty"`
}

type questionRequestView struct {
	ID        string               `json:"id"`
	SessionID string               `json:"sessionID"`
	Questions []questionPromptView `json:"questions"`
}

type questionReplyInput struct {
	SessionID string     `json:"sessionID,omitempty"`
	Answers   [][]string `json:"answers"`
}

type questionRejectInput struct {
	SessionID string `json:"sessionID,omitempty"`
}

type questionResolution struct {
	answers  [][]string
	rejected bool
}

type nativeQuestionWaiter struct {
	request  questionRequestView
	response chan questionResolution
}

type questionContract struct {
	state  *appState
	events *liveEventBus
	mu     sync.Mutex
	pending map[string]*nativeQuestionWaiter
}

func newQuestionContract(state *appState, events *liveEventBus) *questionContract {
	return &questionContract{
		state: state,
		events: events,
		pending: map[string]*nativeQuestionWaiter{},
	}
}

func (c *questionContract) publish(sessionID, action string) {
	if c != nil && c.events != nil {
		c.events.publish(liveEventView{
			Type: "attention.changed", Action: action,
			SessionID: strings.TrimSpace(sessionID), AttentionKind: "question",
		})
	}
}

func normalizeQuestionPrompts(prompts []questionPromptView) ([]questionPromptView, error) {
	if len(prompts) == 0 || len(prompts) > 20 {
		return nil, errors.New("at least one question is required")
	}
	result := make([]questionPromptView, 0, len(prompts))
	for _, prompt := range prompts {
		prompt.Header = strings.TrimSpace(prompt.Header)
		prompt.Question = strings.TrimSpace(prompt.Question)
		prompt.Default = strings.TrimSpace(prompt.Default)
		if prompt.Question == "" {
			return nil, errors.New("question text is required")
		}
		if len(prompt.Question) > 8000 || len(prompt.Header) > 500 || len(prompt.Default) > 8000 {
			return nil, errors.New("question content is too long")
		}
		if len(prompt.Options) > 100 {
			return nil, errors.New("too many question options")
		}
		options := make([]questionOptionView, 0, len(prompt.Options))
		for _, option := range prompt.Options {
			option.Label = strings.TrimSpace(option.Label)
			option.Description = strings.TrimSpace(option.Description)
			if option.Label == "" {
				continue
			}
			if len(option.Label) > 1000 || len(option.Description) > 4000 {
				return nil, errors.New("question option is too long")
			}
			options = append(options, option)
		}
		prompt.Options = options
		if len(prompt.Options) == 0 && !prompt.Custom {
			prompt.Custom = true
		}
		result = append(result, prompt)
	}
	return result, nil
}

func normalizeQuestionAnswers(answers [][]string) ([][]string, error) {
	if len(answers) == 0 || len(answers) > 20 {
		return nil, errors.New("question answers are required")
	}
	result := make([][]string, len(answers))
	for index, answer := range answers {
		if len(answer) == 0 || len(answer) > 50 {
			return nil, errors.New("every question requires at least one answer")
		}
		for _, value := range answer {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			if len(value) > 8000 {
				return nil, errors.New("question answer is too long")
			}
			result[index] = append(result[index], value)
		}
		if len(result[index]) == 0 {
			return nil, errors.New("every question requires at least one answer")
		}
	}
	return result, nil
}

func (c *questionContract) Ask(ctx context.Context, sessionID string, prompts []questionPromptView) ([][]string, bool, error) {
	if c == nil {
		return nil, false, errors.New("native question manager is unavailable")
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, false, errors.New("session id is required")
	}
	normalized, err := normalizeQuestionPrompts(prompts)
	if err != nil {
		return nil, false, err
	}
	token, err := randomSecret(12)
	if err != nil {
		return nil, false, err
	}
	requestID := "tlsq_" + token
	waiter := &nativeQuestionWaiter{
		request: questionRequestView{ID: requestID, SessionID: sessionID, Questions: normalized},
		response: make(chan questionResolution, 1),
	}

	c.mu.Lock()
	c.pending[requestID] = waiter
	c.mu.Unlock()
	c.publish(sessionID, "requested")
	defer func() {
		c.mu.Lock()
		delete(c.pending, requestID)
		c.mu.Unlock()
		c.publish(sessionID, "resolved")
	}()

	select {
	case resolution := <-waiter.response:
		return resolution.answers, resolution.rejected, nil
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}

func (c *questionContract) list(sessionID string) []questionRequestView {
	if c == nil {
		return []questionRequestView{}
	}
	sessionID = strings.TrimSpace(sessionID)
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make([]questionRequestView, 0, len(c.pending))
	for _, waiter := range c.pending {
		if sessionID != "" && waiter.request.SessionID != sessionID {
			continue
		}
		item := waiter.request
		item.Questions = append([]questionPromptView(nil), item.Questions...)
		result = append(result, item)
	}
	return result
}

func (c *questionContract) resolve(requestID, sessionID string, resolution questionResolution) error {
	requestID = strings.TrimSpace(requestID)
	sessionID = strings.TrimSpace(sessionID)
	if requestID == "" || sessionID == "" {
		return errors.New("session id and question request id are required")
	}
	c.mu.Lock()
	waiter, ok := c.pending[requestID]
	c.mu.Unlock()
	if !ok {
		return errors.New("question request is not active")
	}
	if waiter.request.SessionID != sessionID {
		return errors.New("question request is not active for this session")
	}
	select {
	case waiter.response <- resolution:
		return nil
	default:
		return errors.New("question request is already resolved")
	}
}

func writeQuestionError(w http.ResponseWriter, err error) {
	if strings.Contains(err.Error(), "not active") || strings.Contains(err.Error(), "already resolved") {
		writeJSON(w, http.StatusNotFound, jsonError{Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusBadRequest, jsonError{Error: err.Error()})
}

func registerQuestionRoutes(mux *http.ServeMux, contract *questionContract) {
	mux.HandleFunc("GET /local/questions", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, contract.list(r.URL.Query().Get("sessionID")))
	})

	mux.HandleFunc("POST /local/questions/{requestID}/reply", func(w http.ResponseWriter, r *http.Request) {
		var input questionReplyInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "invalid question reply body"})
			return
		}
		answers, err := normalizeQuestionAnswers(input.Answers)
		if err != nil { writeQuestionError(w, err); return }
		requestID := strings.TrimSpace(r.PathValue("requestID"))
		candidates := contract.list(input.SessionID)
		var expected int
		for _, item := range candidates {
			if item.ID == requestID {
				expected = len(item.Questions)
				break
			}
		}
		if expected == 0 {
			writeQuestionError(w, errors.New("question request is not active for this session"))
			return
		}
		if len(answers) != expected {
			writeQuestionError(w, errors.New("answer count does not match question count"))
			return
		}
		if err := contract.resolve(requestID, input.SessionID, questionResolution{answers: answers}); err != nil {
			writeQuestionError(w, err); return
		}
		writeJSON(w, http.StatusOK, map[string]any{"resolved": true})
	})

	mux.HandleFunc("POST /local/questions/{requestID}/reject", func(w http.ResponseWriter, r *http.Request) {
		var input questionRejectInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, jsonError{Error: "invalid question reject body"})
			return
		}
		if err := contract.resolve(strings.TrimSpace(r.PathValue("requestID")), input.SessionID, questionResolution{rejected: true}); err != nil {
			writeQuestionError(w, err); return
		}
		writeJSON(w, http.StatusOK, map[string]any{"resolved": true})
	})
}
