package tools

import (
	"github.com/auto-code-os/auto-code-os/server/internal/sandbox"
	"github.com/auto-code-os/auto-code-os/server/internal/tool"
	"github.com/auto-code-os/auto-code-os/server/internal/tool/verify"
)

// DefaultRegistry constructs and registers all standard built-in tools.
func DefaultRegistry(runtime sandbox.Runtime, provider tool.AffectedFilesProvider) *tool.Registry {
	r := tool.NewRegistry()

	// 1. Filesystem Tools
	r.Register(&ReadFileTool{})
	r.Register(&ListFilesTool{})
	r.Register(&FileExistsTool{})

	// 2. Editing Tools (with verification pipeline)
	var pipeline *tool.VerifyPipeline
	if runtime != nil {
		pipeline = &tool.VerifyPipeline{
			Hooks: []tool.VerifyHook{
				verify.NewGofmtHook(runtime),
				verify.NewCompileCheckHook(runtime),
			},
		}
	}
	r.Register(NewSearchReplaceTool(pipeline))
	r.Register(NewCreateFileTool(pipeline))

	// 3. Search Tools
	r.Register(&GrepSearchTool{})
	r.Register(&FindSymbolTool{})

	// 4. Git Tools
	if runtime != nil {
		r.Register(NewGitDiffTool(runtime))
		r.Register(NewGitStatusTool(runtime))
		r.Register(NewGitCheckpointTool(runtime))
		r.Register(NewGitRestoreTool(runtime))
	}

	// 5. Build/Test Tools
	if runtime != nil {
		r.Register(NewRunTestsTool(runtime))
		r.Register(NewRunBuildTool(runtime))
		r.Register(NewRunLintTool(runtime))
	}

	// 6. Context Tools
	r.Register(&ReadSpecTool{})
	if provider != nil {
		r.Register(NewReadAffectedFilesTool(provider))
	}

	return r
}
