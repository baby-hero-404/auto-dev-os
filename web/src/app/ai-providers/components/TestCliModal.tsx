import React from "react";
import { Dialog } from "@/components/ui/dialog";
import dynamic from "next/dynamic";

const InteractiveTerminal = dynamic(
  () => import("./InteractiveTerminal").then((mod) => mod.InteractiveTerminal),
  { ssr: false }
);

interface TestCliModalProps {
  isOpen: boolean;
  onClose: () => void;
  orgID: string;
  token: string;
  credentialID: string;
  provider: string; // e.g. "cli:claude"
}

export function TestCliModal({ isOpen, onClose, orgID, token, credentialID, provider }: TestCliModalProps) {
  const toolName = provider.replace("cli:", "");
  return (
    <Dialog open={isOpen} onClose={onClose} title={`Test ${toolName} CLI`} size="xl">
      <p className="mb-4 text-sm text-content-muted">
        The workspace has been prepared with your saved credential. You can type commands directly (e.g. <code>{toolName} --version</code>) to verify it works.
      </p>
      {isOpen && (
        <InteractiveTerminal
          orgID={orgID}
          token={token}
          provider={provider}
          mode="test"
          credentialID={credentialID}
          onExit={onClose}
        />
      )}
    </Dialog>
  );
}
