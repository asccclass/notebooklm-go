# notebooklm-mcp-go

[English](#english) | [繁體中文](#繁體中文)

## English

A **Golang rewrite** of [PleasePrompto/notebooklm-mcp](https://github.com/PleasePrompto/notebooklm-mcp) — an MCP server that lets AI agents (Claude Code, Cursor, Codex…) query Google NotebookLM notebooks for source-grounded, citation-backed answers powered by Gemini.

## What's different from the TypeScript original?

| Feature | TypeScript original | Go rewrite |
|---|---|---|
| Runtime | Node.js + npx | Single static binary |
| Browser automation | Patchright (Playwright fork) | go-rod (Chrome DevTools Protocol) |
| Startup time | ~3–5 s (node bootstrap) | ~100 ms |
| Memory | ~150 MB idle | ~25 MB idle |
| Binary size | ~100 MB (node_modules) | ~14 MB |
| MCP transport | @modelcontextprotocol/sdk | Hand-rolled JSON-RPC 2.0 over stdio |

All **16 tools** from the original are implemented.

---

## Quick Start

```bash
# Build
go build -o notebooklm-mcp-go .

# Register as an MCP server (Claude Code)
claude mcp add notebooklm ./notebooklm-mcp-go

# Or add to your claude_desktop_config.json:
# {
#   "mcpServers": {
#     "notebooklm": {
#       "command": "/path/to/notebooklm-mcp-go"
#     }
#   }
# }
```

## How To Use This MCP

This project is an MCP server. You usually do **not** call its tools manually.
Instead, you:

1. Build the binary
2. Register it in your MCP client
3. Ask your AI agent to use NotebookLM
4. The agent calls these tools for you over MCP

### Typical user flow

1. Build and register the server.
2. Ask your agent to authenticate:
   - `Set up NotebookLM authentication`
   - This calls `setup_auth` and opens Chrome for Google login.
3. Verify status:
   - `Check NotebookLM health`
   - This calls `get_health`.
4. Add a notebook to the local library:
   - `Add my NotebookLM notebook`
   - The agent should collect the NotebookLM URL, description, topics, and use cases, then call `add_notebook`.
5. Select a default notebook if needed:
   - `Use <notebook name> as my default notebook`
   - This calls `select_notebook`.
6. Start asking questions:
   - `Ask my notebook to summarize the API design`
   - `Ask the same session to compare v1 and v2 behaviour`
   - This calls `ask_question`, optionally reusing `session_id`.

### How MCP clients typically interact with it

- Claude Code / Claude Desktop:
  Register the binary as an MCP server, then ask for actions in natural language.
- Cursor / Codex / other MCP-aware clients:
  Add the server command in that client's MCP/server config, then instruct the agent normally.
- Direct execution:
  Running `./notebooklm-mcp-go` starts a stdio JSON-RPC MCP server. This is for MCP clients, not for interactive terminal use.

### Example prompts you can give your agent

- `Set up NotebookLM authentication`
- `Show NotebookLM server health`
- `List my notebooks`
- `Add this NotebookLM notebook to the library`
- `Select the notebook named <name>`
- `Ask the active notebook: what are the main design tradeoffs?`
- `Continue the previous NotebookLM session and ask for implementation steps`
- `Reset the current NotebookLM session`
- `Clean up NotebookLM data but keep my library`

### Client-specific MCP configuration examples

The exact MCP UI and file format can change by client version. The examples below match the current documented patterns for `stdio`-based local MCP servers.

#### Claude Desktop / Claude Code

Claude Code can register the server directly from the CLI:

```bash
claude mcp add notebooklm ./notebooklm-mcp-go
```

If you want to configure Claude Desktop manually, add an entry like this to your MCP config JSON:

```json
{
  "mcpServers": {
    "notebooklm": {
      "command": "C:\\Users\\andyl\\Myprogram\\notebooklm-go\\notebooklm-mcp-go.exe",
      "args": [],
      "env": {
        "DEBUG": "0"
      }
    }
  }
}
```

Notes:

- Replace the `command` path with your actual built binary location.
- On macOS/Linux, use the absolute binary path such as `/Users/you/path/notebooklm-mcp-go`.
- If you build frequently, keep the binary path stable so the client config does not need updating.

#### Cursor

Cursor reads MCP configuration from project-level `.cursor/mcp.json` or global `~/.cursor/mcp.json`.

Project-local example:

```json
{
  "mcpServers": {
    "notebooklm": {
      "command": "${workspaceFolder}/notebooklm-mcp-go",
      "args": [],
      "env": {
        "DEBUG": "0"
      }
    }
  }
}
```

Windows example using an absolute path:

```json
{
  "mcpServers": {
    "notebooklm": {
      "command": "C:\\Users\\andyl\\Myprogram\\notebooklm-go\\notebooklm-mcp-go.exe",
      "args": [],
      "env": {
        "DEBUG": "0"
      }
    }
  }
}
```

Notes:

- If you keep the binary in the repo root, `${workspaceFolder}/notebooklm-mcp-go` is convenient on macOS/Linux.
- On Windows, prefer the `.exe` path.
- After editing `mcp.json`, restart Cursor or reload MCP servers.

#### Codex

Codex supports MCP through the CLI and shared config.

CLI registration example:

```bash
codex mcp add notebooklm -- C:\Users\andyl\Myprogram\notebooklm-go\notebooklm-mcp-go.exe
```

If you prefer config file setup, add this to `~/.codex/config.toml`:

```toml
[mcp_servers.notebooklm]
command = "C:\\Users\\andyl\\Myprogram\\notebooklm-go\\notebooklm-mcp-go.exe"
args = []

[mcp_servers.notebooklm.env]
DEBUG = "0"
```

macOS/Linux example:

```toml
[mcp_servers.notebooklm]
command = "/absolute/path/to/notebooklm-mcp-go"
args = []

[mcp_servers.notebooklm.env]
DEBUG = "0"
```

Notes:

- `codex mcp list` can be used to verify the server is registered.
- Codex shares MCP configuration between the CLI and IDE extension.
- Keep the server name short and descriptive, for example `notebooklm`.

---

## Tools

### Core
| Tool | Description |
|---|---|
| `ask_question` | Ask a question to a NotebookLM notebook. Supports session continuity, notebook selection, and browser options. |

### Session Management
| Tool | Description |
|---|---|
| `list_sessions` | List all active browser sessions with age, message count, last activity |
| `close_session` | Close a specific session by ID |
| `reset_session` | Reset chat history for a session (reload notebook page) |

### System
| Tool | Description |
|---|---|
| `get_health` | Auth state, active sessions, configuration summary |
| `setup_auth` | Open a visible browser for manual Google login |
| `re_auth` | Switch accounts / fresh authentication (clears all data) |
| `cleanup_data` | Deep cleanup of all MCP data files with preview + confirm |

### Library Management
| Tool | Description |
|---|---|
| `add_notebook` | Add a NotebookLM notebook to your persistent library |
| `list_notebooks` | List all notebooks with metadata |
| `get_notebook` | Get a specific notebook by ID |
| `select_notebook` | Set the active notebook |
| `update_notebook` | Update notebook metadata |
| `remove_notebook` | Remove a notebook from the library |
| `search_notebooks` | Search notebooks by name, description, topics, or tags |
| `get_library_stats` | Usage statistics for your notebook library |

### What each tool is for

| Tool | When to use it | Notes |
|---|---|---|
| `ask_question` | Ask NotebookLM a question grounded in notebook sources | Supports `session_id`, `notebook_id`, `notebook_url`, and browser overrides |
| `list_sessions` | Inspect active browser-backed chat sessions | Useful for debugging or reusing session context |
| `close_session` | End one session and free browser resources | Requires `session_id` |
| `reset_session` | Keep the same session ID but clear chat history | Reloads the notebook page |
| `get_health` | Check auth state, configuration, and current session stats | Good first diagnostic command |
| `setup_auth` | First-time Google login for NotebookLM | Opens a visible browser |
| `re_auth` | Login again with the same or a different Google account | Clears old auth and closes sessions |
| `cleanup_data` | Preview or delete local NotebookLM MCP data | Supports `confirm` and `preserve_library` |
| `add_notebook` | Save a NotebookLM notebook into the local library | Requires notebook metadata |
| `list_notebooks` | Show all notebooks in the local library | Includes metadata and URLs |
| `get_notebook` | Show one notebook's details | Requires notebook `id` |
| `select_notebook` | Set the default notebook used by `ask_question` | Requires notebook `id` |
| `update_notebook` | Edit saved notebook metadata | Requires notebook `id` |
| `remove_notebook` | Remove a notebook from the local library | Does not delete the real NotebookLM notebook |
| `search_notebooks` | Find notebooks by topic/name/tag/description | Useful once the library grows |
| `get_library_stats` | Show usage counts and active notebook status | Good for operational visibility |

### Recommended operating sequence

If you are using this MCP for the first time, follow this order:

1. `setup_auth`
2. `get_health`
3. `add_notebook`
4. `list_notebooks`
5. `select_notebook`
6. `ask_question`

For ongoing use:

1. `get_health`
2. `list_notebooks` or `search_notebooks`
3. `ask_question`
4. `list_sessions` / `reset_session` / `close_session` if you need session management

---

## Configuration

### Environment Variables

```bash
# NotebookLM
NOTEBOOK_URL=https://notebooklm.google.com/notebook/...

# Browser
HEADLESS=true                    # Run headless (default: true)
BROWSER_TIMEOUT=30000            # Timeout in ms

# Sessions
MAX_SESSIONS=10                  # Max concurrent sessions
SESSION_TIMEOUT=900              # Session idle timeout in seconds

# Auto-login (optional)
AUTO_LOGIN_ENABLED=false
LOGIN_EMAIL=you@gmail.com
LOGIN_PASSWORD=yourpassword

# Stealth (human-like behavior)
STEALTH_ENABLED=true
STEALTH_HUMAN_TYPING=true
TYPING_WPM_MIN=160
TYPING_WPM_MAX=240
MIN_DELAY_MS=100
MAX_DELAY_MS=400

# Tool profile
NOTEBOOKLM_PROFILE=full          # minimal | standard | full
NOTEBOOKLM_DISABLED_TOOLS=cleanup_data,re_auth
```

### CLI Config Commands

```bash
# Show current settings
./notebooklm-mcp-go config get

# Set tool profile
./notebooklm-mcp-go config set profile minimal
./notebooklm-mcp-go config set profile standard
./notebooklm-mcp-go config set profile full

# Disable specific tools
./notebooklm-mcp-go config set disabled-tools "cleanup_data,re_auth"

# Reset to defaults
./notebooklm-mcp-go config reset
```

Settings are saved to `~/.config/notebooklm-mcp/settings.json`.

### What the CLI config affects

- `profile`: Limits which tools are exposed to the MCP client.
- `disabled-tools`: Hides selected tools even if the current profile would normally allow them.
- These settings affect the tool list returned by the server at startup.

### Profiles

| Profile | Tools included |
|---|---|
| `minimal` | ask_question, get_health, setup_auth |
| `standard` | + re_auth, list_sessions, close_session, list_notebooks, select_notebook, add_notebook |
| `full` | All 16 tools (default) |

---

## Data Paths

| Platform | Data directory |
|---|---|
| Linux | `~/.local/share/notebooklm-mcp/` |
| macOS | `~/Library/Application Support/notebooklm-mcp/` |
| Windows | `%APPDATA%\notebooklm-mcp\` |

Library: `<data>/library.json`  
Auth state: `<data>/browser_state/state.json`  
Chrome profile: `<data>/chrome_profile/`

---

## Architecture

```
Your Task
  └─► Claude / Codex / Cursor
        └─► MCP Server (JSON-RPC over stdio)
              └─► go-rod (Chrome DevTools Protocol)
                    └─► Chrome / Chromium
                          └─► NotebookLM (Gemini 2.5)
                                └─► Your Docs
```

### Package Structure

```
.
├── main.go                          # Entry point + CLI config
├── internal/
│   ├── config/config.go             # Configuration (env vars + defaults)
│   ├── auth/auth_manager.go         # Browser state / cookie persistence
│   ├── library/
│   │   ├── types.go                 # Library data types
│   │   └── library.go               # Persistent notebook library (library.json)
│   ├── session/session.go           # Browser sessions + session manager
│   ├── tools/
│   │   ├── definitions.go           # MCP tool definitions (schemas + descriptions)
│   │   └── handlers.go              # Tool implementation logic
│   ├── mcp/server.go                # JSON-RPC 2.0 MCP server (stdio transport)
│   └── utils/
│       ├── logger.go                # Structured logging to stderr
│       └── settings.go              # Profile/tool filtering settings
└── go.mod
```

---

## First-Time Setup

1. Build the binary: `go build -o notebooklm-mcp-go .`
2. Register with your MCP client
3. Ask Claude: **"Set up NotebookLM authentication"** → calls `setup_auth`
4. A Chrome window opens — log in to your Google account
5. After login, auth state is saved to disk automatically
6. Verify: **"Check NotebookLM health"** → calls `get_health`
7. Add a notebook: **"Add my NotebookLM notebook"** → Claude guides you through `add_notebook`
8. Start querying: **"Ask my notebook about X"** → calls `ask_question`

## Common Workflows

### 1. First-time authentication

1. Start the MCP server through your client.
2. Ask the agent to run `setup_auth`.
3. Log in to Google in the browser window.
4. Ask the agent to run `get_health`.
5. Confirm `authenticated=true`.

### 2. Add a notebook and query it

1. Create or open a notebook in NotebookLM.
2. Copy its shareable URL.
3. Ask the agent to add it via `add_notebook`.
4. Optionally call `select_notebook`.
5. Ask questions through `ask_question`.

### 3. Continue a conversation

1. Ask a first question with `ask_question`.
2. Keep the returned `session_id`.
3. Ask follow-up questions using the same `session_id`.
4. Use `reset_session` if you want a clean conversation without deleting the session record.

### 4. Troubleshooting

- If NotebookLM stops answering as expected, run `get_health`.
- If the Google account changed or the login expired, run `re_auth`.
- If browser/session state is corrupted, run `cleanup_data` with preview first, then confirm deletion.
- If you want to keep your saved notebooks during cleanup, set `preserve_library=true`.

---

## Rate Limits

Free Google accounts: **50 queries/day**. When the limit is hit:

- `ask_question` returns a `RateLimitError`
- Use `re_auth` to switch to a different Google account
- Or wait until the next day

Upgrade to **Google AI Pro/Ultra** for 5× higher limits.

---

## License

MIT — same as the original TypeScript project.

## 繁體中文

中文版請參考 [README.zh-TW.md](README.zh-TW.md)。
