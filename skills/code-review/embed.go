package codereview

import (
	"embed"
	"fmt"
)

const defaultSkillFile = "SKILL.md"

//go:embed SKILL.md
var files embed.FS

// Content returns embedded default code review skill content.
func Content() (string, error) {
	b, err := files.ReadFile(defaultSkillFile)
	if err != nil {
		return "", fmt.Errorf("failed to read embedded skill content: %w", err)
	}
	return string(b), nil
}
