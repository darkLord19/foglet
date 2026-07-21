package tracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/task"
)

const linearAPI = "https://api.linear.app/graphql"

// Linear mirrors issues from Linear.
//
// Linear is the easier of the two integrations because its workflow states
// carry a machine-readable `type` (backlog / unstarted / started / completed /
// canceled) alongside the human name. Fog matches on name first, so a team's
// own "In Review" state is honoured, and falls back to type when the name is
// unfamiliar.
type Linear struct {
	apiKey   string
	teamKey  string
	http     *http.Client
	statuses StatusMap
}

// NewLinear builds a Linear provider. teamKey is optional; empty means every
// team the key can see.
func NewLinear(apiKey, teamKey string, statuses StatusMap) (*Linear, error) {
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return nil, ErrNotConfigured
	}
	return &Linear{
		apiKey:   apiKey,
		teamKey:  strings.TrimSpace(teamKey),
		http:     &http.Client{Timeout: 20 * time.Second},
		statuses: statuses,
	}, nil
}

func (l *Linear) Name() task.Provider { return task.ProviderLinear }

type linearIssueNode struct {
	ID          string `json:"id"`
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	URL         string `json:"url"`
	UpdatedAt   string `json:"updatedAt"`
	State       struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"state"`
}

const linearListQuery = `
query Issues($filter: IssueFilter) {
  issues(filter: $filter, first: 100, orderBy: updatedAt) {
    nodes {
      id identifier title description url updatedAt
      state { name type }
    }
  }
}`

// List fetches issues assigned to the authenticated user, so a personal board
// doesn't fill with the whole team's backlog.
func (l *Linear) List(ctx context.Context) ([]Issue, error) {
	filter := map[string]any{
		"assignee": map[string]any{"isMe": map[string]any{"eq": true}},
	}
	if l.teamKey != "" {
		filter["team"] = map[string]any{"key": map[string]any{"eq": l.teamKey}}
	}

	var out struct {
		Issues struct {
			Nodes []linearIssueNode `json:"nodes"`
		} `json:"issues"`
	}
	if err := l.query(ctx, linearListQuery, map[string]any{"filter": filter}, &out); err != nil {
		return nil, err
	}

	issues := make([]Issue, 0, len(out.Issues.Nodes))
	for _, n := range out.Issues.Nodes {
		updated, _ := time.Parse(time.RFC3339, n.UpdatedAt)
		issues = append(issues, Issue{
			ID:        n.ID,
			Key:       n.Identifier,
			Title:     n.Title,
			Body:      n.Description,
			Status:    n.State.Name,
			URL:       n.URL,
			UpdatedAt: updated,
		})
	}
	return issues, nil
}

const linearStatesQuery = `
query States($teamKey: String) {
  workflowStates(filter: { team: { key: { eq: $teamKey } } }, first: 50) {
    nodes { id name type }
  }
}`

const linearUpdateMutation = `
mutation Update($id: String!, $stateId: String!) {
  issueUpdate(id: $id, input: { stateId: $stateId }) {
    success
  }
}`

// SetStatus moves an issue to the workflow state matching a Fog status.
func (l *Linear) SetStatus(ctx context.Context, issueID string, status task.Status) error {
	wanted, ok := l.statuses.ToRemote(status)
	if !ok {
		return fmt.Errorf("no Linear state mapped for %s", status)
	}

	var states struct {
		WorkflowStates struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Type string `json:"type"`
			} `json:"nodes"`
		} `json:"workflowStates"`
	}
	if err := l.query(ctx, linearStatesQuery, map[string]any{"teamKey": l.teamKey}, &states); err != nil {
		return err
	}

	stateID := ""
	for _, s := range states.WorkflowStates.Nodes {
		if normalise(s.Name) == normalise(wanted) {
			stateID = s.ID
			break
		}
	}
	// Fall back to state type when the team renamed its columns.
	if stateID == "" {
		if wantType := linearTypeFor(status); wantType != "" {
			for _, s := range states.WorkflowStates.Nodes {
				if s.Type == wantType {
					stateID = s.ID
					break
				}
			}
		}
	}
	if stateID == "" {
		return fmt.Errorf("Linear has no workflow state called %q", wanted)
	}

	var res struct {
		IssueUpdate struct {
			Success bool `json:"success"`
		} `json:"issueUpdate"`
	}
	if err := l.query(ctx, linearUpdateMutation,
		map[string]any{"id": issueID, "stateId": stateID}, &res); err != nil {
		return err
	}
	if !res.IssueUpdate.Success {
		return fmt.Errorf("Linear rejected the status update for %s", issueID)
	}
	return nil
}

// linearTypeFor is the coarse fallback when a state name doesn't match.
// In Review has no Linear equivalent — it is a started state there — so it
// deliberately returns nothing rather than mapping to something wrong.
func linearTypeFor(status task.Status) string {
	switch status {
	case task.StatusTodo:
		return "unstarted"
	case task.StatusInProgress:
		return "started"
	case task.StatusDone:
		return "completed"
	default:
		return ""
	}
}

func (l *Linear) query(ctx context.Context, query string, vars map[string]any, out any) error {
	body, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, linearAPI, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", l.apiKey)

	resp, err := l.http.Do(req)
	if err != nil {
		return fmt.Errorf("linear request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("linear rejected the API key")
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("linear returned HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return fmt.Errorf("decode linear response: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("linear: %s", envelope.Errors[0].Message)
	}
	return json.Unmarshal(envelope.Data, out)
}
