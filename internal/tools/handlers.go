package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/asccclass/notebooklm-go/internal/auth"
	"github.com/asccclass/notebooklm-go/internal/config"
	"github.com/asccclass/notebooklm-go/internal/library"
	"github.com/asccclass/notebooklm-go/internal/mcp"
	"github.com/asccclass/notebooklm-go/internal/session"
	"github.com/asccclass/notebooklm-go/internal/utils"
)

const followUpReminder = "\n\nEXTREMELY IMPORTANT: Is that ALL you need to know? You can always ask another question using the same session ID! Think carefully: before you reply to the user, review their original request and this answer. If anything is unclear or missing, ask another question first."

// ─── Handler ─────────────────────────────────────────────────────────────────

// Handler implements the MCP tool handler interface.
type Handler struct {
	sessionMgr  *session.Manager
	authMgr     *auth.Manager
	lib         *library.Manager
	cfg         *config.Config
	allowedTools map[string]bool
}

// New creates a Handler.
func New(
	sessionMgr *session.Manager,
	authMgr *auth.Manager,
	lib *library.Manager,
	cfg *config.Config,
	allowedToolNames []string,
) *Handler {
	allowed := make(map[string]bool, len(allowedToolNames))
	for _, t := range allowedToolNames {
		allowed[t] = true
	}
	return &Handler{
		sessionMgr:   sessionMgr,
		authMgr:      authMgr,
		lib:          lib,
		cfg:          cfg,
		allowedTools: allowed,
	}
}

// ListTools returns the current tool definitions (dynamic based on library state).
func (h *Handler) ListTools() []mcp.Tool {
	return BuildToolDefinitions(h.lib, h.allowedTools)
}

// CallTool dispatches a tool call by name.
func (h *Handler) CallTool(ctx context.Context, name string, args json.RawMessage, sendProgress func(string)) (*mcp.ToolResult, error) {
	slog.Info("🔧 Tool called", "name", name)

	switch name {
	case ToolAskQuestion:
		return h.handleAskQuestion(ctx, args, sendProgress)
	case ToolListSessions:
		return h.handleListSessions()
	case ToolCloseSession:
		return h.handleCloseSession(args)
	case ToolResetSession:
		return h.handleResetSession(args)
	case ToolGetHealth:
		return h.handleGetHealth()
	case ToolSetupAuth:
		return h.handleSetupAuth(args, sendProgress)
	case ToolReAuth:
		return h.handleReAuth(args, sendProgress)
	case ToolCleanupData:
		return h.handleCleanupData(args)
	case ToolAddNotebook:
		return h.handleAddNotebook(args)
	case ToolListNotebooks:
		return h.handleListNotebooks()
	case ToolGetNotebook:
		return h.handleGetNotebook(args)
	case ToolSelectNotebook:
		return h.handleSelectNotebook(args)
	case ToolUpdateNotebook:
		return h.handleUpdateNotebook(args)
	case ToolRemoveNotebook:
		return h.handleRemoveNotebook(args)
	case ToolSearchNotebooks:
		return h.handleSearchNotebooks(args)
	case ToolGetLibraryStats:
		return h.handleGetLibraryStats()
	default:
		return errResult(fmt.Sprintf("unknown tool: %s", name)), nil
	}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func okResult(data interface{}) *mcp.ToolResult {
	b, _ := json.MarshalIndent(data, "", "  ")
	return &mcp.ToolResult{Content: []mcp.ContentItem{mcp.TextContent(string(b))}}
}

func errResult(msg string) *mcp.ToolResult {
	return &mcp.ToolResult{
		Content: []mcp.ContentItem{mcp.TextContent(msg)},
		IsError: true,
	}
}

func unmarshal(raw json.RawMessage, v interface{}) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, v)
}

// ─── ask_question ─────────────────────────────────────────────────────────────

type askQuestionArgs struct {
	Question      string                  `json:"question"`
	SessionID     string                  `json:"session_id"`
	NotebookID    string                  `json:"notebook_id"`
	NotebookURL   string                  `json:"notebook_url"`
	ShowBrowser   *bool                   `json:"show_browser"`
	BrowserOptions *config.BrowserOptions `json:"browser_options"`
}

