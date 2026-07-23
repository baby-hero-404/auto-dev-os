export { ApiError, request } from "./client";

import * as auth from "./auth";
import * as projects from "./projects";
import * as agents from "./agents";
import * as analytics from "./analytics";
import * as audit from "./audit";
import * as gateway from "./gateway";
import * as memories from "./memories";
import * as attestations from "./attestations";
import * as governance from "./governance";
import * as learnedSkills from "./learned-skills";

export { auth, projects, agents, analytics, audit, gateway, memories, attestations, governance, learnedSkills };

export const api = {
  auth,
  projects,
  agents,
  analytics,
  audit,
  gateway,
  memories,
  attestations,
  governance,
  learnedSkills,

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
  deleteRepository: projects.repositories.delete,

  listTasks: projects.tasks.list,
  getTask: projects.tasks.get,
  createTask: projects.tasks.create,
  updateTask: projects.tasks.update,
  deleteTask: projects.tasks.delete,
  analyzeTask: projects.tasks.analyze,
  approveTaskAnalysis: projects.tasks.approveAnalysis,
  requestTaskChanges: projects.tasks.requestChanges,
  clarifyTask: projects.tasks.clarify,
  executeTask: projects.tasks.execute,
  retryTask: projects.tasks.retry,
  pauseTask: projects.tasks.pause,
  cancelTask: projects.tasks.cancel,
  taskWorkflow: projects.tasks.workflow,
  taskLogs: projects.tasks.logs,
  streamTaskLogs: projects.tasks.streamLogs,
  approveTaskWorkflow: projects.tasks.approveWorkflow,
  approvePR: projects.tasks.approvePR,
  rejectPR: projects.tasks.rejectPR,
  startReview: projects.tasks.startReview,
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
  listSkillSources: agents.skills.listSources,
  createSkillSource: agents.skills.addSource,
  deleteSkillSource: agents.skills.deleteSource,
  syncSkillSource: agents.skills.syncSource,
  listSkillSourceFiles: agents.skills.listSourceFiles,
  getSkillSourceFileContent: agents.skills.getSourceFileContent,

  tokenUsage: analytics.tokenUsage,
  analyticsOverview: analytics.overview,
  analyticsAgents: analytics.agents,
  analyticsTasks: analytics.tasks,
  analyticsGatewayUsage: analytics.gatewayUsage,
  analyticsWorkflows: analytics.workflows,
  analyticsFailures: analytics.failures,

  auditLogs: audit.logs,
  auditSummary: audit.summary,

  listProviderCredentials: gateway.providerCredentials.list,
  createProviderCredential: gateway.providerCredentials.create,
  updateProviderCredential: gateway.providerCredentials.update,
  deleteProviderCredential: gateway.providerCredentials.remove,
  testProviderCredential: gateway.providerCredentials.test,
  testProviderCredentialInput: gateway.providerCredentials.testInput,
  listProviderModels: gateway.providerModels.list,
  createProviderModel: gateway.providerModels.create,
  updateProviderModel: gateway.providerModels.update,
  deleteProviderModel: gateway.providerModels.remove,

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
