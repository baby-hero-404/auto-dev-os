// Command mcp-context is a local, stdio-only MCP server bundled into the CLI
// sandbox image. It bridges the AST/dependency-graph/skill engines that
// already power the API-native flow's context_load step to CLI coding agents
// (claude/codex/agy) that otherwise cannot call into the Go backend.
//
// It never opens a network listener — see
// docs/openspecs/cli-orchestrator-update/design.md, "Alternatives Considered".
package main

import (
	"context"
	"flag"
	"os"
	"strconv"

	"github.com/auto-code-os/auto-code-os/server/internal/mcpcontext"
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
)

func main() {
	root := flag.String("root", ".", "workspace root directory to index and serve context for")
	dbPath := flag.String("db", "", "path to the workspace AST cache SQLite db (defaults to <root>/context/workspace_cache.db)")
	contextDir := flag.String("context-dir", "", "materialized skills/rules directory for this run (defaults to $AUTOCODE_CONTEXT_DIR, see PromptAssembler.MaterializeCLIContext)")
	// logDir defaults to sandbox.LogsContainerDir — the fixed path
	// internal/sandbox/docker.go bind-mounts CommandRequest.LogsHostDir to
	// (Phase 6, "Docker Log Bind-Mount"). Overridable for local/non-sandbox
	// runs (e.g. the smoke tests in tasks.md Phase 1) where that mount
	// doesn't exist; NewAppLogger/OpenTraceWriter degrade gracefully to
	// stderr-only/disabled if the directory can't be created or opened.
	logDir := flag.String("log-dir", sandbox.LogsContainerDir, "directory for mcp-server.log and (if --trace) mcp-trace.jsonl; falls back to stderr-only logging if unwritable")
	trace := flag.Bool("trace", false, "dump every JSON-RPC request/response pair into <log-dir>/mcp-trace.jsonl (Context Payload Tracing)")
	flag.Parse()

	dbFile := *dbPath
	if dbFile == "" {
		dbFile = *root + "/context/workspace_cache.db"
	}
	ctxDir := *contextDir
	if ctxDir == "" {
		ctxDir = os.Getenv("AUTOCODE_CONTEXT_DIR")
	}
	traceEnabled := *trace
	if !traceEnabled {
		if v, err := strconv.ParseBool(os.Getenv("AUTOCODE_MCP_TRACE")); err == nil {
			traceEnabled = v
		}
	}

	errLog, closeLog := mcpcontext.NewAppLogger(*logDir, os.Stderr)
	defer closeLog()

	handlers, err := mcpcontext.NewHandlers(*root, dbFile, ctxDir)
	if err != nil {
		errLog.Fatalf("init handlers: %v", err)
	}

	srv := mcpcontext.NewServer("autocode-mcp-context", "0.1.0", errLog)
	handlers.RegisterAll(srv)

	if traceEnabled {
		if w, err := mcpcontext.OpenTraceWriter(*logDir); err != nil {
			errLog.Printf("trace mode requested but could not open trace file: %v (continuing without tracing)", err)
		} else if w != nil {
			defer w.Close()
			srv.SetTrace(w)
		}
	}

	if err := srv.Serve(context.Background(), os.Stdin, os.Stdout); err != nil {
		errLog.Fatalf("serve: %v", err)
	}
}
