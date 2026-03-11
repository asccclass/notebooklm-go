// Package tools defines all MCP tools and their handlers.
package tools

import (
	"fmt"
	"strings"

	"github.com/asccclass/notebooklm-go/internal/library"
	"github.com/asccclass/notebooklm-go/internal/mcp"
)

// ─── Tool name constants ──────────────────────────────────────────────────────

const (
	ToolAskQuestion     = "ask_question"
	ToolListSessions    = "list_sessions"
	ToolCloseSession    = "close_session"
	ToolResetSession    = "reset_session"
	ToolGetHealth       = "get_health"
	ToolSetupAuth       = "setup_auth"
	ToolReAuth          = "re_auth"
	ToolCleanupData     = "cleanup_data"
	ToolAddNotebook     = "add_notebook"
	ToolListNotebooks   = "list_notebooks"
	ToolGetNotebook     = "get_notebook"
	ToolSelectNotebook  = "select_notebook"
	ToolUpdateNotebook  = "update_notebook"
	ToolRemoveNotebook  = "remove_notebook"
	ToolSearchNotebooks = "search_notebooks"
	ToolGetLibraryStats = "get_library_stats"
)

// AllToolNames lists all tools in declaration order.
var AllToolNames = []string{
	ToolAskQuestion,
	ToolListSessions, ToolCloseSession, ToolResetSession,
	ToolGetHealth, ToolSetupAuth, ToolReAuth, ToolCleanupData,
	ToolAddNotebook, ToolListNotebooks, ToolGetNotebook,
	ToolSelectNotebook, ToolUpdateNotebook, ToolRemoveNotebook,
	ToolSearchNotebooks, ToolGetLibraryStats,
}

// ─── Dynamic description ─────────────────────────────────────────────────────

// buildAskQuestionDescription generates the ask_question description based on
// the current active notebook (or lack thereof).
func buildAskQuestionDescription(lib *library.Manager) string {
	active := lib.GetActiveNotebook()
	if active != nil {
		topics := strings.Join(active.Topics, ", ")
		useCasesLines := make([]string, len(active.UseCases))
		for i, uc := range active.UseCases {
			useCasesLines[i] = "  - " + uc
		}
		useCases := strings.Join(useCasesLines, "\n")

		return fmt.Sprintf(`# Conversational Research Partner (NotebookLM • Gemini 2.5 • Session RAG)

**Active Notebook:** %s
**Content:** %s
**Topics:** %s

> Auth tip: If login is required, use setup_auth and then verify with get_health.

## What This Tool Is
- Full conversational research with Gemini grounded on your notebook sources
- Session-based: each follow-up uses prior context for deeper answers
- Source-cited responses designed to minimise hallucinations

## When To Use
%s

## Rules
- Always prefer continuing an existing session for the same task
- Keep the session_id for follow-up questions
- Ask clarifying questions before implementing
- If authentication fails, run re_auth and verify with get_health

## Notebook Selection
- Default: active notebook (%s)
- Set notebook_id to use a specific library notebook
- Set notebook_url for ad-hoc notebooks not in your library`,
			active.Name, active.Description, topics, useCases, active.ID)
	}

	return `# Conversational Research Partner (NotebookLM • Gemini 2.5 • Session RAG)

## No Active Notebook
- Visit https://notebooklm.google to create a notebook
- Use **add_notebook** to register it in your library
- Use **list_notebooks** to see available sources
- Use **select_notebook** to activate one

> Auth tip: If login is required, use setup_auth and verify with get_health.`
}

// ─── Tool definitions ─────────────────────────────────────────────────────────

// BuildToolDefinitions returns all MCP tool definitions, filtered by the
// provided set of allowed names (nil = all allowed).
func BuildToolDefinitions(lib *library.Manager, allowed map[string]bool) []mcp.Tool {
	all := allTools(lib)
	if allowed == nil {
		return all
	}
	var result []mcp.Tool
	for _, t := range all {
		if allowed[t.Name] {
			result = append(result, t)
		}
	}
	return result
}

