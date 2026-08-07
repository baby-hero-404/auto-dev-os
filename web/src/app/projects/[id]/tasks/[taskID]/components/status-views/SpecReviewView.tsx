"use client";

import { SpecViewer } from "./SpecViewer";
import { RequestChangesModal } from "../RequestChangesModal";

/**
 * The single approval gate in the status-driven workspace (`spec_review`).
 * Wraps the existing SpecPanel/SpecReviewGate/CLISpecPanel implementation —
 * their ~870 lines of stateful spec-tab/edit/collapse logic aren't duplicated
 * here. Those three components are deleted in Phase 5 once this view (and
 * DynamicActionBar) are confirmed to fully replace their affordances.
 *
 * RequestChangesModal renders here (previously it was never mounted anywhere
 * in the tree, so the classic flow's "Request Changes" button set
 * isRequestingChanges=true with no visible effect).
 */
export function SpecReviewView() {
  return (
    <>
      <SpecViewer />
      <RequestChangesModal />
    </>
  );
}
