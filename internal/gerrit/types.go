package gerrit

import (
	"encoding/json"
	"strings"
	"time"
)

// GerritTime is a custom time type that handles Gerrit's various time formats
type GerritTime struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler for GerritTime
func (gt *GerritTime) UnmarshalJSON(data []byte) error {
	// Remove quotes
	s := strings.Trim(string(data), `"`)
	if s == "null" || s == "" {
		return nil
	}

	// Try different time formats that Gerrit uses
	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02 15:04:05.000000000",
		"2006-01-02 15:04:05.000000",
		"2006-01-02 15:04:05",
	}

	var parseErr error
	for _, format := range formats {
		t, err := time.Parse(format, s)
		if err == nil {
			gt.Time = t
			return nil
		}
		parseErr = err
	}

	return parseErr
}

// MarshalJSON implements json.Marshaler for GerritTime
func (gt GerritTime) MarshalJSON() ([]byte, error) {
	if gt.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(gt.Time.Format(time.RFC3339))
}

// ChangeInfo represents information about a Gerrit change
type ChangeInfo struct {
	ID              string                  `json:"id"`
	Project         string                  `json:"project"`
	Branch          string                  `json:"branch"`
	ChangeID        string                  `json:"change_id"`
	Subject         string                  `json:"subject"`
	Status          string                  `json:"status"`
	Created         GerritTime              `json:"created"`
	Updated         GerritTime              `json:"updated"`
	Submitted       *GerritTime             `json:"submitted,omitempty"`
	Submitter       *AccountInfo            `json:"submitter,omitempty"`
	Owner           AccountInfo             `json:"owner"`
	Topic           string                  `json:"topic,omitempty"`
	Hashtags        []string                `json:"hashtags,omitempty"`
	Labels          map[string]*LabelInfo   `json:"labels,omitempty"`
	Messages        []ChangeMessageInfo     `json:"messages,omitempty"`
	CurrentRevision string                  `json:"current_revision,omitempty"`
	Revisions       map[string]*RevisionInfo `json:"revisions,omitempty"`
	Number          int                     `json:"_number"`
	Mergeable       bool                    `json:"mergeable,omitempty"`
	Insertions      int                     `json:"insertions,omitempty"`
	Deletions       int                     `json:"deletions,omitempty"`
	UnresolvedCommentCount int              `json:"unresolved_comment_count,omitempty"`
	TotalCommentCount int                   `json:"total_comment_count,omitempty"`
}

// AccountInfo represents a Gerrit user account
type AccountInfo struct {
	AccountID int    `json:"_account_id,omitempty"`
	Name      string `json:"name,omitempty"`
	Email     string `json:"email,omitempty"`
	Username  string `json:"username,omitempty"`
}

// LabelInfo represents information about a label (e.g., Code-Review)
type LabelInfo struct {
	Optional bool                   `json:"optional,omitempty"`
	Approved *AccountInfo           `json:"approved,omitempty"`
	Rejected *AccountInfo           `json:"rejected,omitempty"`
	Recommended *AccountInfo        `json:"recommended,omitempty"`
	Disliked *AccountInfo           `json:"disliked,omitempty"`
	Blocking bool                   `json:"blocking,omitempty"`
	Value    int                    `json:"value,omitempty"`
	DefaultValue int                `json:"default_value,omitempty"`
	All      []ApprovalInfo         `json:"all,omitempty"`
}

// ApprovalInfo represents a single vote/approval on a label
type ApprovalInfo struct {
	AccountInfo
	Value int        `json:"value"`
	Date  GerritTime `json:"date,omitempty"`
}

// ChangeMessageInfo represents a message on a change
type ChangeMessageInfo struct {
	ID      string       `json:"id"`
	Author  *AccountInfo `json:"author,omitempty"`
	Date    GerritTime   `json:"date"`
	Message string       `json:"message"`
	Tag     string       `json:"tag,omitempty"`
}

// RevisionInfo represents information about a patchset/revision
type RevisionInfo struct {
	Kind        string              `json:"kind"`
	Number      int                 `json:"_number"`
	Created     GerritTime          `json:"created"`
	Uploader    AccountInfo         `json:"uploader"`
	Ref         string              `json:"ref"`
	Fetch       map[string]*FetchInfo `json:"fetch,omitempty"`
	Commit      *CommitInfo         `json:"commit,omitempty"`
	Files       map[string]*FileInfo `json:"files,omitempty"`
	Description string              `json:"description,omitempty"`
}

// FetchInfo represents fetch information for a revision
type FetchInfo struct {
	URL      string            `json:"url"`
	Ref      string            `json:"ref"`
	Commands map[string]string `json:"commands,omitempty"`
}

