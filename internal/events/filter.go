package events

import (
	"strings"
)

// FilterConfig defines event filtering rules
type FilterConfig struct {
	Projects []string // Empty = allow all
	Exclude  []string // Exclude these projects
}

// Filter filters Gerrit events based on configuration
type Filter struct {
	config FilterConfig
}

// NewFilter creates a new event filter
func NewFilter(config FilterConfig) *Filter {
	return &Filter{config: config}
}

// ShouldProcess returns true if the event should be processed
func (f *Filter) ShouldProcess(event Event) bool {
	// Only patchset-created events
	if event.Type != "patchset-created" {
		return false
	}

	if event.Change == nil {
		return false
	}

	project := event.Change.Project

	// Check exclude list
	for _, excl := range f.config.Exclude {
		if strings.TrimSpace(excl) == project {
			return false
		}
	}

	// If no whitelist, allow all (except excluded)
	if len(f.config.Projects) == 0 {
		return true
	}

	// Check whitelist
	for _, allowed := range f.config.Projects {
		if strings.TrimSpace(allowed) == project {
			return true
		}
	}

	return false
}