func (h *Handler) handleAskQuestion(ctx context.Context, raw json.RawMessage, progress func(string)) (*mcp.ToolResult, error) {
	var args askQuestionArgs
	if err := unmarshal(raw, &args); err != nil {
		return errResult("invalid arguments: " + err.Error()), nil
	}
	if args.Question == "" {
		return errResult("question is required"), nil
	}

	slog.Info("ask_question", "question", truncate(args.Question, 100), "session", args.SessionID)

	// Resolve notebook URL.
	notebookURL := args.NotebookURL
	if notebookURL == "" && args.NotebookID != "" {
		nb := h.lib.IncrementUseCount(args.NotebookID)
		if nb == nil {
			return errResult("notebook not found: " + args.NotebookID), nil
		}
		notebookURL = nb.URL
	}
	if notebookURL == "" {
		active := h.lib.GetActiveNotebook()
		if active != nil {
			nb := h.lib.IncrementUseCount(active.ID)
			if nb != nil {
				notebookURL = nb.URL
			}
		}
	}
	if notebookURL == "" {
		return errResult("no notebook URL available. Use add_notebook or set a notebook URL."), nil
	}

	// Effective config with browser option overrides.
	effectiveCfg := h.cfg.Apply(args.BrowserOptions, args.ShowBrowser)

	if progress != nil {
		progress("Getting or creating browser session...")
	}

	sess, err := h.sessionMgr.GetOrCreateSession(ctx, args.SessionID, notebookURL, effectiveCfg)
	if err != nil {
		return errResult("session error: " + err.Error()), nil
	}

	if progress != nil {
		progress("Asking question to NotebookLM...")
	}

	answer, err := sess.Ask(args.Question, progress)
	if err != nil {
		if errors.Is(err, session.ErrRateLimit) {
			return errResult(
				"NotebookLM rate limit reached (50 queries/day for free accounts).\n\n" +
					"You can:\n" +
					"1. Use 're_auth' to login with a different Google account\n" +
					"2. Wait until tomorrow for quota reset\n" +
					"3. Upgrade to Google AI Pro/Ultra for higher limits",
			), nil
		}
		return errResult("ask error: " + err.Error()), nil
	}

	info := sess.GetInfo()
	result := map[string]interface{}{
		"status":      "success",
		"question":    args.Question,
		"answer":      strings.TrimRight(answer, " \n") + followUpReminder,
		"session_id":  sess.SessionID,
		"notebook_url": sess.NotebookURL,
		"session_info": map[string]interface{}{
			"age_seconds":     info.AgeSeconds,
			"message_count":   info.MessageCount,
			"last_activity":   info.LastActivity,
		},
	}

	if progress != nil {
		progress("Question answered successfully!")
	}

	return okResult(result), nil
}

// ─── Session tools ────────────────────────────────────────────────────────────

func (h *Handler) handleListSessions() (*mcp.ToolResult, error) {
	stats := h.sessionMgr.GetStats()
	infos := h.sessionMgr.AllSessionsInfo()
	result := map[string]interface{}{
		"active_sessions":        stats.ActiveSessions,
		"max_sessions":           stats.MaxSessions,
		"session_timeout":        stats.SessionTimeout,
		"oldest_session_seconds": stats.OldestSessionSeconds,
		"total_messages":         stats.TotalMessages,
		"sessions":               infos,
	}
	return okResult(result), nil
}

func (h *Handler) handleCloseSession(raw json.RawMessage) (*mcp.ToolResult, error) {
	var args struct {
		SessionID string `json:"session_id"`
	}
	if err := unmarshal(raw, &args); err != nil || args.SessionID == "" {
		return errResult("session_id is required"), nil
	}
	if !h.sessionMgr.CloseSession(args.SessionID) {
		return errResult("session not found: " + args.SessionID), nil
	}
	return okResult(map[string]interface{}{
		"status":     "success",
		"message":    "Session " + args.SessionID + " closed successfully",
		"session_id": args.SessionID,
	}), nil
}

func (h *Handler) handleResetSession(raw json.RawMessage) (*mcp.ToolResult, error) {
	var args struct {
		SessionID string `json:"session_id"`
	}
	if err := unmarshal(raw, &args); err != nil || args.SessionID == "" {
		return errResult("session_id is required"), nil
	}
	sess := h.sessionMgr.GetSession(args.SessionID)
	if sess == nil {
		return errResult("session not found: " + args.SessionID), nil
	}
	if err := sess.Reset(); err != nil {
		return errResult("reset failed: " + err.Error()), nil
	}
	return okResult(map[string]interface{}{
		"status":     "success",
		"message":    "Session " + args.SessionID + " reset successfully",
		"session_id": args.SessionID,
	}), nil
}

