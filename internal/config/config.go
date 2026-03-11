// Package config provides configuration for the NotebookLM MCP Server.
// Priority: Hardcoded Defaults → Environment Variables → Tool Parameters (at runtime)
package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

const (
	NotebookLMAuthURL = "https://accounts.google.com/v3/signin/identifier?continue=https%3A%2F%2Fnotebooklm.google.com%2F&flowName=GlifWebSignIn&flowEntry=ServiceLogin"
)

// Viewport defines browser window dimensions.
type Viewport struct {
	Width  int
	Height int
}

// StealthOptions controls human-like browser behavior.
type StealthOptions struct {
	Enabled        bool
	RandomDelays   bool
	HumanTyping    bool
	MouseMovements bool
	TypingWPMMin   int
	TypingWPMMax   int
	MinDelayMs     int
	MaxDelayMs     int
}

// Config is the global server configuration.
type Config struct {
	// NotebookLM
	NotebookURL string

	// Browser
	Headless       bool
	BrowserTimeout int // ms
	Viewport       Viewport

	// Sessions
	MaxSessions    int
	SessionTimeout int // seconds

	// Auth
	AutoLoginEnabled   bool
	LoginEmail         string
	LoginPassword      string
	AutoLoginTimeoutMs int

	// Stealth
	Stealth StealthOptions

	// Paths (cross-platform)
	ConfigDir         string
	DataDir           string
	BrowserStateDir   string
	ChromeProfileDir  string
	ChromeInstanceDir string

	// Library defaults
	NotebookDescription  string
	NotebookTopics       []string
	NotebookContentTypes []string
	NotebookUseCases     []string

	// Multi-instance
	ProfileStrategy           string // "auto"|"single"|"isolated"
	CloneProfileOnIsolated    bool
	CleanupInstancesOnStartup bool
	CleanupInstancesOnShutdown bool
	InstanceProfileTTLHours   int
	InstanceProfileMaxCount   int
}

// dataDirs returns the platform-appropriate data/config directories.
func dataDirs(appName string) (dataDir, configDir string) {
	switch runtime.GOOS {
	case "windows":
		base := os.Getenv("APPDATA")
		if base == "" {
			base = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
		dataDir = filepath.Join(base, appName)
		configDir = dataDir
	case "darwin":
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, "Library", "Application Support", appName)
		configDir = dataDir
	default: // Linux / BSD
		home, _ := os.UserHomeDir()
		xdgData := os.Getenv("XDG_DATA_HOME")
		if xdgData == "" {
			xdgData = filepath.Join(home, ".local", "share")
		}
		xdgConf := os.Getenv("XDG_CONFIG_HOME")
		if xdgConf == "" {
			xdgConf = filepath.Join(home, ".config")
		}
		dataDir = filepath.Join(xdgData, appName)
		configDir = filepath.Join(xdgConf, appName)
	}
	return
}

// defaults returns the baseline configuration.
func defaults() Config {
	dataDir, configDir := dataDirs("notebooklm-mcp")
	return Config{
		NotebookURL:    "",
		Headless:       true,
		BrowserTimeout: 30000,
		Viewport:       Viewport{Width: 1024, Height: 768},

		MaxSessions:    10,
		SessionTimeout: 900,

		AutoLoginEnabled:   false,
		LoginEmail:         "",
		LoginPassword:      "",
		AutoLoginTimeoutMs: 120000,

		Stealth: StealthOptions{
			Enabled:        true,
			RandomDelays:   true,
			HumanTyping:    true,
			MouseMovements: true,
			TypingWPMMin:   160,
			TypingWPMMax:   240,
			MinDelayMs:     100,
			MaxDelayMs:     400,
		},

		ConfigDir:         configDir,
		DataDir:           dataDir,
		BrowserStateDir:   filepath.Join(dataDir, "browser_state"),
		ChromeProfileDir:  filepath.Join(dataDir, "chrome_profile"),
		ChromeInstanceDir: filepath.Join(dataDir, "chrome_profile_instances"),

		NotebookDescription:  "General knowledge base",
		NotebookTopics:       []string{"General topics"},
		NotebookContentTypes: []string{"documentation", "examples"},
		NotebookUseCases:     []string{"General research"},

		ProfileStrategy:            "auto",
		CloneProfileOnIsolated:     false,
		CleanupInstancesOnStartup:  true,
		CleanupInstancesOnShutdown: true,
		InstanceProfileTTLHours:    72,
		InstanceProfileMaxCount:    20,
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(os.Getenv(key))
	switch v {
	case "true", "1":
		return true
	case "false", "0":
		return false
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}

func envSlice(key string, fallback []string) []string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			result = append(result, s)
		}
	}
	return result
}

