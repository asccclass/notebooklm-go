// Package session manages browser sessions for NotebookLM.
package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/asccclass/notebooklm-go/internal/auth"
	"github.com/asccclass/notebooklm-go/internal/config"
)

// ErrRateLimit is returned when NotebookLM daily query limit is hit.
var ErrRateLimit = errors.New("notebooklm rate limit reached (50 queries/day for free accounts)")

// Info is a snapshot of a session's runtime state.
type Info struct {
	ID              string `json:"id"`
	CreatedAt       int64  `json:"created_at"`
	LastActivity    int64  `json:"last_activity"`
	AgeSeconds      int64  `json:"age_seconds"`
	InactiveSeconds int64  `json:"inactive_seconds"`
	MessageCount    int    `json:"message_count"`
	NotebookURL     string `json:"notebook_url"`
}

// BrowserSession represents one NotebookLM conversation.
type BrowserSession struct {
	SessionID   string
	NotebookURL string

	createdAt    time.Time
	lastActivity time.Time
	messageCount int

	page        *rod.Page
	cfg         config.Config
	authManager *auth.Manager
	mu          sync.Mutex
	initialized bool
}

func newSession(id string, cfg config.Config, authMgr *auth.Manager, url string) *BrowserSession {
	return &BrowserSession{
		SessionID:    id,
		NotebookURL:  url,
		createdAt:    time.Now(),
		lastActivity: time.Now(),
		cfg:          cfg,
		authManager:  authMgr,
	}
}

// Init navigates a page to the notebook and waits for the interface.
func (s *BrowserSession) Init(browser *rod.Browser) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.initialized {
		return nil
	}
	slog.Info("🚀 Initialising session", "id", s.SessionID)

	page, err := browser.Page(proto.TargetCreateTarget{URL: s.NotebookURL})
	if err != nil {
		return fmt.Errorf("open page: %w", err)
	}
	s.page = page
	_ = page.WaitLoad()
	time.Sleep(2*time.Second + time.Duration(randInt(1000))*time.Millisecond)

	if err := s.waitForInterface(); err != nil {
		_ = page.Close()
		s.page = nil
		return err
	}

	if sessData, _ := s.authManager.LoadSessionStorage(); len(sessData) > 0 {
		s.restoreSessionStorage(sessData)
	}

	s.initialized = true
	s.lastActivity = time.Now()
	slog.Info("✅ Session initialised", "id", s.SessionID)
	return nil
}

func (s *BrowserSession) waitForInterface() error {
	for _, sel := range []string{
		"textarea.query-box-input",
		`textarea[aria-label="Feld für Anfragen"]`,
		"textarea",
	} {
		el, err := s.page.Timeout(10 * time.Second).Element(sel)
		if err == nil && el != nil {
			return nil
		}
	}
	return errors.New("NotebookLM chat input not found – ensure the notebook is publicly accessible")
}

func (s *BrowserSession) restoreSessionStorage(data map[string]string) {
	js := `(d) => { for(const [k,v] of Object.entries(d)){try{sessionStorage.setItem(k,v)}catch(_){}} }`
	if _, err := s.page.Eval(js, data); err != nil {
		slog.Warn("restoreSessionStorage failed", "err", err)
	}
}

// Ask types a question and waits for the full answer.
func (s *BrowserSession) Ask(question string, progress func(string)) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.initialized || s.page == nil {
		return "", errors.New("session not initialised")
	}

	prevCount := s.countResponses()

	ta, err := s.findTextarea()
	if err != nil {
		return "", err
	}

	if progress != nil {
		progress("Typing question...")
	}
	_ = ta.Click(proto.InputMouseButtonLeft, 1)
	time.Sleep(200 * time.Millisecond)
	if err := s.humanType(ta, question); err != nil {
		return "", err
	}
	time.Sleep(300 * time.Millisecond)
	if err := ta.Type(input.Enter); err != nil {
		return "", fmt.Errorf("submit: %w", err)
	}

	if progress != nil {
		progress("Waiting for answer...")
	}
	answer, err := s.waitForAnswer(prevCount)
	if err != nil {
		return "", err
	}

	lower := strings.ToLower(answer)
	if strings.Contains(lower, "you've reached your daily limit") ||
		strings.Contains(lower, "daily query limit") ||
		strings.Contains(lower, "try again tomorrow") {
		return "", ErrRateLimit
	}

	s.messageCount++
	s.lastActivity = time.Now()
	return answer, nil
}

