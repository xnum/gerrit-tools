package gerrit

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/gerrit-ai-review/gerrit-tools/pkg/types"
)

type localHTTPTestServer struct {
	URL   string
	close func()
}

func (s *localHTTPTestServer) Close() {
	if s.close != nil {
		s.close()
	}
}

func newLocalHTTPTestServer(t *testing.T, handler http.Handler) *localHTTPTestServer {
	t.Helper()

	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping network-dependent test: %v", err)
		return nil
	}

	server := &http.Server{Handler: handler}
	go func() {
		_ = server.Serve(listener)
	}()

	return &localHTTPTestServer{
		URL: "http://" + listener.Addr().String(),
		close: func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = server.Shutdown(ctx)
			_ = listener.Close()
		},
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("https://gerrit.example.com", "user", "pass")

	if client.baseURL != "https://gerrit.example.com" {
		t.Errorf("Expected baseURL 'https://gerrit.example.com', got '%s'", client.baseURL)
	}
	if client.username != "user" {
		t.Errorf("Expected username 'user', got '%s'", client.username)
	}
	if client.password != "pass" {
		t.Errorf("Expected password 'pass', got '%s'", client.password)
	}
}

func TestNewClient_TrimTrailingSlash(t *testing.T) {
	client := NewClient("https://gerrit.example.com/", "user", "pass")

	if client.baseURL != "https://gerrit.example.com" {
		t.Errorf("Expected trailing slash to be trimmed, got '%s'", client.baseURL)
	}
}

func TestBuildReviewInput(t *testing.T) {
	client := NewClient("https://gerrit.example.com", "user", "pass")

	result := &types.ReviewResult{
		Summary: "Looks good!",
		Vote:    1,
		Comments: []types.Comment{
			{File: "main.go", Line: 10, Message: "Good code"},
			{File: "main.go", Line: 20, Message: "Consider refactoring"},
			{File: "test.go", Line: 5, Message: "Add more tests"},
		},
	}

	input := client.buildReviewInput(result)

	// Check message
	if input.Message == "" {
		t.Error("Expected non-empty message")
	}
	if !contains(input.Message, "🤖") {
		t.Error("Expected message to contain robot emoji")
	}
	if !contains(input.Message, "Looks good!") {
		t.Error("Expected message to contain summary")
	}

	// Check labels
	if input.Labels["Code-Review"] != 1 {
		t.Errorf("Expected Code-Review label 1, got %d", input.Labels["Code-Review"])
	}

	// Check comments are grouped by file
	if len(input.Comments) != 2 {
		t.Errorf("Expected 2 files in comments, got %d", len(input.Comments))
	}

	// Check main.go has 2 comments
	mainComments := input.Comments["main.go"]
	if len(mainComments) != 2 {
		t.Errorf("Expected 2 comments for main.go, got %d", len(mainComments))
	}

	// Check test.go has 1 comment
	testComments := input.Comments["test.go"]
	if len(testComments) != 1 {
		t.Errorf("Expected 1 comment for test.go, got %d", len(testComments))
	}

	// Check that comments are marked as unresolved
	if !mainComments[0].Unresolved {
		t.Error("Expected comments to be marked as unresolved")
	}
}

func TestFormatReviewMessage(t *testing.T) {
	client := NewClient("https://gerrit.example.com", "user", "pass")

	result := &types.ReviewResult{
		Summary: "Code looks good",
		Vote:    1,
	}

	msg := client.formatReviewMessage(result)

	// Check message contains expected parts
	if msg == "" {
		t.Error("Expected non-empty message")
	}

	// Should contain summary
	if !contains(msg, "Code looks good") {
		t.Error("Message should contain summary")
	}

	// Should contain automated signature
	if !contains(msg, "Automated review by Gerrit AI Reviewer") {
		t.Error("Message should contain automated signature")
	}
}

