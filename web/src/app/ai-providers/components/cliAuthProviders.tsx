import type { ReactNode } from "react";

export interface CliAuthProviderConfig {
  title: string;
  /** Intro line shown above the terminal, before anything is detected. */
  description: ReactNode;
  /** Banner shown only while waiting for the auth URL (e.g. Codex's device-auth prerequisite). */
  preAuthWarning?: ReactNode;
  /** Command sent to the PTY once connected. Defaults to "<tool> login\n". */
  initialCommand?: string;
  /** Numbered steps shown inside the "please authenticate" banner. */
  steps: string[];
  /** Extra note rendered below the auth button/steps. */
  footerNote?: ReactNode;
  extractUrl: (rawBuf: string, cleanBuf: string) => string | null;
  extractCode?: (rawBuf: string, cleanBuf: string) => string | null;
  isSuccess: (rawBuf: string, cleanBuf: string) => boolean;
}

// Shared by CLIs (Claude, Codex) that print one of these plain-English
// success phrases; normalized to alphanumeric-only, lowercase, before match
// so spacing/punctuation differences between CLI versions don't matter.
function hasCommonSuccessPhrase(cleanBuf: string): boolean {
  const s = cleanBuf.replace(/[^a-zA-Z0-9]/g, "").toLowerCase();
  return (
    s.includes("loginsuccessful") ||
    s.includes("loggedinas") ||
    s.includes("authenticationcomplete") ||
    s.includes("authenticationsuccessful") ||
    s.includes("successfullyloggedin")
  );
}

export const CLI_AUTH_PROVIDERS: Record<string, CliAuthProviderConfig> = {
  "cli:claude": {
    title: "Connect Claude CLI",
    description: (
      <>
        The system will automatically run <code>claude login</code> for you, and automatically capture the session once authentication is successful.
      </>
    ),
    steps: [
      "Authenticate in your browser",
      "Copy the provided authorization code",
      "Paste the code into the terminal below (use Ctrl+Shift+V or Right-click) and press Enter",
    ],
    // Claude CLI may use OSC 8 hyperlinks (try first) or plain text.
    // Falls back to a simple regex on the ANSI-stripped buffer.
    extractUrl: (rawBuf, cleanBuf) => {
      const oscMatch = rawBuf.match(/\x1b\]8;[^;]*;(https:\/\/(?:claude\.ai|claude\.com|anthropic\.com|auth\.anthropic\.com)[^\x07\x1b]*)/);
      if (oscMatch) return oscMatch[1];
      const textMatch = cleanBuf.match(/https:\/\/(?:claude\.ai|claude\.com|anthropic\.com|auth\.anthropic\.com)[^\s\x00-\x1F\x7F]*/);
      return textMatch ? textMatch[0] : null;
    },
    // Claude CLI prints "Logged in as" or similar on success.
    isSuccess: (_rawBuf, cleanBuf) => hasCommonSuccessPhrase(cleanBuf),
  },

  "cli:codex": {
    title: "Connect Codex CLI",
    description: (
      <>
        Codex uses a headless login flow. The system will automatically run <code>codex login</code> for you. If prompted for a device code, copy it from the terminal below.
      </>
    ),
    preAuthWarning: (
      <>
        Note: You must enable device code authentication for Codex in your <span className="underline font-bold">ChatGPT Security Settings</span>, then rerun the command &quot;codex login --device-auth&quot;.
        <br />
        <span className="text-xs opacity-80">(Bật tính năng xác thực mã thiết bị cho Codex trong Cài đặt bảo mật ChatGPT, sau đó chạy lại lệnh &quot;codex login --device-auth&quot;)</span>
      </>
    ),
    initialCommand: "codex login --device-auth\n",
    steps: [
      "Copy the Device Code above",
      "Click the authentication button to open your browser",
      "Paste the code into the browser page to complete login",
    ],
    // Codex CLI outputs plain text (no OSC 8 hyperlinks).
    // Match the auth URL directly from the ANSI-stripped buffer.
    extractUrl: (_rawBuf, cleanBuf) => {
      const m = cleanBuf.match(/https:\/\/(?:auth\.openai\.com|openai\.com)[^\s\x00-\x1F\x7F]*/);
      return m ? m[0] : null;
    },
    extractCode: (_rawBuf, cleanBuf) => {
      const codeRegex = /(?:Enter this one-time code[^\n]*\n\s*([A-Z0-9]{4}-[A-Z0-9]{4,5})|enter the code ([A-Z0-9-]+) to authenticate)/i;
      const match = cleanBuf.match(codeRegex);
      return match ? (match[1] || match[2]) : null;
    },
    isSuccess: (_rawBuf, cleanBuf) => hasCommonSuccessPhrase(cleanBuf),
  },

  "cli:antigravity": {
    title: "Connect Antigravity CLI",
    description: (
      <>
        The system will automatically run <code>agy</code> for you, and capture the session once authentication is successful.
      </>
    ),
    steps: [],
    footerNote: (
      <>
        After authenticating, the session will be captured automatically. <br />
        <span className="text-content-muted mt-1 inline-block">If the terminal doesn&apos;t close on its own, type <code>exit</code> or press Ctrl+D.</span>
      </>
    ),
    // Extract URL from OSC 8 hyperlink metadata (ESC]8;params;URI BEL).
    // Bubbletea wraps each visual terminal line in its own OSC 8 hyperlink
    // where the URI parameter is always the COMPLETE, unwrapped URL.
    extractUrl: (rawBuf) => {
      const m = rawBuf.match(/\x1b\]8;[^;]*;(https:\/\/accounts\.google\.com[^\x07\x1b]*)/);
      return m ? m[1] : null;
    },
    // Antigravity TUI (Bubbletea) shows its version banner when login succeeds.
    isSuccess: (_rawBuf, cleanBuf) => {
      const s = cleanBuf.replace(/[^a-zA-Z0-9]/g, "").toLowerCase();
      return s.includes("antigravitycli1");
    },
  },
};