func (s *BrowserSession) findTextarea() (*rod.Element, error) {
	for _, sel := range []string{"textarea.query-box-input", `textarea[aria-label="Feld für Anfragen"]`, "textarea"} {
		el, err := s.page.Timeout(5 * time.Second).Element(sel)
		if err == nil && el != nil {
			return el, nil
		}
	}
	return nil, errors.New("textarea not found")
}

func (s *BrowserSession) humanType(el *rod.Element, text string) error {
	if !s.cfg.Stealth.HumanTyping {
		return el.Input(text)
	}
	wpmMin, wpmMax := s.cfg.Stealth.TypingWPMMin, s.cfg.Stealth.TypingWPMMax
	if wpmMin <= 0 {
		wpmMin = 160
	}
	if wpmMax <= wpmMin {
		wpmMax = wpmMin + 80
	}
	wpm := wpmMin + randInt(wpmMax-wpmMin)
	msPerChar := int(math.Round(60000.0 / float64(wpm*5)))

	for _, ch := range text {
		key := input.Key(ch)
		if err := el.Type(key); err != nil {
			// ignore unsupported keys
		}
		jitter := randInt(msPerChar/2) - msPerChar/4
		delay := msPerChar + jitter
		if delay < 10 {
			delay = 10
		}
		time.Sleep(time.Duration(delay) * time.Millisecond)
	}
	return nil
}

func (s *BrowserSession) countResponses() int {
	for _, sel := range []string{".response-container", "[data-test-id='chat-response']", ".model-response", "message-content"} {
		els, err := s.page.Elements(sel)
		if err == nil {
			return len(els)
		}
	}
	return 0
}

func (s *BrowserSession) getLatestResponse() string {
	for _, sel := range []string{".response-container", "[data-test-id='chat-response']", ".model-response", "message-content"} {
		els, err := s.page.Elements(sel)
		if err == nil && len(els) > 0 {
			text, _ := els[len(els)-1].Text()
			return text
		}
	}
	return ""
}

func (s *BrowserSession) isLoading() bool {
	for _, sel := range []string{".loading-indicator", "[aria-label='Loading']", ".spinner", "[data-loading='true']"} {
		el, err := s.page.Timeout(200 * time.Millisecond).Element(sel)
		if err == nil && el != nil {
			return true
		}
	}
	return false
}

func (s *BrowserSession) waitForAnswer(prevCount int) (string, error) {
	deadline := time.Now().Add(time.Duration(s.cfg.BrowserTimeout) * time.Millisecond)
	var lastText string
	stableCount := 0
	for time.Now().Before(deadline) {
		time.Sleep(1500 * time.Millisecond)
		if s.countResponses() <= prevCount || s.isLoading() {
			stableCount = 0
			continue
		}
		newest := s.getLatestResponse()
		if newest == lastText && strings.TrimSpace(newest) != "" {
			stableCount++
		} else {
			stableCount = 0
			lastText = newest
		}
		if stableCount >= 2 {
			return strings.TrimSpace(newest), nil
		}
	}
	if lastText != "" {
		return strings.TrimSpace(lastText), nil
	}
	return "", fmt.Errorf("timeout (%dms) waiting for answer", s.cfg.BrowserTimeout)
}

// Reset reloads the notebook page to clear conversation history.
func (s *BrowserSession) Reset() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.page == nil {
		return errors.New("session not initialised")
	}
	if err := s.page.Navigate(s.NotebookURL); err != nil {
		return err
	}
	_ = s.page.WaitLoad()
	if err := s.waitForInterface(); err != nil {
		return err
	}
	s.messageCount = 0
	s.lastActivity = time.Now()
	return nil
}

// Close closes the underlying browser page.
func (s *BrowserSession) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.page != nil {
		_ = s.page.Close()
		s.page = nil
	}
	s.initialized = false
}

// GetInfo returns session metadata.
func (s *BrowserSession) GetInfo() Info {
	now := time.Now()
	return Info{
		ID:              s.SessionID,
		CreatedAt:       s.createdAt.Unix(),
		LastActivity:    s.lastActivity.Unix(),
		AgeSeconds:      int64(now.Sub(s.createdAt).Seconds()),
		InactiveSeconds: int64(now.Sub(s.lastActivity).Seconds()),
		MessageCount:    s.messageCount,
		NotebookURL:     s.NotebookURL,
	}
}

