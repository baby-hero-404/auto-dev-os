package engine

import (
	"encoding/json"
	"fmt"

	"github.com/auto-code-os/auto-code-os/server/pkg/models"
	"github.com/auto-code-os/auto-code-os/server/pkg/paths"
)

// mcpContextBinary is the stdio MCP server bundled into docker/Dockerfile.sandbox
// (see the mcpbuilder stage), bridging internal/context/* to the CLI agent.
const mcpContextBinary = "/usr/local/bin/mcp-context"

type mcpServerEntry struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
}

type mcpServersFile struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

// mcpBootstrap holds the shell fragments RunCodeStep splices into its script
// to wire up autocode-mcp-context for the CLI in use, keyed off
// CLIEngineConfig.ProfileRef (set by the Execution Router). Unknown/custom
// profiles get no MCP wiring — a custom CLI's config format isn't known.
type mcpBootstrap struct {
	prefix    string   // shell commands to run before the CLI invocation
	cleanup   string   // shell commands to run after (best-effort, own exit code ignored)
	extraArgs []string // appended to the CLI's invocation, e.g. claude's --mcp-config
}

// buildMCPBootstrap returns the (possibly empty) shell wiring needed for cfg's
// CLI to discover the mcp-context server, per
// docs/guides/headless-cli-tools.md#mcp-server-configuration.
func buildMCPBootstrap(cfg *models.CLIEngineConfig, containerWorkDir, autocodeDir string) mcpBootstrap {
	mcpArgs := []string{"--root", containerWorkDir, "--context-dir", autocodeDir + "/context"}
	entry := mcpServerEntry{Command: mcpContextBinary, Args: mcpArgs}

	switch cfg.ProfileRef {
	case "claude_code":
		cfgPath := autocodeDir + "/mcp-claude.json"
		body, _ := json.Marshal(mcpServersFile{MCPServers: map[string]mcpServerEntry{"autocode-context": entry}})
		return mcpBootstrap{
			prefix:    heredocWrite(cfgPath, string(body)),
			extraArgs: []string{"--mcp-config", cfgPath},
		}
	case "openai_codex":
		block := fmt.Sprintf("\n[mcp_servers.autocode-context]\ncommand = %s\nargs = [%s, %s, %s, %s]\n",
			tomlString(entry.Command), tomlString(mcpArgs[0]), tomlString(mcpArgs[1]), tomlString(mcpArgs[2]), tomlString(mcpArgs[3]))
		return mcpBootstrap{
			prefix: fmt.Sprintf(
				"mkdir -p ~/.codex; grep -q 'mcp_servers.autocode-context' ~/.codex/config.toml 2>/dev/null || cat >> ~/.codex/config.toml <<'AUTOCODE_MCP_EOF'%sAUTOCODE_MCP_EOF\n",
				block,
			),
		}
	case "antigravity":
		// Workspace-level mcp_config.json at the project root, per Antigravity
		// CLI's documented config surface. Only written if absent (never
		// clobber a repo-committed config) and removed again afterward if we
		// were the ones who created it, so it never ends up committed.
		//
		// Git-exclude guard: append mcp_config.json* to .git/info/exclude so
		// that even if the CLI agent runs `git add .` mid-session the config
		// file (and its .autocode_created sentinel) can never be staged.
		// Using .git/info/exclude (not .gitignore) keeps this invisible to the
		// repo — it's a local, per-worktree-only ignore, never committed.
		cfgPath := containerWorkDir + "/mcp_config.json"
		body, _ := json.Marshal(mcpServersFile{MCPServers: map[string]mcpServerEntry{"autocode-context": entry}})
		quoted := paths.QuoteShellArg(cfgPath)
		gitExcludeGuard := fmt.Sprintf(
			"mkdir -p %s/.git/info && grep -qxF 'mcp_config.json*' %s/.git/info/exclude 2>/dev/null || echo 'mcp_config.json*' >> %s/.git/info/exclude\n",
			paths.QuoteShellArg(containerWorkDir),
			paths.QuoteShellArg(containerWorkDir),
			paths.QuoteShellArg(containerWorkDir),
		)
		return mcpBootstrap{
			prefix: gitExcludeGuard + fmt.Sprintf("if [ ! -f %s ]; then %s touch %s.autocode_created; fi\n",
				quoted, heredocWrite(cfgPath, string(body)), quoted),
			cleanup: fmt.Sprintf("[ -f %s.autocode_created ] && rm -f %s %s.autocode_created\n", quoted, quoted, quoted),
		}
	default:
		return mcpBootstrap{}
	}
}

func heredocWrite(path, content string) string {
	return fmt.Sprintf("cat > %s <<'AUTOCODE_MCP_EOF'\n%s\nAUTOCODE_MCP_EOF\n", paths.QuoteShellArg(path), content)
}

// tomlString renders a Go string as a double-quoted TOML basic string. Inputs
// here are always our own generated paths (no user input reaches this), so a
// minimal escape of the two characters TOML basic strings forbid unescaped is
// sufficient.
func tomlString(s string) string {
	out := make([]byte, 0, len(s)+2)
	out = append(out, '"')
	for _, r := range s {
		if r == '"' || r == '\\' {
			out = append(out, '\\')
		}
		out = append(out, string(r)...)
	}
	out = append(out, '"')
	return string(out)
}
