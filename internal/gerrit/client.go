package gerrit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gerrit-ai-review/gerrit-tools/pkg/types"
)

// Client handles communication with Gerrit REST API
type Client struct {
	baseURL    string
	username   string
	password   string
	httpClient *http.Client
}

// NewClient creates a new Gerrit REST API client
func NewClient(baseURL, username, password string) *Client {
	return &Client{
		baseURL:  strings.TrimSuffix(baseURL, "/"),
		username: username,
		password: password,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ReviewInput represents the JSON payload for posting a review
type ReviewInput struct {
	Message  string                       `json:"message"`
	Labels   map[string]int               `json:"labels,omitempty"`
	Comments map[string][]CommentInput    `json:"comments,omitempty"`
	Drafts   string                       `json:"drafts,omitempty"`
}

// CommentInput represents a single inline comment
type CommentInput struct {
	Line       int    `json:"line,omitempty"`
	Message    string `json:"message"`
	Unresolved bool   `json:"unresolved,omitempty"`
}

// PostReview posts a code review with vote and comments to Gerrit
func (c *Client) PostReview(ctx context.Context, changeNum, patchsetNum int, result *types.ReviewResult) error {
	// Build review input
	input := c.buildReviewInput(result)

	// Construct API endpoint
	// Format: /a/changes/{change-id}/revisions/{revision-id}/review
	url := fmt.Sprintf("%s/a/changes/%d/revisions/%d/review",
		c.baseURL, changeNum, patchsetNum)

	// Marshal to JSON
	jsonData, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal review input: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.username, c.password)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	// Check status code
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// buildReviewInput constructs the ReviewInput from ReviewResult
func (c *Client) buildReviewInput(result *types.ReviewResult) *ReviewInput {
	input := &ReviewInput{
		Message: c.formatReviewMessage(result),
		Labels: map[string]int{
			"Code-Review": result.Vote,
		},
		Drafts: "PUBLISH", // Publish all draft comments when posting the review
	}

	// Add inline comments if present
	if len(result.Comments) > 0 {
		input.Comments = c.groupCommentsByFile(result.Comments)
	}

	return input
}

// formatReviewMessage formats the review message with summary
func (c *Client) formatReviewMessage(result *types.ReviewResult) string {
	var msg strings.Builder

	msg.WriteString("🤖 AI Code Review\n\n")
	msg.WriteString(result.Summary)
	msg.WriteString("\n\n---\n")
	msg.WriteString("_Automated review by Claude_")

	return msg.String()
}

// groupCommentsByFile groups comments by file path for the API
func (c *Client) groupCommentsByFile(comments []types.Comment) map[string][]CommentInput {
	grouped := make(map[string][]CommentInput)

	for _, comment := range comments {
		commentInput := CommentInput{
			Line:       comment.Line,
			Message:    comment.Message,
			Unresolved: true, // Mark all AI comments as unresolved by default
		}

		grouped[comment.File] = append(grouped[comment.File], commentInput)
	}

	return grouped
}

// GetChange retrieves information about a change (for future use)
func (c *Client) GetChange(ctx context.Context, changeNum int) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/a/changes/%d", c.baseURL, changeNum)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Gerrit prepends ")]}'" to JSON responses for security
	bodyStr := strings.TrimPrefix(string(body), ")]}'")

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(bodyStr), &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return result, nil
}

