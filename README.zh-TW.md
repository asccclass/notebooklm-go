# notebooklm-mcp-go

這是 [PleasePrompto/notebooklm-mcp](https://github.com/PleasePrompto/notebooklm-mcp) 的 **Golang 重寫版**。它是一個 MCP 伺服器，讓 AI 代理（Claude Code、Cursor、Codex…）可以查詢 Google NotebookLM 筆記本，取得由 Gemini 產生、根據來源內容整理、並附帶引用脈絡的答案。

## 跟 TypeScript 原版有什麼差別？

| 項目 | TypeScript 原版 | Go 重寫版 |
|---|---|---|
| 執行環境 | Node.js + npx | 單一靜態二進位檔 |
| 瀏覽器自動化 | Patchright（Playwright 分支） | go-rod（Chrome DevTools Protocol） |
| 啟動速度 | 約 3–5 秒（node 啟動） | 約 100 ms |
| 記憶體占用 | 閒置約 150 MB | 閒置約 25 MB |
| 二進位大小 | 約 100 MB（node_modules） | 約 14 MB |
| MCP 傳輸方式 | `@modelcontextprotocol/sdk` | 自行實作的 stdio JSON-RPC 2.0 |

原版的 **16 個工具** 都已完整實作。

---

## 快速開始

```bash
# Build
go build -o notebooklm-mcp-go .

# 註冊為 MCP server（Claude Code）
claude mcp add notebooklm ./notebooklm-mcp-go
```

如果要手動加入 Claude Desktop 設定：

```json
{
  "mcpServers": {
    "notebooklm": {
      "command": "/path/to/notebooklm-mcp-go"
    }
  }
}
```

---

## 這個 MCP 要怎麼用？

這個專案是一個 MCP server。一般情況下你**不需要手動呼叫工具**，實際使用方式通常是：

1. 先編譯 binary
2. 在 MCP client 裡註冊這個 server
3. 用自然語言要求 AI agent 幫你操作 NotebookLM
4. Agent 會透過 MCP 自動呼叫對應工具

### 典型使用流程

1. 編譯並註冊 server。
2. 請 agent 協助做登入驗證：
   - `Set up NotebookLM authentication`
   - 這會呼叫 `setup_auth`，並開啟 Chrome 讓你登入 Google。
3. 確認狀態：
   - `Check NotebookLM health`
   - 這會呼叫 `get_health`。
4. 把筆記本加入本機 library：
   - `Add my NotebookLM notebook`
   - Agent 會先整理 NotebookLM URL、描述、主題與使用情境，再呼叫 `add_notebook`。
5. 如有需要，設定預設筆記本：
   - `Use <notebook name> as my default notebook`
   - 這會呼叫 `select_notebook`。
6. 開始提問：
   - `Ask my notebook to summarize the API design`
   - `Ask the same session to compare v1 and v2 behaviour`
   - 這會呼叫 `ask_question`，也可以重複使用 `session_id` 來延續同一段對話。

### MCP client 通常怎麼跟它互動

- Claude Code / Claude Desktop：
  把 binary 註冊成 MCP server，之後直接用自然語言請 agent 幫你做事。
- Cursor / Codex / 其他支援 MCP 的 client：
  在各自的 MCP/server 設定中加入這個 command，之後照平常方式指示 agent 即可。
- 直接執行：
  執行 `./notebooklm-mcp-go` 會啟動 stdio JSON-RPC MCP server。這是給 MCP client 連線用的，不是給終端機互動式操作用的。

### 可以直接對 agent 說的提示詞範例

- `Set up NotebookLM authentication`
- `Show NotebookLM server health`
- `List my notebooks`
- `Add this NotebookLM notebook to the library`
- `Select the notebook named <name>`
- `Ask the active notebook: what are the main design tradeoffs?`
- `Continue the previous NotebookLM session and ask for implementation steps`
- `Reset the current NotebookLM session`
- `Clean up NotebookLM data but keep my library`

### 各客戶端 MCP 設定範例

不同 client 的 MCP 介面與設定格式可能會隨版本變動。以下範例對應的是目前常見的 `stdio` 本機 MCP server 設定方式。

#### Claude Desktop / Claude Code

Claude Code 可以直接用 CLI 註冊：

```bash
claude mcp add notebooklm ./notebooklm-mcp-go
```

如果想手動設定 Claude Desktop，可在 MCP 設定 JSON 中加入：

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

說明：

- 請把 `command` 換成實際編譯出的 binary 路徑。
- 在 macOS/Linux 上，請使用像 `/Users/you/path/notebooklm-mcp-go` 這樣的絕對路徑。
- 如果你會頻繁重新 build，建議把 binary 放在固定位置，之後就不用一直改設定。

#### Cursor

Cursor 會從專案層級 `.cursor/mcp.json` 或全域 `~/.cursor/mcp.json` 讀取 MCP 設定。

專案內設定範例：

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

Windows 絕對路徑範例：

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

說明：

- 如果 binary 放在 repo 根目錄，macOS/Linux 使用 `${workspaceFolder}/notebooklm-mcp-go` 會比較方便。
- 在 Windows 上，建議直接使用 `.exe` 的絕對路徑。
- 修改 `mcp.json` 後，請重新啟動 Cursor 或重新載入 MCP servers。

#### Codex

Codex 支援透過 CLI 與共用設定檔使用 MCP。

CLI 註冊範例：

```bash
codex mcp add notebooklm -- C:\Users\andyl\Myprogram\notebooklm-go\notebooklm-mcp-go.exe
```

如果想改用設定檔，可在 `~/.codex/config.toml` 中加入：

```toml
[mcp_servers.notebooklm]
command = "C:\\Users\\andyl\\Myprogram\\notebooklm-go\\notebooklm-mcp-go.exe"
args = []

[mcp_servers.notebooklm.env]
DEBUG = "0"
```

macOS/Linux 範例：

```toml
[mcp_servers.notebooklm]
command = "/absolute/path/to/notebooklm-mcp-go"
args = []

[mcp_servers.notebooklm.env]
DEBUG = "0"
```

說明：

- 可用 `codex mcp list` 確認 server 是否已正確註冊。
- Codex CLI 與 IDE extension 會共用 MCP 設定。
- server 名稱建議簡短清楚，例如 `notebooklm`。

---

## 工具

### 核心工具
| Tool | 說明 |
|---|---|
| `ask_question` | 向 NotebookLM 筆記本提問。支援 session 延續、筆記本選擇與瀏覽器選項覆寫。 |

### Session 管理
| Tool | 說明 |
|---|---|
| `list_sessions` | 列出所有作用中的瀏覽器 session，包括存活時間、訊息數與最後活動時間 |
| `close_session` | 依 session ID 關閉指定 session |
| `reset_session` | 重設某個 session 的聊天歷史（重新載入筆記本頁面） |

### 系統工具
| Tool | 說明 |
|---|---|
| `get_health` | 取得認證狀態、作用中 session 與設定摘要 |
| `setup_auth` | 開啟可見瀏覽器，手動登入 Google |
| `re_auth` | 切換帳號或重新驗證（會清除所有資料） |
| `cleanup_data` | 預覽並清除所有 MCP 本機資料 |

### Library 管理
| Tool | 說明 |
|---|---|
| `add_notebook` | 將 NotebookLM 筆記本加入本機持久化 library |
| `list_notebooks` | 列出所有筆記本與其中繼資料 |
| `get_notebook` | 取得單一筆記本詳細資料 |
| `select_notebook` | 設定目前的預設筆記本 |
| `update_notebook` | 更新筆記本中繼資料 |
| `remove_notebook` | 從 library 移除筆記本 |
| `search_notebooks` | 依名稱、描述、主題或標籤搜尋筆記本 |
| `get_library_stats` | 顯示 library 使用統計 |

### 每個工具是拿來做什麼的

| Tool | 使用時機 | 補充 |
|---|---|---|
| `ask_question` | 對 NotebookLM 發問，取得依來源內容整理的答案 | 支援 `session_id`、`notebook_id`、`notebook_url` 與瀏覽器參數覆寫 |
| `list_sessions` | 查看目前作用中的瀏覽器對話 session | 方便除錯或延續上下文 |
| `close_session` | 結束某個 session 並釋放資源 | 需要 `session_id` |
| `reset_session` | 保留同一個 session ID，但清空對話內容 | 會重新載入 notebook 頁面 |
| `get_health` | 檢查認證、設定與目前 session 狀態 | 很適合當第一個診斷工具 |
| `setup_auth` | 第一次登入 NotebookLM | 會開啟可見瀏覽器 |
| `re_auth` | 用相同或不同 Google 帳號重新登入 | 會清除舊認證並關閉所有 session |
| `cleanup_data` | 預覽或刪除本機 NotebookLM MCP 資料 | 支援 `confirm` 與 `preserve_library` |
| `add_notebook` | 把 NotebookLM 筆記本保存到本機 library | 需要筆記本中繼資料 |
| `list_notebooks` | 查看 library 中所有筆記本 | 會包含中繼資料與 URL |
| `get_notebook` | 查詢單一筆記本詳細資料 | 需要筆記本 `id` |
| `select_notebook` | 指定 `ask_question` 預設使用哪個筆記本 | 需要筆記本 `id` |
| `update_notebook` | 編輯已儲存的筆記本中繼資料 | 需要筆記本 `id` |
| `remove_notebook` | 從本機 library 中移除筆記本 | 不會刪除真正的 NotebookLM 筆記本 |
| `search_notebooks` | 依主題、名稱、標籤或描述搜尋筆記本 | library 變大後特別有用 |
| `get_library_stats` | 查看使用次數與目前活動筆記本狀態 | 適合掌握整體使用情況 |

### 建議操作順序

如果你是第一次使用，建議照這個順序：

1. `setup_auth`
2. `get_health`
3. `add_notebook`
4. `list_notebooks`
5. `select_notebook`
6. `ask_question`

平常使用時：

1. `get_health`
2. `list_notebooks` 或 `search_notebooks`
3. `ask_question`
4. 如有需要，再用 `list_sessions` / `reset_session` / `close_session` 管理 session

---

## 設定

### 環境變數

```bash
# NotebookLM
NOTEBOOK_URL=https://notebooklm.google.com/notebook/...

# Browser
HEADLESS=true                    # 是否以 headless 執行（預設：true）
BROWSER_TIMEOUT=30000            # timeout，單位毫秒

# Sessions
MAX_SESSIONS=10                  # 最大同時 session 數
SESSION_TIMEOUT=900              # session 閒置逾時秒數

# Auto-login（可選）
AUTO_LOGIN_ENABLED=false
LOGIN_EMAIL=you@gmail.com
LOGIN_PASSWORD=yourpassword

# Stealth（模擬人類行為）
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

### CLI 設定指令

```bash
# 顯示目前設定
./notebooklm-mcp-go config get

# 設定工具 profile
./notebooklm-mcp-go config set profile minimal
./notebooklm-mcp-go config set profile standard
./notebooklm-mcp-go config set profile full

# 停用特定工具
./notebooklm-mcp-go config set disabled-tools "cleanup_data,re_auth"

# 重設為預設值
./notebooklm-mcp-go config reset
```

設定會儲存在 `~/.config/notebooklm-mcp/settings.json`。

### CLI 設定會影響什麼

- `profile`：限制 MCP client 可見的工具。
- `disabled-tools`：即使 profile 原本允許，也會隱藏指定工具。
- 這些設定會影響 server 啟動時回傳的工具清單。

### Profiles

| Profile | 包含的工具 |
|---|---|
| `minimal` | ask_question、get_health、setup_auth |
| `standard` | 再加上 re_auth、list_sessions、close_session、list_notebooks、select_notebook、add_notebook |
| `full` | 全部 16 個工具（預設） |

---

## 資料路徑

| 平台 | 資料目錄 |
|---|---|
| Linux | `~/.local/share/notebooklm-mcp/` |
| macOS | `~/Library/Application Support/notebooklm-mcp/` |
| Windows | `%APPDATA%\notebooklm-mcp\` |

Library：`<data>/library.json`  
Auth state：`<data>/browser_state/state.json`  
Chrome profile：`<data>/chrome_profile/`

---

## 架構

```
你的任務
  └─► Claude / Codex / Cursor
        └─► MCP Server（JSON-RPC over stdio）
              └─► go-rod（Chrome DevTools Protocol）
                    └─► Chrome / Chromium
                          └─► NotebookLM（Gemini 2.5）
                                └─► 你的文件
```

### 專案結構

```
.
├── main.go                          # 入口點 + CLI config
├── internal/
│   ├── config/config.go             # 設定（環境變數 + 預設值）
│   ├── auth/auth_manager.go         # 瀏覽器 state / cookie 持久化
│   ├── library/
│   │   ├── types.go                 # Library 資料型別
│   │   └── library.go               # 持久化 notebook library（library.json）
│   ├── session/session.go           # 瀏覽器 session + session manager
│   ├── tools/
│   │   ├── definitions.go           # MCP tool 定義（schema + description）
│   │   └── handlers.go              # Tool 實作邏輯
│   ├── mcp/server.go                # JSON-RPC 2.0 MCP server（stdio transport）
│   └── utils/
│       ├── logger.go                # 輸出到 stderr 的 structured logging
│       └── settings.go              # Profile / tool filtering settings
└── go.mod
```

---

## 第一次設定

1. 編譯 binary：`go build -o notebooklm-mcp-go .`
2. 在你的 MCP client 中註冊它
3. 對 Claude 說：**`Set up NotebookLM authentication`** → 會呼叫 `setup_auth`
4. Chrome 視窗會開啟，登入你的 Google 帳號
5. 登入後，認證狀態會自動儲存到磁碟
6. 驗證：**`Check NotebookLM health`** → 會呼叫 `get_health`
7. 加入筆記本：**`Add my NotebookLM notebook`** → Claude 會引導你完成 `add_notebook`
8. 開始提問：**`Ask my notebook about X`** → 會呼叫 `ask_question`

## 常見工作流程

### 1. 第一次登入驗證

1. 透過 MCP client 啟動 server。
2. 請 agent 執行 `setup_auth`。
3. 在瀏覽器中登入 Google。
4. 請 agent 執行 `get_health`。
5. 確認 `authenticated=true`。

### 2. 加入筆記本並開始提問

1. 在 NotebookLM 建立或開啟一個筆記本。
2. 複製可分享的 URL。
3. 請 agent 透過 `add_notebook` 加入它。
4. 如有需要，再呼叫 `select_notebook`。
5. 之後就能透過 `ask_question` 提問。

### 3. 延續同一段對話

1. 先用 `ask_question` 問第一個問題。
2. 保存回傳的 `session_id`。
3. 後續問題帶同一個 `session_id`。
4. 如果想清空對話但保留 session，可使用 `reset_session`。

### 4. 疑難排解

- 如果 NotebookLM 回答怪怪的，先執行 `get_health`。
- 如果 Google 帳號換了，或登入失效，執行 `re_auth`。
- 如果瀏覽器或 session 狀態壞掉，先用 `cleanup_data` 預覽，再決定是否刪除。
- 如果清理資料時想保留筆記本 library，請設 `preserve_library=true`。

---

## 速率限制

免費 Google 帳號：**每天 50 次查詢**。達到上限時：

- `ask_question` 會回傳 `RateLimitError`
- 可用 `re_auth` 切換到另一個 Google 帳號
- 或等到隔天重置

升級到 **Google AI Pro/Ultra** 可獲得約 5 倍更高的限制。

---

## 授權

MIT，與原始 TypeScript 專案相同。
