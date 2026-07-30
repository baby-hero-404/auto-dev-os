import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";
import { api } from "@/lib/api";

// Must match resizeSentinel in server/internal/handler/cli_auth.go. Starts
// with a NUL byte, which xterm.js's onData stream never produces from real
// keystrokes/paste, so it can't collide with actual stdin content sent over
// the same WS connection.
const RESIZE_SENTINEL = "\x00RESIZE:";

interface InteractiveTerminalProps {
  orgID: string;
  token: string;
  provider: string;
  mode?: "auth" | "test";
  credentialID?: string;
  initialCommand?: string;
  /**
   * Provider-specific URL extractor. Receives the raw PTY buffer and an
   * ANSI-stripped copy. Return the URL string, or null if not yet found.
   * Each AuthFlow component defines its own logic so providers never
   * interfere with each other.
   */
  extractUrl?: (rawBuf: string, cleanBuf: string) => string | null;
  /**
   * Provider-specific one-time code extractor (e.g. device auth codes).
   * Return the code string, or null if not yet found.
   */
  extractCode?: (rawBuf: string, cleanBuf: string) => string | null;
  /**
   * Provider-specific success detector. Return true when the auth flow
   * has completed successfully and the terminal should begin its exit sequence.
   */
  isSuccess?: (rawBuf: string, cleanBuf: string) => boolean;
  onExit: (payload: Record<string, string>) => void;
  onError?: (error: string) => void;
  onUrlFound?: (url: string) => void;
  onCodeFound?: (code: string) => void;
  onSuccessFound?: () => void;
}