func TestGroupCommentsByFile(t *testing.T) {
	client := NewClient("https://gerrit.example.com", "user", "pass")

	comments := []types.Comment{
		{File: "a.go", Line: 1, Message: "msg1"},
		{File: "b.go", Line: 2, Message: "msg2"},
		{File: "a.go", Line: 3, Message: "msg3"},
	}

	grouped := client.groupCommentsByFile(comments)

	if len(grouped) != 2 {
		t.Errorf("Expected 2 files, got %d", len(grouped))
	}

	if len(grouped["a.go"]) != 2 {
		t.Errorf("Expected 2 comments for a.go, got %d", len(grouped["a.go"]))
	}

	if len(grouped["b.go"]) != 1 {
		t.Errorf("Expected 1 comment for b.go, got %d", len(grouped["b.go"]))
	}
}

func TestPostReview(t *testing.T) {
	// Create a test server
	server := newLocalHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != "POST" {
			t.Errorf("Expected POST request, got %s", r.Method)
		}

		// Verify URL path
		expectedPath := "/a/changes/12345/revisions/3/review"
		if r.URL.Path != expectedPath {
			t.Errorf("Expected path %s, got %s", expectedPath, r.URL.Path)
		}

		// Verify authentication
		username, password, ok := r.BasicAuth()
		if !ok {
			t.Error("Expected basic auth to be present")
		}
		if username != "test-user" || password != "test-pass" {
			t.Errorf("Expected auth test-user:test-pass, got %s:%s", username, password)
		}

		// Verify content type
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Parse and verify request body
		var input ReviewInput
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Errorf("Failed to decode request body: %v", err)
		}

		if input.Labels["Code-Review"] != 1 {
			t.Errorf("Expected vote 1, got %d", input.Labels["Code-Review"])
		}

		// Send success response
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-user", "test-pass")

	// Create review result
	result := &types.ReviewResult{
		Summary: "Test review",
		Vote:    1,
		Comments: []types.Comment{
			{File: "test.go", Line: 10, Message: "Test comment"},
		},
	}

	// Post review
	ctx := context.Background()
	err := client.PostReview(ctx, 12345, 3, result)
	if err != nil {
		t.Errorf("PostReview() failed: %v", err)
	}
}

func TestPostReview_ErrorResponse(t *testing.T) {
	// Create a test server that returns an error
	server := newLocalHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Bad request"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-user", "test-pass")

	result := &types.ReviewResult{
		Summary: "Test",
		Vote:    1,
	}

	ctx := context.Background()
	err := client.PostReview(ctx, 12345, 3, result)
	if err == nil {
		t.Error("Expected error for bad request, got nil")
	}
}

