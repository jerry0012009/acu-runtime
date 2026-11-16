import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { readFileSync, writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

export function parseLeaderboard(html, observedAt = new Date().toISOString().slice(0, 10)) {
  const chunks = [...html.matchAll(/self\.__next_f\.push\((\[.*?\])\)<\/script>/g)]
    .map(match => { try { return JSON.parse(match[1])[1]; } catch { return ''; } })
    .filter(value => typeof value === 'string').join('');
  let models;
  const findModels = value => {
    if (!value || typeof value !== 'object') return;
    if (Array.isArray(value.initialModels)) models = value.initialModels;
    Object.values(value).forEach(findModels);
  };
  for (const line of chunks.split('\n')) {
    try { findModels(JSON.parse(line.slice(line.indexOf(':') + 1))); } catch { /* Non-JSON Flight records. */ }
  }
  assert(models?.length, 'Public leaderboard model data missing');
  const benchmark = chunks.match(/"name":"(Artificial Analysis Intelligence Index v[0-9.]+)"/)?.[1];
  assert(benchmark, 'Benchmark version missing');
  const grouped = {};
  for (const model of models) {
    const slug = model.release?.slug;
    const score = model.intelligenceIndex;
    if (model.deprecated || !/^[a-z0-9-]+$/.test(slug ?? '') || typeof score !== 'number' || !Number.isFinite(score) || score < 0 || score > 100) continue;
    const variants = grouped[slug] ??= [];
    assert(typeof model.name === 'string' && model.name.length > 0);
    variants.push({ name: model.name, score });
  }
  for (const variants of Object.values(grouped)) variants.sort((a, b) => b.score - a.score || a.name.localeCompare(b.name));
  assert(Object.keys(grouped).length > 20, 'Incomplete leaderboard snapshot');
  return { benchmark, sourceUrl: 'https://artificialanalysis.ai/leaderboards/models', observedAt,
    selection: 'highest_published_configuration', sourceSha256: createHash('sha256').update(html).digest('hex'), models: grouped };
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const input = process.argv[2];
  assert(input, 'Usage: node scripts/refresh-model-benchmarks.mjs LEADERBOARD_HTML [OUTPUT_JSON]');
  const snapshot = parseLeaderboard(readFileSync(input, 'utf8'));
  const output = process.argv[3] ?? new URL('../model_benchmarks.json', import.meta.url);
  writeFileSync(output, JSON.stringify(snapshot, null, 2) + '\n');
  console.log(JSON.stringify({ benchmark: snapshot.benchmark, models: Object.keys(snapshot.models).length, observedAt: snapshot.observedAt }));
}