// CommitInfo represents commit information
type CommitInfo struct {
	Commit    string        `json:"commit"`
	Parents   []CommitInfo  `json:"parents,omitempty"`
	Author    GitPersonInfo `json:"author"`
	Committer GitPersonInfo `json:"committer"`
	Subject   string        `json:"subject"`
	Message   string        `json:"message"`
}

// GitPersonInfo represents author/committer information
type GitPersonInfo struct {
	Name  string     `json:"name"`
	Email string     `json:"email"`
	Date  GerritTime `json:"date"`
}

// FileInfo represents information about a file in a revision
type FileInfo struct {
	Status        string `json:"status,omitempty"` // 'M' (modified), 'A' (added), 'D' (deleted), 'R' (renamed), 'C' (copied)
	Binary        bool   `json:"binary,omitempty"`
	OldPath       string `json:"old_path,omitempty"`
	LinesInserted int    `json:"lines_inserted,omitempty"`
	LinesDeleted  int    `json:"lines_deleted,omitempty"`
	SizeDelta     int    `json:"size_delta,omitempty"`
	Size          int    `json:"size,omitempty"`
}

// DiffInfo represents diff information for a file
type DiffInfo struct {
	MetaA         *DiffFileMetaInfo `json:"meta_a,omitempty"`
	MetaB         *DiffFileMetaInfo `json:"meta_b,omitempty"`
	ChangeType    string            `json:"change_type"`
	IntralineStatus string          `json:"intraline_status,omitempty"`
	DiffHeader    []string          `json:"diff_header,omitempty"`
	Content       []DiffContent     `json:"content,omitempty"`
	Binary        bool              `json:"binary,omitempty"`
}

// DiffFileMetaInfo represents metadata about a file in a diff
type DiffFileMetaInfo struct {
	Name        string         `json:"name"`
	ContentType string         `json:"content_type"`
	Lines       int            `json:"lines,omitempty"`
	WebLinks    []WebLinkInfo  `json:"web_links,omitempty"`
}

// WebLinkInfo represents a web link
type WebLinkInfo struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	ImageURL string `json:"image_url,omitempty"`
}

// DiffContent represents a chunk of diff content
type DiffContent struct {
	A      []string `json:"a,omitempty"`      // Lines from side A (old file)
	B      []string `json:"b,omitempty"`      // Lines from side B (new file)
	AB     []string `json:"ab,omitempty"`     // Lines common to both sides
	Skip   int      `json:"skip,omitempty"`   // Number of lines to skip
	Common bool     `json:"common,omitempty"` // Whether this is common context
}

// CommentInfo represents a comment on a change
type CommentInfo struct {
	PatchSet    int          `json:"patch_set,omitempty"`
	ID          string       `json:"id"`
	Path        string       `json:"path,omitempty"`
	Side        string       `json:"side,omitempty"` // "PARENT" or "REVISION"
	Line        int          `json:"line,omitempty"`
	Range       *CommentRange `json:"range,omitempty"`
	InReplyTo   string       `json:"in_reply_to,omitempty"`
	Message     string       `json:"message"`
	Updated     GerritTime   `json:"updated"`
	Author      *AccountInfo `json:"author,omitempty"`
	Tag         string       `json:"tag,omitempty"`
	Unresolved  bool         `json:"unresolved,omitempty"`
}

// CommentRange represents a range of text in a comment
type CommentRange struct {
	StartLine      int `json:"start_line"`
	StartCharacter int `json:"start_character"`
	EndLine        int `json:"end_line"`
	EndCharacter   int `json:"end_character"`
}

// RobotCommentInfo represents a robot comment (extends CommentInfo)
type RobotCommentInfo struct {
	CommentInfo
	RobotID   string          `json:"robot_id"`
	RobotRunID string         `json:"robot_run_id"`
	Properties map[string]string `json:"properties,omitempty"`
	FixSuggestions []FixSuggestionInfo `json:"fix_suggestions,omitempty"`
}

// FixSuggestionInfo represents a suggested fix
type FixSuggestionInfo struct {
	FixID       string               `json:"fix_id"`
	Description string               `json:"description"`
	Replacements []FixReplacementInfo `json:"replacements"`
}

// FixReplacementInfo represents a replacement in a fix
type FixReplacementInfo struct {
	Path        string        `json:"path"`
	Range       CommentRange  `json:"range"`
	Replacement string        `json:"replacement"`
}

// DraftInput represents input for creating or updating draft comments
type DraftInput struct {
	Path       string        `json:"path"`                  // File path
	Line       int           `json:"line,omitempty"`        // Line number (for line comments)
	Range      *CommentRange `json:"range,omitempty"`       // Character range (for range comments)
	Message    string        `json:"message"`               // Comment text
	Unresolved *bool         `json:"unresolved,omitempty"`  // Mark as unresolved (pointer to distinguish false from unset)
	InReplyTo  string        `json:"in_reply_to,omitempty"` // Reply to another comment ID
}
