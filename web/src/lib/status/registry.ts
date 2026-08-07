import type { ComponentType } from "react";
import type { TaskStatus } from "@/lib/types";
import { TodoView } from "@/app/projects/[id]/tasks/[taskID]/components/status-views/TodoView";
import { ExecutionProgressView } from "@/app/projects/[id]/tasks/[taskID]/components/status-views/ExecutionProgressView";
import { SpecReviewView } from "@/app/projects/[id]/tasks/[taskID]/components/status-views/SpecReviewView";
import { CodingProgressView } from "@/app/projects/[id]/tasks/[taskID]/components/status-views/CodingProgressView";
import { ReviewProgressView } from "@/app/projects/[id]/tasks/[taskID]/components/status-views/ReviewProgressView";
import { FixProgressView } from "@/app/projects/[id]/tasks/[taskID]/components/status-views/FixProgressView";
import { TestProgressView } from "@/app/projects/[id]/tasks/[taskID]/components/status-views/TestProgressView";
import { PrCreatedView } from "@/app/projects/[id]/tasks/[taskID]/components/status-views/PrCreatedView";
import { BlockedView } from "@/app/projects/[id]/tasks/[taskID]/components/status-views/BlockedView";
import { MergedView } from "@/app/projects/[id]/tasks/[taskID]/components/status-views/MergedView";
import { FailedView } from "@/app/projects/[id]/tasks/[taskID]/components/status-views/FailedView";

export type StatusViewConfig = {
  component: ComponentType;
  defaultTab?: "control" | "activity";
};

export const StatusViewRegistry: Record<TaskStatus, StatusViewConfig> = {
  todo: { component: TodoView, defaultTab: "control" },
  context_loading: { component: ExecutionProgressView, defaultTab: "activity" },
  analyzing: { component: ExecutionProgressView, defaultTab: "activity" },
  spec_review: { component: SpecReviewView, defaultTab: "control" },
  coding: { component: CodingProgressView, defaultTab: "activity" },
  reviewing: { component: ReviewProgressView, defaultTab: "activity" },
  fixing: { component: FixProgressView, defaultTab: "activity" },
  testing: { component: TestProgressView, defaultTab: "activity" },
  pr_ready: { component: PrCreatedView, defaultTab: "activity" },
  human_review: { component: PrCreatedView, defaultTab: "activity" },
  blocked: { component: BlockedView, defaultTab: "control" },
  merged: { component: MergedView, defaultTab: "control" },
  failed: { component: FailedView, defaultTab: "control" },
};
