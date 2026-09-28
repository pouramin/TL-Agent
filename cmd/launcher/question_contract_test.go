package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestNativeQuestionAskReplyResumesCaller(t *testing.T) {
	bus := newLiveEventBus()
	contract := newQuestionContract(&appState{project: t.TempDir()}, bus)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	type result struct {
		answers  [][]string
		rejected bool
		err      error
	}
	done := make(chan result, 1)
	go func() {
		answers, rejected, err := contract.Ask(ctx, "tls_session", []questionPromptView{{
			Header: "Choice", Question: "Which option?",
			Options: []questionOptionView{{Label: "A"}, {Label: "B"}},
			Custom: true,
		}})
		done <- result{answers: answers, rejected: rejected, err: err}
	}()

	var pending []questionRequestView
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending = contract.list("tls_session")
		if len(pending) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(pending) != 1 {
		t.Fatal("native question did not become pending")
	}
	if pending[0].SessionID != "tls_session" || len(pending[0].Questions) != 1 {
		t.Fatalf("unexpected pending question %#v", pending[0])
	}

	mux := http.NewServeMux()
	registerQuestionRoutes(mux, contract)
	server := httptest.NewServer(mux)
	defer server.Close()

	body := `{"sessionID":"tls_session","answers":[["B","custom detail"]]}`
	res, err := http.Post(server.URL+"/local/questions/"+pending[0].ID+"/reply", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("reply status=%d", res.StatusCode)
	}

	select {
	case got := <-done:
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.rejected || len(got.answers) != 1 || len(got.answers[0]) != 2 || got.answers[0][0] != "B" {
			t.Fatalf("unexpected question result %#v", got)
		}
	case <-time.After(time.Second):
		t.Fatal("native Agent question waiter did not resume")
	}
	if len(contract.list("tls_session")) != 0 {
		t.Fatal("resolved question remained pending")
	}
}

func TestNativeQuestionRejectAndCancellation(t *testing.T) {
	contract := newQuestionContract(&appState{project: t.TempDir()}, newLiveEventBus())
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan bool, 1)
	go func() {
		_, rejected, err := contract.Ask(ctx, "tls_reject", []questionPromptView{{
			Question: "Continue?", Options: []questionOptionView{{Label: "Yes"}, {Label: "No"}},
		}})
		done <- err == nil && rejected
	}()

	var pending []questionRequestView
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		pending = contract.list("tls_reject")
		if len(pending) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(pending) != 1 {
		t.Fatal("native reject question did not become pending")
	}
	if err := contract.resolve(pending[0].ID, "tls_reject", questionResolution{rejected: true}); err != nil {
		t.Fatal(err)
	}
	if !<-done {
		t.Fatal("native question reject did not resume caller")
	}

	cancelCtx, cancelNow := context.WithCancel(context.Background())
	cancelDone := make(chan error, 1)
	go func() {
		_, _, err := contract.Ask(cancelCtx, "tls_cancel", []questionPromptView{{Question: "Wait?", Custom: true}})
		cancelDone <- err
	}()
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(contract.list("tls_cancel")) == 1 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancelNow()
	if err := <-cancelDone; err == nil {
		t.Fatal("cancelled native question must return context cancellation")
	}
}
