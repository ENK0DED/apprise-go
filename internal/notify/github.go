package notify

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

type GitHubTarget struct {
	token, owner, repo, kind, category string
	number                             int64
	labels, assignees                  []string
}

var githubTokenPattern = regexp.MustCompile(`(?i)^[a-z0-9_-]+$`)
var githubOwnerPattern = regexp.MustCompile(`(?i)^[a-z0-9-]+$`)
var githubRepoPattern = regexp.MustCompile(`(?i)^[a-z0-9_.-]+$`)
var githubNumberPattern = regexp.MustCompile(`^\s*#?([1-9][0-9]{0,9})\s*$`)

func NewGitHubTarget(p *ParsedURL) (*GitHubTarget, error) {
	t := &GitHubTarget{token: p.User, owner: p.Host, kind: "issues", category: strings.TrimSpace(p.Query["category"]), labels: sortedUniqueTargets(parseDelimitedList(p.Query["labels"])), assignees: sortedUniqueTargets(parseDelimitedList(p.Query["assignees"]))}
	if p.Query["token"] != "" {
		t.token = p.Query["token"]
	}
	if t.category == "" {
		t.category = "General"
	}
	entries := splitPath(p.Path)
	if len(entries) > 0 {
		t.repo = entries[0]
		entries = entries[1:]
	}
	if !githubTokenPattern.MatchString(t.token) || !githubOwnerPattern.MatchString(t.owner) || !githubRepoPattern.MatchString(t.repo) {
		return nil, fmt.Errorf("invalid GitHub credentials or repository")
	}
	if len(entries) > 0 {
		t.kind = map[string]string{"issue": "issues", "issues": "issues", "pull": "pull", "pulls": "pull", "discussion": "discussions", "discussions": "discussions"}[strings.ToLower(entries[0])]
		entries = entries[1:]
	}
	if t.kind == "" {
		return nil, fmt.Errorf("invalid GitHub target type")
	}
	if len(entries) > 0 {
		if strings.EqualFold(entries[0], "new") {
			entries = entries[1:]
		} else {
			match := githubNumberPattern.FindStringSubmatch(entries[0])
			if match == nil {
				return nil, fmt.Errorf("invalid GitHub target number")
			}
			t.number, _ = strconv.ParseInt(match[1], 10, 64)
			entries = entries[1:]
		}
	}
	for _, entry := range entries {
		if githubNumberPattern.MatchString(entry) {
			return nil, fmt.Errorf("multiple GitHub target numbers")
		}
	}
	if t.kind == "pull" && t.number == 0 {
		return nil, fmt.Errorf("GitHub pull request requires a number")
	}
	return t, nil
}

func (t *GitHubTarget) request(endpoint string, payload map[string]any) (RequestSpec, error) {
	data, err := json.Marshal(payload)
	return RequestSpec{Method: http.MethodPost, URL: endpoint, Headers: map[string]string{"User-Agent": "Apprise", "Accept": "application/vnd.github+json", "Authorization": "Bearer " + t.token, "Content-Type": "application/json", "X-GitHub-Api-Version": "2022-11-28"}, Body: string(data)}, err
}

func (t *GitHubTarget) BuildRequest(body, title string, _ NotifyType) (RequestSpec, error) {
	if title == "" {
		title = appriseAppDesc
	}
	if t.kind == "discussions" {
		variables := map[string]any{"owner": t.owner, "repo": t.repo}
		query := githubQueryCategories
		if t.number > 0 {
			query = githubQueryDiscussion
			variables["number"] = t.number
		}
		return t.request("https://api.github.com/graphql", map[string]any{"query": query, "variables": variables})
	}
	endpoint := "https://api.github.com/repos/" + t.owner + "/" + t.repo + "/issues"
	payload := map[string]any{"body": body}
	if t.number > 0 {
		endpoint += "/" + strconv.FormatInt(t.number, 10) + "/comments"
	} else {
		payload["title"] = title
		if len(t.labels) > 0 {
			payload["labels"] = t.labels
		}
		if len(t.assignees) > 0 {
			payload["assignees"] = t.assignees
		}
	}
	return t.request(endpoint, payload)
}

func (t *GitHubTarget) Send(body, title string, kind NotifyType) error {
	spec, err := t.BuildRequest(body, title, kind)
	if err != nil {
		return err
	}
	content, err := sendGitHubRequest(spec)
	if err != nil || t.kind != "discussions" {
		return err
	}
	data, _ := content["data"].(map[string]any)
	repository, _ := data["repository"].(map[string]any)
	variables := map[string]any{"body": body}
	query := githubMutationComment
	if t.number > 0 {
		discussion, _ := repository["discussion"].(map[string]any)
		id, _ := discussion["id"].(string)
		if id == "" {
			return fmt.Errorf("GitHub discussion not found")
		}
		variables["id"] = id
	} else {
		id, _ := repository["id"].(string)
		categories, _ := repository["discussionCategories"].(map[string]any)
		nodes, _ := categories["nodes"].([]any)
		categoryID := ""
		for _, raw := range nodes {
			category, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			name, _ := category["name"].(string)
			slug, _ := category["slug"].(string)
			if strings.EqualFold(name, t.category) || strings.EqualFold(slug, t.category) {
				categoryID, _ = category["id"].(string)
			}
		}
		if id == "" || categoryID == "" {
			return fmt.Errorf("GitHub discussion category not found")
		}
		if title == "" {
			title = appriseAppDesc
		}
		query = githubMutationDiscussion
		variables["repo"] = id
		variables["category"] = categoryID
		variables["title"] = title
	}
	spec, err = t.request("https://api.github.com/graphql", map[string]any{"query": query, "variables": variables})
	if err != nil {
		return err
	}
	_, err = sendGitHubRequest(spec)
	return err
}

func sendGitHubRequest(spec RequestSpec) (map[string]any, error) {
	req, err := spec.HTTPRequest()
	if err != nil {
		return nil, err
	}
	response, err := httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var content map[string]any
	_ = json.NewDecoder(response.Body).Decode(&content)
	if response.StatusCode != 200 && response.StatusCode != 201 {
		return content, &HTTPStatusError{StatusCode: response.StatusCode}
	}
	if jsonValueTruthy(content["errors"]) {
		return content, fmt.Errorf("GitHub reported API errors")
	}
	return content, nil
}

func jsonValueTruthy(value any) bool {
	switch v := value.(type) {
	case nil:
		return false
	case bool:
		return v
	case float64:
		return v != 0
	case string:
		return v != ""
	case []any:
		return len(v) > 0
	case map[string]any:
		return len(v) > 0
	default:
		return true
	}
}
