package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

// Moshi WebSocket protocol message types.
const (
	msgHandshake byte = 0x00
	msgAudio     byte = 0x01
	msgText      byte = 0x02
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	modelName := r.PathValue("model")
	if modelName == "" {
		http.Error(w, "model name required", http.StatusBadRequest)
		return
	}

	// Look up the running instance
	inst, err := s.instanceService.FindByModel(r.Context(), modelName)
	if err != nil {
		http.Error(w, fmt.Sprintf("model %q is not running", modelName), http.StatusNotFound)
		return
	}

	// Build upstream URL preserving query params (voice, prompt, etc.)
	upstreamURL := inst.WebSocketURL
	if r.URL.RawQuery != "" {
		upstreamURL += "?" + r.URL.RawQuery
	}

	// Upgrade client connection
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("ws upgrade failed: %v", err)
		return
	}
	defer clientConn.Close()

	// Dial upstream model server
	upstreamConn, _, err := websocket.DefaultDialer.Dial(upstreamURL, nil)
	if err != nil {
		log.Printf("ws dial upstream failed: %v", err)
		clientConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "cannot connect to model"))
		return
	}
	defer upstreamConn.Close()

	// Clear and get transcript for this session
	s.transcripts.Clear(modelName)
	transcript := s.transcripts.GetOrCreate(modelName)

	errc := make(chan error, 2)

	// client -> model (transparent passthrough)
	go func() {
		errc <- pumpMessages(clientConn, upstreamConn)
	}()

	// model -> client (protocol-aware: extract text tokens)
	go func() {
		errc <- pumpModelToClient(upstreamConn, clientConn, transcript)
	}()

	// Wait for first close/error
	<-errc
}

// pumpMessages is a transparent bidirectional message pump.
func pumpMessages(src, dst *websocket.Conn) error {
	for {
		msgType, data, err := src.ReadMessage()
		if err != nil {
			return err
		}
		if err := dst.WriteMessage(msgType, data); err != nil {
			return err
		}
	}
}

// pumpModelToClient forwards messages from the model to the browser,
// parsing the Moshi framing protocol to extract text tokens.
func pumpModelToClient(model, client *websocket.Conn, transcript *Transcript) error {
	for {
		msgType, data, err := model.ReadMessage()
		if err != nil {
			return err
		}

		// Non-binary messages: forward as-is
		if msgType != websocket.BinaryMessage || len(data) == 0 {
			if err := client.WriteMessage(msgType, data); err != nil {
				return err
			}
			continue
		}

		kind := data[0]

		switch kind {
		case msgText:
			// Extract text token for transcript
			if len(data) > 1 {
				text := string(data[1:])
				transcript.Append(text)
				log.Printf("ws text token: %q", text)
			}
			// Forward to browser (browser parses the type byte too)
			if err := client.WriteMessage(websocket.BinaryMessage, data); err != nil {
				return err
			}

		case msgHandshake, msgAudio:
			// Forward as-is
			if err := client.WriteMessage(websocket.BinaryMessage, data); err != nil {
				return err
			}

		default:
			// Unknown type: forward as-is
			if err := client.WriteMessage(websocket.BinaryMessage, data); err != nil {
				return err
			}
		}
	}
}

func (s *Server) handleTranscript(w http.ResponseWriter, r *http.Request) {
	modelName := r.PathValue("model")
	if modelName == "" {
		writeError(w, http.StatusBadRequest, "model name required")
		return
	}

	t := s.transcripts.Get(modelName)
	if t == nil {
		writeJSON(w, http.StatusOK, TranscriptResponse{
			Model:  modelName,
			Text:   "",
			Tokens: []TranscriptToken{},
		})
		return
	}

	writeJSON(w, http.StatusOK, TranscriptResponse{
		Model:  modelName,
		Text:   t.Text(),
		Tokens: t.Tokens(),
	})
}
