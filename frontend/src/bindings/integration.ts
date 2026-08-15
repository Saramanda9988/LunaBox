import type { service } from "./models";

import * as GeneratedIntegrationService from "../../bindings/lunabox/internal/service/integrationservice";

export type SteamCompatibilityTool = {
  name: string;
  display_name: string;
  path: string;
  built_in: boolean;
};

export type SteamCompatibilityInfo = {
  supported: boolean;
  steam_installed: boolean;
  steam_root: string;
  app_id: string;
  proton_prefix: string;
  current_tool: string;
  default_tool: string;
  tools: SteamCompatibilityTool[];
};

type IntegrationServiceCompat = typeof GeneratedIntegrationService & {
  GetGameSteamCompatibility?: (
    gameID: string,
  ) => Promise<SteamCompatibilityInfo>;
  SetGameSteamCompatibilityTool?: (
    gameID: string,
    toolName: string,
  ) => Promise<SteamCompatibilityInfo>;
  SetGameSteamLaunchOptions?: (
    gameID: string,
    launchOptions: string,
  ) => Promise<service.SteamLaunchStatus>;
  OpenGameSteamProtonPrefix?: (gameID: string) => Promise<string>;
  RestartSteamClient?: () => Promise<void>;
};

const integrationService
  = GeneratedIntegrationService as IntegrationServiceCompat;

function missingBinding<T>(method: string): Promise<T> {
  return Promise.reject(
    new Error(`${method} binding is not generated yet`),
  );
}

export function GetGameSteamCompatibility(
  gameID: string,
): Promise<SteamCompatibilityInfo> {
  if (integrationService.GetGameSteamCompatibility) {
    return integrationService.GetGameSteamCompatibility(gameID);
  }
  return missingBinding<SteamCompatibilityInfo>("GetGameSteamCompatibility");
}

export function SetGameSteamCompatibilityTool(
  gameID: string,
  toolName: string,
): Promise<SteamCompatibilityInfo> {
  if (integrationService.SetGameSteamCompatibilityTool) {
    return integrationService.SetGameSteamCompatibilityTool(gameID, toolName);
  }
  return missingBinding<SteamCompatibilityInfo>("SetGameSteamCompatibilityTool");
}

export function SetGameSteamLaunchOptions(
  gameID: string,
  launchOptions: string,
): Promise<service.SteamLaunchStatus> {
  if (integrationService.SetGameSteamLaunchOptions) {
    return integrationService.SetGameSteamLaunchOptions(gameID, launchOptions);
  }
  return missingBinding<service.SteamLaunchStatus>("SetGameSteamLaunchOptions");
}

export function OpenGameSteamProtonPrefix(gameID: string): Promise<string> {
  if (integrationService.OpenGameSteamProtonPrefix) {
    return integrationService.OpenGameSteamProtonPrefix(gameID);
  }
  return missingBinding<string>("OpenGameSteamProtonPrefix");
}

export function RestartSteamClient(): Promise<void> {
  if (integrationService.RestartSteamClient) {
    return integrationService.RestartSteamClient();
  }
  return missingBinding<void>("RestartSteamClient");
}
