package api

import (
	"strings"
	"sync"
	"time"
)

// TranscriptToken is a single text token with a timestamp.
type TranscriptToken struct {
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

// Transcript holds the accumulated text from a model's WebSocket session.
type Transcript struct {
	ModelName string
	tokens    []TranscriptToken
	text      strings.Builder
	mu        sync.RWMutex
}

// Append adds a text token to the transcript.
func (t *Transcript) Append(text string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.tokens = append(t.tokens, TranscriptToken{Text: text, At: time.Now()})
	t.text.WriteString(text)
}

// Text returns the full accumulated transcript text.
func (t *Transcript) Text() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.text.String()
}

// Tokens returns a copy of all transcript tokens.
func (t *Transcript) Tokens() []TranscriptToken {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]TranscriptToken, len(t.tokens))
	copy(out, t.tokens)
	return out
}

// TranscriptStore manages per-model transcript buffers.
type TranscriptStore struct {
	mu          sync.RWMutex
	transcripts map[string]*Transcript
}

// NewTranscriptStore creates a new TranscriptStore.
func NewTranscriptStore() *TranscriptStore {
	return &TranscriptStore{
		transcripts: make(map[string]*Transcript),
	}
}

// Get returns the transcript for a model, or nil if none exists.
func (s *TranscriptStore) Get(model string) *Transcript {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.transcripts[model]
}

// GetOrCreate returns the transcript for a model, creating one if needed.
func (s *TranscriptStore) GetOrCreate(model string) *Transcript {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.transcripts[model]
	if !ok {
		t = &Transcript{ModelName: model}
		s.transcripts[model] = t
	}
	return t
}

// Clear resets the transcript for a model (called on new session).
func (s *TranscriptStore) Clear(model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.transcripts[model] = &Transcript{ModelName: model}
}
