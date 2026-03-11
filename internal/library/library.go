package library

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/asccclass/notebooklm-go/internal/config"
)

var nonAlNum = regexp.MustCompile(`[^a-z0-9]+`)

// Manager handles persistent notebook library operations.
type Manager struct {
	mu          sync.RWMutex
	libraryPath string
	library     Library
	cfg         *config.Config
}

// New creates a Manager and loads the library from disk.
func New(cfg *config.Config) (*Manager, error) {
	m := &Manager{
		libraryPath: filepath.Join(cfg.DataDir, "library.json"),
		cfg:         cfg,
	}
	if err := m.load(); err != nil {
		return nil, err
	}
	slog.Info("📚 NotebookLibrary initialised",
		"path", m.libraryPath,
		"notebooks", len(m.library.Notebooks))
	return m, nil
}

// ─── private helpers ────────────────────────────────────────────────────────

func (m *Manager) load() error {
	data, err := os.ReadFile(m.libraryPath)
	if err == nil {
		var lib Library
		if jsonErr := json.Unmarshal(data, &lib); jsonErr == nil {
			m.library = lib
			slog.Info("Loaded library", "notebooks", len(lib.Notebooks))
			return nil
		}
	}
	// Fresh library
	slog.Info("Creating new library")
	m.library = m.createDefault()
	return m.save()
}

func (m *Manager) createDefault() Library {
	lib := Library{
		Notebooks:    []*NotebookEntry{},
		LastModified: time.Now(),
		Version:      "1.0.0",
	}
	if m.cfg.NotebookURL != "" && m.cfg.NotebookDescription != "" &&
		m.cfg.NotebookDescription != "General knowledge base" {
		id := generateID(m.cfg.NotebookDescription, lib.Notebooks)
		name := m.cfg.NotebookDescription
		if len(name) > 50 {
			name = name[:50]
		}
		nb := &NotebookEntry{
			ID:           id,
			URL:          m.cfg.NotebookURL,
			Name:         name,
			Description:  m.cfg.NotebookDescription,
			Topics:       m.cfg.NotebookTopics,
			ContentTypes: m.cfg.NotebookContentTypes,
			UseCases:     m.cfg.NotebookUseCases,
			AddedAt:      time.Now(),
			LastUsed:     time.Now(),
			Tags:         []string{},
		}
		lib.Notebooks = append(lib.Notebooks, nb)
		lib.ActiveNotebookID = &id
	}
	return lib
}

func (m *Manager) save() error {
	m.library.LastModified = time.Now()
	data, err := json.MarshalIndent(m.library, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal library: %w", err)
	}
	if err := os.WriteFile(m.libraryPath, data, 0o644); err != nil {
		return fmt.Errorf("write library: %w", err)
	}
	slog.Info("💾 Library saved", "notebooks", len(m.library.Notebooks))
	return nil
}

func generateID(name string, existing []*NotebookEntry) string {
	base := nonAlNum.ReplaceAllString(strings.ToLower(name), "-")
	base = strings.Trim(base, "-")
	if len(base) > 30 {
		base = base[:30]
	}
	id := base
	counter := 1
	for {
		found := false
		for _, nb := range existing {
			if nb.ID == id {
				found = true
				break
			}
		}
		if !found {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, counter)
		counter++
	}
}

// ─── public API ─────────────────────────────────────────────────────────────

// AddNotebook adds a new notebook and persists the library.
func (m *Manager) AddNotebook(in AddNotebookInput) (*NotebookEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := generateID(in.Name, m.library.Notebooks)
	contentTypes := in.ContentTypes
	if len(contentTypes) == 0 {
		contentTypes = []string{"documentation", "examples"}
	}
	useCases := in.UseCases
	if len(useCases) == 0 {
		useCases = []string{
			fmt.Sprintf("Learning about %s", in.Name),
			fmt.Sprintf("Implementing features with %s", in.Name),
		}
	}
	tags := in.Tags
	if tags == nil {
		tags = []string{}
	}

	nb := &NotebookEntry{
		ID:           id,
		URL:          in.URL,
		Name:         in.Name,
		Description:  in.Description,
		Topics:       in.Topics,
		ContentTypes: contentTypes,
		UseCases:     useCases,
		AddedAt:      time.Now(),
		LastUsed:     time.Now(),
		Tags:         tags,
	}
	m.library.Notebooks = append(m.library.Notebooks, nb)
	if len(m.library.Notebooks) == 1 {
		m.library.ActiveNotebookID = &id
	}
	if err := m.save(); err != nil {
		return nil, err
	}
	slog.Info("✅ Notebook added", "id", id)
	return nb, nil
}

