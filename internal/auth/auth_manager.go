// Package auth handles Google authentication for NotebookLM.
package auth

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/asccclass/notebooklm-go/internal/config"
)

// Critical Google cookie names required for NotebookLM authentication.
var criticalCookies = []string{
	"SID", "HSID", "SSID",
	"APISID", "SAPISID",
	"OSID", "__Secure-OSID",
	"__Secure-1PSID", "__Secure-3PSID",
}

// BrowserState holds the saved cookies/storage.
type BrowserState struct {
	Cookies []Cookie        `json:"cookies"`
	Origins []OriginStorage `json:"origins"`
}

// Cookie represents a browser cookie.
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite,omitempty"`
}

// OriginStorage represents localStorage/sessionStorage for one origin.
type OriginStorage struct {
	Origin  string         `json:"origin"`
	Storage []StorageEntry `json:"localStorage,omitempty"`
}

// StorageEntry is a single key-value pair.
type StorageEntry struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Manager handles browser state persistence for authentication.
type Manager struct {
	stateFilePath   string
	sessionFilePath string
}

// New creates an AuthManager.
func New(cfg *config.Config) *Manager {
	return &Manager{
		stateFilePath:   filepath.Join(cfg.BrowserStateDir, "state.json"),
		sessionFilePath: filepath.Join(cfg.BrowserStateDir, "session.json"),
	}
}

// HasSavedState returns true if a browser state file exists.
func (m *Manager) HasSavedState() bool {
	_, err := os.Stat(m.stateFilePath)
	return err == nil
}

// GetStatePath returns the state file path or "" if missing.
func (m *Manager) GetStatePath() string {
	if m.HasSavedState() {
		return m.stateFilePath
	}
	return ""
}

// IsStateExpired returns true if the state file is older than 24 hours.
func (m *Manager) IsStateExpired() bool {
	info, err := os.Stat(m.stateFilePath)
	if err != nil {
		return true
	}
	return time.Since(info.ModTime()) > 24*time.Hour
}

// GetValidStatePath returns the state path only if present and not expired.
func (m *Manager) GetValidStatePath() string {
	if !m.HasSavedState() {
		return ""
	}
	if m.IsStateExpired() {
		slog.Warn("Saved state is expired (>24h old)")
		return ""
	}
	return m.stateFilePath
}

// LoadState reads saved browser state from disk.
func (m *Manager) LoadState() (*BrowserState, error) {
	data, err := os.ReadFile(m.stateFilePath)
	if err != nil {
		return nil, fmt.Errorf("read state: %w", err)
	}
	var state BrowserState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("parse state: %w", err)
	}
	return &state, nil
}

// SaveState writes browser state to disk.
func (m *Manager) SaveState(state *BrowserState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	if err := os.WriteFile(m.stateFilePath, data, 0o600); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	slog.Info("✅ Browser state saved")
	return nil
}

// LoadSessionStorage reads saved sessionStorage data.
func (m *Manager) LoadSessionStorage() (map[string]string, error) {
	data, err := os.ReadFile(m.sessionFilePath)
	if err != nil {
		return nil, nil // not found is fine
	}
	var result map[string]string
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, nil
	}
	return result, nil
}

// SaveSessionStorage persists sessionStorage data.
func (m *Manager) SaveSessionStorage(data map[string]string) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.sessionFilePath, b, 0o600)
}

// ClearAllAuthData removes all authentication files.
func (m *Manager) ClearAllAuthData() error {
	for _, path := range []string{m.stateFilePath, m.sessionFilePath} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}
	slog.Info("✅ Auth data cleared")
	return nil
}

// HasCriticalCookies checks whether the state contains all critical Google cookies.
func (m *Manager) HasCriticalCookies(state *BrowserState) bool {
	if state == nil {
		return false
	}
	found := make(map[string]bool)
	for _, c := range state.Cookies {
		for _, name := range criticalCookies {
			if c.Name == name && c.Value != "" {
				found[name] = true
			}
		}
	}
	// Require at least SID or __Secure-1PSID
	return found["SID"] || found["__Secure-1PSID"] || found["OSID"] || found["__Secure-OSID"]
}

// StateFilePath exposes the state file path (for browser to load).
func (m *Manager) StateFilePath() string {
	return m.stateFilePath
}
