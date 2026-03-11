---
name: notebooklm-go
description: Use this skill when Codex needs to work with the notebooklm-go MCP server, register or configure the local NotebookLM server, authenticate with Google NotebookLM, manage the local notebook library, inspect or reset NotebookLM browser sessions, or ask source-grounded questions against a NotebookLM notebook through MCP tools such as ask_question, setup_auth, get_health, add_notebook, select_notebook, list_sessions, cleanup_data, and related library-management tools.
---

# NotebookLM Go

Use this skill for tasks involving the local `notebooklm-go` / `notebooklm-mcp-go` MCP server.

Treat it as an MCP-backed research connector for Google NotebookLM, not as a general web scraper or a direct CLI chat app.

## What This Server Does

- Runs a local stdio MCP server implemented in Go.
- Lets MCP-aware agents query NotebookLM notebooks and receive source-grounded answers.
- Persists a local notebook library so users can save, search, select, and update NotebookLM notebook metadata.
- Maintains browser-backed sessions so follow-up questions can reuse the same NotebookLM conversation context.

## Default Workflow

Follow this order unless the user asks for a different operation:

1. Check readiness with `get_health`.
2. If not authenticated, run `setup_auth`.
3. If no suitable notebook is active, inspect library state with `list_notebooks`.
4. If needed, add a notebook with `add_notebook`, then set it with `select_notebook`.
5. Ask the user’s question with `ask_question`.
6. Reuse `session_id` for follow-up questions on the same topic.

## Tool Selection

### Ask and continue research

- `ask_question`:
  - Use for all NotebookLM question-answering.
  - Prefer reusing an existing `session_id` for follow-up questions.
  - Use `notebook_id` to target a saved notebook.
  - Use `notebook_url` only for ad-hoc notebooks or when the notebook is not yet in the library.

### Authentication and health

- `get_health`:
  - Use first for diagnostics.
  - Use after `setup_auth` or `re_auth` to verify state.
- `setup_auth`:
  - Use for first-time Google login.
  - Expect a visible browser login flow.
- `re_auth`:
  - Use when the auth state is broken, expired, or the user wants another Google account.
  - This closes all sessions and clears prior auth state.
- `cleanup_data`:
  - Use only for cleanup or recovery.
  - Preview with `confirm=false` before deletion.
  - Use `preserve_library=true` when the user wants to keep saved notebooks.

### Notebook library

- `list_notebooks`: inspect saved notebooks before asking the user to add duplicates.
- `get_notebook`: retrieve one notebook’s saved metadata by ID.
- `select_notebook`: set the default notebook used by `ask_question`.
- `search_notebooks`: find notebooks by name, description, topics, or tags.
- `get_library_stats`: inspect overall library usage.
- `update_notebook`: revise notebook metadata when the current description or topics are weak.
- `remove_notebook`: delete a saved notebook only when the user explicitly asks.

### Session management

- `list_sessions`: inspect current browser-backed sessions and reuse context where appropriate.
- `reset_session`: keep the same session but clear its chat history.
- `close_session`: free resources when a session is no longer needed.

## Rules For Adding Notebooks

Do not call `add_notebook` until the user has explicitly asked to add a notebook.

Collect and confirm this metadata first:

1. NotebookLM URL
2. Display name
3. One or two sentences describing the notebook’s knowledge
4. Three to five topics
5. Optional use cases, content types, and tags

If the user does not know how to get the URL, instruct them:

- Open `https://notebooklm.google`
- Open the notebook
- Use Share
- Enable link sharing if needed
- Copy the notebook link

## Operating Guidance

- Prefer `get_health` before attempting recovery actions.
- Prefer `setup_auth` over `re_auth` unless the existing auth state is clearly unusable.
- Prefer selecting a notebook once and then omitting notebook arguments for follow-up work.
- Prefer continuing the same session when the user is refining or extending an earlier question.
- If NotebookLM returns rate-limit failures, explain the quota issue and stop retrying blindly.
- If no notebook is available, direct the user to add or select one before calling `ask_question`.

## Build And Registration Context

When the user asks for setup help, the normal local flow is:

1. Build the binary with `go build -o notebooklm-mcp-go .`
2. Register it in the MCP client
3. Let the MCP client call tools over stdio

Common registration examples:

- Claude Code: `claude mcp add notebooklm ./notebooklm-mcp-go`
- Codex: `codex mcp add notebooklm -- C:\Users\andyl\Myprogram\notebooklm-go\notebooklm-mcp-go.exe`

## Boundaries

- Do not present this server as a replacement for NotebookLM itself.
- Do not claim direct NotebookLM API access; this project automates the NotebookLM web experience through browser sessions.
- Do not ask users to call JSON-RPC methods manually unless they explicitly want low-level MCP debugging.