// Load builds the final configuration from defaults + environment variables.
func Load() Config {
	cfg := defaults()

	cfg.NotebookURL = envStr("NOTEBOOK_URL", cfg.NotebookURL)
	cfg.Headless = envBool("HEADLESS", cfg.Headless)
	cfg.BrowserTimeout = envInt("BROWSER_TIMEOUT", cfg.BrowserTimeout)
	cfg.MaxSessions = envInt("MAX_SESSIONS", cfg.MaxSessions)
	cfg.SessionTimeout = envInt("SESSION_TIMEOUT", cfg.SessionTimeout)
	cfg.AutoLoginEnabled = envBool("AUTO_LOGIN_ENABLED", cfg.AutoLoginEnabled)
	cfg.LoginEmail = envStr("LOGIN_EMAIL", cfg.LoginEmail)
	cfg.LoginPassword = envStr("LOGIN_PASSWORD", cfg.LoginPassword)
	cfg.AutoLoginTimeoutMs = envInt("AUTO_LOGIN_TIMEOUT_MS", cfg.AutoLoginTimeoutMs)

	cfg.Stealth.Enabled = envBool("STEALTH_ENABLED", cfg.Stealth.Enabled)
	cfg.Stealth.RandomDelays = envBool("STEALTH_RANDOM_DELAYS", cfg.Stealth.RandomDelays)
	cfg.Stealth.HumanTyping = envBool("STEALTH_HUMAN_TYPING", cfg.Stealth.HumanTyping)
	cfg.Stealth.MouseMovements = envBool("STEALTH_MOUSE_MOVEMENTS", cfg.Stealth.MouseMovements)
	cfg.Stealth.TypingWPMMin = envInt("TYPING_WPM_MIN", cfg.Stealth.TypingWPMMin)
	cfg.Stealth.TypingWPMMax = envInt("TYPING_WPM_MAX", cfg.Stealth.TypingWPMMax)
	cfg.Stealth.MinDelayMs = envInt("MIN_DELAY_MS", cfg.Stealth.MinDelayMs)
	cfg.Stealth.MaxDelayMs = envInt("MAX_DELAY_MS", cfg.Stealth.MaxDelayMs)

	cfg.NotebookDescription = envStr("NOTEBOOK_DESCRIPTION", cfg.NotebookDescription)
	cfg.NotebookTopics = envSlice("NOTEBOOK_TOPICS", cfg.NotebookTopics)
	cfg.NotebookContentTypes = envSlice("NOTEBOOK_CONTENT_TYPES", cfg.NotebookContentTypes)
	cfg.NotebookUseCases = envSlice("NOTEBOOK_USE_CASES", cfg.NotebookUseCases)

	if s := os.Getenv("NOTEBOOK_PROFILE_STRATEGY"); s != "" {
		cfg.ProfileStrategy = s
	}
	cfg.CloneProfileOnIsolated = envBool("NOTEBOOK_CLONE_PROFILE", cfg.CloneProfileOnIsolated)
	cfg.CleanupInstancesOnStartup = envBool("NOTEBOOK_CLEANUP_ON_STARTUP", cfg.CleanupInstancesOnStartup)
	cfg.CleanupInstancesOnShutdown = envBool("NOTEBOOK_CLEANUP_ON_SHUTDOWN", cfg.CleanupInstancesOnShutdown)
	cfg.InstanceProfileTTLHours = envInt("NOTEBOOK_INSTANCE_TTL_HOURS", cfg.InstanceProfileTTLHours)
	cfg.InstanceProfileMaxCount = envInt("NOTEBOOK_INSTANCE_MAX_COUNT", cfg.InstanceProfileMaxCount)

	// Ensure required directories exist
	for _, dir := range []string{cfg.DataDir, cfg.BrowserStateDir, cfg.ChromeProfileDir, cfg.ChromeInstanceDir} {
		_ = os.MkdirAll(dir, 0o755)
	}

	return cfg
}

// BrowserOptions can be passed per-tool to override browser behavior.
type BrowserOptions struct {
	Show       *bool
	Headless   *bool
	TimeoutMs  *int
	Stealth    *StealthOverride
	Viewport   *Viewport
}

// StealthOverride carries per-call stealth overrides.
type StealthOverride struct {
	Enabled        *bool
	RandomDelays   *bool
	HumanTyping    *bool
	MouseMovements *bool
	TypingWPMMin   *int
	TypingWPMMax   *int
	DelayMinMs     *int
	DelayMaxMs     *int
}

// Apply returns a shallow copy of cfg with BrowserOptions applied.
func (cfg *Config) Apply(opts *BrowserOptions, legacyShowBrowser *bool) Config {
	out := *cfg
	if legacyShowBrowser != nil {
		out.Headless = !*legacyShowBrowser
	}
	if opts == nil {
		return out
	}
	if opts.Show != nil {
		out.Headless = !*opts.Show
	}
	if opts.Headless != nil {
		out.Headless = *opts.Headless
	}
	if opts.TimeoutMs != nil {
		out.BrowserTimeout = *opts.TimeoutMs
	}
	if opts.Viewport != nil {
		if opts.Viewport.Width > 0 {
			out.Viewport.Width = opts.Viewport.Width
		}
		if opts.Viewport.Height > 0 {
			out.Viewport.Height = opts.Viewport.Height
		}
	}
	if s := opts.Stealth; s != nil {
		st := out.Stealth
		if s.Enabled != nil {
			st.Enabled = *s.Enabled
		}
		if s.RandomDelays != nil {
			st.RandomDelays = *s.RandomDelays
		}
		if s.HumanTyping != nil {
			st.HumanTyping = *s.HumanTyping
		}
		if s.MouseMovements != nil {
			st.MouseMovements = *s.MouseMovements
		}
		if s.TypingWPMMin != nil {
			st.TypingWPMMin = *s.TypingWPMMin
		}
		if s.TypingWPMMax != nil {
			st.TypingWPMMax = *s.TypingWPMMax
		}
		if s.DelayMinMs != nil {
			st.MinDelayMs = *s.DelayMinMs
		}
		if s.DelayMaxMs != nil {
			st.MaxDelayMs = *s.DelayMaxMs
		}
		out.Stealth = st
	}
	return out
}
