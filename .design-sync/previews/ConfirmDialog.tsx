import { createPortal } from "react-dom";
import { ConfirmDialog } from "../../web/src/components/ui/confirm-dialog";

// ConfirmDialog renders `position: fixed`, which is viewport-relative only
// when no ancestor has a CSS `transform` — the preview card wrapper sets one
// (for compositing), so this portals straight to <body> to escape it and
// match how the dialog actually renders in the app.
function Overlay({ children }: { children: React.ReactNode }) {
  return createPortal(children, document.body);
}

export function DeleteTask() {
  return (
    <Overlay>
      <ConfirmDialog
        isOpen
        title="Delete Task"
        description="Are you sure you want to delete this task? This action cannot be undone."
        confirmText="Delete"
        variant="danger"
        onConfirm={() => {}}
        onClose={() => {}}
      />
    </Overlay>
  );
}

export function DiscardChanges() {
  return (
    <Overlay>
      <ConfirmDialog
        isOpen
        title="Discard changes?"
        description="You have unsaved edits to this rule. Leaving now will discard them."
        confirmText="Discard"
        cancelText="Keep editing"
        variant="warning"
        onConfirm={() => {}}
        onClose={() => {}}
      />
    </Overlay>
  );
}

export function Confirming() {
  return (
    <Overlay>
      <ConfirmDialog
        isOpen
        title="Delete Task"
        description="Are you sure you want to delete this task? This action cannot be undone."
        confirmText="Delete"
        variant="danger"
        isLoading
        onConfirm={() => {}}
        onClose={() => {}}
      />
    </Overlay>
  );
}