// Ping checks if the Gerrit server is reachable and credentials are valid
func (c *Client) Ping(ctx context.Context) error {
	url := fmt.Sprintf("%s/a/accounts/self", c.baseURL)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to gerrit: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		return fmt.Errorf("authentication failed: invalid credentials")
	}

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gerrit returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListChanges queries for changes matching the given query string
// query: Gerrit search query (e.g., "status:open project:myproject")
// options: Additional options like "CURRENT_REVISION", "DETAILED_ACCOUNTS", etc.
// limit: Maximum number of results to return (0 for default)
func (c *Client) ListChanges(ctx context.Context, query string, options []string, limit int) ([]ChangeInfo, error) {
	// Build URL with query parameters - URL encode the query
	url := fmt.Sprintf("%s/a/changes/?q=%s", c.baseURL, url.QueryEscape(query))

	// Add options if provided
	for _, opt := range options {
		url += fmt.Sprintf("&o=%s", opt)
	}

	// Add limit if specified
	if limit > 0 {
		url += fmt.Sprintf("&n=%d", limit)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Remove Gerrit's XSSI prefix
	bodyStr := strings.TrimPrefix(string(body), ")]}'")

	var changes []ChangeInfo
	if err := json.Unmarshal([]byte(bodyStr), &changes); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return changes, nil
}

// GetChangeDetail retrieves detailed information about a specific change
// changeID: Change identifier (numeric ID, "I..." ID, or "project~branch~I..." triplet)
// options: Additional options like "CURRENT_REVISION", "DETAILED_ACCOUNTS", "MESSAGES", etc.
func (c *Client) GetChangeDetail(ctx context.Context, changeID string, options []string) (*ChangeInfo, error) {
	// Build URL
	url := fmt.Sprintf("%s/a/changes/%s", c.baseURL, changeID)

	// Add options if provided
	if len(options) > 0 {
		url += "?"
		for i, opt := range options {
			if i > 0 {
				url += "&"
			}
			url += fmt.Sprintf("o=%s", opt)
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Remove Gerrit's XSSI prefix
	bodyStr := strings.TrimPrefix(string(body), ")]}'")

	var change ChangeInfo
	if err := json.Unmarshal([]byte(bodyStr), &change); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &change, nil
}

// GetRevisionFiles retrieves the list of files modified in a revision
// changeID: Change identifier
// revisionID: Revision identifier (e.g., "current", "1", "2", or commit SHA)
// base: Optional base patchset to compare against (empty string means compare against parent commit)
func (c *Client) GetRevisionFiles(ctx context.Context, changeID, revisionID, base string) (map[string]*FileInfo, error) {
	apiURL := fmt.Sprintf("%s/a/changes/%s/revisions/%s/files/", c.baseURL, changeID, revisionID)

	// Add base parameter if provided
	if base != "" {
		apiURL += fmt.Sprintf("?base=%s", base)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Remove Gerrit's XSSI prefix
	bodyStr := strings.TrimPrefix(string(body), ")]}'")

	var files map[string]*FileInfo
	if err := json.Unmarshal([]byte(bodyStr), &files); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return files, nil
}

// GetRevisionDiff retrieves the diff for a specific file in a revision
// changeID: Change identifier
// revisionID: Revision identifier (e.g., "current", "1", "2", or commit SHA)
// filePath: Path to the file (will be URL encoded)
// base: Optional base patchset to compare against (empty string means compare against parent commit)
func (c *Client) GetRevisionDiff(ctx context.Context, changeID, revisionID, filePath, base string) (*DiffInfo, error) {
	// URL encode the file path
	encodedPath := url.PathEscape(filePath)
	apiURL := fmt.Sprintf("%s/a/changes/%s/revisions/%s/files/%s/diff", c.baseURL, changeID, revisionID, encodedPath)

	// Add base parameter if provided
	if base != "" {
		apiURL += fmt.Sprintf("?base=%s", base)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Remove Gerrit's XSSI prefix
	bodyStr := strings.TrimPrefix(string(body), ")]}'")

	var diff DiffInfo
	if err := json.Unmarshal([]byte(bodyStr), &diff); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &diff, nil
}

// ListComments retrieves all comments for a specific revision
// changeID: Change identifier
// revisionID: Revision identifier (e.g., "current", "1", "2", or commit SHA)
// Returns a map of file paths to their comments
func (c *Client) ListComments(ctx context.Context, changeID, revisionID string) (map[string][]CommentInfo, error) {
	apiURL := fmt.Sprintf("%s/a/changes/%s/revisions/%s/comments/", c.baseURL, changeID, revisionID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Remove Gerrit's XSSI prefix
	bodyStr := strings.TrimPrefix(string(body), ")]}'")

	var comments map[string][]CommentInfo
	if err := json.Unmarshal([]byte(bodyStr), &comments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return comments, nil
}

// CreateDraft creates a new draft comment
// changeID: Change identifier
// revisionID: Revision identifier (e.g., "current", "1", "2", or commit SHA)
// input: Draft comment input
// Returns the created draft comment
func (c *Client) CreateDraft(ctx context.Context, changeID, revisionID string, input *DraftInput) (*CommentInfo, error) {
	apiURL := fmt.Sprintf("%s/a/changes/%s/revisions/%s/drafts", c.baseURL, changeID, revisionID)

	// Marshal input to JSON
	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal draft input: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Remove Gerrit's XSSI prefix
	bodyStr := strings.TrimPrefix(string(body), ")]}'")

	var comment CommentInfo
	if err := json.Unmarshal([]byte(bodyStr), &comment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &comment, nil
}

// ListDrafts retrieves all draft comments for the current user
// changeID: Change identifier
// revisionID: Revision identifier (e.g., "current", "1", "2", or commit SHA)
// Returns a map of file paths to their draft comments
func (c *Client) ListDrafts(ctx context.Context, changeID, revisionID string) (map[string][]CommentInfo, error) {
	apiURL := fmt.Sprintf("%s/a/changes/%s/revisions/%s/drafts/", c.baseURL, changeID, revisionID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Remove Gerrit's XSSI prefix
	bodyStr := strings.TrimPrefix(string(body), ")]}'")

	var drafts map[string][]CommentInfo
	if err := json.Unmarshal([]byte(bodyStr), &drafts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return drafts, nil
}

// GetDraft retrieves a specific draft comment
// changeID: Change identifier
// revisionID: Revision identifier (e.g., "current", "1", "2", or commit SHA)
// draftID: Draft comment ID
func (c *Client) GetDraft(ctx context.Context, changeID, revisionID, draftID string) (*CommentInfo, error) {
	apiURL := fmt.Sprintf("%s/a/changes/%s/revisions/%s/drafts/%s", c.baseURL, changeID, revisionID, draftID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Remove Gerrit's XSSI prefix
	bodyStr := strings.TrimPrefix(string(body), ")]}'")

	var draft CommentInfo
	if err := json.Unmarshal([]byte(bodyStr), &draft); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &draft, nil
}

// UpdateDraft updates an existing draft comment
// changeID: Change identifier
// revisionID: Revision identifier (e.g., "current", "1", "2", or commit SHA)
// draftID: Draft comment ID
// input: Updated draft comment input
func (c *Client) UpdateDraft(ctx context.Context, changeID, revisionID, draftID string, input *DraftInput) (*CommentInfo, error) {
	apiURL := fmt.Sprintf("%s/a/changes/%s/revisions/%s/drafts/%s", c.baseURL, changeID, revisionID, draftID)

	// Marshal input to JSON
	jsonData, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal draft input: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", apiURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Remove Gerrit's XSSI prefix
	bodyStr := strings.TrimPrefix(string(body), ")]}'")

	var comment CommentInfo
	if err := json.Unmarshal([]byte(bodyStr), &comment); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &comment, nil
}

// DeleteDraft deletes a draft comment
// changeID: Change identifier
// revisionID: Revision identifier (e.g., "current", "1", "2", or commit SHA)
// draftID: Draft comment ID
func (c *Client) DeleteDraft(ctx context.Context, changeID, revisionID, draftID string) error {
	apiURL := fmt.Sprintf("%s/a/changes/%s/revisions/%s/drafts/%s", c.baseURL, changeID, revisionID, draftID)

	req, err := http.NewRequestWithContext(ctx, "DELETE", apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 204 && resp.StatusCode != 200 {
		return fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ListAllComments retrieves all comments for a change across all patchsets
// changeID: Change identifier
// Returns a map of file paths to their comments (includes comments from all revisions)
func (c *Client) ListAllComments(ctx context.Context, changeID string) (map[string][]CommentInfo, error) {
	apiURL := fmt.Sprintf("%s/a/changes/%s/comments/", c.baseURL, changeID)

	req, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.SetBasicAuth(c.username, c.password)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("gerrit API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Remove Gerrit's XSSI prefix
	bodyStr := strings.TrimPrefix(string(body), ")]}'")

	var comments map[string][]CommentInfo
	if err := json.Unmarshal([]byte(bodyStr), &comments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return comments, nil
}
