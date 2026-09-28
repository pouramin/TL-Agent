package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const liveEventContractVersion = 1

type liveEventView struct {
	Version       int    `json:"version"`
	Type          string `json:"type"`
	Action        string `json:"action,omitempty"`
	SessionID     string `json:"sessionID,omitempty"`
	MessageID     string `json:"messageID,omitempty"`
	AttentionKind string `json:"attentionKind,omitempty"`
	Path          string `json:"path,omitempty"`
}

type liveEventContract struct {
	state *appState
	bus   *liveEventBus
}

func newLiveEventContract(state *appState) *liveEventContract {
	return &liveEventContract{state: state, bus: newLiveEventBus()}
}

func (c *liveEventContract) setBus(bus *liveEventBus) {
	if bus != nil {
		c.bus = bus
	}
}

func writeLiveEvent(w io.Writer, event liveEventView) error {
	event.Version = liveEventContractVersion
	data, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	return err
}

func registerLiveEventRoutes(mux *http.ServeMux, contract *liveEventContract) {
	mux.HandleFunc("GET /local/events", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Content-Type-Options", "nosniff")

		flusher, _ := w.(http.Flusher)
		var events <-chan liveEventView
		var subscriptionID uint64
		if contract != nil && contract.bus != nil {
			subscriptionID, events = contract.bus.subscribe()
			defer contract.bus.unsubscribe(subscriptionID)
		}

		if err := writeLiveEvent(w, liveEventView{Type: "stream.ready"}); err != nil {
			return
		}
		if flusher != nil {
			flusher.Flush()
		}

		ticker := time.NewTicker(20 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				if err := writeLiveEvent(w, event); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			case <-ticker.C:
				if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
					return
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		}
	})
}
