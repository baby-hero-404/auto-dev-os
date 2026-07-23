import { useEffect, useRef, useState } from "react";
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import "@xterm/xterm/css/xterm.css";

interface InteractiveTerminalProps {
  wsUrl: string;
  onExit: (payload: Record<string, string>) => void;
  onError: (error: string) => void;
}

export function InteractiveTerminal({ wsUrl, onExit, onError }: InteractiveTerminalProps) {
  const terminalRef = useRef<HTMLDivElement>(null);
  const term = useRef<Terminal | null>(null);
  const socket = useRef<WebSocket | null>(null);
  const fitAddon = useRef<FitAddon | null>(null);
  const [isConnected, setIsConnected] = useState(false);

  useEffect(() => {
    if (!terminalRef.current) return;

    term.current = new Terminal({
      cursorBlink: true,
      theme: {
        background: "#000000",
        foreground: "#ffffff",
      },
      fontFamily: 'Menlo, Monaco, "Courier New", monospace',
      fontSize: 13,
    });
    fitAddon.current = new FitAddon();
    term.current.loadAddon(fitAddon.current);
    term.current.open(terminalRef.current);
    
    // Defer fit to allow DOM to render and container to get dimensions
    setTimeout(() => {
      try {
        fitAddon.current?.fit();
      } catch (e) {
        console.warn("xterm fit error", e);
      }
    }, 10);

    try {
      socket.current = new WebSocket(wsUrl);

      socket.current.onopen = () => {
        setIsConnected(true);
      };

      socket.current.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data);
          if (data.type === "stdout" && term.current) {
            term.current.write(data.data);
          } else if (data.type === "exit") {
            onExit(data.payload);
          } else if (data.type === "error") {
            onError(data.message);
          }
        } catch (e) {
          console.error("Failed to parse websocket message", e);
        }
      };

      socket.current.onerror = (e) => {
        console.error("WebSocket Error", e);
        onError("WebSocket connection error");
      };

      socket.current.onclose = () => {
        setIsConnected(false);
      };

      term.current.onData((data) => {
        if (socket.current?.readyState === WebSocket.OPEN) {
          socket.current.send(data);
        }
      });
    } catch (err: any) {
      onError(err.message);
    }

    const handleResize = () => {
      try {
        fitAddon.current?.fit();
      } catch (e) {
        console.warn("xterm resize fit error", e);
      }
    };
    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      if (socket.current) {
        socket.current.close();
      }
      if (term.current) {
        term.current.dispose();
      }
    };
  }, [wsUrl, onError, onExit]);

  return (
    <div className="w-full h-full relative overflow-hidden bg-black rounded-md border border-stroke">
      {!isConnected && (
        <div className="absolute inset-0 flex items-center justify-center bg-black/80 z-10 text-white text-sm">
          Connecting to sandbox...
        </div>
      )}
      <div ref={terminalRef} className="w-full h-[360px]" />
    </div>
  );
}
