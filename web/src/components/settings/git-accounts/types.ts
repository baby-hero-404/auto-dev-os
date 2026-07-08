export type ProviderOption = "github" | "gitlab";
export type ActionState = "idle" | "testing" | "success" | "error";

export const initialForm = {
  provider: "github" as ProviderOption,
  displayName: "My GitHub account",
  token: "",
  baseURL: "",
};
