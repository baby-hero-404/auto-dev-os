package mcpcontext

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"time"
)

// ToolHandler executes one MCP tool call and returns its text result.
type ToolHandler func(ctx context.Context, args json.RawMessage) (string, error)

// Tool bundles an MCP tool's advertised definition with its handler.
type Tool struct {
	Def     toolDefinition
	Handler ToolHandler
}

// Server is a minimal stdio JSON-RPC 2.0 MCP server. It intentionally
// implements only the subset of the MCP spec needed by claude/codex/agy to
// discover and call tools (initialize, tools/list, tools/call) — no
// resources/prompts/sampling support, since none of the 6 MVP tools need them.
type Server struct {
	Name    string
	Version string
	tools   map[string]Tool
	order   []string
	logger  *log.Logger
	trace   io.Writer
}

func NewServer(name, version string, errLog *log.Logger) *Server {
	return &Server{
		Name:    name,
		Version: version,
		tools:   make(map[string]Tool),
		logger:  errLog,
	}
}

// SetTrace enables Context Payload Tracing (Phase 6): every incoming
// JSON-RPC request and its outgoing response are dumped as one JSON object
// per line to w (conventionally logDir/mcp-trace.jsonl, opened via
// OpenTraceWriter). Passing nil disables tracing (the default). Must be
// called before Serve; not safe to change concurrently with a running Serve
// loop.
func (s *Server) SetTrace(w io.Writer) {
	s.trace = w
}

// traceEvent is one line of mcp-trace.jsonl — the request and its
// (possibly-nil, for notifications) response, so a single line captures a
// full round trip instead of needing to correlate two separate lines by ID.
type traceEvent struct {
	Time     time.Time       `json:"time"`
	Request  json.RawMessage `json:"request"`
	Response *rpcResponse    `json:"response,omitempty"`
}

// Register adds a tool. Call before Serve.
func (s *Server) Register(t Tool) {
	if _, exists := s.tools[t.Def.Name]; !exists {
		s.order = append(s.order, t.Def.Name)
	}
	s.tools[t.Def.Name] = t
}

// Serve runs the read-eval-print loop over stdio until EOF or a read error.
// One JSON-RPC message per line (newline-delimited JSON), matching the
// framing claude/codex/agy's stdio MCP clients use.
func (s *Server) Serve(ctx context.Context, r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	enc := json.NewEncoder(w)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			s.logf("invalid request: %v", err)
			continue
		}
		resp := s.handle(ctx, req)
		s.traceRoundTrip(line, resp)
		if resp == nil {
			// Notification (no id) — no response expected.
			continue
		}
		if err := enc.Encode(resp); err != nil {
			return fmt.Errorf("encode response: %w", err)
		}
	}
	return scanner.Err()
}

func (s *Server) handle(ctx context.Context, req rpcRequest) *rpcResponse {
	switch req.Method {
	case "initialize":
		return s.reply(req, initializeResult{
			ProtocolVersion: "2024-11-05",
			ServerInfo:      serverInfo{Name: s.Name, Version: s.Version},
			Capabilities:    map[string]any{"tools": map[string]any{}},
		}, nil)
	case "notifications/initialized":
		return nil
	case "tools/list":
		defs := make([]toolDefinition, 0, len(s.order))
		for _, name := range s.order {
			defs = append(defs, s.tools[name].Def)
		}
		return s.reply(req, toolsListResult{Tools: defs}, nil)
	case "tools/call":
		return s.handleToolCall(ctx, req)
	case "ping":
		return s.reply(req, map[string]any{}, nil)
	default:
		if len(req.ID) == 0 {
			return nil // unknown notification, ignore
		}
		return s.reply(req, nil, &rpcError{Code: -32601, Message: "method not found: " + req.Method})
	}
}

func (s *Server) handleToolCall(ctx context.Context, req rpcRequest) *rpcResponse {
	var params toolCallParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.reply(req, nil, &rpcError{Code: -32602, Message: "invalid params: " + err.Error()})
	}
	tool, ok := s.tools[params.Name]
	if !ok {
		return s.reply(req, toolCallResult{
			Content: []toolContent{{Type: "text", Text: "unknown tool: " + params.Name}},
			IsError: true,
		}, nil)
	}
	text, err := tool.Handler(ctx, params.Arguments)
	if err != nil {
		return s.reply(req, toolCallResult{
			Content: []toolContent{{Type: "text", Text: err.Error()}},
			IsError: true,
		}, nil)
	}
	return s.reply(req, toolCallResult{Content: []toolContent{{Type: "text", Text: text}}}, nil)
}

func (s *Server) reply(req rpcRequest, result any, rpcErr *rpcError) *rpcResponse {
	if len(req.ID) == 0 {
		return nil
	}
	return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: result, Error: rpcErr}
}

// traceRoundTrip writes one traceEvent line if tracing is enabled. Best-
// effort: a trace-write failure must never interrupt the JSON-RPC stream
// (this is a diagnostic aid, not part of the protocol contract), so errors
// are logged (to the app logger, i.e. stderr/mcp-server.log — never stdout)
// and otherwise swallowed.
func (s *Server) traceRoundTrip(rawReq json.RawMessage, resp *rpcResponse) {
	if s.trace == nil {
		return
	}
	// rawReq is reused across scanner.Scan() iterations by bufio.Scanner —
	// copy it before it's overwritten by the next line.
	reqCopy := make(json.RawMessage, len(rawReq))
	copy(reqCopy, rawReq)
	line, err := json.Marshal(traceEvent{Time: time.Now().UTC(), Request: reqCopy, Response: resp})
	if err != nil {
		s.logf("trace marshal failed: %v", err)
		return
	}
	line = append(line, '\n')
	if _, err := s.trace.Write(line); err != nil {
		s.logf("trace write failed: %v", err)
	}
}

func (s *Server) logf(format string, args ...any) {
	if s.logger != nil {
		s.logger.Printf(format, args...)
	}
}
