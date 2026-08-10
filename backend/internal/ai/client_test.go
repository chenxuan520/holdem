package ai

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"holdem/backend/internal/config"
)

func TestClientDecideRetriesUntilSuccess(t *testing.T) {
	attempt := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		w.Header().Set("Content-Type", "application/json")
		switch attempt {
		case 1:
			fmt.Fprint(w, `{"choices":[{"message":{"content":"{"}}]}`)
		case 2:
			fmt.Fprint(w, `{"choices":[{"message":{"tool_calls":[{"function":{"name":"submit_action","arguments":"{\"amount\":5"}}]}}]}`)
		default:
			fmt.Fprint(w, `{"choices":[{"message":{"tool_calls":[{"function":{"name":"submit_action","arguments":"{\"action\":\"call\",\"amount\":5,\"public_reason\":\"跟注\",\"private_reason\":\"赔率合适\"}"}}]}}]}`)
		}
	}))
	defer server.Close()

	client := NewClient()
	decision, log, err := client.Decide(context.Background(), config.Preset{
		Endpoint: server.URL,
		Token:    "test-token",
		Model:    "deepseek-v4-flash",
	}, PromptInput{})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if attempt != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempt)
	}
	if decision.Action != "call" || decision.Amount != 5 {
		t.Fatalf("unexpected decision: %+v", decision)
	}
	if len(log.Attempts) != 3 {
		t.Fatalf("expected 3 attempt logs, got %d", len(log.Attempts))
	}
}

func TestClientDecideSendsExtraHeadersAndParsesArrayContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Source"); got != "ttadk" {
			t.Fatalf("expected X-Source header ttadk, got %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"choices":[{"message":{"content":[{"type":"thinking","text":""},{"type":"text","text":"{\"action\":\"fold\",\"amount\":0,\"public_reason\":\"稳健\",\"private_reason\":\"没优势\"}"}]}}]}`)
	}))
	defer server.Close()

	client := NewClient()
	decision, _, err := client.Decide(context.Background(), config.Preset{
		Endpoint:     server.URL,
		Token:        "test-token",
		Model:        "glm-5.1",
		ExtraHeaders: map[string]string{"X-Source": "ttadk"},
	}, PromptInput{})
	if err != nil {
		t.Fatalf("Decide returned error: %v", err)
	}
	if decision.Action != "fold" || decision.Amount != 0 {
		t.Fatalf("unexpected decision: %+v", decision)
	}
}
