package tracker

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/darkLord19/foglet/internal/task"
)

// Jira mirrors issues from Jira Cloud.
//
// Jira is harder than Linear for one specific reason: its workflow states are
// configured per project and carry no reliable machine-readable type. "In
// Progress" may not exist; a team may use "Dev In Flight" and "Peer Check".
// There is therefore no sensible default — a Jira setup that diverges from
// DefaultStatusMap needs an explicit map, and Validate() will say so.
//
// Status changes also can't be written directly: Jira moves an issue by
// executing a *transition*, and the available transitions depend on the issue's
// current state. SetStatus resolves the transition by target-state name.
type Jira struct {
	baseURL  string
	email    string
	apiToken string
	jql      string
	http     *http.Client
	statuses StatusMap
}

// NewJira builds a Jira provider.
//
// jql scopes what gets mirrored. Empty defaults to issues assigned to the
// authenticated user that aren't finished — syncing an entire Jira instance
// into a personal board would be unusable.
func NewJira(baseURL, email, apiToken, jql string, statuses StatusMap) (*Jira, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	email = strings.TrimSpace(email)
	apiToken = strings.TrimSpace(apiToken)

	if baseURL == "" || email == "" || apiToken == "" {
		return nil, ErrNotConfigured
	}
	if jql = strings.TrimSpace(jql); jql == "" {
		jql = "assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC"
	}

	return &Jira{
		baseURL:  baseURL,
		email:    email,
		apiToken: apiToken,
		jql:      jql,
		http:     &http.Client{Timeout: 20 * time.Second},
		statuses: statuses,
	}, nil
}

func (j *Jira) Name() task.Provider { return task.ProviderJira }

type jiraIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Fields struct {
		Summary     string `json:"summary"`
		Description any    `json:"description"`
		Updated     string `json:"updated"`
		Status      struct {
			Name string `json:"name"`
		} `json:"status"`
	} `json:"fields"`
}

// List runs the configured JQL and maps the results.
func (j *Jira) List(ctx context.Context) ([]Issue, error) {
	q := url.Values{}
	q.Set("jql", j.jql)
	q.Set("maxResults", "100")
	q.Set("fields", "summary,description,status,updated")

	var out struct {
		Issues []jiraIssue `json:"issues"`
	}
	if err := j.do(ctx, http.MethodGet, "/rest/api/3/search?"+q.Encode(), nil, &out); err != nil {
		return nil, err
	}

	issues := make([]Issue, 0, len(out.Issues))
	for _, it := range out.Issues {
		// Jira Cloud timestamps carry an offset without a colon.
		updated, _ := time.Parse("2006-01-02T15:04:05.000-0700", it.Fields.Updated)

		issues = append(issues, Issue{
			ID:        it.ID,
			Key:       it.Key,
			Title:     it.Fields.Summary,
			Body:      flattenADF(it.Fields.Description),
			Status:    it.Fields.Status.Name,
			URL:       j.baseURL + "/browse/" + it.Key,
			UpdatedAt: updated,
		})
	}
	return issues, nil
}

// SetStatus executes the Jira transition whose target state matches the Fog
// status. Jira will not accept a status write directly.
func (j *Jira) SetStatus(ctx context.Context, issueID string, status task.Status) error {
	wanted, ok := j.statuses.ToRemote(status)
	if !ok {
		return fmt.Errorf("no Jira status mapped for %s", status)
	}

	var available struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			To   struct {
				Name string `json:"name"`
			} `json:"to"`
		} `json:"transitions"`
	}
	path := "/rest/api/3/issue/" + url.PathEscape(issueID) + "/transitions"
	if err := j.do(ctx, http.MethodGet, path, nil, &available); err != nil {
		return err
	}

	transitionID := ""
	for _, t := range available.Transitions {
		if normalise(t.To.Name) == normalise(wanted) {
			transitionID = t.ID
			break
		}
	}
	// Some workflows name the transition rather than the destination.
	if transitionID == "" {
		for _, t := range available.Transitions {
			if normalise(t.Name) == normalise(wanted) {
				transitionID = t.ID
				break
			}
		}
	}
	if transitionID == "" {
		names := make([]string, 0, len(available.Transitions))
		for _, t := range available.Transitions {
			names = append(names, t.To.Name)
		}
		return fmt.Errorf(
			"no Jira transition from here to %q (available: %s) — check the status map in Settings",
			wanted, strings.Join(names, ", "),
		)
	}

	body := map[string]any{"transition": map[string]string{"id": transitionID}}
	return j.do(ctx, http.MethodPost, path, body, nil)
}

func (j *Jira) do(ctx context.Context, method, path string, body, out any) error {
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, j.baseURL+path, reader)
	if err != nil {
		return err
	}
	auth := base64.StdEncoding.EncodeToString([]byte(j.email + ":" + j.apiToken))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := j.http.Do(req)
	if err != nil {
		return fmt.Errorf("jira request: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return fmt.Errorf("jira rejected the credentials")
	case resp.StatusCode == http.StatusForbidden:
		return fmt.Errorf("jira denied access — check the account's project permissions")
	case resp.StatusCode >= 400:
		return fmt.Errorf("jira returned HTTP %d", resp.StatusCode)
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode jira response: %w", err)
	}
	return nil
}

// flattenADF renders Jira's Atlassian Document Format down to plain text.
//
// Fog only needs the description as agent context, so structure is dropped and
// text nodes are concatenated. Older Jira instances return a plain string,
// which is handled too.
func flattenADF(raw any) string {
	switch v := raw.(type) {
	case nil:
		return ""
	case string:
		return v
	case map[string]any:
		var b strings.Builder
		walkADF(v, &b)
		return strings.TrimSpace(b.String())
	default:
		return ""
	}
}

func walkADF(node map[string]any, b *strings.Builder) {
	if text, ok := node["text"].(string); ok {
		b.WriteString(text)
	}
	if node["type"] == "paragraph" || node["type"] == "heading" {
		defer b.WriteString("\n")
	}
	if content, ok := node["content"].([]any); ok {
		for _, child := range content {
			if m, ok := child.(map[string]any); ok {
				walkADF(m, b)
			}
		}
	}
}