// ListNotebooks returns all notebooks.
func (m *Manager) ListNotebooks() []*NotebookEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*NotebookEntry, len(m.library.Notebooks))
	copy(out, m.library.Notebooks)
	return out
}

// GetNotebook returns a notebook by ID, or nil.
func (m *Manager) GetNotebook(id string) *NotebookEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.findByID(id)
}

func (m *Manager) findByID(id string) *NotebookEntry {
	for _, nb := range m.library.Notebooks {
		if nb.ID == id {
			return nb
		}
	}
	return nil
}

// GetActiveNotebook returns the currently active notebook, or nil.
func (m *Manager) GetActiveNotebook() *NotebookEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.library.ActiveNotebookID == nil {
		return nil
	}
	return m.findByID(*m.library.ActiveNotebookID)
}

// SelectNotebook sets the active notebook.
func (m *Manager) SelectNotebook(id string) (*NotebookEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	nb := m.findByID(id)
	if nb == nil {
		return nil, fmt.Errorf("notebook not found: %s", id)
	}
	m.library.ActiveNotebookID = &id
	nb.LastUsed = time.Now()
	if err := m.save(); err != nil {
		return nil, err
	}
	return nb, nil
}

// UpdateNotebook updates fields of an existing notebook.
func (m *Manager) UpdateNotebook(in UpdateNotebookInput) (*NotebookEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	nb := m.findByID(in.ID)
	if nb == nil {
		return nil, fmt.Errorf("notebook not found: %s", in.ID)
	}
	if in.Name != nil {
		nb.Name = *in.Name
	}
	if in.Description != nil {
		nb.Description = *in.Description
	}
	if in.Topics != nil {
		nb.Topics = in.Topics
	}
	if in.ContentTypes != nil {
		nb.ContentTypes = in.ContentTypes
	}
	if in.UseCases != nil {
		nb.UseCases = in.UseCases
	}
	if in.Tags != nil {
		nb.Tags = in.Tags
	}
	if in.URL != nil {
		nb.URL = *in.URL
	}
	return nb, m.save()
}

// RemoveNotebook removes a notebook and returns true if found.
func (m *Manager) RemoveNotebook(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	newList := make([]*NotebookEntry, 0, len(m.library.Notebooks))
	found := false
	for _, nb := range m.library.Notebooks {
		if nb.ID == id {
			found = true
			continue
		}
		newList = append(newList, nb)
	}
	if !found {
		return false
	}
	m.library.Notebooks = newList
	if m.library.ActiveNotebookID != nil && *m.library.ActiveNotebookID == id {
		if len(newList) > 0 {
			m.library.ActiveNotebookID = &newList[0].ID
		} else {
			m.library.ActiveNotebookID = nil
		}
	}
	_ = m.save()
	return true
}

// IncrementUseCount increments the use counter and updates last_used.
func (m *Manager) IncrementUseCount(id string) *NotebookEntry {
	m.mu.Lock()
	defer m.mu.Unlock()
	nb := m.findByID(id)
	if nb == nil {
		return nil
	}
	nb.UseCount++
	nb.LastUsed = time.Now()
	_ = m.save()
	return nb
}

// SearchNotebooks returns notebooks matching query (name, description, topics, tags).
func (m *Manager) SearchNotebooks(query string) []*NotebookEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()
	q := strings.ToLower(query)
	var results []*NotebookEntry
	for _, nb := range m.library.Notebooks {
		if strings.Contains(strings.ToLower(nb.Name), q) ||
			strings.Contains(strings.ToLower(nb.Description), q) ||
			sliceContains(nb.Topics, q) ||
			sliceContains(nb.Tags, q) {
			results = append(results, nb)
		}
	}
	return results
}

func sliceContains(slice []string, sub string) bool {
	for _, s := range slice {
		if strings.Contains(strings.ToLower(s), sub) {
			return true
		}
	}
	return false
}

// GetStats returns library statistics.
func (m *Manager) GetStats() LibraryStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := 0
	var mostUsed *string
	maxUse := -1
	for _, nb := range m.library.Notebooks {
		total += nb.UseCount
		if nb.UseCount > maxUse {
			maxUse = nb.UseCount
			id := nb.ID
			mostUsed = &id
		}
	}
	return LibraryStats{
		TotalNotebooks:   len(m.library.Notebooks),
		ActiveNotebook:   m.library.ActiveNotebookID,
		MostUsedNotebook: mostUsed,
		TotalQueries:     total,
		LastModified:     m.library.LastModified.Format(time.RFC3339),
	}
}

// LibraryPath returns the path to the library file.
func (m *Manager) LibraryPath() string {
	return m.libraryPath
}
