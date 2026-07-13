import { ConfirmDialog } from "../../web/src/components/ui/confirm-dialog";

export function DeleteTask() {
  return (
    <ConfirmDialog
      isOpen
      title="Delete Task"
      description="Are you sure you want to delete this task? This action cannot be undone."
      confirmText="Delete"
      variant="danger"
      onConfirm={() => {}}
      onClose={() => {}}
    />
  );
}

export function DiscardChanges() {
  return (
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
  );
}

export function Confirming() {
  return (
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
  );
}
