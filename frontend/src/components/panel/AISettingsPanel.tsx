import type { AppConfig } from "../../../bindings/lunabox/internal/appconf";
import { DefaultPrompts } from "../../consts/prompts";

const PROMPT_LABELS: Record<string, string> = {
  DefaultSystemPrompt: "幽默评论员",
  MeowZakoPrompt: "雌小鬼猫娘",
  StrictTutorPrompt: "严厉导师",
};

interface AISettingsProps {
  formData: AppConfig;
  onChange: (data: AppConfig) => void;
}

export function AISettingsPanel({ formData, onChange }: AISettingsProps) {
  const handleChange = (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement>) => {
    const { name, value } = e.target;
    onChange({ ...formData, [name]: value } as AppConfig);
  };

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">AI 服务商</label>
        <select name="ai_provider" value={formData.ai_provider || ""} onChange={handleChange} className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white">
          <option value="">请选择</option>
          <option value="openai">OpenAI</option>
          <option value="deepseek">DeepSeek</option>
          <option value="custom">自定义 (OpenAI兼容)</option>
        </select>
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">API Base URL</label>
        <input type="text" name="ai_base_url" value={formData.ai_base_url || ""} onChange={handleChange} placeholder="https://api.openai.com/v1" className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white" />
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">API Key</label>
        <input type="password" name="ai_api_key" value={formData.ai_api_key || ""} onChange={handleChange} placeholder="sk-..." className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white" />
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">模型名称</label>
        <input type="text" name="ai_model" value={formData.ai_model || ""} onChange={handleChange} placeholder="gpt-3.5-turbo" className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white" />
      </div>
      <div className="space-y-2">
        <label className="block text-sm font-medium text-brand-700 dark:text-brand-300">系统提示语 (System Prompt)</label>
        <div className="flex gap-2 mb-2">
          {DefaultPrompts.map(({ name, prompt }) => (
            <button
              type="button"
              key={name}
              onClick={() => onChange({ ...formData, ai_system_prompt: prompt } as AppConfig)}
              className="px-2 py-1 text-xs bg-brand-100 dark:bg-brand-700 text-brand-600 dark:text-brand-300 rounded hover:bg-brand-200 dark:hover:bg-brand-600 transition-colors"
            >
              {PROMPT_LABELS[name] || name}
            </button>
          ))}
        </div>
        <textarea
          name="ai_system_prompt"
          value={formData.ai_system_prompt || ""}
          onChange={e => onChange({ ...formData, ai_system_prompt: e.target.value } as AppConfig)}
          rows={4}
          placeholder="输入自定义的 AI 系统提示语..."
          className="w-full px-3 py-2 border border-brand-300 dark:border-brand-600 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-neutral-500 dark:bg-brand-700 dark:text-white text-sm"
        />
        <p className="text-xs text-brand-500 dark:text-brand-400">AI 将根据此提示语来生成统计总结。你可以点击上方预设快速填充。</p>
      </div>
    </div>
  );
}