// ─── System tools ─────────────────────────────────────────────────────────────

func (h *Handler) handleGetHealth() (*mcp.ToolResult, error) {
	authenticated := h.authMgr.GetValidStatePath() != ""
	stats := h.sessionMgr.GetStats()

	result := map[string]interface{}{
		"status":              "ok",
		"authenticated":       authenticated,
		"notebook_url":        h.cfg.NotebookURL,
		"active_sessions":     stats.ActiveSessions,
		"max_sessions":        stats.MaxSessions,
		"session_timeout":     stats.SessionTimeout,
		"total_messages":      stats.TotalMessages,
		"headless":            h.cfg.Headless,
		"auto_login_enabled":  h.cfg.AutoLoginEnabled,
		"stealth_enabled":     h.cfg.Stealth.Enabled,
	}
	if !authenticated {
		result["troubleshooting_tip"] = "For a fresh start: Close all Chrome instances → cleanup_data(confirm=true, preserve_library=true) → setup_auth"
	}
	return okResult(result), nil
}

func (h *Handler) handleSetupAuth(raw json.RawMessage, progress func(string)) (*mcp.ToolResult, error) {
	start := time.Now()

	if progress != nil {
		progress("Initialising authentication setup...")
	}

	err := h.sessionMgr.SetupAuth(progress)
	duration := time.Since(start).Seconds()

	if err != nil {
		slog.Error("setup_auth failed", "err", err, "duration", utils.FormatDuration(time.Since(start)))
		return errResult(fmt.Sprintf("Authentication failed: %v", err)), nil
	}

	return okResult(map[string]interface{}{
		"status":           "authenticated",
		"message":          "Successfully authenticated and saved browser state",
		"authenticated":    true,
		"duration_seconds": duration,
	}), nil
}

func (h *Handler) handleReAuth(raw json.RawMessage, progress func(string)) (*mcp.ToolResult, error) {
	start := time.Now()

	if progress != nil {
		progress("Preparing re-authentication...")
	}

	err := h.sessionMgr.ReAuth(progress)
	duration := time.Since(start).Seconds()

	if err != nil {
		return errResult(fmt.Sprintf("Re-authentication failed: %v", err)), nil
	}

	return okResult(map[string]interface{}{
		"status":           "authenticated",
		"message":          "Successfully re-authenticated. All previous sessions closed.",
		"authenticated":    true,
		"duration_seconds": duration,
	}), nil
}

// ─── Cleanup ──────────────────────────────────────────────────────────────────

func (h *Handler) handleCleanupData(raw json.RawMessage) (*mcp.ToolResult, error) {
	var args struct {
		Confirm         bool `json:"confirm"`
		PreserveLibrary bool `json:"preserve_library"`
	}
	if err := unmarshal(raw, &args); err != nil {
		return errResult("invalid arguments"), nil
	}

	paths, err := scanCleanupPaths(h.cfg, args.PreserveLibrary)
	if err != nil {
		return errResult("scan failed: " + err.Error()), nil
	}

	if !args.Confirm {
		// Preview only.
		items := make([]map[string]interface{}, 0, len(paths))
		totalBytes := int64(0)
		for _, p := range paths {
			info, err := os.Stat(p)
			size := int64(0)
			if err == nil {
				size = info.Size()
				totalBytes += size
			}
			items = append(items, map[string]interface{}{
				"path":  p,
				"bytes": size,
			})
		}
		return okResult(map[string]interface{}{
			"status":           "preview",
			"total_paths":      len(paths),
			"total_size_bytes": totalBytes,
			"paths":            items,
			"message":          "Run with confirm=true to delete these files.",
		}), nil
	}

	// Actually delete.
	deleted := []string{}
	failed := []string{}
	for _, p := range paths {
		if err := os.RemoveAll(p); err != nil {
			failed = append(failed, p)
		} else {
			deleted = append(deleted, p)
		}
	}

	return okResult(map[string]interface{}{
		"status":        "completed",
		"deleted_paths": deleted,
		"failed_paths":  failed,
		"deleted_count": len(deleted),
		"failed_count":  len(failed),
	}), nil
}