export function InteractiveTerminal({ orgID, token, provider, mode = "auth", credentialID, initialCommand, extractUrl, extractCode, isSuccess, onExit, onError, onUrlFound, onCodeFound, onSuccessFound }: InteractiveTerminalProps) {
  const terminalRef = useRef<HTMLDivElement>(null);
  const stdoutBuffer = useRef("");
  const term = useRef<Terminal | null>(null);
  const socket = useRef<WebSocket | null>(null);
  const fitAddon = useRef<FitAddon | null>(null);
  const lastSize = useRef<{ cols: number; rows: number } | null>(null);
  const exitIntervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const [isConnected, setIsConnected] = useState(false);

  // @xterm/addon-fit clamps its own output to a minimum of 2 cols / 1 row,
  // which it *will* return if it measures the container before layout/font
  // metrics have settled (e.g. the very first fit on mount). Since every
  // resize we compute gets forwarded to the real PTY via ContainerResize,
  // and that size sticks until the next resize event, a single bad
  // measurement would otherwise permanently wedge the session at ~2 columns
  // (every CLI writing one character per line). Refuse to propagate
  // anything implausibly small; a real terminal container is never this
  // narrow in practice.
  const MIN_PLAUSIBLE_COLS = 20;
  const MIN_PLAUSIBLE_ROWS = 5;

  const sendResize = (cols: number, rows: number) => {
    if (cols < MIN_PLAUSIBLE_COLS || rows < MIN_PLAUSIBLE_ROWS) {
      console.warn(`Ignoring implausible terminal resize ${cols}x${rows}`);
      return;
    }
    lastSize.current = { cols, rows };
    if (socket.current?.readyState === WebSocket.OPEN) {
      socket.current.send(`${RESIZE_SENTINEL}${cols}:${rows}`);
    }
  };

  // Keep the latest callbacks in refs so the connect effect below only depends
  // on [orgID, token, provider] and isn't torn down/reconnected by unrelated
  // parent re-renders (the ws ticket it mints is single-use, so reconnecting
  // without a fresh ticket would fail).
  const onExitRef = useRef(onExit);
  const onErrorRef = useRef(onError);
  useEffect(() => {
    onExitRef.current = onExit;
  }, [onExit]);
  useEffect(() => {
    onErrorRef.current = onError;
  }, [onError]);

  useEffect(() => {
    if (!terminalRef.current) return;
    let cancelled = false;

    term.current = new Terminal({
      cursorBlink: true,
      theme: {
        background: "#000000",
        foreground: "#ffffff",
      },
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      fontSize: 13,
      cols: 80,
      rows: 24,
    });
    fitAddon.current = new FitAddon();
    term.current.loadAddon(fitAddon.current);
    term.current.open(terminalRef.current);
    term.current.focus();
    setTimeout(() => {
      try {
        if (terminalRef.current && terminalRef.current.clientWidth > 0) {
          fitAddon.current?.fit();
        }
      } catch (e) {
        console.warn("xterm initial fit error", e);
      }
    }, 50);

    // Whenever the terminal's actual cols/rows change (from fit() or
    // otherwise), tell the backend PTY so full-screen TUIs (ratatui/ink-style
    // agent UIs, vim, etc.) render at the real size instead of assuming a
    // fixed 80x24.
    const resizeSub = term.current.onResize(({ cols, rows }) => sendResize(cols, rows));

    const resizeObserver = new ResizeObserver(() => {
      requestAnimationFrame(() => {
        try {
          if (terminalRef.current && terminalRef.current.clientWidth > 0) {
            fitAddon.current?.fit();
          }
        } catch (e) {
          console.warn("xterm resize fit error", e);
        }
      });
    });
    resizeObserver.observe(terminalRef.current);

    // xterm.js doesn't answer DECRQM queries for modes it doesn't implement
    // (e.g. 2026 synchronized-output, 2027 grapheme-clustering). Some CLIs
    // (Antigravity's bubbletea-based TUI) send these on startup and block
    // forever waiting for a response before drawing anything, which looks
    // like a black screen. Answer with "not recognized" (Ps=0) so they stop
    // waiting and fall back to not using the feature, like a real terminal
    // that doesn't support the mode would.
    const decrqmHandler = term.current.parser.registerCsiHandler(
      { prefix: "?", intermediates: "$", final: "p" },
      (params) => {
        const mode = params[0];
        if (socket.current?.readyState === WebSocket.OPEN) {
          socket.current.send(`\x1b[?${mode};0$y`);
        }
        return true;
      }
    );

    setTimeout(() => {
      try {
        term.current?.focus();
      } catch (e) {
        console.warn("xterm focus error", e);
      }
    }, 10);

    // Reconnecting here can't just resume a stream like log-tailing does —
    // each WS ticket is single-use and reconnecting spawns a brand-new PTY
    // session in the sandbox, discarding any in-progress login state (e.g.
    // mid-OAuth-device-code). So unlike streamLogs' unbounded backoff loop,
    // this retries a small, bounded number of times (transient network
    // blips only) before surfacing the drop via onError so the caller can
    // offer the user an explicit restart instead of silently respawning
    // CLI processes forever.
    const MAX_RECONNECT_ATTEMPTS = 3;
    let reconnectAttempt = 0;
    let retryDelay = 1000;

    const connect = async () => {
      try {
        let ticket = "";
        if (mode === "auth") {
          const res = await api.mintCliAuthWSTicket(orgID, token, provider);
          ticket = res.ticket;
        } else if (mode === "test" && credentialID) {
          const res = await api.mintCliTestWSTicket(orgID, token, credentialID);
          ticket = res.ticket;
        } else {
          return;
        }

        if (cancelled) return;

        const apiUrl = process.env.NEXT_PUBLIC_API_URL || "http://localhost:32080/api/v1";
        const wsBaseUrl = apiUrl.replace(/^http/, "ws");
        const wsUrl = mode === "auth"
          ? `${wsBaseUrl}/organizations/${orgID}/cli-auth/terminal?provider=${provider}&ticket=${ticket}`
          : `${wsBaseUrl}/organizations/${orgID}/cli-test/terminal?ticket=${ticket}`;

        const ws = new WebSocket(wsUrl);
        socket.current = ws;

        ws.onopen = () => {
          if (!cancelled) {
            setIsConnected(true);
            stdoutBuffer.current = "";
            term.current?.focus();
            // Re-measure now that the terminal is confirmed mounted and
            // visible, in case the mount-time fit() ran before layout/font
            // metrics were ready (see sendResize's plausibility guard
            // above). This fires onResize -> sendResize again if the size
            // actually changed, correcting any earlier bad guess before the
            // PTY has done anything size-sensitive.
            try {
              fitAddon.current?.fit();
            } catch (e) {
              console.warn("xterm connect-time fit error", e);
            }
            if (lastSize.current) {
              sendResize(lastSize.current.cols, lastSize.current.rows);
            }
            setTimeout(() => {
              if (ws.readyState === WebSocket.OPEN) {
                if (initialCommand) {
                  ws.send(initialCommand);
                } else if (mode === "auth" && provider.startsWith("cli:")) {
                  let toolName = provider.replace("cli:", "");
                  const cmd = "login";
                  if (toolName === "antigravity") {
                    toolName = "agy";
                  }
                  ws.send(`${toolName} ${cmd}\n`);
                }
              }
            }, 500);
          }
        };

        let hasExited = false;
        ws.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            if (data.type === "stdout" && term.current) {
              term.current.write(data.data);

              if (!hasExited) {
                stdoutBuffer.current += data.data;
                if (stdoutBuffer.current.length > 5000) {
                  stdoutBuffer.current = stdoutBuffer.current.slice(-5000);
                }
                const cleanStr = stdoutBuffer.current.replace(/\x1b\[[0-9;?]*[a-zA-Z]/g, "");

                if (onUrlFound && extractUrl) {
                  const url = extractUrl(stdoutBuffer.current, cleanStr);
                  if (url) onUrlFound(url);
                }

                if (onCodeFound && extractCode) {
                  const code = extractCode(stdoutBuffer.current, cleanStr);
                  if (code) onCodeFound(code);
                }

                if (onSuccessFound && isSuccess) {
                  if (isSuccess(stdoutBuffer.current, cleanStr)) {
                    hasExited = true;
                    onSuccessFound();

                    let step = 0;
                    exitIntervalRef.current = setInterval(() => {
                      if (ws.readyState === WebSocket.OPEN) {
                        if (step === 0) ws.send("\x03\r");
                        else if (step === 1) ws.send("/exit\r");
                        else if (step === 2) ws.send("\x04");
                        else if (step === 3) ws.send("\x04");
                        else if (step === 4) ws.send("exit\r");
                      }
                      step++;
                      if (step >= 5 && exitIntervalRef.current) {
                        clearInterval(exitIntervalRef.current);
                        exitIntervalRef.current = null;
                      }
                    }, 1000);
                  }
                }
              }
            } else if (data.type === "exit") {
              onExitRef.current(data.payload);
            } else if (data.type === "error") {
              onErrorRef.current?.(data.message);
            }
          } catch (e) {
            console.error("Failed to parse websocket message", e);
          }
        };

        ws.onerror = (e) => {
          console.error("WebSocket Error", e);
        };

        ws.onclose = () => {
          if (cancelled) return;
          setIsConnected(false);
          if (hasExited) return;
          if (reconnectAttempt >= MAX_RECONNECT_ATTEMPTS) {
            onErrorRef.current?.("WebSocket connection lost");
            return;
          }
          reconnectAttempt += 1;
          const delay = retryDelay;
          retryDelay = Math.min(retryDelay * 2, 5000);
          setTimeout(() => {
            if (!cancelled) connect();
          }, delay);
        };

        term.current?.onData((data) => {
          if (socket.current?.readyState === WebSocket.OPEN) {
            socket.current.send(data);
          }
        });
      } catch (err) {
        if (cancelled) return;
        if (reconnectAttempt < MAX_RECONNECT_ATTEMPTS) {
          reconnectAttempt += 1;
          const delay = retryDelay;
          retryDelay = Math.min(retryDelay * 2, 5000);
          setTimeout(() => {
            if (!cancelled) connect();
          }, delay);
          return;
        }
        onErrorRef.current?.(err instanceof Error ? err.message : "Failed to start terminal session");
      }
    };

    connect();

    return () => {
      cancelled = true;
      resizeObserver.disconnect();
      resizeSub.dispose();
      decrqmHandler.dispose();
      if (exitIntervalRef.current) {
        clearInterval(exitIntervalRef.current);
        exitIntervalRef.current = null;
      }
      if (socket.current) {
        socket.current.close();
      }
      if (term.current) {
        term.current.dispose();
      }
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div className="w-full h-full relative overflow-hidden bg-black rounded-md border border-stroke">
      {!isConnected && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/80 z-10 text-white text-sm">
          Connecting to sandbox...
        </div>
      )}
      <div ref={terminalRef} className="w-full h-[65vh] min-h-[420px] max-h-[760px]" />
    </div>
  );
}
