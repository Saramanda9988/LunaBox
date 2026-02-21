import type { appconf } from "../../../wailsjs/go/models";
import { useTranslation } from "react-i18next";
import { enums } from "../../../wailsjs/go/models";
import { BetterSelect } from "../ui/BetterSelect";

interface AISettingsProps {
  formData: appconf.AppConfig;
  onChange: (data: appconf.AppConfig) => void;
}

export function AISettingsPanel({ formData, onChange }: AISettingsProps) {
  const { t } = useTranslation();

  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    onChange({ ...formData, [name]: value } as appconf.AppConfig);
  };

  const PROMPT_LABELS: Record<string, string> = {
    DEFAULT_SYSTEM: t("settings.ai.promptLabels.DEFAULT_SYSTEM"),
    MEOW_ZAKO: t("settings.ai.promptLabels.MEOW_ZAKO"),
    STRICT_TUTOR: t("settings.ai.promptLabels.STRICT_TUTOR"),
  };

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">{t("settings.ai.providerLabel")}</label>
        <BetterSelect
          name="ai_provider"
          value={formData.ai_provider || ""}
          onChange={value => onChange({ ...formData, ai_provider: value } as appconf.AppConfig)}
          options={[
            { value: "", label: t("settings.ai.providerPlaceholder") },
            { value: "openai", label: "OpenAI" },
            { value: "deepseek", label: "DeepSeek" },
            { value: "custom", label: t("settings.ai.providerCustom") },
          ]}
        />
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">API Base URL</label>
        <input type="text" name="ai_base_url" value={formData.ai_base_url || ""} onChange={handleChange} placeholder="https://api.openai.com/v1" className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white" />
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">API Key</label>
        <input type="password" name="ai_api_key" value={formData.ai_api_key || ""} onChange={handleChange} placeholder="sk-..." className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white" />
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">{t("settings.ai.modelLabel")}</label>
        <input type="text" name="ai_model" value={formData.ai_model || ""} onChange={handleChange} placeholder="gpt-3.5-turbo" className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white" />
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">{t("settings.ai.systemPromptLabel")}</label>
        <div className="flex gap-2 mb-2">
          {Object.entries(enums.PromptType).map(([name, prompt]) => (
            <button
              key={name}
              type="button"
              onClick={() => onChange({ ...formData, ai_system_prompt: prompt } as appconf.AppConfig)}
              className="px-2 py-1 text-xs bg-brand-100 dark:bg-brand-700 text-brand-600 dark:text-brand-300 rounded hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
            >
              {PROMPT_LABELS[name] || name}
            </button>
          ))}
        </div>
        <textarea
          name="ai_system_prompt"
          value={formData.ai_system_prompt || ""}
          onChange={e => onChange({ ...formData, ai_system_prompt: e.target.value } as appconf.AppConfig)}
          rows={4}
          placeholder={t("settings.ai.systemPromptPlaceholder")}
          className="glass-input w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm"
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">{t("settings.ai.systemPromptHint")}</p>
      </div>
    </div>
  );
}