func TestPing(t *testing.T) {
	// Create a test server that simulates /a/accounts/self endpoint
	server := newLocalHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/a/accounts/self" {
			t.Errorf("Expected path /a/accounts/self, got %s", r.URL.Path)
		}

		username, password, ok := r.BasicAuth()
		if !ok || username != "test-user" || password != "test-pass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(")]}'\n{\"name\": \"Test User\"}"))
	}))
	defer server.Close()

	t.Run("valid credentials", func(t *testing.T) {
		client := NewClient(server.URL, "test-user", "test-pass")
		ctx := context.Background()
		err := client.Ping(ctx)
		if err != nil {
			t.Errorf("Ping() failed: %v", err)
		}
	})

	t.Run("invalid credentials", func(t *testing.T) {
		client := NewClient(server.URL, "wrong-user", "wrong-pass")
		ctx := context.Background()
		err := client.Ping(ctx)
		if err == nil {
			t.Error("Expected error for invalid credentials, got nil")
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestListChanges(t *testing.T) {
	// Create a test server
	server := newLocalHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Verify query parameters
		query := r.URL.Query().Get("q")
		if query != "status:open" {
			t.Errorf("Expected query 'status:open', got '%s'", query)
		}

		// Verify authentication
		username, password, ok := r.BasicAuth()
		if !ok || username != "test-user" || password != "test-pass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Send response with XSSI prefix
		w.WriteHeader(http.StatusOK)
		response := []ChangeInfo{
			{
				ID:      "test-project~main~I1234",
				Project: "test-project",
				Branch:  "main",
				Subject: "Test change",
				Status:  "NEW",
				Number:  12345,
			},
		}
		data, _ := json.Marshal(response)
		w.Write([]byte(")]}'\n"))
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-user", "test-pass")
	ctx := context.Background()

	changes, err := client.ListChanges(ctx, "status:open", []string{}, 0)
	if err != nil {
		t.Errorf("ListChanges() failed: %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(changes))
	}

	if changes[0].Number != 12345 {
		t.Errorf("Expected change number 12345, got %d", changes[0].Number)
	}
}

func TestGetChangeDetail(t *testing.T) {
	// Create a test server
	server := newLocalHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Verify URL path
		if r.URL.Path != "/a/changes/12345" {
			t.Errorf("Expected path /a/changes/12345, got %s", r.URL.Path)
		}

		// Verify authentication
		username, password, ok := r.BasicAuth()
		if !ok || username != "test-user" || password != "test-pass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// Send response with XSSI prefix
		w.WriteHeader(http.StatusOK)
		response := ChangeInfo{
			ID:      "test-project~main~I1234",
			Project: "test-project",
			Branch:  "main",
			Subject: "Test change",
			Status:  "NEW",
			Number:  12345,
		}
		data, _ := json.Marshal(response)
		w.Write([]byte(")]}'\n"))
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-user", "test-pass")
	ctx := context.Background()

	change, err := client.GetChangeDetail(ctx, "12345", []string{})
	if err != nil {
		t.Errorf("GetChangeDetail() failed: %v", err)
	}

	if change.Number != 12345 {
		t.Errorf("Expected change number 12345, got %d", change.Number)
	}

	if change.Project != "test-project" {
		t.Errorf("Expected project 'test-project', got '%s'", change.Project)
	}
}

func TestGetRevisionFiles(t *testing.T) {
	// Create a test server
	server := newLocalHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Verify URL path
		if r.URL.Path != "/a/changes/12345/revisions/current/files/" {
			t.Errorf("Expected path /a/changes/12345/revisions/current/files/, got %s", r.URL.Path)
		}

		// Send response
		w.WriteHeader(http.StatusOK)
		response := map[string]*FileInfo{
			"/COMMIT_MSG": {Status: "A"},
			"src/main.go": {
				Status:        "M",
				LinesInserted: 10,
				LinesDeleted:  5,
			},
		}
		data, _ := json.Marshal(response)
		w.Write([]byte(")]}'\n"))
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-user", "test-pass")
	ctx := context.Background()

	files, err := client.GetRevisionFiles(ctx, "12345", "current", "")
	if err != nil {
		t.Errorf("GetRevisionFiles() failed: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	mainFile, ok := files["src/main.go"]
	if !ok {
		t.Error("Expected src/main.go to be in results")
	}

	if mainFile.Status != "M" {
		t.Errorf("Expected status 'M', got '%s'", mainFile.Status)
	}
}

func TestGetRevisionFilesWithBase(t *testing.T) {
	// Create a test server
	server := newLocalHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Verify URL path
		if r.URL.Path != "/a/changes/12345/revisions/3/files/" {
			t.Errorf("Expected path /a/changes/12345/revisions/3/files/, got %s", r.URL.Path)
		}

		// Verify base parameter
		base := r.URL.Query().Get("base")
		if base != "1" {
			t.Errorf("Expected base parameter '1', got '%s'", base)
		}

		// Send response
		w.WriteHeader(http.StatusOK)
		response := map[string]*FileInfo{
			"src/main.go": {
				Status:        "M",
				LinesInserted: 5,
				LinesDeleted:  2,
			},
		}
		data, _ := json.Marshal(response)
		w.Write([]byte(")]}'\n"))
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-user", "test-pass")
	ctx := context.Background()

	files, err := client.GetRevisionFiles(ctx, "12345", "3", "1")
	if err != nil {
		t.Errorf("GetRevisionFiles() with base failed: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 file, got %d", len(files))
	}

	mainFile, ok := files["src/main.go"]
	if !ok {
		t.Error("Expected src/main.go to be in results")
	}

	if mainFile.Status != "M" {
		t.Errorf("Expected status 'M', got '%s'", mainFile.Status)
	}
}

func TestGetRevisionDiff(t *testing.T) {
	// Create a test server
	server := newLocalHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Send response
		w.WriteHeader(http.StatusOK)
		response := DiffInfo{
			ChangeType: "MODIFIED",
			Content: []DiffContent{
				{
					AB: []string{"unchanged line"},
				},
				{
					A: []string{"old line"},
					B: []string{"new line"},
				},
			},
		}
		data, _ := json.Marshal(response)
		w.Write([]byte(")]}'\n"))
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-user", "test-pass")
	ctx := context.Background()

	diff, err := client.GetRevisionDiff(ctx, "12345", "current", "src/main.go", "")
	if err != nil {
		t.Errorf("GetRevisionDiff() failed: %v", err)
	}

	if diff.ChangeType != "MODIFIED" {
		t.Errorf("Expected change type 'MODIFIED', got '%s'", diff.ChangeType)
	}

	if len(diff.Content) != 2 {
		t.Errorf("Expected 2 content blocks, got %d", len(diff.Content))
	}
}

func TestGetRevisionDiffWithBase(t *testing.T) {
	// Create a test server
	server := newLocalHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Verify base parameter
		base := r.URL.Query().Get("base")
		if base != "1" {
			t.Errorf("Expected base parameter '1', got '%s'", base)
		}

		// Send response
		w.WriteHeader(http.StatusOK)
		response := DiffInfo{
			ChangeType: "MODIFIED",
			Content: []DiffContent{
				{
					AB: []string{"unchanged line"},
				},
				{
					A: []string{"old line from PS1"},
					B: []string{"new line in PS3"},
				},
			},
		}
		data, _ := json.Marshal(response)
		w.Write([]byte(")]}'\n"))
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-user", "test-pass")
	ctx := context.Background()

	diff, err := client.GetRevisionDiff(ctx, "12345", "3", "src/main.go", "1")
	if err != nil {
		t.Errorf("GetRevisionDiff() with base failed: %v", err)
	}

	if diff.ChangeType != "MODIFIED" {
		t.Errorf("Expected change type 'MODIFIED', got '%s'", diff.ChangeType)
	}

	if len(diff.Content) != 2 {
		t.Errorf("Expected 2 content blocks, got %d", len(diff.Content))
	}
}

func TestListComments(t *testing.T) {
	// Create a test server
	server := newLocalHTTPTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HTTP method
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}

		// Verify URL path
		if r.URL.Path != "/a/changes/12345/revisions/current/comments/" {
			t.Errorf("Expected path /a/changes/12345/revisions/current/comments/, got %s", r.URL.Path)
		}

		// Send response
		w.WriteHeader(http.StatusOK)
		response := map[string][]CommentInfo{
			"src/main.go": {
				{
					ID:         "comment1",
					Message:    "Test comment",
					Line:       10,
					Unresolved: true,
				},
			},
		}
		data, _ := json.Marshal(response)
		w.Write([]byte(")]}'\n"))
		w.Write(data)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-user", "test-pass")
	ctx := context.Background()

	comments, err := client.ListComments(ctx, "12345", "current")
	if err != nil {
		t.Errorf("ListComments() failed: %v", err)
	}

	if len(comments) != 1 {
		t.Errorf("Expected 1 file with comments, got %d", len(comments))
	}

	mainComments, ok := comments["src/main.go"]
	if !ok {
		t.Error("Expected src/main.go to have comments")
	}

	if len(mainComments) != 1 {
		t.Errorf("Expected 1 comment, got %d", len(mainComments))
	}

	if mainComments[0].Message != "Test comment" {
		t.Errorf("Expected message 'Test comment', got '%s'", mainComments[0].Message)
	}
}
