import { test, expect } from "@playwright/test";
import { deriveImplementationItems } from "../src/app/projects/[id]/tasks/[taskID]/components/TaskDetailContext";
import type { TaskAnalysis, WorkflowCheckpoint } from "../src/lib/types";

test.describe("deriveImplementationItems mapping logic", () => {
  const mockAnalysis: Partial<TaskAnalysis> = {
    execution_units: [
      {
        id: "unit-1",
        objective: "Database Setup",
        tasks: [],
        execution_profile: { agent: "backend", skills: [] },
        constraints: { parallelizable: false, max_files: 5, estimated_tokens: 100, max_risk: "low" }
      },
      {
        id: "unit-2",
        objective: "Auth Service",
        tasks: [],
        execution_profile: { agent: "backend", skills: [] },
        constraints: { parallelizable: false, max_files: 5, estimated_tokens: 100, max_risk: "low" }
      },
      {
        id: "unit-3",
        objective: "Login UI",
        tasks: [],
        execution_profile: { agent: "frontend", skills: [] },
        constraints: { parallelizable: false, max_files: 5, estimated_tokens: 100, max_risk: "low" }
      }
    ]
  };

  test("all units done -> all status done", () => {
    const checkpoints: WorkflowCheckpoint[] = [
      {
        id: "cp-1",
        task_id: "t-1",
        step: "code_backend_0",
        state: { status: "success" },
        created_at: ""
      },
      {
        id: "cp-2",
        task_id: "t-1",
        step: "code_backend_1",
        state: { status: "recorded" },
        created_at: ""
      },
      {
        id: "cp-3",
        task_id: "t-1",
        step: "code_frontend_0",
        state: { status: "skipped" },
        created_at: ""
      }
    ];

    const items = deriveImplementationItems(mockAnalysis as TaskAnalysis, checkpoints, undefined);
    expect(items).toHaveLength(3);
    expect(items[0].status).toBe("done");
    expect(items[1].status).toBe("done");
    expect(items[2].status).toBe("done");
  });

  test("middle unit running -> correct running + pending split", () => {
    const checkpoints: WorkflowCheckpoint[] = [
      {
        id: "cp-1",
        task_id: "t-1",
        step: "code_backend_0",
        state: { status: "success" },
        created_at: ""
      }
    ];

    const items = deriveImplementationItems(mockAnalysis as TaskAnalysis, checkpoints, "code_backend_1");
    expect(items).toHaveLength(3);
    expect(items[0].status).toBe("done");
    expect(items[1].status).toBe("running");
    expect(items[2].status).toBe("pending");
  });

  test("no execution_units -> empty list (graceful fallback)", () => {
    const items = deriveImplementationItems({} as TaskAnalysis, [], undefined);
    expect(items).toEqual([]);
  });

  test("checkpoint names don't match -> all pending", () => {
    const checkpoints: WorkflowCheckpoint[] = [
      {
        id: "cp-1",
        task_id: "t-1",
        step: "different_step_name",
        state: { status: "success" },
        created_at: ""
      }
    ];

    const items = deriveImplementationItems(mockAnalysis as TaskAnalysis, checkpoints, undefined);
    expect(items).toHaveLength(3);
    expect(items[0].status).toBe("pending");
    expect(items[1].status).toBe("pending");
    expect(items[2].status).toBe("pending");
  });
});