// IsExpired returns true when the session has been idle longer than timeoutSec.
func (s *BrowserSession) IsExpired(timeoutSec int) bool {
	return time.Since(s.lastActivity) > time.Duration(timeoutSec)*time.Second
}

// ─── Manager ─────────────────────────────────────────────────────────────────

// Stats holds aggregate session statistics.
type Stats struct {
	ActiveSessions       int `json:"active_sessions"`
	MaxSessions          int `json:"max_sessions"`
	SessionTimeout       int `json:"session_timeout"`
	OldestSessionSeconds int `json:"oldest_session_seconds"`
	TotalMessages        int `json:"total_messages"`
}

// Manager owns the shared browser and all active sessions.
type Manager struct {
	mu          sync.Mutex
	sessions    map[string]*BrowserSession
	browser     *rod.Browser
	authManager *auth.Manager
	cfg         *config.Config
	cleanup     *time.Ticker
	done        chan struct{}
}

// NewManager creates a Manager. Call Close() when done.
func NewManager(cfg *config.Config, authMgr *auth.Manager) *Manager {
	interval := imax(60, imin(cfg.SessionTimeout/2, 300))
	m := &Manager{
		sessions:    make(map[string]*BrowserSession),
		authManager: authMgr,
		cfg:         cfg,
		done:        make(chan struct{}),
		cleanup:     time.NewTicker(time.Duration(interval) * time.Second),
	}
	go m.cleanupLoop()
	slog.Info("🎯 SessionManager ready", "max", cfg.MaxSessions, "timeout", cfg.SessionTimeout)
	return m
}

func (m *Manager) cleanupLoop() {
	for {
		select {
		case <-m.cleanup.C:
			m.cleanupInactive()
		case <-m.done:
			return
		}
	}
}

func (m *Manager) cleanupInactive() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		if s.IsExpired(m.cfg.SessionTimeout) {
			slog.Info("♻️  Expiring session", "id", id)
			s.Close()
			delete(m.sessions, id)
		}
	}
}

func (m *Manager) ensureBrowser(headless bool) (*rod.Browser, error) {
	if m.browser != nil {
		return m.browser, nil
	}
	slog.Info("🌐 Launching browser", "headless", headless)
	l := launcher.New().
		Headless(headless).
		UserDataDir(m.cfg.ChromeProfileDir).
		Set("disable-blink-features", "AutomationControlled").
		Set("no-sandbox", "").
		Set("disable-dev-shm-usage", "")
	u, err := l.Launch()
	if err != nil {
		return nil, fmt.Errorf("launch: %w", err)
	}
	m.browser = rod.New().ControlURL(u).MustConnect()
	slog.Info("✅ Browser ready")
	return m.browser, nil
}

// GetOrCreateSession returns an existing session by ID or creates a new one.
func (m *Manager) GetOrCreateSession(_ context.Context, sessionID, notebookURL string, cfg config.Config) (*BrowserSession, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if notebookURL == "" {
		return nil, errors.New("notebook URL required")
	}
	if !strings.HasPrefix(notebookURL, "http") {
		return nil, errors.New("notebook URL must start with http")
	}

	if sessionID != "" {
		if s, ok := m.sessions[sessionID]; ok {
			slog.Info("♻️  Reusing session", "id", sessionID)
			s.lastActivity = time.Now()
			return s, nil
		}
	}
	if sessionID == "" {
		sessionID = generateID()
	}

	if len(m.sessions) >= m.cfg.MaxSessions {
		m.evictOldest()
	}

	browser, err := m.ensureBrowser(cfg.Headless)
	if err != nil {
		return nil, err
	}

	s := newSession(sessionID, cfg, m.authManager, notebookURL)
	if err := s.Init(browser); err != nil {
		return nil, err
	}
	m.sessions[sessionID] = s
	return s, nil
}

func (m *Manager) evictOldest() {
	var oldest *BrowserSession
	for _, s := range m.sessions {
		if oldest == nil || s.lastActivity.Before(oldest.lastActivity) {
			oldest = s
		}
	}
	if oldest != nil {
		slog.Info("🗑️  Evicting session", "id", oldest.SessionID)
		oldest.Close()
		delete(m.sessions, oldest.SessionID)
	}
}

// GetSession returns a session by ID or nil.
func (m *Manager) GetSession(id string) *BrowserSession {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[id]
}

// CloseSession closes and removes a session by ID.
func (m *Manager) CloseSession(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[id]
	if !ok {
		return false
	}
	s.Close()
	delete(m.sessions, id)
	return true
}

