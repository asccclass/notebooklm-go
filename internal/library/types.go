package library

import "time"

// Library is the on-disk notebook library structure.
type Library struct {
	Notebooks        []*NotebookEntry `json:"notebooks"`
	ActiveNotebookID *string          `json:"active_notebook_id,omitempty"`
	LastModified     time.Time        `json:"last_modified"`
	Version          string           `json:"version"`
}

// NotebookEntry stores notebook metadata and usage counters.
type NotebookEntry struct {
	ID           string    `json:"id"`
	URL          string    `json:"url"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Topics       []string  `json:"topics"`
	ContentTypes []string  `json:"content_types"`
	UseCases     []string  `json:"use_cases"`
	AddedAt      time.Time `json:"added_at"`
	LastUsed     time.Time `json:"last_used"`
	UseCount     int       `json:"use_count"`
	Tags         []string  `json:"tags"`
}

// AddNotebookInput is the payload for adding a notebook.
type AddNotebookInput struct {
	URL          string   `json:"url"`
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	Topics       []string `json:"topics"`
	ContentTypes []string `json:"content_types"`
	UseCases     []string `json:"use_cases"`
	Tags         []string `json:"tags"`
}

// UpdateNotebookInput is the payload for updating notebook metadata.
type UpdateNotebookInput struct {
	ID           string   `json:"id"`
	Name         *string  `json:"name,omitempty"`
	Description  *string  `json:"description,omitempty"`
	Topics       []string `json:"topics,omitempty"`
	ContentTypes []string `json:"content_types,omitempty"`
	UseCases     []string `json:"use_cases,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	URL          *string  `json:"url,omitempty"`
}

// LibraryStats is a summary of library usage.
type LibraryStats struct {
	TotalNotebooks   int     `json:"total_notebooks"`
	ActiveNotebook   *string `json:"active_notebook,omitempty"`
	MostUsedNotebook *string `json:"most_used_notebook,omitempty"`
	TotalQueries     int     `json:"total_queries"`
	LastModified     string  `json:"last_modified"`
}
