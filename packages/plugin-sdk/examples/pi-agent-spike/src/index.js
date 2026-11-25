// Feasibility spike: run the pi agent stack (@earendil-works/pi-ai + pi-agent-core,
// formerly @mariozechner/*) inside the LunaBox plugin runtime (moejs).
//
// apply(ctx) only registers nothing heavy; the Go test in
// packages/backend/plugin/pi_spike_test.go calls the exported run* functions directly.
import { Agent } from '@earendil-works/pi-agent-core';
import { createModels, createProvider } from '@earendil-works/pi-ai/models';
import { openAICompletionsApi } from '@earendil-works/pi-ai/api/openai-completions.lazy';
import { anthropicMessagesApi } from '@earendil-works/pi-ai/api/anthropic-messages.lazy';

const zeroCost = { input: 0, output: 0, cacheRead: 0, cacheWrite: 0 };

function buildModels({ providerId, api, apiImpl, baseUrl, apiKey, modelId }) {
  const model = {
    id: modelId,
    name: modelId,
    api,
    provider: providerId,
    baseUrl,
    reasoning: false,
    input: ['text'],
    cost: zeroCost,
    contextWindow: 128000,
    maxTokens: 4096,
    compat: api === 'openai-completions'
      ? { supportsDeveloperRole: false, supportsReasoningEffort: false }
      : undefined,
  };
  const provider = createProvider({
    id: providerId,
    name: providerId,
    baseUrl,
    auth: { apiKey: { name: providerId, resolve: async () => ({ auth: { apiKey } }) } },
    models: [model],
    api: apiImpl,
  });
  const models = createModels();
  models.setProvider(provider);
  return { models, model: models.getModel(providerId, modelId) };
}

// One agent run with a single constant tool. Returns a JSON-able summary.
async function runAgent(cfg, { withTool }) {
  const { models, model } = buildModels(cfg);
  if (!model) throw new Error('model not registered');

  const toolCalls = [];
  const deltas = [];
  const events = [];

  const tools = withTool
    ? [{
        name: 'get_game_count',
        label: 'Game count',
        description: 'Return how many games are in the local LunaBox library',
        parameters: {
          type: 'object',
          properties: { status: { type: 'string', enum: ['all', 'installed'], description: 'Which games to count' } },
          additionalProperties: false,
        },
        execute: async (toolCallId, params) => {
          toolCalls.push({ toolCallId, params });
          return { content: [{ type: 'text', text: '42' }], details: { count: 42 } };
        },
      }]
    : [];

  const agent = new Agent({
    initialState: {
      systemPrompt: 'You are the LunaBox library assistant.',
      model,
      tools,
    },
    streamFn: models.streamSimple.bind(models),
    identity: null,
  });

  agent.subscribe((event) => {
    events.push(event.type);
    if (event.type === 'message_update' && event.assistantMessageEvent.type === 'text_delta') {
      deltas.push(event.assistantMessageEvent.delta);
      if (cfg.abortOnFirstDelta && deltas.length === 1) agent.abort();
    }
  });

  await agent.prompt(cfg.prompt || 'How many games do I have?');

  const last = agent.state.messages[agent.state.messages.length - 1];
  return {
    toolCalls,
    deltas,
    text: deltas.join(''),
    events,
    errorMessage: agent.state.errorMessage || (last && last.errorMessage) || '',
    stopReason: last && last.stopReason,
    messageRoles: agent.state.messages.map((m) => m.role),
  };
}

export function runOpenAI(opts) {
  return runAgent({
    providerId: 'spike-openai',
    api: 'openai-completions',
    apiImpl: openAICompletionsApi(),
    baseUrl: opts.baseUrl,
    apiKey: opts.apiKey,
    modelId: opts.model || 'spike-model',
    prompt: opts.prompt,
    abortOnFirstDelta: !!opts.abortOnFirstDelta,
  }, { withTool: !opts.noTool });
}

export function runAnthropic(opts) {
  return runAgent({
    providerId: 'spike-anthropic',
    api: 'anthropic-messages',
    apiImpl: anthropicMessagesApi(),
    baseUrl: opts.baseUrl,
    apiKey: opts.apiKey,
    modelId: opts.model || 'claude-spike',
    prompt: opts.prompt,
  }, { withTool: !!opts.withTool });
}

// Reports which globals the runtime provides (after the bundle's injected polyfills,
// which only rebind free identifiers inside the bundle, not globalThis).
export function probeGlobals() {
  const names = ['structuredClone', 'crypto', 'FinalizationRegistry', 'WeakRef', 'process', 'FormData', 'Blob', 'File',
    'queueMicrotask', 'TextDecoderStream', 'ReadableStream', 'AbortSignal', 'DOMException', 'Event', 'EventTarget',
    'navigator', 'performance', 'Buffer', 'atob', 'btoa', 'Intl'];
  const out = {};
  for (const n of names) out[n] = typeof globalThis[n];
  out['crypto.getRandomValues'] = typeof globalThis.crypto?.getRandomValues;
  out['crypto.randomUUID'] = typeof globalThis.crypto?.randomUUID;
  out['AbortSignal.any'] = typeof globalThis.AbortSignal?.any;
  out['AbortSignal.timeout'] = typeof globalThis.AbortSignal?.timeout;
  out['ReadableStream.prototype[Symbol.asyncIterator]'] = typeof globalThis.ReadableStream?.prototype?.[Symbol.asyncIterator];
  return out;
}

export function apply(ctx) {
  ctx.logger.info('pi-agent-spike loaded (no UI; exercised by pi_spike_test.go)');
}
