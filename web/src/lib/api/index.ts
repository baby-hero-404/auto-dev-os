export { ApiError, request } from "./client";

import * as auth from "./auth";
import * as projects from "./projects";
import * as agents from "./agents";
import * as analytics from "./analytics";
import * as audit from "./audit";
import * as gateway from "./gateway";
import * as memories from "./memories";

export { auth, projects, agents, analytics, audit, gateway, memories };

export const api = {
  auth,
  projects,
  agents,
  analytics,
  audit,
  gateway,
  memories,

  register: auth.register,
  login: auth.login,
  getOrganization: auth.getOrganization,

  listProjects: projects.list,
  createProject: projects.create,
  getProject: projects.get,
  updateProject: projects.update,
  deleteProject: projects.remove,

  listRepositories: projects.repositories.list,
  createRepository: projects.repositories.create,
  validateRepository: projects.repositories.validate,
  cloneRepository: projects.repositories.clone,
  updateRepository: projects.repositories.update,
  getRemoteBranches: projects.repositories.getBranches,

  listTasks: projects.tasks.list,
  getTask: projects.tasks.get,
  createTask: projects.tasks.create,
  analyzeTask: projects.tasks.analyze,
  approveTaskAnalysis: projects.tasks.approveAnalysis,
  requestTaskChanges: projects.tasks.requestChanges,
  executeTask: projects.tasks.execute,
  taskWorkflow: projects.tasks.workflow,
  taskLogs: projects.tasks.logs,
  approveTaskWorkflow: projects.tasks.approveWorkflow,
  approvePR: projects.tasks.approvePR,
  rejectPR: projects.tasks.rejectPR,
  taskArtifacts: projects.tasks.artifacts,

  listAgents: agents.list,
  createAgent: agents.create,
  listOrgAgents: agents.listOrganization,
  hireAgent: agents.hire,
  listRoleTemplates: agents.roleTemplates,
  updateAgent: agents.update,
  deleteAgent: agents.remove,

  listRules: projects.rules.list,
  listGlobalRules: projects.rules.listGlobal,
  createRule: projects.rules.create,
  createGlobalRule: projects.rules.createGlobal,
  seedGlobalRules: projects.rules.seedGlobal,
  updateRule: projects.rules.update,
  deleteRule: projects.rules.remove,
  seedRules: projects.rules.seed,

  listSkills: agents.skills.list,
  seedSkills: agents.skills.seed,
  createSkill: agents.skills.create,
  updateSkill: agents.skills.update,
  deleteSkill: agents.skills.remove,

  tokenUsage: analytics.tokenUsage,
  analyticsOverview: analytics.overview,
  analyticsAgents: analytics.agents,
  analyticsTasks: analytics.tasks,
  analyticsWorkflows: analytics.workflows,

  auditLogs: audit.logs,
  auditSummary: audit.summary,

  listProviderCredentials: gateway.providerCredentials.list,
  createProviderCredential: gateway.providerCredentials.create,
  updateProviderCredential: gateway.providerCredentials.update,
  deleteProviderCredential: gateway.providerCredentials.remove,
  testProviderCredential: gateway.providerCredentials.test,
  testProviderCredentialInput: gateway.providerCredentials.testInput,
  listVirtualKeys: gateway.virtualKeys.list,
  createVirtualKey: gateway.virtualKeys.create,
  getVirtualKey: gateway.virtualKeys.get,
  updateVirtualKey: gateway.virtualKeys.update,
  revokeVirtualKey: gateway.virtualKeys.revoke,
  listModelRoutes: gateway.modelRoutes.list,
  createModelRoute: gateway.modelRoutes.create,
  updateModelRoute: gateway.modelRoutes.update,
  deleteModelRoute: gateway.modelRoutes.remove,

  listMemories: memories.list,
  searchMemories: memories.search,
  getMemory: memories.get,
  deleteMemory: memories.remove,
  listSuggestions: memories.suggestions.list,
  getSuggestion: memories.suggestions.get,
  approveSuggestion: memories.suggestions.approve,
  rejectSuggestion: memories.suggestions.reject,

  listGitAccounts: projects.gitAccounts.list,
  createGitAccount: projects.gitAccounts.create,
  deleteGitAccount: projects.gitAccounts.remove,
  testGitAccount: projects.gitAccounts.test,
};
