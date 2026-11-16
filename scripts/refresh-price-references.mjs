import { readFileSync, writeFileSync } from 'node:fs';

// Public provider tariffs only. Upstream credit conversions never enter this snapshot.
const data = process.env.MODELS_DEV_FIXTURE
  ? JSON.parse(readFileSync(process.env.MODELS_DEV_FIXTURE, 'utf8'))
  : await (async () => {
    const response = await fetch('https://models.dev/api.json', { signal: AbortSignal.timeout(15000) });
    if (!response.ok) throw new Error(`Public reference HTTP ${response.status}`);
    return response.json();
  })();
const models = {};
for (const provider of ['openai', 'anthropic', 'google', 'deepseek', 'xai', 'moonshotai', 'zai', 'minimax', 'alibaba', 'longcat']) {
  for (const [id, model] of Object.entries(data[provider]?.models || {})) {
    const cost = model.cost;
    if (!Number.isFinite(cost?.input) || cost.input <= 0 || !Number.isFinite(cost.output) || cost.output <= 0) continue;
    const prices = value => ({ inputPricePerMillion: value.input, outputPricePerMillion: value.output,
      ...(Number.isFinite(value.cache_read) ? { cachedInputPricePerMillion: value.cache_read } : {}),
      ...(Number.isFinite(value.cache_write) ? { cacheWritePricePerMillion: value.cache_write } : {}) });
    const entry = { ...prices(cost), source: `https://models.dev/api.json · ${provider}/${id}` };
    const tiers = (cost.tiers || []).filter(tier => tier.tier?.type === 'context');
    if (tiers.length === 1 && Number.isSafeInteger(tiers[0].tier.size) && tiers[0].tier.size > 0 && Number.isFinite(tiers[0].input) && Number.isFinite(tiers[0].output)) {
      entry.contextTiers = { thresholdTokens: tiers[0].tier.size, standard: prices(cost), longContext: prices(tiers[0]) };
    }
    if (!models[id]) models[id] = entry;
  }
}
if (!models['gpt-6-astra'] || !models['gpt-6-sol'] || !models['gpt-6.1-sol']) throw new Error('Incomplete public reference catalog');
writeFileSync(new URL('../price_references.json', import.meta.url), JSON.stringify({ observedAt: new Date().toISOString().slice(0, 10), currency: 'USD', models }, null, 2) + '\n');
