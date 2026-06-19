import os
import re

file_path = "server/internal/orchestrator/orchestrator.go"
with open(file_path, "r") as f:
    content = f.read()

# Define the lists of functions to extract
workspace_funcs = [
    "StartWorkspacePruner", "ensureWorkspaceCloned", "cleanupWorkspaceAfterFinalState",
    "pruneWorkspaces", "removeWorkspace", "resetExistingWorkspace", "StartLogPruner", "pruneLogFiles"
]

steps_funcs = [
    "stepRunners", "getSuccessfulCheckpoint", "getSavedPatch", "withCheckpointRecovery",
    "runLLMStep", "runSandboxStep", "deriveWorkflowAnalysis", "parseJSONMarkdown",
    "applyPatch", "captureWorkspaceDiff", "saveArtifact", "extractPatch", "deriveChangeName",
    "taskReadyForExecution"
]

worker_funcs = [
    "StartWorker", "Wait", "run", "fail", "checkpoint", "log", "updateTaskStatus"
]

def extract_funcs(src, func_names):
    extracted = []
    remaining = src
    
    for fn in func_names:
        # Regex to match func (recv) Name(...) or func Name(...)
        pattern = re.compile(r"^func (?:\([^)]+\)\s+)?" + fn + r"\(.*?\{", re.MULTILINE)
        match = pattern.search(remaining)
        if not match:
            print(f"Warning: function {fn} not found!")
            continue
            
        start_idx = match.start()
        
        # Count braces to find the end
        brace_count = 0
        in_string = False
        string_char = ''
        in_comment = False
        in_line_comment = False
        
        end_idx = -1
        i = start_idx
        while i < len(remaining):
            c = remaining[i]
            
            # Simplified string handling
            if in_line_comment:
                if c == '\n':
                    in_line_comment = False
            elif in_string:
                if c == '\\':
                    i += 1 # skip next char
                elif c == string_char:
                    in_string = False
            else:
                if c == '"' or c == '`' or c == "'":
                    in_string = True
                    string_char = c
                elif c == '/' and i+1 < len(remaining) and remaining[i+1] == '/':
                    in_line_comment = True
                    i += 1
                elif c == '{':
                    brace_count += 1
                elif c == '}':
                    brace_count -= 1
                    if brace_count == 0:
                        end_idx = i + 1
                        break
            i += 1
            
        if end_idx != -1:
            extracted.append(remaining[start_idx:end_idx])
            remaining = remaining[:start_idx] + remaining[end_idx:]
            
    return remaining, extracted

# Do extraction
rem1, ws_blocks = extract_funcs(content, workspace_funcs)
rem2, step_blocks = extract_funcs(rem1, steps_funcs)
final_rem, worker_blocks = extract_funcs(rem2, worker_funcs)

# We need imports for the new files. We can just copy the same imports from orchestrator.go for simplicity,
# or let goimports fix it. Since we might not have goimports, we'll just copy the top package and imports block.
header_match = re.search(r"package orchestrator\s*\n\s*import \(\n.*?\n\)\n", content, re.DOTALL)
if header_match:
    header = header_match.group(0)
else:
    # Fallback if imports are single line or grouped differently
    header = "package orchestrator\n\nimport (\n\t\"context\"\n\t\"encoding/json\"\n\t\"errors\"\n\t\"fmt\"\n\t\"log/slog\"\n\t\"os\"\n\t\"os/exec\"\n\t\"path/filepath\"\n\t\"regexp\"\n\t\"strings\"\n\t\"sync\"\n\t\"time\"\n\n\t\"github.com/auto-code-os/auto-code-os/server/internal/observability\"\n\t\"github.com/auto-code-os/auto-code-os/server/internal/repository\"\n\t\"github.com/auto-code-os/auto-code-os/server/internal/sandbox\"\n\t\"github.com/auto-code-os/auto-code-os/server/internal/workflow\"\n\t\"github.com/auto-code-os/auto-code-os/server/pkg/llm\"\n\t\"github.com/auto-code-os/auto-code-os/server/pkg/models\"\n\t\"go.opentelemetry.io/otel\"\n\t\"gorm.io/gorm\"\n)\n\n"

def write_file(filename, blocks):
    if blocks:
        with open(filename, "w") as f:
            f.write(header + "\n" + "\n\n".join(blocks) + "\n")

write_file("server/internal/orchestrator/orchestrator_workspace.go", ws_blocks)
write_file("server/internal/orchestrator/orchestrator_steps.go", step_blocks)
write_file("server/internal/orchestrator/orchestrator_worker.go", worker_blocks)

with open("server/internal/orchestrator/orchestrator.go", "w") as f:
    f.write(final_rem)

print("Split completed successfully.")
