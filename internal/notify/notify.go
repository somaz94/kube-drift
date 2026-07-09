// Package notify delivers drift notifications to external webhooks. It is
// decoupled from the DriftCheck API types: the controller maps a DriftCheck's
// status into an Event and hands it to a Notifier.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Type selects the payload format posted to a webhook.
type Type string

const (
	// Slack posts a {"text": ...} message to a Slack incoming webhook.
	Slack Type = "Slack"
	// Generic posts a structured JSON body describing the drift.
	Generic Type = "Generic"
)

// Webhook is a single resolved notification endpoint. The URL is already
// dereferenced from any Secret by the caller.
type Webhook struct {
	Type Type
	URL  string
}

// Resource identifies one drifted resource in a notification payload.
type Resource struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Namespace  string `json:"namespace,omitempty"`
	Status     string `json:"status"`
}

// Summary tallies the comparison outcome carried in a notification.
type Summary struct {
	Changed   int `json:"changed"`
	New       int `json:"new"`
	Deleted   int `json:"deleted"`
	Unchanged int `json:"unchanged"`
}

// Event is the drift observation delivered to a webhook.
type Event struct {
	// Name and Namespace identify the DriftCheck.
	Name      string
	Namespace string
	// Summary tallies the comparison outcome.
	Summary Summary
	// Drifted lists the resources that differ from desired (empty when Resolved).
	Drifted []Resource
	// Resolved is true when the drift previously reported has cleared.
	Resolved bool
}

// Notifier delivers an Event to a Webhook. It is an interface so the controller
// can inject a fake in tests.
type Notifier interface {
	Send(ctx context.Context, wh Webhook, ev Event) error
}

// Sender is the default HTTP-backed Notifier.
type Sender struct {
	// Client sends the webhook request. When nil a client with a 10s timeout
	// is used.
	Client *http.Client
}

// NewSender returns a Sender with a bounded-timeout HTTP client.
func NewSender() *Sender {
	return &Sender{Client: &http.Client{Timeout: 10 * time.Second}}
}

// Send posts the event to the webhook using the type-appropriate payload. It
// returns an error on transport failure or a non-2xx response so the caller can
// retry on the next reconcile.
func (s *Sender) Send(ctx context.Context, wh Webhook, ev Event) error {
	body, err := payload(wh.Type, ev)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, wh.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("post webhook: %w", err)
	}
	defer resp.Body.Close()
	// Drain the body so the connection can be reused.
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

// payload renders the JSON body for the given webhook type.
func payload(t Type, ev Event) ([]byte, error) {
	switch t {
	case Slack:
		return json.Marshal(map[string]string{"text": slackText(ev)})
	case Generic, "":
		return json.Marshal(genericPayload(ev))
	default:
		return nil, fmt.Errorf("unknown webhook type %q", t)
	}
}

// genericPayload is the structured JSON body for a Generic webhook.
type genericBody struct {
	DriftCheck string     `json:"driftCheck"`
	Namespace  string     `json:"namespace,omitempty"`
	Resolved   bool       `json:"resolved"`
	Summary    Summary    `json:"summary"`
	Drifted    []Resource `json:"drifted,omitempty"`
}

func genericPayload(ev Event) genericBody {
	return genericBody{
		DriftCheck: ev.Name,
		Namespace:  ev.Namespace,
		Resolved:   ev.Resolved,
		Summary:    ev.Summary,
		Drifted:    ev.Drifted,
	}
}

// slackText renders a human-readable Slack message for the event.
func slackText(ev Event) string {
	id := ev.Name
	if ev.Namespace != "" {
		id = ev.Namespace + "/" + ev.Name
	}
	if ev.Resolved {
		return fmt.Sprintf(":white_check_mark: DriftCheck *%s*: drift resolved — all resources match desired state.", id)
	}

	var b strings.Builder
	fmt.Fprintf(&b, ":rotating_light: DriftCheck *%s*: drift detected — %d changed, %d new, %d deleted.",
		id, ev.Summary.Changed, ev.Summary.New, ev.Summary.Deleted)
	for _, r := range ev.Drifted {
		name := r.Name
		if r.Namespace != "" {
			name = r.Namespace + "/" + r.Name
		}
		fmt.Fprintf(&b, "\n• [%s] %s %s", r.Status, r.Kind, name)
	}
	return b.String()
}
