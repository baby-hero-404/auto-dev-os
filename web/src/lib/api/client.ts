import { useAuthStore } from "../store/use-auth-store";
import type { User } from "../types";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:32080/api/v1";

export type RequestOptions = RequestInit & {
  token?: string | null;
};

export class ApiError extends Error {
  status: number;

  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

let isRefreshing = false;
let refreshSubscribers: ((token: string) => void)[] = [];

function subscribeTokenRefresh(cb: (token: string) => void) {
  refreshSubscribers.push(cb);
}

function onRefreshed(token: string) {
  refreshSubscribers.forEach((cb) => cb(token));
  refreshSubscribers = [];
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers(options.headers);
  headers.set("Content-Type", "application/json");
  
  const token = options.token || (typeof window !== "undefined" ? useAuthStore.getState().session?.token : undefined);
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    headers,
  });

  if (!response.ok) {
    let message = response.statusText;
    try {
      const body = (await response.json()) as { error?: string };
      message = body.error ?? message;
    } catch {
      // Keep status text when response is not JSON.
    }

    if (
      response.status === 401 ||
      message.includes("token expired") ||
      message === "validation: token expired"
    ) {
      if (typeof window !== "undefined") {
        const session = useAuthStore.getState().session;
        if (session && session.refresh_token) {
          if (!isRefreshing) {
            isRefreshing = true;
            try {
              const refreshResponse = await fetch(`${API_BASE}/auth/refresh`, {
                method: "POST",
                headers: {
                  "Content-Type": "application/json",
                },
                body: JSON.stringify({ refresh_token: session.refresh_token }),
              });
              if (refreshResponse.ok) {
                const data = (await refreshResponse.json()) as {
                  user: User;
                  tokens: {
                    access_token: string;
                    refresh_token: string;
                  };
                };
                useAuthStore.getState().setSession({
                  token: data.tokens.access_token,
                  refresh_token: data.tokens.refresh_token,
                  user: data.user,
                });
                isRefreshing = false;
                onRefreshed(data.tokens.access_token);
              } else {
                isRefreshing = false;
                useAuthStore.getState().clearSession();
                throw new ApiError(response.status, message);
              }
            } catch {
              isRefreshing = false;
              useAuthStore.getState().clearSession();
              throw new ApiError(response.status, message);
            }
          }

          const newToken = await new Promise<string>((resolve) => {
            subscribeTokenRefresh((token) => resolve(token));
          });

          return request<T>(path, {
            ...options,
            token: newToken,
          });
        } else {
          useAuthStore.getState().clearSession();
        }
      }
    }

    throw new ApiError(response.status, message);
  }

  if (response.status === 204) {
    return undefined as T;
  }
  return response.json() as Promise<T>;
}
