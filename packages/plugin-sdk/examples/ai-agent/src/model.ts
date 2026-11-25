// 模型与提供方：默认复用宿主设置页配置的 AI 服务（OpenAI 兼容），也可以在插件设置里改用 Anthropic 或单独的端点。
import type { Context } from 'lunabox';
import { createModels, createProvider } from '@earendil-works/pi-ai/models';
import { openAICompletionsApi } from '@earendil-works/pi-ai/api/openai-completions.lazy';
import { anthropicMessagesApi } from '@earendil-works/pi-ai/api/anthropic-messages.lazy';

export type Protocol = 'openai' | 'anthropic';

export interface AgentSettings {
  protocol: Protocol;
  baseUrl: string;
  apiKey: string;
  model: string;
  maxTokens: number;
  confirmWrites: boolean;
  systemPrompt: string;
}

export const CONFIG_SCHEMA = {
  title: 'AI 游戏库助手',
  description: '留空的项沿用宿主「设置 → AI」里的配置（OpenAI 兼容接口）。',
  icon: 'robot-happy',
  fields: [
    {
      key: 'protocol',
      type: 'enum' as const,
      label: '接口协议',
      default: 'openai',
      options: [
        { value: 'openai', label: 'OpenAI 兼容（/chat/completions）' },
        { value: 'anthropic', label: 'Anthropic（/v1/messages）' },
      ],
    },
    { key: 'baseUrl', type: 'string' as const, label: 'API Base URL', placeholder: '留空使用宿主设置', default: '' },
    { key: 'apiKey', type: 'secret' as const, label: 'API Key', placeholder: '留空使用宿主设置', default: '' },
    { key: 'model', type: 'string' as const, label: '模型', placeholder: '留空使用宿主设置', default: '' },
    { key: 'maxTokens', type: 'number' as const, label: '单次回复最大 token', default: 8192, min: 512, max: 131072, step: 512 },
    {
      key: 'confirmWrites',
      type: 'boolean' as const,
      label: '修改游戏库前确认',
      description: '新增、修改游戏与收藏夹前弹窗确认。删除游戏与执行命令始终需要确认。',
      default: true,
    },
    { key: 'systemPrompt', type: 'string' as const, label: '附加系统提示', placeholder: '例如：回答尽量简短', default: '' },
  ],
};

const ANTHROPIC_DEFAULT_URL = 'https://api.anthropic.com';

export function readSettings(ctx: Context): AgentSettings {
  const cfg = ctx.config.get<Record<string, unknown>>();
  const host = ctx.ai.getCredentials();
  const protocol: Protocol = cfg.protocol === 'anthropic' ? 'anthropic' : 'openai';
  const str = (v: unknown) => (typeof v === 'string' ? v.trim() : '');
  // 改用 Anthropic 时宿主的 OpenAI 兼容端点不能直接用，只沿用 Key（多数用户填的是同一家的中转）
  const fallbackUrl = protocol === 'anthropic' ? ANTHROPIC_DEFAULT_URL : host.baseUrl;
  const maxTokens = Number(cfg.maxTokens);
  return {
    protocol,
    baseUrl: (str(cfg.baseUrl) || fallbackUrl).replace(/\/+$/, ''),
    apiKey: str(cfg.apiKey) || host.apiKey,
    model: str(cfg.model) || host.model || (protocol === 'anthropic' ? 'claude-sonnet-4-5' : 'gpt-4o-mini'),
    maxTokens: Number.isFinite(maxTokens) && maxTokens > 0 ? maxTokens : 8192,
    confirmWrites: cfg.confirmWrites !== false,
    systemPrompt: str(cfg.systemPrompt),
  };
}

/** 模型显示名：用于界面上的徽标 */
export function modelLabel(s: AgentSettings): string {
  return s.apiKey ? s.model : '未配置';
}

const zeroCost = { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 };

/** 按当前设置创建 pi 的模型注册表与模型；没有 API Key 时返回 null */
export function buildModel(s: AgentSettings) {
  if (!s.apiKey) return null;
  const providerId = s.protocol === 'anthropic' ? 'lunabox-anthropic' : 'lunabox-openai';
  const api = s.protocol === 'anthropic' ? 'anthropic-messages' : 'openai-completions';
  const model = {
    id: s.model,
    name: s.model,
    api,
    provider: providerId,
    baseUrl: s.baseUrl,
    reasoning: false,
    input: ['text'],
    cost: zeroCost,
    contextWindow: 128000,
    maxTokens: s.maxTokens,
    // 兼容各家 OpenAI 中转：不用 developer 角色与 reasoning_effort
    compat: api === 'openai-completions' ? { supportsDeveloperRole: false, supportsReasoningEffort: false } : undefined,
  };
  const apiKey = s.apiKey;
  const provider = createProvider({
    id: providerId,
    name: providerId,
    baseUrl: s.baseUrl,
    auth: { apiKey: { name: providerId, resolve: async () => ({ auth: { apiKey } }) } },
    models: [model as never],
    api: (s.protocol === 'anthropic' ? anthropicMessagesApi() : openAICompletionsApi()) as never,
  });
  const models = createModels();
  models.setProvider(provider);
  const resolved = models.getModel(providerId, s.model);
  if (!resolved) return null;
  return { models, model: resolved };
}
