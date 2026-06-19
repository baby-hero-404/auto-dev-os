import { request } from "./client";
import type {
  CreateProviderModelInput,
  CreateProviderCredentialInput,
  ProviderModel,
  ProviderCredential,
  TestProviderCredentialInput,
  UpdateProviderModelInput,
  UpdateProviderCredentialInput,
} from "../types";

export const providerCredentials = {
  list(orgID: string, token: string) {
    return request<ProviderCredential[]>(`/organizations/${orgID}/provider-credentials`, { token });
  },
  create(orgID: string, token: string, input: CreateProviderCredentialInput) {
    return request<ProviderCredential>(`/organizations/${orgID}/provider-credentials`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  update(orgID: string, credentialID: string, token: string, input: UpdateProviderCredentialInput) {
    return request<ProviderCredential>(`/organizations/${orgID}/provider-credentials/${credentialID}`, {
      method: "PUT",
      token,
      body: JSON.stringify(input),
    });
  },
  remove(orgID: string, credentialID: string, token: string) {
    return request<void>(`/organizations/${orgID}/provider-credentials/${credentialID}`, {
      method: "DELETE",
      token,
    });
  },
  test(orgID: string, credentialID: string, token: string) {
    return request<{ status: string }>(`/organizations/${orgID}/provider-credentials/${credentialID}/test`, {
      method: "POST",
      token,
    });
  },
  testInput(orgID: string, token: string, input: TestProviderCredentialInput) {
    return request<{ status: string }>(`/organizations/${orgID}/provider-credentials/test`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
};


export const providerModels = {
  list(orgID: string, token: string) {
    return request<ProviderModel[]>(`/organizations/${orgID}/provider-models`, { token });
  },
  create(orgID: string, token: string, input: CreateProviderModelInput) {
    return request<ProviderModel>(`/organizations/${orgID}/provider-models`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  update(orgID: string, modelID: string, token: string, input: UpdateProviderModelInput) {
    return request<ProviderModel>(`/organizations/${orgID}/provider-models/${modelID}`, {
      method: "PUT",
      token,
      body: JSON.stringify(input),
    });
  },
  remove(orgID: string, modelID: string, token: string) {
    return request<void>(`/organizations/${orgID}/provider-models/${modelID}`, {
      method: "DELETE",
      token,
    });
  },
};
