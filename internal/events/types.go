package events

// Event represents a Gerrit stream-events JSON line
type Event struct {
	Type           string     `json:"type"`
	Change         *Change    `json:"change,omitempty"`
	PatchSet       *PatchSet  `json:"patchSet,omitempty"`
	EventCreatedOn int64      `json:"eventCreatedOn"`
}

// Change represents change information in an event
type Change struct {
	Project string   `json:"project"`
	Branch  string   `json:"branch"`
	Number  int      `json:"number"`
	Subject string   `json:"subject"`
	Owner   *Account `json:"owner,omitempty"`
	URL     string   `json:"url,omitempty"`
}

// PatchSet represents patchset information in an event
type PatchSet struct {
	Number   int      `json:"number"`
	Ref      string   `json:"ref"`
	Revision string   `json:"revision"`
	Uploader *Account `json:"uploader,omitempty"`
}

// Account represents a Gerrit user account
type Account struct {
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Username string `json:"username,omitempty"`
}
