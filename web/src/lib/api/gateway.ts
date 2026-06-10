import { request } from "./client";
import type {
  CreateModelRouteInput,
  CreateProviderCredentialInput,
  CreateVirtualKeyInput,
  ModelRoute,
  ProviderCredential,
  UpdateModelRouteInput,
  UpdateProviderCredentialInput,
  UpdateVirtualKeyInput,
  CreatedVirtualKey,
  VirtualKey,
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
};

export const virtualKeys = {
  list(orgID: string, token: string) {
    return request<VirtualKey[]>(`/organizations/${orgID}/virtual-keys`, { token });
  },
  create(orgID: string, token: string, input: CreateVirtualKeyInput) {
    return request<CreatedVirtualKey>(`/organizations/${orgID}/virtual-keys`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  get(orgID: string, virtualKeyID: string, token: string) {
    return request<VirtualKey>(`/organizations/${orgID}/virtual-keys/${virtualKeyID}`, { token });
  },
  update(orgID: string, virtualKeyID: string, token: string, input: UpdateVirtualKeyInput) {
    return request<VirtualKey>(`/organizations/${orgID}/virtual-keys/${virtualKeyID}`, {
      method: "PUT",
      token,
      body: JSON.stringify(input),
    });
  },
  revoke(orgID: string, virtualKeyID: string, token: string) {
    return request<void>(`/organizations/${orgID}/virtual-keys/${virtualKeyID}`, {
      method: "DELETE",
      token,
    });
  },
};

export const modelRoutes = {
  list(orgID: string, token: string) {
    return request<ModelRoute[]>(`/organizations/${orgID}/model-routes`, { token });
  },
  create(orgID: string, token: string, input: CreateModelRouteInput) {
    return request<ModelRoute>(`/organizations/${orgID}/model-routes`, {
      method: "POST",
      token,
      body: JSON.stringify(input),
    });
  },
  update(orgID: string, routeID: string, token: string, input: UpdateModelRouteInput) {
    return request<ModelRoute>(`/organizations/${orgID}/model-routes/${routeID}`, {
      method: "PUT",
      token,
      body: JSON.stringify(input),
    });
  },
  remove(orgID: string, routeID: string, token: string) {
    return request<void>(`/organizations/${orgID}/model-routes/${routeID}`, {
      method: "DELETE",
      token,
    });
  },
};
