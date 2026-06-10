"use client";

import Link from "next/link";
import { FormEvent, use, useState } from "react";
import useSWR from "swr";
import { ArrowLeft, CheckCircle2, GitBranch, Play, Plus, RefreshCw, ShieldAlert, Bot, Settings, List, Save, ShieldCheck } from "lucide-react";
import { api, ApiError } from "@/lib/api";
import { useSession } from "@/lib/session";
import { useAuthedSWR } from "@/lib/use-authed-swr";
import type { Agent, Repository, Task, Skill } from "@/lib/types";
import { Badge } from "@/components/ui/badge";
import { InfoBlock } from "@/components/ui/info-block";

export default function ProjectDetail({ params }: { params: Promise<{ id: string }> }) {
  const { id: projectID } = use(params);
  const session = useSession();
  const [activeTab, setActiveTab] = useState<"tasks" | "members" | "settings">("tasks");
  const [repoURL, setRepoURL] = useState("");
  const [repoToken, setRepoToken] = useState("");
  const [taskTitle, setTaskTitle] = useState("");
  const [taskDescription, setTaskDescription] = useState("");
  const [taskComplexity, setTaskComplexity] = useState<"easy" | "medium" | "hard">("easy");
  const [changeRequest, setChangeRequest] = useState("");
  const [repoError, setRepoError] = useState("");
  const [taskError, setTaskError] = useState("");
  const [selectedGitAccountID, setSelectedGitAccountID] = useState("");

  // Settings states
  const [updatedName, setUpdatedName] = useState("");
  const [updatedDescription, setUpdatedDescription] = useState("");
  const [isProjectDataLoaded, setIsProjectDataLoaded] = useState(false);
  const [projectUpdateError, setProjectUpdateError] = useState("");
  const [isUpdatingProject, setIsUpdatingProject] = useState(false);

  // Rules states
  const [ruleContent, setRuleContent] = useState("");
  const [ruleError, setRuleError] = useState("");
  const [isAddingRule, setIsAddingRule] = useState(false);

  // Skill assignment states
  const [assigningSkillMap, setAssigningSkillMap] = useState<Record<string, string>>({}); // agentID -> skillID
  const [skillErrorMap, setSkillErrorMap] = useState<Record<string, string>>({});

  // Staff assignment states
  const [selectedStaff, setSelectedStaff] = useState("");
  const [assignError, setAssignError] = useState("");
  const [isAssigning, setIsAssigning] = useState(false);

  const token = session?.token ?? "";
  const orgID = session?.user.org_id ?? "";

  const { data: project, mutate: mutateProject } = useSWR(
    projectID && token ? ["project", projectID] : null,
    async () => {
      const p = await api.getProject(projectID, token);
      if (p && !isProjectDataLoaded) {
        setUpdatedName(p.name);
        setUpdatedDescription(p.description || "");
        setIsProjectDataLoaded(true);
      }
      return p;
    }
  );

  const { data: repositories = [], mutate: mutateRepos } = useSWR(projectID && token ? ["repositories", projectID] : null, () => api.listRepositories(projectID, token));
  const { data: tasks = [], mutate: mutateTasks } = useSWR(projectID && token ? ["tasks", projectID] : null, () => api.listTasks(projectID, token));

  // Fetch project members (assigned agents)
  const { data: projectAgents = [], mutate: mutateProjectAgents } = useSWR(
    projectID && token ? ["project-agents", projectID] : null,
    () => api.listAgents(projectID, token),
  );

  // Fetch organization staff pool agents
  const { data: orgAgents = [] } = useAuthedSWR(
    orgID ? ["org-agents", orgID] : null,
    (t) => api.listOrgAgents(orgID, t),
  );

  // Fetch project rules
  const { data: rules = [], mutate: mutateRules } = useSWR(
    projectID && token ? ["rules", projectID] : null,
    () => api.listRules(projectID, token),
  );

  // Fetch organization Git Accounts
  const { data: gitAccounts = [] } = useAuthedSWR(
    orgID ? ["git-accounts", orgID] : null,
    (t) => api.listGitAccounts(orgID, t),
  );

  // Fetch all global skills
  const { data: globalSkills = [] } = useAuthedSWR(
    ["global-skills"],
    (t) => api.listSkills(t),
  );

  // Fetch assigned skills for each agent assigned to this project
  const { data: agentSkills = {}, mutate: mutateAgentSkills } = useSWR(
    session && projectAgents.length > 0 ? ["agent-skills", projectAgents.map((a) => a.id).join(",")] : null,
    async () => {
      const ids = projectAgents.map((a) => a.id);
      const map: Record<string, Skill[]> = {};
      await Promise.all(
        ids.map(async (aid) => {
          try {
            const list = await api.listAgentSkills(aid, token);
            map[aid] = list;
          } catch (e) {
            console.error("Failed to fetch skills for agent", aid, e);
          }
        })
      );
      return map;
    }
  );

  // Filter staff agents that are not already assigned to this project (excluding auto_join strategy agents)
  const assignableStaff = orgAgents.filter(
    (staff) =>
      staff.assignment_strategy !== "auto_join" &&
      !projectAgents.some((pa) => pa.id === staff.id)
  );
  const inheritedAgents = projectAgents.filter((agent) => agent.assignment_strategy === "auto_join");
  const manualProjectAgents = projectAgents.filter((agent) => agent.assignment_strategy !== "auto_join");

  async function handleAssignStaff(e: FormEvent) {
    e.preventDefault();
    if (!projectID || !selectedStaff || !token) return;

    const staff = orgAgents.find((s) => s.id === selectedStaff);
    if (!staff) return;

    setAssignError("");
    setIsAssigning(true);
    try {
      await api.createAgent(projectID, token, {
        name: staff.name,
        role: staff.role,
        goal: staff.goal,
        autonomy_level: staff.autonomy_level,
        model_route: staff.model_route,
        assignment_strategy: staff.assignment_strategy,
        agent_id: staff.id,
      });
      setSelectedStaff("");
      mutateProjectAgents();
    } catch (err) {
      setAssignError(err instanceof ApiError ? err.message : "Failed to assign agent");
    } finally {
      setIsAssigning(false);
    }
  }

  async function handleUpdateProject(e: FormEvent) {
    e.preventDefault();
    if (!projectID || !token) return;

    setProjectUpdateError("");
    setIsUpdatingProject(true);
    try {
      await api.updateProject(projectID, token, {
        name: updatedName.trim(),
        description: updatedDescription.trim(),
      });
      mutateProject();
    } catch (err) {
      setProjectUpdateError(err instanceof ApiError ? err.message : "Failed to update project");
    } finally {
      setIsUpdatingProject(false);
    }
  }

  async function handleAddRule(e: FormEvent) {
    e.preventDefault();
    if (!projectID || !token) return;

    const content = ruleContent.trim();
    if (!content) {
      setRuleError("Rule content cannot be empty.");
      return;
    }

    setRuleError("");
    setIsAddingRule(true);
    try {
      await api.createRule(projectID, token, {
        content,
        scope: "project",
        enforcement: "strict",
      });
      setRuleContent("");
      mutateRules();
    } catch (err) {
      setRuleError(err instanceof ApiError ? err.message : "Failed to add rule");
    } finally {
      setIsAddingRule(false);
    }
  }

  async function handleAssignSkill(agentID: string) {
    const skillID = assigningSkillMap[agentID];
    if (!skillID || !token) return;

    setSkillErrorMap((prev) => ({ ...prev, [agentID]: "" }));
    try {
      await api.assignSkillToAgent(agentID, skillID, token);
      setAssigningSkillMap((prev) => {
        const next = { ...prev };
        delete next[agentID];
        return next;
      });
      mutateAgentSkills();
    } catch (err) {
      setSkillErrorMap((prev) => ({
        ...prev,
        [agentID]: err instanceof ApiError ? err.message : "Failed to assign skill",
      }));
    }
  }

  async function createRepository(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!projectID || !token) return;
    
    const url = repoURL.trim();
    if (!url) {
      setRepoError("Repository URL is required.");
      return;
    }

    setRepoError("");
    try {
      await api.createRepository(projectID, token, {
        url,
        provider: "github",
        branch: "main",
        token: repoToken.trim(),
        git_account_id: selectedGitAccountID || undefined,
      });
      setRepoURL("");
      setRepoToken("");
      setSelectedGitAccountID("");
      mutateRepos();
    } catch (err) {
      setRepoError(errorMessage(err, "Failed to link repository"));
    }
  }

  async function createTask(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!projectID || !token) return;
    
    const title = taskTitle.trim();
    if (!title) {
      setTaskError("Task title is required.");
      return;
    }

    setTaskError("");
    try {
      await api.createTask(projectID, token, {
        title,
        description: taskDescription.trim(),
        complexity: taskComplexity,
        priority: 0,
        labels: [],
      });
      setTaskTitle("");
      setTaskDescription("");
      setTaskComplexity("easy");
      mutateTasks();
    } catch (err) {
      setTaskError(errorMessage(err, "Failed to create task"));
    }
  }

  async function analyze(taskID: string) {
    await api.analyzeTask(taskID, token);
    mutateTasks();
  }

  async function approve(taskID: string) {
    await api.approveTaskAnalysis(taskID, token);
    mutateTasks();
  }

  async function requestChanges(taskID: string) {
    await api.requestTaskChanges(taskID, token, changeRequest || "Please refine the task spec before execution.");
    setChangeRequest("");
    mutateTasks();
  }

  async function cloneRepository(repoID: string) {
    await api.cloneRepository(repoID, token);
    mutateRepos();
  }

  if (!session) {
    return (
      <main className="grid min-h-screen place-items-center p-6">
        <div className="rounded-lg border border-stroke bg-panel p-6">
          <p className="mb-4 text-sm text-content-muted">Login from the dashboard before opening a project.</p>
          <Link className="rounded-md bg-brand-primary px-4 py-2 font-semibold text-slate-950" href="/">
            Back to login
          </Link>
        </div>
      </main>
    );
  }

  return (
    <main className="min-h-screen p-5">
      <header className="mb-6 flex flex-col justify-between gap-4 border-b border-stroke pb-5 md:flex-row md:items-end">
        <div>
          <Link href="/" className="mb-4 inline-flex items-center gap-2 text-sm text-content-muted transition hover:text-white">
            <ArrowLeft size={16} />
            Projects
          </Link>
          <h1 className="font-mono text-3xl font-semibold">{project?.name ?? "Project"}</h1>
          <p className="mt-1 text-sm text-content-muted">{project?.description ?? "Repository and task workspace"}</p>
        </div>
        <div className="rounded-md border border-stroke bg-panel px-3 py-2 text-sm text-content-muted">Project ID: {projectID}</div>
      </header>

      <div className="grid gap-5 xl:grid-cols-[420px_1fr]">
        <section className="space-y-5">
          {/* Repositories Card */}
          <div className="rounded-lg border border-stroke bg-panel p-5">
            <div className="mb-4 flex items-center gap-2">
              <GitBranch size={18} className="text-brand-primary" />
              <h2 className="font-mono text-lg font-semibold">Repositories</h2>
            </div>
            <form className="space-y-3" onSubmit={createRepository}>
              <input value={repoURL} onChange={(e) => setRepoURL(e.target.value)} placeholder="https://github.com/org/repo.git" className="w-full rounded-md border border-stroke bg-slate-950 px-3 py-2 text-sm text-white" />
              <div className="flex flex-col gap-1">
                <span className="text-[10px] font-mono uppercase tracking-wider text-content-muted pl-1">Git Account Credential</span>
                <select
                  value={selectedGitAccountID}
                  onChange={(e) => setSelectedGitAccountID(e.target.value)}
                  className="w-full rounded-md border border-stroke bg-slate-950 px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
                >
                  <option value="">None / Manual Override Token</option>
                  {gitAccounts.map((acc) => (
                    <option key={acc.id} value={acc.id}>
                      {acc.display_name} ({acc.base_url ? "GitHub Enterprise" : "GitHub"})
                    </option>
                  ))}
                </select>
              </div>
              <input value={repoToken} onChange={(e) => setRepoToken(e.target.value)} placeholder="Manual token override (optional)" type="password" className="w-full rounded-md border border-stroke bg-slate-950 px-3 py-2 text-sm text-white" />
              {repoError && (
                <p className="rounded border border-red-400/30 bg-red-950/40 p-2 text-xs text-red-200">
                  {repoError}
                </p>
              )}
              <button className="flex w-full items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-slate-950 cursor-pointer" type="submit">
                <Plus size={16} />
                Link repository
              </button>
            </form>
            <div className="mt-4 space-y-3">
              {repositories.map((repo: Repository) => (
                <div key={repo.id} className="rounded-md border border-stroke bg-slate-950 p-3">
                  <div className="break-all text-sm">{repo.url}</div>
                  <div className="mt-2 flex items-center justify-between text-xs text-content-muted">
                    <span>{repo.clone_status}</span>
                    <button className="inline-flex items-center gap-1 rounded border border-stroke px-2 py-1 transition hover:bg-panel-muted cursor-pointer" onClick={() => cloneRepository(repo.id)} type="button">
                      <RefreshCw size={13} />
                      Clone
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>

          {/* Create Task Card */}
          <div className="rounded-lg border border-stroke bg-panel p-5">
            <h2 className="mb-4 font-mono text-lg font-semibold">Create task</h2>
            <form className="space-y-3" onSubmit={createTask}>
              <input value={taskTitle} onChange={(e) => setTaskTitle(e.target.value)} placeholder="Task title" className="w-full rounded-md border border-stroke bg-slate-950 px-3 py-2 text-sm text-white" />
              <select
                value={taskComplexity}
                onChange={(e) => setTaskComplexity(e.target.value as "easy" | "medium" | "hard")}
                className="w-full rounded-md border border-stroke bg-slate-950 px-3 py-2 text-sm text-white"
              >
                <option value="easy">easy</option>
                <option value="medium">medium</option>
                <option value="hard">hard</option>
              </select>
              <textarea value={taskDescription} onChange={(e) => setTaskDescription(e.target.value)} placeholder="Description, context, files, expected behavior" rows={5} className="w-full rounded-md border border-stroke bg-slate-950 px-3 py-2 text-sm text-white" />
              {taskError && (
                <p className="rounded border border-red-400/30 bg-red-950/40 p-2 text-xs text-red-200">
                  {taskError}
                </p>
              )}
              <button className="flex w-full items-center justify-center gap-2 rounded-md bg-brand-primary px-3 py-2 text-sm font-semibold text-slate-950 cursor-pointer" type="submit">
                <Plus size={16} />
                Create task
              </button>
            </form>
          </div>
        </section>

        {/* Right Section containing Tabs */}
        <section className="rounded-lg border border-stroke bg-panel p-5 flex flex-col">
          {/* Tab Navigation header */}
          <div className="mb-5 flex gap-5 border-b border-stroke">
            <button
              onClick={() => setActiveTab("tasks")}
              className={`flex items-center gap-2 pb-2.5 text-sm font-mono font-bold uppercase tracking-wider transition cursor-pointer border-b-2 ${
                activeTab === "tasks"
                  ? "border-brand-primary text-white"
                  : "border-transparent text-content-muted hover:text-white"
              }`}
            >
              <List size={16} />
              Tasks
            </button>
            <button
              onClick={() => setActiveTab("members")}
              className={`flex items-center gap-2 pb-2.5 text-sm font-mono font-bold uppercase tracking-wider transition cursor-pointer border-b-2 ${
                activeTab === "members"
                  ? "border-brand-primary text-white"
                  : "border-transparent text-content-muted hover:text-white"
              }`}
            >
              <Bot size={16} />
              Members
            </button>
            <button
              onClick={() => setActiveTab("settings")}
              className={`flex items-center gap-2 pb-2.5 text-sm font-mono font-bold uppercase tracking-wider transition cursor-pointer border-b-2 ${
                activeTab === "settings"
                  ? "border-brand-primary text-white"
                  : "border-transparent text-content-muted hover:text-white"
              }`}
            >
              <Settings size={16} />
              Settings & Rules
            </button>
          </div>

          {/* TAB CONTENT: TASKS */}
          {activeTab === "tasks" && (
            <div className="space-y-4">
              {tasks.length === 0 ? (
                <p className="text-sm text-content-muted italic">No tasks created yet.</p>
              ) : (
                tasks.map((task: Task) => (
                  <article key={task.id} className="rounded-lg border border-stroke bg-slate-950 p-4">
                    <div className="flex flex-col justify-between gap-3 md:flex-row md:items-start">
                      <div>
                        <div className="flex flex-wrap items-center gap-2">
                          <h3 className="font-mono font-semibold text-white">{task.title}</h3>
                          <Badge value={task.complexity} />
                          <Badge value={task.spec_status} />
                          <Badge value={task.status} />
                        </div>
                        <p className="mt-2 text-sm text-content-muted">{task.description || "No description"}</p>
                      </div>
                      <div className="flex shrink-0 flex-wrap gap-2">
                        <Link className="inline-flex items-center gap-2 rounded-md border border-stroke px-3 py-2 text-sm transition hover:bg-panel-muted cursor-pointer" href={`/projects/${projectID}/tasks/${task.id}/monitor`}>
                          Workflow
                        </Link>
                        <button className="inline-flex items-center gap-2 rounded-md border border-stroke px-3 py-2 text-sm transition hover:bg-panel-muted cursor-pointer" onClick={() => analyze(task.id)} type="button">
                          <Play size={15} />
                          Analyze
                        </button>
                        <button className="inline-flex items-center gap-2 rounded-md border border-emerald-400/40 px-3 py-2 text-sm text-emerald-200 transition hover:bg-emerald-400/10 cursor-pointer" onClick={() => approve(task.id)} type="button">
                          <CheckCircle2 size={15} />
                          Approve
                        </button>
                      </div>
                    </div>

                    {task.analysis && (
                      <div className="mt-4 grid gap-3 lg:grid-cols-2">
                        <InfoBlock title="Scope" items={[task.analysis.scope]} />
                        <InfoBlock title="Risks" items={task.analysis.risks ?? []} />
                        <InfoBlock title="Plan" items={task.analysis.execution_plan ?? []} />
                        <InfoBlock title="Questions" items={task.analysis.clarification_questions ?? []} />
                      </div>
                    )}

                    <div className="mt-4 flex flex-col gap-2 md:flex-row">
                      <input value={changeRequest} onChange={(e) => setChangeRequest(e.target.value)} placeholder="Request spec changes" className="min-w-0 flex-1 rounded-md border border-stroke bg-page px-3 py-2 text-sm text-white focus:outline-none focus:border-brand-primary" />
                      <button className="inline-flex items-center justify-center gap-2 rounded-md border border-amber-400/40 px-3 py-2 text-sm text-amber-200 transition hover:bg-amber-400/10 cursor-pointer" onClick={() => requestChanges(task.id)} type="button">
                        <ShieldAlert size={15} />
                        Request changes
                      </button>
                    </div>
                  </article>
                ))
              )}
            </div>
          )}

          {/* TAB CONTENT: MEMBERS */}
          {activeTab === "members" && (
            <div className="space-y-6">
              <div className="grid gap-3 sm:grid-cols-3">
                <ProjectMetric label="Total members" value={projectAgents.length.toString()} />
                <ProjectMetric label="Inherited" value={inheritedAgents.length.toString()} />
                <ProjectMetric label="Project-specific" value={manualProjectAgents.length.toString()} />
              </div>

              <section className="rounded-lg border border-stroke bg-slate-950 p-5">
                <div className="mb-4 flex items-center justify-between gap-3 border-b border-stroke pb-3">
                  <div>
                    <h3 className="font-mono font-semibold text-white">Inherited from Global</h3>
                    <p className="text-sm text-content-muted">Auto-join agents are inherited by every project.</p>
                  </div>
                  <Badge value="auto_join" />
                </div>

                {inheritedAgents.length === 0 ? (
                  <p className="text-sm italic text-content-muted">No auto-join agents in the organization pool.</p>
                ) : (
                  <div className="grid gap-3 md:grid-cols-2">
                    {inheritedAgents.map((agent) => (
                      <ProjectAgentCard
                        key={agent.id}
                        agent={agent}
                        badge="Global"
                        skillsCount={(agentSkills[agent.id] || []).length}
                      />
                    ))}
                  </div>
                )}
              </section>

              <section className="rounded-lg border border-stroke bg-slate-950 p-5">
                <div className="mb-4 flex items-center justify-between gap-3 border-b border-stroke pb-3">
                  <div>
                    <h3 className="font-mono font-semibold text-white">Project-specific</h3>
                    <p className="text-sm text-content-muted">Manual assignments from the organization agent pool.</p>
                  </div>
                  <Badge value="manual" />
                </div>

                {manualProjectAgents.length === 0 ? (
                  <p className="text-sm italic text-content-muted">No manual agents assigned to this project.</p>
                ) : (
                  <div className="grid gap-3 md:grid-cols-2">
                    {manualProjectAgents.map((agent) => (
                      <ProjectAgentCard
                        key={agent.id}
                        agent={agent}
                        badge="Manual"
                        skillsCount={(agentSkills[agent.id] || []).length}
                      />
                    ))}
                  </div>
                )}

                <form className="mt-5 border-t border-stroke pt-4" onSubmit={handleAssignStaff}>
                  <label className="mb-1.5 block font-mono text-xs font-bold uppercase tracking-wider text-content-muted">
                    Assign from organization pool
                  </label>
                  <div className="flex flex-col gap-2 sm:flex-row">
                    <select
                      value={selectedStaff}
                      onChange={(e) => setSelectedStaff(e.target.value)}
                      className="min-w-0 flex-1 rounded border border-stroke bg-slate-900 px-3 py-2 text-sm text-white focus:border-brand-primary focus:outline-none"
                      disabled={isAssigning || assignableStaff.length === 0}
                    >
                      <option value="">
                        {assignableStaff.length === 0 ? "No manual agents available" : "Choose a manual agent"}
                      </option>
                      {assignableStaff.map((staff) => (
                        <option key={staff.id} value={staff.id}>
                          {staff.name} ({staff.role})
                        </option>
                      ))}
                    </select>
                    <button
                      disabled={!selectedStaff || isAssigning}
                      className="rounded bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50"
                      type="submit"
                    >
                      {isAssigning ? "Assigning..." : "Assign"}
                    </button>
                  </div>
                  {assignError && <p className="mt-2 text-xs text-red-400">{assignError}</p>}
                  <p className="mt-2 text-xs text-content-muted">
                    Removing a project-only assignment is hidden until the backend supports deleting from `project_agents`.
                  </p>
                </form>
              </section>
            </div>
          )}

          {/* TAB CONTENT: SETTINGS & SKILLS */}
          {activeTab === "settings" && (
            <div className="space-y-6">
              {/* Project General info Form */}
              <div className="rounded-lg border border-stroke bg-slate-950 p-5">
                <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-2">
                  <Settings size={18} className="text-brand-primary" />
                  <h3 className="font-mono font-semibold text-white">Project Metadata</h3>
                </div>
                <form onSubmit={handleUpdateProject} className="space-y-4">
                  <div className="flex flex-col gap-1.5">
                    <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Project Name</label>
                    <input
                      value={updatedName}
                      onChange={(e) => setUpdatedName(e.target.value)}
                      className="rounded border border-stroke bg-slate-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-brand-primary"
                      required
                      disabled={isUpdatingProject}
                    />
                  </div>
                  <div className="flex flex-col gap-1.5">
                    <label className="text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Description</label>
                    <textarea
                      value={updatedDescription}
                      onChange={(e) => setUpdatedDescription(e.target.value)}
                      className="min-h-[80px] rounded border border-stroke bg-slate-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-brand-primary resize-none"
                      disabled={isUpdatingProject}
                    />
                  </div>
                  {projectUpdateError && (
                    <p className="text-xs text-red-400">{projectUpdateError}</p>
                  )}
                  <button
                    type="submit"
                    disabled={isUpdatingProject}
                    className="flex items-center gap-2 rounded bg-brand-primary px-4 py-2 text-sm font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
                  >
                    <Save size={16} />
                    {isUpdatingProject ? "Saving..." : "Save Project Settings"}
                  </button>
                </form>
              </div>

              {/* Project Rules Card */}
              <div className="rounded-lg border border-stroke bg-slate-950 p-5">
                <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-2">
                  <ShieldCheck size={18} className="text-brand-primary" />
                  <h3 className="font-mono font-semibold text-white">Project Rules</h3>
                </div>

                {/* Display existing rules */}
                <div className="space-y-3 mb-4 max-h-[200px] overflow-y-auto pr-1">
                  {rules.length === 0 ? (
                    <p className="text-xs text-content-muted italic">No rules defined for this project.</p>
                  ) : (
                    rules.map((rule) => (
                      <div key={rule.id} className="rounded border border-stroke/50 bg-slate-900 p-3 text-sm">
                        <p className="text-white whitespace-pre-wrap">{rule.content}</p>
                        <div className="mt-2 flex items-center gap-2 text-[10px] text-content-muted font-mono">
                          <span>Enforcement: <span className="text-emerald-300 font-bold uppercase">{rule.enforcement}</span></span>
                          <span>•</span>
                          <span>Scope: <span className="text-white uppercase">{rule.scope}</span></span>
                        </div>
                      </div>
                    ))
                  )}
                </div>

                {/* Add new project rule form */}
                <form onSubmit={handleAddRule} className="space-y-3 pt-3 border-t border-stroke/40">
                  <label className="block text-xs font-mono font-bold uppercase tracking-wider text-content-muted">Add Custom Rule</label>
                  <textarea
                    value={ruleContent}
                    onChange={(e) => setRuleContent(e.target.value)}
                    placeholder="e.g. Always write Unit Tests for all new repository helper functions."
                    className="min-h-[85px] w-full rounded border border-stroke bg-slate-900 px-3 py-2 text-sm text-white focus:outline-none focus:border-brand-primary resize-none"
                    disabled={isAddingRule}
                  />
                  {ruleError && <p className="text-xs text-red-400">{ruleError}</p>}
                  <button
                    type="submit"
                    disabled={isAddingRule || !ruleContent.trim()}
                    className="flex items-center gap-2 rounded bg-brand-primary px-3 py-1.5 text-xs font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
                  >
                    <Plus size={14} />
                    Add Rule
                  </button>
                </form>
              </div>

              {/* Agent Skills Management panel */}
              <div className="rounded-lg border border-stroke bg-slate-950 p-5">
                <div className="mb-4 flex items-center gap-2 border-b border-stroke pb-2">
                  <Bot size={18} className="text-brand-primary" />
                  <h3 className="font-mono font-semibold text-white">Agent Skills Configuration</h3>
                </div>

                <div className="space-y-4">
                  {projectAgents.length === 0 ? (
                    <p className="text-xs text-content-muted italic">No agents assigned to this project yet.</p>
                  ) : (
                    projectAgents.map((agent) => {
                      const assignedSkills = agentSkills[agent.id] || [];
                      // Filter global skills to only those NOT currently assigned to this agent
                      const assignableSkills = globalSkills.filter(
                        (gs) => !assignedSkills.some((as) => as.id === gs.id)
                      );

                      return (
                        <div key={agent.id} className="rounded border border-stroke/40 bg-slate-900 p-4 space-y-3">
                          <div className="flex items-center justify-between">
                            <div>
                              <h4 className="font-mono text-sm font-bold text-white">{agent.name}</h4>
                              <p className="text-xs text-content-muted uppercase font-mono tracking-wide">{agent.role}</p>
                            </div>
                            <Badge value={agent.autonomy_level} />
                          </div>

                          {/* Render skills badges */}
                          <div>
                            <span className="block text-[10px] font-mono font-bold uppercase tracking-wider text-content-muted mb-1">Active Skills:</span>
                            {assignedSkills.length === 0 ? (
                              <p className="text-xs text-content-muted italic">No skills assigned.</p>
                            ) : (
                              <div className="flex flex-wrap gap-1.5">
                                {assignedSkills.map((s) => (
                                  <span key={s.id} className="rounded bg-slate-950 border border-stroke px-2 py-0.5 text-xs text-white" title={s.description}>
                                    {s.name}
                                  </span>
                                ))}
                              </div>
                            )}
                          </div>

                          {/* Skill Assignment form */}
                          {assignableSkills.length > 0 && (
                            <div className="space-y-2 border-t border-stroke/30 pt-2">
                              <div className="flex gap-2">
                                <select
                                  value={assigningSkillMap[agent.id] || ""}
                                  onChange={(e) =>
                                    setAssigningSkillMap((prev) => ({
                                      ...prev,
                                      [agent.id]: e.target.value,
                                    }))
                                  }
                                  className="flex-1 rounded border border-stroke bg-slate-950 px-2 py-1 text-xs text-white focus:outline-none focus:border-brand-primary cursor-pointer"
                                >
                                  <option value="">— Assign Skill —</option>
                                  {assignableSkills.map((s) => (
                                    <option key={s.id} value={s.id} title={s.description}>
                                      {s.name}
                                    </option>
                                  ))}
                                </select>
                                <button
                                  onClick={() => handleAssignSkill(agent.id)}
                                  disabled={!assigningSkillMap[agent.id]}
                                  className="rounded bg-brand-primary px-3 py-1 text-xs font-semibold text-slate-950 transition hover:opacity-90 disabled:opacity-50 cursor-pointer"
                                  type="button"
                                >
                                  Assign
                                </button>
                              </div>
                              {skillErrorMap[agent.id] && (
                                <p className="rounded border border-red-500/20 bg-red-950/40 p-2 text-xs text-red-200">
                                  {skillErrorMap[agent.id]}
                                </p>
                              )}
                            </div>
                          )}
                        </div>
                      );
                    })
                  )}
                </div>
              </div>
            </div>
          )}
        </section>
      </div>
    </main>
  );
}

function errorMessage(err: unknown, fallback: string) {
  return err instanceof ApiError ? err.message : fallback;
}

function ProjectMetric({ label, value }: { label: string; value: string }) {
  return (
    <article className="rounded-lg border border-stroke bg-slate-950 p-4">
      <div className="font-mono text-lg font-semibold text-white">{value}</div>
      <div className="text-xs text-content-muted">{label}</div>
    </article>
  );
}

function ProjectAgentCard({
  agent,
  badge,
  skillsCount,
}: {
  agent: Agent;
  badge: string;
  skillsCount: number;
}) {
  return (
    <article className="rounded-lg border border-stroke bg-slate-900 p-4">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h4 className="truncate font-mono font-semibold text-white">{agent.name}</h4>
          <p className="truncate text-xs text-content-muted">{agent.model_route}</p>
        </div>
        <span className="rounded border border-stroke bg-slate-950 px-2 py-0.5 font-mono text-[10px] font-bold uppercase tracking-wider text-content-muted">
          {badge}
        </span>
      </div>
      <div className="flex flex-wrap gap-2">
        <Badge value={agent.role} />
        <Badge value={agent.autonomy_level || "supervised"} />
        <Badge value={agent.status || "idle"} />
        <span className="rounded border border-stroke bg-slate-950 px-2 py-0.5 text-xs text-slate-300">
          {skillsCount} skills
        </span>
      </div>
    </article>
  );
}
