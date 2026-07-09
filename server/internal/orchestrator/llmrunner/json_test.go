package llmrunner

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseJSONMarkdown_Standard(t *testing.T) {
	content := `{
  "files_changed": ["a.go"],
  "summary": "hello world",
  "patch": "diff --git a/a.go b/a.go"
}`
	res, err := ParseJSONMarkdown(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expected := map[string]any{
		"files_changed": []any{"a.go"},
		"summary":       "hello world",
		"patch":         "diff --git a/a.go b/a.go",
	}

	if !reflect.DeepEqual(res, expected) {
		t.Errorf("expected %v, got %v", expected, res)
	}
}

func TestParseJSONMarkdown_UnescapedQuotes(t *testing.T) {
	content := `{
  "files_changed": ["a.go"],
  "summary": "this contains unescaped "quotes" in the middle",
  "patch": "diff --git a/a.go b/a.go\nCreatedAt  time.Time ` + "`" + `json:"created_at"` + "`" + `"
}`
	res, err := ParseJSONMarkdown(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedSummary := `this contains unescaped "quotes" in the middle`
	expectedPatch := "diff --git a/a.go b/a.go\nCreatedAt  time.Time `json:\"created_at\"`"

	if res["summary"] != expectedSummary {
		t.Errorf("expected summary %q, got %q", expectedSummary, res["summary"])
	}
	if res["patch"] != expectedPatch {
		t.Errorf("expected patch %q, got %q", expectedPatch, res["patch"])
	}
}

func TestParseJSONMarkdown_UserResponse(t *testing.T) {
	rawResponse := "```json\n{\n  \"files_changed\": [\n    \"tool_zentao/gitlab/client.go\",\n    \"tool_zentao/gitlab/client_test.go\"\n  ],\n  \"summary\": \"Implement GitLab client features including the Commit and Project structs, robust pagination & authentication in GetCommits via user's project memberships, and write comprehensive unit tests with httptest server mock.\",\n  \"patch\": \"diff --git a/tool_zentao/gitlab/client.go b/tool_zentao/gitlab/client.go\\n--- a/tool_zentao/gitlab/client.go\\n+++ b/tool_zentao/gitlab/client.go\\n@@ -1,6 +1,10 @@\\n package gitlab\\n \\n import (\\n+\\t\\\"encoding/json\\\"\\n+\\t\\\"fmt\\\"\\n+\\t\\\"net/url\\\"\\n+\\t\\\"strconv\\\"\\n \\t\\\"net/http\\\"\\n \\t\\\"time\\\"\\n )\\n@@ -19,6 +23,11 @@\\n \\tCreatedAt  time.Time `json:\\\"created_at\\\"`\\n }\\n \\n+type Project struct {\\n+\\tID   int    `json:\\\"id\\\"`\\n+\\tName string `json:\\\"name\\\"`\\n+}\\n+\\n func NewClient(baseURL, token string) *Client {\\n \\treturn \u0026Client{\\n \\t\\tbaseURL: baseURL,\\n@@ -27,6 +36,87 @@\\n \\t}\\n }\\n \\n+func (c *Client) doRequest(req *http.Request) (*http.Response, error) {\\n+\\treq.Header.Set(\\\"PRIVATE-TOKEN\\\", c.token)\\n+\\treq.Header.Set(\\\"Accept\\\", \\\"application/json\\\")\\n+\\treturn c.client.Do(req)\\n+}\\n+\\n+func (c *Client) getProjects() ([]Project, error) {\\n+\\tvar projects []Project\\n+\\tpage := 1\\n+\\tfor {\\n+\\t\\tu, err := url.Parse(fmt.Sprintf(\\\"%s/api/v4/projects\\\", c.baseURL))\\n+\\t\\tif err != nil {\\n+\\t\\t\treturn nil, err\\n+\\t\\t}\\n+\\t\\tq := u.Query()\\n+\\t\\tq.Set(\\\"membership\\\", \\\"true\\\")\\n+\\t\\tq.Set(\\\"simple\\\", \\\"true\\\")\\n+\\t\\tq.Set(\\\"per_page\\\", \\\"100\\\")\\n+\\t\\tq.Set(\\\"page\\\", strconv.Itoa(page))\\n+\\t\\tu.RawQuery = q.Encode()\\n+\\n+\\t\\treq, err := http.NewRequest(\\\"GET\\\", u.String(), nil)\\n+\\t\\tif err != nil {\\n+\\t\\t\treturn nil, err\\n+\\t\\t}\\n+\\n+\\t\\tresp, err := c.doRequest(req)\\n+\\t\\tif err != nil {\\n+\\t\\t\treturn nil, err\\n+\\t\\t}\\n+\\t\\tdefer resp.Body.Close()\\n \\n+\\t\\tif resp.StatusCode != http.StatusOK {\\n+\\t\\t\treturn nil, fmt.Errorf(\\\"unexpected status code: %d\\\", resp.StatusCode)\\n+\\t\\t}\\n+\\n+\\t\\tvar pageProjects []Project\\n+\\t\\tif err := json.NewDecoder(resp.Body).Decode(\u0026pageProjects); err != nil {\\n+\\t\\t\treturn nil, err\\n+\\t\\t}\\n+\\n+\\t\\tif len(pageProjects) == 0 {\\n+\\t\\t\\tbreak\\n+\\t\\t}\\n+\\n+\\t\\tprojects = append(projects, pageProjects...)\\n+\\t\\tpage++\\n+\\t}\\n+\\treturn projects, nil\\n+}\\n+\\n func (c *Client) GetCommits(since time.Time) ([]Commit, error) {\\n-\\treturn []Commit{}, nil\\n+\\tprojects, err := c.getProjects()\\n+\\tif err != nil {\\n+\\t\\treturn nil, err\\n+\\t}\\n+\\n+\\tvar allCommits []Commit\\n+\\tfor _, project := range projects {\\n+\\t\\tpage := 1\\n+\\t\\tfor {\\n+\\t\\t\\tu, err := url.Parse(fmt.Sprintf(\\\"%s/api/v4/projects/%d/repository/commits\\\", c.baseURL, project.ID))\\n+\\t\\t\\tif err != nil {\\n+\\t\\t\\t\\treturn nil, err\\n+\\t\\t\\t}\\n+\\t\\t\tq := u.Query()\\n+\\t\\t\tif !since.IsZero() {\\n+\\t\\t\t\tq.Set(\\\"since\\\", since.Format(time.RFC3339))\\n+\\t\\t\t}\\n+\\t\\t\tq.Set(\\\"per_page\\\", \\\"100\\\")\\n+\\t\\t\tq.Set(\\\"page\\\", strconv.Itoa(page))\\n+\\t\\t\\tu.RawQuery = q.Encode()\\n+\\n+\\t\\treq, err := http.NewRequest(\\\"GET\\\", u.String(), nil)\\n+\\t\\t\\tif err != nil {\\n+\\t\\t\\t\\treturn nil, err\\n+\\t\\t\\t}\\n+\\n+\\t\\t\tresp, err := c.doRequest(req)\\n+\\t\\t\\tif err != nil {\\n+\\t\\t\\t\\treturn nil, err\\n+\\t\\t\\t}\\n+\\t\\t\tdefer resp.Body.Close()\\n+\\n+\\t\\t\tif resp.StatusCode == http.StatusNotFound {\\n+\\t\\t\\t\\tbreak\\n+\\t\\t\t}\\n+\\t\\t\tif resp.StatusCode != http.StatusOK {\\n+\\t\\t\t\\treturn nil, fmt.Errorf(\\\"unexpected status code %d for project %d\\\", resp.StatusCode, project.ID)\\n+\\t\\t\\t}\\n+\\n+\\t\\t\tvar pageCommits []Commit\\n+\\t\\t\tif err := json.NewDecoder(resp.Body).Decode(\u0026pageCommits); err != nil {\\n+\\t\\t\\t\\treturn nil, err\\n+\\t\\t\\t}\\n+\\n+\\t\\t\tif len(pageCommits) == 0 {\\n+\\t\\t\\t\\tbreak\\n+\\t\\t\\t}\\n+\\n+\\t\\t\\tallCommits = append(allCommits, pageCommits...)\\n+\\t\\t\tpage++\\n+\\t\\t}\\n+\\t}\\n+\\n+\\treturn allCommits, nil\\n }\\ndiff --git a/tool_zentao/gitlab/client_test.go b/tool_zentao/gitlab/client_test.go\\nnew file mode 100644\\n--- /dev/null\\n+++ b/tool_zentao/gitlab/client_test.go\\n@@ -0,0 +1,91 @@\\n+package gitlab\\n+\\n+import (\\n+\\t\\\"encoding/json\\\"\\n+\\t\\\"net/http\\\"\\n+\\t\\\"net/http/httptest\\\"\\n+\\t\\\"strings\\\"\\n+\\t\\\"testing\\\"\\n+\\t\\\"time\\\"\\n+)\\n+\\n+func TestGetCommits_Success(t *testing.T) {\\n+\\tserver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\\n+\\t\\tif r.Header.Get(\\\"PRIVATE-TOKEN\\\") != \\\"test-token\\\" {\\n+\\t\\t\\tw.WriteHeader(http.StatusUnauthorized)\\n+\\t\\t\\treturn\\n+\\t\\t}\\n+\\n+\\t\\tif strings.HasSuffix(r.URL.Path, \\\"/api/v4/projects\\\") {\\n+\\t\\t\\tpage := r.URL.Query().Get(\\\"page\\\")\\n+\\t\\t\\tif page == \\\"2\\\" {\\n+\\t\\t\\t\\tw.Header().Set(\\\"Content-Type\\\", \\\"application/json\\\")\\n+\\t\\t\\t\\t_, _ = w.Write([]byte(\\\"[]\\\"))\\n+\\t\\t\\t\\treturn\\n+\\t\\t\\t}\\n+\\t\\t\\tprojects := []Project{\\n+\\t\\t\\t\\t{ID: 123, Name: \\\"Project A\\\"},\\n+\\t\\t\\t}\\n+\\t\\t\\tw.Header().Set(\\\"Content-Type\\\", \\\"application/json\\\")\\n+\\t\\t\\t_ = json.NewEncoder(w).Encode(projects)\\n+\\t\\t\\treturn\\n+\\t\\t}\\n+\\n+\\t\\tif strings.HasSuffix(r.URL.Path, \\\"/api/v4/projects/123/repository/commits\\\") {\\n+\\t\\t\\tpage := r.URL.Query().Get(\\\"page\\\")\\n+\\t\\t\\tif page == \\\"2\\\" {\\n+\\t\\t\\t\\tw.Header().Set(\\\"Content-Type\\\", \\\"application/json\\\")\\n+\\t\\t\\t\\t_, _ = w.Write([]byte(\\\"[]\\\"))\\n+\\t\\t\\t\\treturn\\n+\\t\\t\\t}\\n+\\t\\t\\tsinceStr := r.URL.Query().Get(\\\"since\\\")\\n+\\t\\t\\tif sinceStr == \\\"\\\" {\\n+\\t\\t\\t\\tt.Error(\\\"Expected since query parameter\\\")\\n+\\t\\t\\t}\\n+\\t\\t\\tcommits := []Commit{\\n+\\t\\t\\t\\t{\\n+\\t\\t\\t\\t\\tID:         \\\"abc123456\\\",\\n+\\t\\t\\t\\t\\tTitle:      \\\"feat: some test commit\\\",\\n+\\t\\t\\t\\t\\tMessage:    \\\"feat: some test commit\\\",\\n+\\t\\t\\t\\t\\tAuthorName: \\\"John Doe\\\",\\n+\\t\\t\\t\\t\\tCreatedAt:  time.Now(),\\n+\\t\\t\\t\\t\\t},\\n+\\t\\t\\t}\\n+\\t\\t\\tw.Header().Set(\\\"Content-Type\\\", \\\"application/json\\\")\\n+\\t\\t\\t_ = json.NewEncoder(w).Encode(commits)\\n+\\t\\t\\treturn\\n+\\t\\t}\\n+\\n+\\t\\tw.WriteHeader(http.StatusNotFound)\\n+\\t}))\\n+\\tdefer server.Close()\\n+\n+\\tclient := NewClient(server.URL, \\\"test-token\\\")\\n+\\tsince := time.Now().Add(-24 * time.Hour)\\n+\\tcommits, err := client.GetCommits(since)\\n+\\tif err != nil {\\n+\\t\\tt.Fatalf(\\\"Expected no error, got %v\\\", err)\\n+\\t}\\n+\\n+\\tif len(commits) != 1 {\\n+\\t\\tt.Errorf(\\\"Expected 1 commit, got %v\\\", len(commits))\\n+\\t} else {\\n+\\t\\tif commits[0].ID != \\\"abc123456\\\" {\\n+\\t\\t\\tt.Errorf(\\\"Expected commit ID abc123456, got %s\\\", commits[0].ID)\\n+\\t\\t}\\n+\\t}\\n+}\\n+\\n+func TestGetCommits_Unauthorized(t *testing.T) {\\n+\\tserver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\\n+\\t\\tw.WriteHeader(http.StatusUnauthorized)\\n+\\t}))\\n+\\tdefer server.Close()\\n+\\n+\\tclient := NewClient(server.URL, \\\"invalid-token\\\")\\n+\\t_ , err := client.GetCommits(time.Now())\\n+\\tif err == nil {\\n+\\t\\tt.Error(\\\"Expected error with unauthorized token, got nil\\\")\\n+\\t}\\n+}\\n\"\n}"

	res, err := ParseJSONMarkdown(rawResponse)
	if err != nil {
		t.Fatalf("expected no error parsing user response, got %v", err)
	}

	filesChanged, ok := res["files_changed"].([]any)
	if !ok || len(filesChanged) != 2 {
		t.Fatalf("expected files_changed of length 2, got: %v", res["files_changed"])
	}
	if filesChanged[0] != "tool_zentao/gitlab/client.go" {
		t.Errorf("expected first file changed tool_zentao/gitlab/client.go, got: %v", filesChanged[0])
	}

	summary := res["summary"].(string)
	if !strings.HasPrefix(summary, "Implement GitLab client features") {
		t.Errorf("unexpected summary: %s", summary)
	}

	patchText := res["patch"].(string)
	if !strings.Contains(patchText, "package gitlab") {
		t.Errorf("unexpected patch text: %s", patchText)
	}
}

func TestParseJSONMarkdown_KeyQuoteInString(t *testing.T) {
	content := `{
  "files_changed": ["a.go"],
  "summary": "testing key quote in string",
  "patch": "req.Header.Set(\"Content-Type\", \"application/json\")\nand some other code",
  "next_key": "next value"
}`
	res, err := ParseJSONMarkdown(content)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	expectedPatch := `req.Header.Set("Content-Type", "application/json")` + "\n" + `and some other code`
	if res["patch"] != expectedPatch {
		t.Errorf("expected patch %q, got %q", expectedPatch, res["patch"])
	}
	if res["next_key"] != "next value" {
		t.Errorf("expected next_key 'next value', got %q", res["next_key"])
	}
}

func TestParseJSONMarkdown_BracketsMismatch(t *testing.T) {
	content := `{
  "files_changed": ["a.go"],
  "required_skills_map": {
    "backend": [
      "golang-best-practices"
    ],
    "qa": [
      "golang-testing"
    }
  }
}`
	res, err := ParseJSONMarkdown(content)
	if err != nil {
		t.Fatalf("expected no error after repairing mismatch, got %v", err)
	}

	skillsMap, ok := res["required_skills_map"].(map[string]any)
	if !ok {
		t.Fatalf("expected required_skills_map to be a map, got %T", res["required_skills_map"])
	}

	qaSkills, ok := skillsMap["qa"].([]any)
	if !ok {
		t.Fatalf("expected qa skills to be array, got %T", skillsMap["qa"])
	}

	if len(qaSkills) != 1 || qaSkills[0] != "golang-testing" {
		t.Errorf("unexpected qa skills contents: %v", qaSkills)
	}
}