// scanCleanupPaths collects all NotebookLM MCP data paths.
func scanCleanupPaths(cfg *config.Config, preserveLibrary bool) ([]string, error) {
	home, _ := os.UserHomeDir()
	candidates := []string{
		cfg.BrowserStateDir,
		cfg.ChromeProfileDir,
		cfg.ChromeInstanceDir,
	}

	// Platform-specific NPM cache.
	switch runtime.GOOS {
	case "windows":
		candidates = append(candidates,
			filepath.Join(home, "AppData", "Local", "npm-cache", "_npx"),
		)
	default:
		candidates = append(candidates,
			filepath.Join(home, ".npm", "_npx"),
			filepath.Join(home, ".cache", "npm"),
		)
	}

	if !preserveLibrary {
		candidates = append(candidates, filepath.Join(cfg.DataDir, "library.json"))
	}

	// Filter to only existing paths.
	var result []string
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			result = append(result, p)
		}
	}
	return result, nil
}

// ─── Library tools ────────────────────────────────────────────────────────────

func (h *Handler) handleAddNotebook(raw json.RawMessage) (*mcp.ToolResult, error) {
	var args library.AddNotebookInput
	if err := unmarshal(raw, &args); err != nil {
		return errResult("invalid arguments"), nil
	}
	if args.URL == "" || args.Name == "" || args.Description == "" || len(args.Topics) == 0 {
		return errResult("url, name, description, and topics are required"), nil
	}
	nb, err := h.lib.AddNotebook(args)
	if err != nil {
		return errResult("add notebook failed: " + err.Error()), nil
	}
	return okResult(map[string]interface{}{"notebook": nb}), nil
}

func (h *Handler) handleListNotebooks() (*mcp.ToolResult, error) {
	nbs := h.lib.ListNotebooks()
	return okResult(map[string]interface{}{"notebooks": nbs}), nil
}

func (h *Handler) handleGetNotebook(raw json.RawMessage) (*mcp.ToolResult, error) {
	var args struct {
		ID string `json:"id"`
	}
	if err := unmarshal(raw, &args); err != nil || args.ID == "" {
		return errResult("id is required"), nil
	}
	nb := h.lib.GetNotebook(args.ID)
	if nb == nil {
		return errResult("notebook not found: " + args.ID), nil
	}
	return okResult(map[string]interface{}{"notebook": nb}), nil
}

func (h *Handler) handleSelectNotebook(raw json.RawMessage) (*mcp.ToolResult, error) {
	var args struct {
		ID string `json:"id"`
	}
	if err := unmarshal(raw, &args); err != nil || args.ID == "" {
		return errResult("id is required"), nil
	}
	nb, err := h.lib.SelectNotebook(args.ID)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return okResult(map[string]interface{}{"notebook": nb}), nil
}

func (h *Handler) handleUpdateNotebook(raw json.RawMessage) (*mcp.ToolResult, error) {
	var args library.UpdateNotebookInput
	if err := unmarshal(raw, &args); err != nil || args.ID == "" {
		return errResult("id is required"), nil
	}
	nb, err := h.lib.UpdateNotebook(args)
	if err != nil {
		return errResult(err.Error()), nil
	}
	return okResult(map[string]interface{}{"notebook": nb}), nil
}

func (h *Handler) handleRemoveNotebook(raw json.RawMessage) (*mcp.ToolResult, error) {
	var args struct {
		ID string `json:"id"`
	}
	if err := unmarshal(raw, &args); err != nil || args.ID == "" {
		return errResult("id is required"), nil
	}
	nb := h.lib.GetNotebook(args.ID)
	if nb == nil {
		return errResult("notebook not found: " + args.ID), nil
	}
	removed := h.lib.RemoveNotebook(args.ID)
	closedSessions := 0
	if removed {
		closedSessions = h.sessionMgr.CloseSessionsForNotebook(nb.URL)
	}
	return okResult(map[string]interface{}{
		"removed":         removed,
		"closed_sessions": closedSessions,
	}), nil
}

func (h *Handler) handleSearchNotebooks(raw json.RawMessage) (*mcp.ToolResult, error) {
	var args struct {
		Query string `json:"query"`
	}
	if err := unmarshal(raw, &args); err != nil || args.Query == "" {
		return errResult("query is required"), nil
	}
	nbs := h.lib.SearchNotebooks(args.Query)
	return okResult(map[string]interface{}{"notebooks": nbs}), nil
}

func (h *Handler) handleGetLibraryStats() (*mcp.ToolResult, error) {
	stats := h.lib.GetStats()
	return okResult(stats), nil
}

// ─── Utility ──────────────────────────────────────────────────────────────────

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
