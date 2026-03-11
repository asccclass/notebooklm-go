// Package mcp implements a Model Context Protocol server over stdin/stdout.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
)

// ─── JSON-RPC primitives ─────────────────────────────────────────────────────

type rawMessage = json.RawMessage

// Request is a JSON-RPC 2.0 request.
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response is a JSON-RPC 2.0 response.
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

// Notification is a JSON-RPC 2.0 notification (no ID).
type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// RPCError represents a JSON-RPC error.
type RPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func (e *RPCError) Error() string { return e.Message }

// Standard JSON-RPC error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// ─── MCP types ───────────────────────────────────────────────────────────────

// ServerInfo describes the MCP server.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ClientInfo is provided by the client during initialisation.
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Capabilities describes what the server supports.
type Capabilities struct {
	Tools     map[string]interface{} `json:"tools,omitempty"`
	Resources map[string]interface{} `json:"resources,omitempty"`
	Prompts   map[string]interface{} `json:"prompts,omitempty"`
	Logging   map[string]interface{} `json:"logging,omitempty"`
}

// InitializeParams is the payload for the "initialize" request.
type InitializeParams struct {
	ProtocolVersion string      `json:"protocolVersion"`
	ClientInfo      ClientInfo  `json:"clientInfo"`
	Capabilities    interface{} `json:"capabilities,omitempty"`
}

// InitializeResult is returned after successful initialisation.
type InitializeResult struct {
	ProtocolVersion string       `json:"protocolVersion"`
	ServerInfo      ServerInfo   `json:"serverInfo"`
	Capabilities    Capabilities `json:"capabilities"`
}

// Tool is an MCP tool definition.
type Tool struct {
	Name        string      `json:"name"`
	Title       string      `json:"title,omitempty"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema is a JSON-Schema fragment.
type InputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	Required   []string               `json:"required,omitempty"`
}

// CallToolParams carries the tool invocation parameters.
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ToolResult is what a tool call returns.
type ToolResult struct {
	Content []ContentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// ContentItem is one piece of tool output.
type ContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// TextContent creates a plain-text ContentItem.
func TextContent(text string) ContentItem {
	return ContentItem{Type: "text", Text: text}
}

// ─── Handler interface ────────────────────────────────────────────────────────

// Handler is implemented by the application layer to handle tool calls.
type Handler interface {
	ListTools() []Tool
	CallTool(ctx context.Context, name string, args json.RawMessage, sendProgress func(string)) (*ToolResult, error)
}

// ─── Server ───────────────────────────────────────────────────────────────────

// Server is an MCP server that communicates over stdin/stdout.
type Server struct {
	info    ServerInfo
	handler Handler
	in      *bufio.Scanner
	out     *json.Encoder
	mu      chan struct{} // write mutex (buffered 1)
}

// New creates an MCP Server.
func New(info ServerInfo, handler Handler) *Server {
	enc := json.NewEncoder(os.Stdout)
	return &Server{
		info:    info,
		handler: handler,
		in:      bufio.NewScanner(os.Stdin),
		out:     enc,
		mu:      make(chan struct{}, 1),
	}
}

// Run reads JSON-RPC messages from stdin and dispatches them until EOF.
func (s *Server) Run(ctx context.Context) error {
	slog.Info("🚀 MCP server started", "name", s.info.Name, "version", s.info.Version)
	s.in.Buffer(make([]byte, 1024*1024), 1024*1024)

	for s.in.Scan() {
		line := s.in.Bytes()
		if len(line) == 0 {
			continue
		}

		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			s.sendError(nil, CodeParseError, "parse error", err.Error())
			continue
		}

		go s.dispatch(ctx, req)
	}

	if err := s.in.Err(); err != nil && err != io.EOF {
		return err
	}
	return nil
}

// dispatch handles a single JSON-RPC request.
func (s *Server) dispatch(ctx context.Context, req Request) {
	slog.Debug("→ RPC", "method", req.Method, "id", req.ID)

	switch req.Method {
	case "initialize":
		s.handleInitialize(req)
	case "initialized":
		// Notification, no response needed.
	case "tools/list":
		s.handleListTools(req)
	case "tools/call":
		s.handleCallTool(ctx, req)
	case "ping":
		s.sendResult(req.ID, map[string]string{})
	default:
		if req.ID != nil {
			s.sendError(req.ID, CodeMethodNotFound, "method not found", req.Method)
		}
	}
}

func (s *Server) handleInitialize(req Request) {
	result := InitializeResult{
		ProtocolVersion: "2024-11-05",
		ServerInfo:      s.info,
		Capabilities: Capabilities{
			Tools:   map[string]interface{}{},
			Logging: map[string]interface{}{},
		},
	}
	s.sendResult(req.ID, result)
}

func (s *Server) handleListTools(req Request) {
	tools := s.handler.ListTools()
	s.sendResult(req.ID, map[string]interface{}{"tools": tools})
}

func (s *Server) handleCallTool(ctx context.Context, req Request) {
	var params CallToolParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		s.sendError(req.ID, CodeInvalidParams, "invalid params", err.Error())
		return
	}

	// Progress sender (uses JSON-RPC notifications).
	sendProgress := func(msg string) {
		s.sendNotification("notifications/progress", map[string]interface{}{
			"progressToken": req.ID,
			"progress":      msg,
		})
	}

	result, err := s.handler.CallTool(ctx, params.Name, params.Arguments, sendProgress)
	if err != nil {
		result = &ToolResult{
			Content: []ContentItem{TextContent(fmt.Sprintf("Error: %v", err))},
			IsError: true,
		}
	}
	s.sendResult(req.ID, result)
}

// ─── Send helpers ─────────────────────────────────────────────────────────────

func (s *Server) write(v interface{}) {
	s.mu <- struct{}{}
	defer func() { <-s.mu }()
	if err := s.out.Encode(v); err != nil {
		slog.Error("write error", "err", err)
	}
}

func (s *Server) sendResult(id interface{}, result interface{}) {
	s.write(Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	})
}

func (s *Server) sendError(id interface{}, code int, message, data string) {
	s.write(Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &RPCError{Code: code, Message: message, Data: data},
	})
}

func (s *Server) sendNotification(method string, params interface{}) {
	s.write(Notification{
		JSONRPC: "2.0",
		Method:  method,
		Params:  params,
	})
}
