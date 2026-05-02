import type { appconf } from "../../../wailsjs/go/models";
import { useTranslation } from "react-i18next";
import { enums } from "../../../wailsjs/go/models";
import { BetterSelect } from "../ui/better/BetterSelect";
import { BetterSwitch } from "../ui/better/BetterSwitch";

interface AISettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

const defaultMCPPort = 39200;

function normalizeMCPPort(port: number | undefined) {
  if (!Number.isInteger(port) || port! < 1 || port! > 65535) {
    return defaultMCPPort;
  }
  return port!;
}

export function AISettingsPanel({ formData, onChange }: AISettingsProps) {
  const { t } = useTranslation();
  const effectiveMCPPort = normalizeMCPPort(formData.mcp_port);
  const mcpEndpoint = `http://127.0.0.1:${effectiveMCPPort}/mcp`;

  const handleChange = (
    e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>,
  ) => {
    const { name, value } = e.target;
    onChange({ ...formData, [name]: value } as appconf.AppConfig);
  };

  const handleMCPPortChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onChange({
      ...formData,
      mcp_port: normalizeMCPPort(e.target.valueAsNumber),
    } as appconf.AppConfig);
  };

  const PROMPT_LABELS: Record<string, string> = {
    DEFAULT_SYSTEM: t("settings.ai.promptLabels.DEFAULT_SYSTEM"),
    MEOW_ZAKO: t("settings.ai.promptLabels.MEOW_ZAKO"),
    STRICT_TUTOR: t("settings.ai.promptLabels.STRICT_TUTOR"),
  };

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.ai.providerLabel")}
        </label>
        <BetterSelect
          name="ai_provider"
          value={formData.ai_provider || ""}
          onChange={value =>
            onChange({ ...formData, ai_provider: value } as appconf.AppConfig)}
          options={[
            { value: "", label: t("settings.ai.providerPlaceholder") },
            { value: "openai", label: "OpenAI" },
            { value: "deepseek", label: "DeepSeek" },
            { value: "custom", label: t("settings.ai.providerCustom") },
          ]}
        />
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          API Base URL
        </label>
        <input
          type="text"
          name="ai_base_url"
          value={formData.ai_base_url || ""}
          onChange={handleChange}
          placeholder="https://api.openai.com/v1"
          className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
        />
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          API Key
        </label>
        <input
          type="password"
          name="ai_api_key"
          value={formData.ai_api_key || ""}
          onChange={handleChange}
          placeholder="sk-..."
          className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
        />
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.ai.modelLabel")}
        </label>
        <input
          type="text"
          name="ai_model"
          value={formData.ai_model || ""}
          onChange={handleChange}
          placeholder="gpt-3.5-turbo"
          className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white"
        />
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.ai.systemPromptLabel")}
        </label>
        <div className="flex flex-wrap gap-2">
          {Object.entries(enums.PromptType).map(([name, prompt]) => (
            <button
              key={name}
              type="button"
              onClick={() =>
                onChange({
                  ...formData,
                  ai_system_prompt: prompt,
                } as appconf.AppConfig)}
              className="px-2 py-1 text-xs bg-brand-100 dark:bg-brand-700 text-brand-600 dark:text-brand-300 rounded hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
            >
              {PROMPT_LABELS[name] || name}
            </button>
          ))}
        </div>
        <textarea
          name="ai_system_prompt"
          value={formData.ai_system_prompt || ""}
          onChange={e =>
            onChange({
              ...formData,
              ai_system_prompt: e.target.value,
            } as appconf.AppConfig)}
          rows={4}
          placeholder={t("settings.ai.systemPromptPlaceholder")}
          className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm"
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.ai.systemPromptHint")}
        </p>
      </div>

      {/* 防剧透等级 */}
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.ai.spoilerLevelLabel")}
        </label>
        <BetterSelect
          name="ai_spoiler_level"
          value={formData.ai_spoiler_level || "none"}
          onChange={value =>
            onChange({
              ...formData,
              ai_spoiler_level: value,
            } as appconf.AppConfig)}
          options={[
            { value: "none", label: t("settings.ai.spoilerLevelOpts.none") },
            { value: "mild", label: t("settings.ai.spoilerLevelOpts.mild") },
            { value: "full", label: t("settings.ai.spoilerLevelOpts.full") },
          ]}
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.ai.spoilerLevelHint")}
        </p>
      </div>

      {/* WebSearch 开关 */}
      <div className="space-y-2">
        <div className="flex items-center justify-between gap-4">
          <div className="flex-1 space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("settings.ai.webSearchLabel")}
            </label>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.ai.webSearchHint")}
            </p>
          </div>
          <BetterSwitch
            id="ai_web_search"
            checked={formData.ai_web_search || false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                ai_web_search: checked,
              } as appconf.AppConfig)}
          />
        </div>
      </div>

      {/* Tavily API Key（WebSearch 启用时显示） */}
      {formData.ai_web_search && (
        <div className="space-y-2">
          <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
            {t("settings.ai.tavilyKeyLabel")}
          </label>
          <input
            type="password"
            name="tavily_api_key"
            value={formData.tavily_api_key || ""}
            onChange={handleChange}
            placeholder="tvly-..."
            className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm"
          />
          <p className="text-xs text-brand-500 dark:text-brand-400">
            {t("settings.ai.tavilyKeyHint")}
          </p>
        </div>
      )}

      <div className="space-y-2">
        <span className="block text-sm font-semibold text-brand-700 dark:text-brand-300">
          {t("settings.ai.mcpSectionLabel")}
        </span>
      </div>

      <div className="space-y-2">
        <div className="flex items-center justify-between gap-4">
          <div className="flex-1 space-y-2">
            <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
              {t("settings.ai.mcpEnableLabel")}
            </label>
            <p className="text-xs text-brand-500 dark:text-brand-400">
              {t("settings.ai.mcpEnableHint")}
            </p>
          </div>
          <BetterSwitch
            id="mcp_enabled"
            checked={formData.mcp_enabled || false}
            onCheckedChange={checked =>
              onChange({
                ...formData,
                mcp_enabled: checked,
              } as appconf.AppConfig)}
          />
        </div>
      </div>

      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">
          {t("settings.ai.mcpPortLabel")}
        </label>
        <input
          type="number"
          name="mcp_port"
          min={1}
          max={65535}
          value={effectiveMCPPort}
          onChange={handleMCPPortChange}
          className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm"
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.ai.mcpPortHint")}
        </p>
        <p className="text-xs text-brand-500 dark:text-brand-400">
          {t("settings.ai.mcpEndpointHint", { endpoint: mcpEndpoint })}
        </p>
      </div>
    </div>
  );
}
