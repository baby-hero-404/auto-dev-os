# PLAN: Phase 4 — Frontend UI Refactoring & Component Deconstruction

This sub-plan outlines the exact target paths, component structures, props interfaces, and utility centralization tasks required to refactor the two largest frontend page views into modular, testable components.

---

## 1. Deconstructing `web/src/app/page.tsx` (Home Dashboard) (DONE)

The Home Dashboard page currently contains 668 lines, mixing page states, SWR fetch queries, modal configurations, project cards, and helper components.

### 1.1. Move/Extract `ProjectCard` (DONE)
*   **Target File**: `web/src/components/dashboard/home/project-card.tsx`
*   **Dependencies**: Extract `useIsNearViewport` into `web/src/lib/hooks/use-is-near-viewport.ts`. Keep `StatusBadge` and `CardStat` colocated inside `project-card.tsx` as private sub-components.
*   **Component Signature**:
    ```tsx
    import type { Project } from "@/lib/types";

    interface ProjectCardProps {
      project: Project;
      token: string;
    }

    export function ProjectCard({ project, token }: ProjectCardProps) { ... }
    export function ProjectCardsSkeleton() { ... }
    ```

### 1.2. Extract `CreateProjectModal` (DONE)
*   **Target File**: `web/src/components/dashboard/home/create-project-modal.tsx`
*   **State Encapsulation**: This modal will completely internalize the two-step form state (name, description), repository linking state (URL, branch, selected git account), error/loading states (`isSubmitting`, `creationError`), and branch fetching. It will call `api.createProject`, `api.createRepository`, and `api.getRemoteBranches` directly instead of lifting those up to the page.
*   **Component Signature**:
    ```tsx
    import type { GitAccount } from "@/lib/types";

    interface CreateProjectModalProps {
      isOpen: boolean;
      onClose: () => void;
      gitAccounts: GitAccount[];
      token: string;
      orgID: string;
      onProjectCreated: () => void;
    }

    export function CreateProjectModal({ ... }: CreateProjectModalProps) { ... }
    ```

---

## 2. Deconstructing `web/src/app/projects/[id]/tasks/[taskID]/page.tsx` (Task Details) (DONE)

At 1,103 lines, the Task Details page contains substantial inline code for parsing diffs, layout widgets, tabs, logs, PR actions, and forms.

### 2.1. Extract `TaskDiffViewer` (DONE)
*   **Target File**: `web/src/components/projects/task-diff-viewer.tsx`
*   **State Details**: The component should either receive the fully parsed arrays or internalize `parseUnifiedDiff`. We will pass the parsed state down.
*   **Component Signature**:
    ```tsx
    export interface ParsedFileDiff {
      filename: string;
      diffLines: string[];
    }

    interface TaskDiffViewerProps {
      displayFiles: string[];
      selectedFile: string | null;
      activeFileDiff: ParsedFileDiff | null;
      onSelectFile: (file: string) => void;
    }

    export function TaskDiffViewer({ ... }: TaskDiffViewerProps) { ... }
    ```

### 2.2. Extract `TaskPRReview` (DONE)
*   **Target File**: `web/src/components/projects/task-pr-review.tsx`
*   **Component Signature**:
    ```tsx
    import type { Task, PRSummary } from "@/lib/types";

    interface TaskPRReviewProps {
      task: Task;
      prSummary?: PRSummary;
      submittingPR: boolean;
      hasPR: boolean;
      
      // UI Data Props
      riskAssessment: { level: string; reason: string };
      riskDomains: string[];
      displayFiles: string[];
      selectedFile: string | null;
      onSelectFile: (file: string) => void;

      // Review State & Actions
      feedback: string;
      onFeedbackChange: (text: string) => void;
      onApprove: () => Promise<void>;
      onReject: () => Promise<void>;
      onStartReview: () => Promise<void>;
    }

    export function TaskPRReview({ ... }: TaskPRReviewProps) { ... }
    ```

### 2.3. Extract `TaskClarificationForm` (DONE)
*   **Target File**: `web/src/components/projects/task-clarification-form.tsx`
*   **State Encapsulation**: This component will manage its own `answers` dictionary and `submittingAnswers` loading state, calling `api.requestTaskChanges` internally and invoking the callback on success.
*   **Component Signature**:
    ```tsx
    interface TaskClarificationFormProps {
      questions: string[];
      token: string;
      taskID: string;
      onAnswersSubmitted: () => Promise<void>;
    }

    export function TaskClarificationForm({ ... }: TaskClarificationFormProps) { ... }
    ```

---

## 3. Centralizing UI Utility Functions (DONE)

We need to resolve conflicts with existing files in `web/src/lib/utils/`:
*   **Consolidate Time Helpers**: Create `web/src/lib/utils/time.ts` and merge `timeAgo` (from `task-utils.ts`) and `formatRelativeTime` (from `page.tsx`). Refactor codebase to use one standard unified relative time function.
*   **Consolidate Task Utilities**: Merge all task-related helpers (`isDoneStatus`, `latestActivity`, `deriveProjectStatus`, `deriveHydratedProjectStatus`) into the existing `web/src/lib/utils/task-utils.ts`. 
*   **Remove Duplicate File**: The existing `web/src/lib/utils/tasks.ts` contains `getRiskAssessment`. We will merge its contents into `task-utils.ts` and delete `tasks.ts` to prevent naming confusion between plural and suffixed file names.
