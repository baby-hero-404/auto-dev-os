export function parseLiveAction(message: string): { action: string; target: string } | null {
  const lower = message.toLowerCase();
  
  // 1. Editing files
  if (lower.includes("search_replace") || lower.includes("create_file") || lower.includes("write_to_file") || lower.includes("replace_file_content") || lower.includes("multi_replace")) {
    const fileMatch = message.match(/(?:file|path|TargetFile)[:\s'"]+([a-zA-Z0-9_\-\.\/\\:]+)/i) || 
                      message.match(/on\s+([a-zA-Z0-9_\-\.\/\\:]+)/i) ||
                      message.match(/([a-zA-Z0-9_\-\.\/\\:]+\.[a-zA-Z0-9_]+)/i);
    const file = fileMatch ? fileMatch[1].split("/").pop() || fileMatch[1] : "file";
    return { action: "Editing", target: file };
  }
  
  // 2. Running tools/commands
  if (lower.includes("run_tests") || lower.includes("run_build") || lower.includes("run_lint") || lower.includes("run_command") || lower.includes("execute")) {
    const toolMatch = message.match(/(?:tool|command|CommandLine)[:\s'"]+([a-zA-Z0-9_\-\.\/\\: ]+)/i) ||
                      message.match(/running\s+([a-zA-Z0-9_\-\.\/\\: ]+)/i);
    const tool = toolMatch ? toolMatch[1].trim() : "tests/build";
    return { action: "Running", target: tool };
  }
  
  // 3. Reading files
  if (lower.includes("read_file") || lower.includes("view_file") || lower.includes("list_files") || lower.includes("read_url")) {
    const fileMatch = message.match(/(?:file|path|AbsolutePath)[:\s'"]+([a-zA-Z0-9_\-\.\/\\:]+)/i) ||
                      message.match(/([a-zA-Z0-9_\-\.\/\\:]+\.[a-zA-Z0-9_]+)/i);
    const file = fileMatch ? fileMatch[1].split("/").pop() || fileMatch[1] : "file";
    return { action: "Reading", target: file };
  }
  
  return null;
}
