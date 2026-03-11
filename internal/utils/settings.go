package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

type Profile string

const (
	ProfileMinimal  Profile = "minimal"
	ProfileStandard Profile = "standard"
	ProfileFull     Profile = "full"
)

// Settings stores persisted tool filtering preferences.
type Settings struct {
	Profile       Profile  `json:"profile"`
	DisabledTools []string `json:"disabled_tools"`
	UpdatedAt     string   `json:"updated_at,omitempty"`
}

// SettingsManager loads and saves settings.json.
type SettingsManager struct {
	path     string
	settings Settings
}

// NewSettingsManager creates a settings manager rooted at configDir.
func NewSettingsManager(configDir string) *SettingsManager {
	m := &SettingsManager{
		path: filepath.Join(configDir, "settings.json"),
		settings: Settings{
			Profile:       ProfileFull,
			DisabledTools: []string{},
		},
	}
	m.load()
	m.applyEnvOverrides()
	return m
}

func (m *SettingsManager) load() {
	data, err := os.ReadFile(m.path)
	if err != nil {
		return
	}
	var s Settings
	if json.Unmarshal(data, &s) != nil {
		return
	}
	if s.Profile == "" {
		s.Profile = ProfileFull
	}
	if s.DisabledTools == nil {
		s.DisabledTools = []string{}
	}
	m.settings = s
}

func (m *SettingsManager) applyEnvOverrides() {
	if v := strings.TrimSpace(os.Getenv("NOTEBOOKLM_PROFILE")); v != "" {
		m.settings.Profile = Profile(v)
	}
	if v := strings.TrimSpace(os.Getenv("NOTEBOOKLM_DISABLED_TOOLS")); v != "" {
		m.settings.DisabledTools = splitCSV(v)
	}
}

func (m *SettingsManager) GetSettings() Settings {
	return Settings{
		Profile:       m.settings.Profile,
		DisabledTools: append([]string(nil), m.settings.DisabledTools...),
		UpdatedAt:     m.settings.UpdatedAt,
	}
}

func (m *SettingsManager) SetProfile(profile Profile) {
	m.settings.Profile = profile
}

func (m *SettingsManager) SetDisabledTools(tools []string) {
	m.settings.DisabledTools = normalizeTools(tools)
}

func (m *SettingsManager) Reset() {
	m.settings = Settings{
		Profile:       ProfileFull,
		DisabledTools: []string{},
	}
}

func (m *SettingsManager) Save() error {
	m.settings.DisabledTools = normalizeTools(m.settings.DisabledTools)
	m.settings.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m.settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.path, data, 0o644)
}

func (m *SettingsManager) FilterTools(all []string) []string {
	allowed := profileTools(m.settings.Profile, all)
	if len(m.settings.DisabledTools) == 0 {
		return allowed
	}

	disabled := make(map[string]struct{}, len(m.settings.DisabledTools))
	for _, name := range m.settings.DisabledTools {
		disabled[name] = struct{}{}
	}

	filtered := make([]string, 0, len(allowed))
	for _, name := range allowed {
		if _, ok := disabled[name]; !ok {
			filtered = append(filtered, name)
		}
	}
	return filtered
}

// FormatDuration returns a readable duration string for logs.
func FormatDuration(d time.Duration) string {
	return d.Round(time.Millisecond).String()
}

func profileTools(profile Profile, all []string) []string {
	switch profile {
	case ProfileMinimal:
		return intersectOrdered(all, []string{"ask_question", "get_health", "setup_auth"})
	case ProfileStandard:
		return intersectOrdered(all, []string{
			"ask_question", "get_health", "setup_auth", "re_auth",
			"list_sessions", "close_session", "list_notebooks",
			"select_notebook", "add_notebook",
		})
	case ProfileFull, "":
		return append([]string(nil), all...)
	default:
		return append([]string(nil), all...)
	}
}

func intersectOrdered(all, subset []string) []string {
	result := make([]string, 0, len(subset))
	for _, name := range all {
		if slices.Contains(subset, name) {
			result = append(result, name)
		}
	}
	return result
}

func normalizeTools(tools []string) []string {
	seen := make(map[string]struct{}, len(tools))
	result := make([]string, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result
}

func splitCSV(v string) []string {
	return normalizeTools(strings.Split(v, ","))
}
