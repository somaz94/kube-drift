package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func drift() Event {
	return Event{
		Name:      "dc",
		Namespace: "default",
		Summary:   Summary{Changed: 1, New: 2, Deleted: 0, Unchanged: 5},
		Drifted: []Resource{
			{APIVersion: "v1", Kind: "ConfigMap", Name: "app-config", Namespace: "default", Status: "changed"},
			{APIVersion: "v1", Kind: "Service", Name: "web", Namespace: "prod", Status: "new"},
		},
	}
}

func TestSender_Slack(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSender()
	if err := s.Send(context.Background(), Webhook{Type: Slack, URL: srv.URL}, drift()); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	text, ok := got["text"].(string)
	if !ok {
		t.Fatalf("payload missing text field: %+v", got)
	}
	if !strings.Contains(text, "drift detected") {
		t.Errorf("slack text missing headline: %q", text)
	}
	// Each drifted resource is listed.
	if !strings.Contains(text, "default/app-config") || !strings.Contains(text, "prod/web") {
		t.Errorf("slack text missing a drifted resource: %q", text)
	}
}

func TestSender_Generic(t *testing.T) {
	var got genericBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s := NewSender()
	if err := s.Send(context.Background(), Webhook{Type: Generic, URL: srv.URL}, drift()); err != nil {
		t.Fatalf("Send() error = %v", err)
	}

	if got.DriftCheck != "dc" || got.Namespace != "default" {
		t.Errorf("identity = %s/%s, want default/dc", got.Namespace, got.DriftCheck)
	}
	if got.Resolved {
		t.Error("Resolved = true, want false")
	}
	if got.Summary.Changed != 1 || got.Summary.New != 2 {
		t.Errorf("summary = %+v", got.Summary)
	}
	if len(got.Drifted) != 2 {
		t.Errorf("drifted = %d, want 2", len(got.Drifted))
	}
}

func TestSender_EmptyTypeDefaultsToGeneric(t *testing.T) {
	var got genericBody
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := NewSender().Send(context.Background(), Webhook{URL: srv.URL}, drift()); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if got.DriftCheck != "dc" {
		t.Errorf("empty type did not render generic body: %+v", got)
	}
}

func TestSender_Non2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := NewSender().Send(context.Background(), Webhook{Type: Generic, URL: srv.URL}, drift())
	if err == nil {
		t.Fatal("Send() error = nil, want non-2xx error")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error = %v, want status 500", err)
	}
}

func TestSender_UnknownType(t *testing.T) {
	err := NewSender().Send(context.Background(), Webhook{Type: "Teams", URL: "http://example.invalid"}, drift())
	if err == nil {
		t.Fatal("Send() error = nil, want unknown-type error")
	}
	if !strings.Contains(err.Error(), "unknown webhook type") {
		t.Errorf("error = %v", err)
	}
}

func TestSlackText_Resolved(t *testing.T) {
	ev := Event{Name: "dc", Namespace: "default", Resolved: true}
	text := slackText(ev)
	if !strings.Contains(text, "resolved") {
		t.Errorf("resolved text = %q", text)
	}
	if strings.Contains(text, "•") {
		t.Errorf("resolved text should not list resources: %q", text)
	}
}

func TestSender_TransportError(t *testing.T) {
	// A closed server address yields a connection error, not a status error.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	err := NewSender().Send(context.Background(), Webhook{Type: Generic, URL: url}, drift())
	if err == nil {
		t.Fatal("Send() error = nil, want transport error")
	}
}