func allTools(lib *library.Manager) []mcp.Tool {
	browserOptionsSchema := map[string]interface{}{
		"type":        "object",
		"description": "Optional browser behaviour settings",
		"properties": map[string]interface{}{
			"show": map[string]interface{}{
				"type":        "boolean",
				"description": "Show browser window",
			},
			"headless": map[string]interface{}{
				"type":        "boolean",
				"description": "Run browser in headless mode (default: true)",
			},
			"timeout_ms": map[string]interface{}{
				"type":        "number",
				"description": "Browser operation timeout in ms (default: 30000)",
			},
			"stealth": map[string]interface{}{
				"type":        "object",
				"description": "Human-like behaviour settings",
				"properties": map[string]interface{}{
					"enabled":         map[string]interface{}{"type": "boolean"},
					"random_delays":   map[string]interface{}{"type": "boolean"},
					"human_typing":    map[string]interface{}{"type": "boolean"},
					"mouse_movements": map[string]interface{}{"type": "boolean"},
					"typing_wpm_min":  map[string]interface{}{"type": "number"},
					"typing_wpm_max":  map[string]interface{}{"type": "number"},
					"delay_min_ms":    map[string]interface{}{"type": "number"},
					"delay_max_ms":    map[string]interface{}{"type": "number"},
				},
			},
		},
	}

	return []mcp.Tool{
		// ── ask_question ─────────────────────────────────────────────────────
		{
			Name:        ToolAskQuestion,
			Description: buildAskQuestionDescription(lib),
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"question": map[string]interface{}{
						"type":        "string",
						"description": "The question to ask NotebookLM",
					},
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Optional session ID. Omit to start a new session.",
					},
					"notebook_id": map[string]interface{}{
						"type":        "string",
						"description": "Optional notebook ID from your library.",
					},
					"notebook_url": map[string]interface{}{
						"type":        "string",
						"description": "Optional direct notebook URL (overrides notebook_id).",
					},
					"show_browser": map[string]interface{}{
						"type":        "boolean",
						"description": "Show browser window for debugging.",
					},
					"browser_options": browserOptionsSchema,
				},
				Required: []string{"question"},
			},
		},

		// ── Session management ────────────────────────────────────────────────
		{
			Name:        ToolListSessions,
			Description: "List all active sessions with stats (age, message count, last activity).",
			InputSchema: mcp.InputSchema{Type: "object"},
		},
		{
			Name:        ToolCloseSession,
			Description: "Close a specific session by session ID.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session ID to close",
					},
				},
				Required: []string{"session_id"},
			},
		},
		{
			Name:        ToolResetSession,
			Description: "Reset a session's chat history (same session ID, clean slate).",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"session_id": map[string]interface{}{
						"type":        "string",
						"description": "Session ID to reset",
					},
				},
				Required: []string{"session_id"},
			},
		},

		// ── System ────────────────────────────────────────────────────────────
		{
			Name:        ToolGetHealth,
			Description: "Get server health: authentication state, active sessions, and configuration.",
			InputSchema: mcp.InputSchema{Type: "object"},
		},
		{
			Name: ToolSetupAuth,
			Description: "Open a browser for manual Google login. " +
				"Returns after saving auth state. Use get_health to verify.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"show_browser":    map[string]interface{}{"type": "boolean"},
					"browser_options": browserOptionsSchema,
				},
			},
		},
		{
			Name: ToolReAuth,
			Description: "Switch Google account or re-authenticate. " +
				"Closes all sessions, clears auth data, opens fresh login.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"show_browser":    map[string]interface{}{"type": "boolean"},
					"browser_options": browserOptionsSchema,
				},
			},
		},
		{
			Name: ToolCleanupData,
			Description: "Deep cleanup of all NotebookLM MCP data files. " +
				"Shows preview first (confirm=false), then deletes (confirm=true). " +
				"Set preserve_library=true to keep your notebook library.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"confirm": map[string]interface{}{
						"type":        "boolean",
						"description": "Set true to execute deletion after reviewing preview.",
					},
					"preserve_library": map[string]interface{}{
						"type":        "boolean",
						"description": "Keep library.json during cleanup. Default: false.",
					},
				},
				Required: []string{"confirm"},
			},
		},

		// ── Library management ────────────────────────────────────────────────
		{
			Name: ToolAddNotebook,
			Description: `PERMISSION REQUIRED — Only when user explicitly asks to add a notebook.

## Workflow
1) Ask URL: "What is the NotebookLM URL?"
2) Ask content: "What knowledge is inside?" (1-2 sentences)
3) Ask topics: "Which topics does it cover?" (3-5)
4) Ask use cases: "When should we consult it?"
5) Propose metadata and confirm before calling this tool

## How to Get a Share Link
Visit https://notebooklm.google/ → Login → Open notebook → Share → "Anyone with the link" → Copy link`,
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"url":           map[string]interface{}{"type": "string", "description": "NotebookLM notebook URL"},
					"name":          map[string]interface{}{"type": "string", "description": "Display name"},
					"description":   map[string]interface{}{"type": "string", "description": "What knowledge is in this notebook"},
					"topics":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Topics covered"},
					"content_types": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Types of content"},
					"use_cases":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "When to use this notebook"},
					"tags":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Optional tags"},
				},
				Required: []string{"url", "name", "description", "topics"},
			},
		},
		{
			Name:        ToolListNotebooks,
			Description: "List all library notebooks with metadata (name, topics, use cases, URL).",
			InputSchema: mcp.InputSchema{Type: "object"},
		},
		{
			Name:        ToolGetNotebook,
			Description: "Get detailed information about a specific notebook by ID.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id": map[string]interface{}{"type": "string", "description": "The notebook ID"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        ToolSelectNotebook,
			Description: "Set a notebook as the active default for ask_question.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id": map[string]interface{}{"type": "string", "description": "Notebook ID to activate"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        ToolUpdateNotebook,
			Description: "Update notebook metadata. Confirm the change with the user before calling.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id":            map[string]interface{}{"type": "string"},
					"name":          map[string]interface{}{"type": "string"},
					"description":   map[string]interface{}{"type": "string"},
					"topics":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"content_types": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"use_cases":     map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"tags":          map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
					"url":           map[string]interface{}{"type": "string"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name: ToolRemoveNotebook,
			Description: "Remove a notebook from the library. " +
				"Requires explicit user confirmation. Does NOT delete the actual NotebookLM notebook.",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"id": map[string]interface{}{"type": "string", "description": "Notebook ID to remove"},
				},
				Required: []string{"id"},
			},
		},
		{
			Name:        ToolSearchNotebooks,
			Description: "Search library by query (name, description, topics, tags).",
			InputSchema: mcp.InputSchema{
				Type: "object",
				Properties: map[string]interface{}{
					"query": map[string]interface{}{"type": "string", "description": "Search query"},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        ToolGetLibraryStats,
			Description: "Get statistics about your notebook library (total notebooks, usage, etc.).",
			InputSchema: mcp.InputSchema{Type: "object"},
		},
	}
}