// CloseAllSessions closes all active sessions.
func (m *Manager) CloseAllSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, s := range m.sessions {
		s.Close()
		delete(m.sessions, id)
	}
}

// CloseSessionsForNotebook closes all sessions for a given notebook URL.
func (m *Manager) CloseSessionsForNotebook(url string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for id, s := range m.sessions {
		if s.NotebookURL == url {
			s.Close()
			delete(m.sessions, id)
			n++
		}
	}
	return n
}

// AllSessionsInfo returns metadata for all active sessions.
func (m *Manager) AllSessionsInfo() []Info {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Info, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s.GetInfo())
	}
	return out
}

// GetStats returns aggregate statistics.
func (m *Manager) GetStats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	total, oldest := 0, 0
	for _, s := range m.sessions {
		total += s.messageCount
		if age := int(time.Since(s.createdAt).Seconds()); age > oldest {
			oldest = age
		}
	}
	return Stats{
		ActiveSessions:       len(m.sessions),
		MaxSessions:          m.cfg.MaxSessions,
		SessionTimeout:       m.cfg.SessionTimeout,
		OldestSessionSeconds: oldest,
		TotalMessages:        total,
	}
}

// Close shuts down the manager.
func (m *Manager) Close() {
	m.cleanup.Stop()
	close(m.done)
	m.CloseAllSessions()
	m.mu.Lock()
	if m.browser != nil {
		_ = m.browser.Close()
		m.browser = nil
	}
	m.mu.Unlock()
}

// SetupAuth opens a visible browser for the user to log in.
func (m *Manager) SetupAuth(progress func(string)) error {
	slog.Info("🔐 Starting auth setup")
	if progress != nil {
		progress("Opening browser for Google login...")
	}
	l := launcher.New().Headless(false).UserDataDir(m.cfg.ChromeProfileDir).Set("no-sandbox", "")
	u, err := l.Launch()
	if err != nil {
		return fmt.Errorf("launch: %w", err)
	}
	browser := rod.New().ControlURL(u).MustConnect()
	defer browser.Close()

	page := browser.MustPage(config.NotebookLMAuthURL)
	if progress != nil {
		progress("Browser open – please log in to Google. You have 10 minutes.")
	}

	deadline := time.Now().Add(10 * time.Minute)
	for time.Now().Before(deadline) {
		time.Sleep(3 * time.Second)
		info, err := page.Info()
		if err != nil {
			continue
		}
		if strings.Contains(info.URL, "notebooklm.google") &&
			!strings.Contains(info.URL, "accounts.google.com") {
			slog.Info("✅ Login detected – saving state")
			if progress != nil {
				progress("Login detected! Saving authentication state...")
			}
			cookies, err := page.Cookies([]string{"https://notebooklm.google.com", "https://accounts.google.com"})
			if err != nil {
				return fmt.Errorf("get cookies: %w", err)
			}
			state := &auth.BrowserState{}
			for _, c := range cookies {
				state.Cookies = append(state.Cookies, auth.Cookie{
					Name:     c.Name,
					Value:    c.Value,
					Domain:   c.Domain,
					Path:     c.Path,
					Expires:  float64(c.Expires),
					HTTPOnly: c.HTTPOnly,
					Secure:   c.Secure,
					SameSite: string(c.SameSite),
				})
			}
			if err := m.authManager.SaveState(state); err != nil {
				return err
			}
			if progress != nil {
				progress("Authentication saved successfully!")
			}
			return nil
		}
	}
	return errors.New("login timeout (10 minutes)")
}

// ReAuth clears auth data then runs SetupAuth.
func (m *Manager) ReAuth(progress func(string)) error {
	if progress != nil {
		progress("Closing all sessions...")
	}
	m.CloseAllSessions()
	if progress != nil {
		progress("Clearing authentication data...")
	}
	if err := m.authManager.ClearAllAuthData(); err != nil {
		return err
	}
	m.mu.Lock()
	if m.browser != nil {
		_ = m.browser.Close()
		m.browser = nil
	}
	m.mu.Unlock()
	return m.SetupAuth(progress)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func generateID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func randInt(n int) int {
	if n <= 0 {
		return 0
	}
	v, _ := rand.Int(rand.Reader, big.NewInt(int64(n)))
	return int(v.Int64())
}

func imax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func imin(a, b int) int {
	if a < b {
		return a
	}
	return b
}
