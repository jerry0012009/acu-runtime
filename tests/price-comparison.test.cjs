const { test } = require('node:test');
const assert = require('node:assert/strict');
require('../web/price-comparison.js');

test('multipliers display exactly three decimal places without rounding prices', () => {
  assert.equal(globalThis.acuFormatPriceMultiplier(0.07), '0.070×');
  assert.equal(globalThis.acuFormatPriceMultiplier(0.014836795), '0.015×');
  assert.equal(globalThis.acuFormatPriceMultiplier(1), '1.000×');
  assert.equal(globalThis.acuFormatPriceMultiplier(undefined), '—');
});

const reference = { input_cny_per_million: 67.4, output_cny_per_million: 337, source_name: 'Public USD reference', fx_cny_per_usd: 6.74 };
test('real-spend multiplier ignores the credit multiplier and compares actual CNY prices', () => {
  const result = globalThis.acuPriceComparison({ payable: { input_cny_per_million: 2, output_cny_per_million: 10, model_multiplier: '0.1' }, comparison_reference: reference });
  assert(Math.abs(result.standard - 0.02967359050445104) < 1e-12);
});
test('different component discounts are reported separately instead of inventing one multiplier', () => {
  const result = globalThis.acuPriceComparison({ payable: { input_cny_per_million: 2, output_cny_per_million: 20 }, comparison_reference: reference });
  assert.equal(result.standard, undefined);
  assert.equal(result.input, 2 / 67.4);
  assert.equal(result.output, 20 / 337);
});
test('long-context comparison uses both long-context tariffs at the same boundary', () => {
  const result = globalThis.acuPriceComparison({ payable: { input_cny_per_million: 2, output_cny_per_million: 10, context_tiers: { thresholdTokens: 272000, longContext: { inputPricePerMillion: 4, outputPricePerMillion: 15 } } }, comparison_reference: { ...reference, context_tiers: { thresholdTokens: 272000, longContext: { inputPricePerMillion: 134.8, outputPricePerMillion: 505.5 } } } });
  assert(Math.abs(result.long - result.standard) < 1e-12);
});
test('missing public references and invalid values never fall back to profile multipliers', () => {
  for (const value of [undefined, null, '', -1, Infinity, NaN]) {
    const result = globalThis.acuPriceComparison({ payable: { input_cny_per_million: value, output_cny_per_million: 10, model_multiplier: '0.1' }, comparison_reference: reference });
    assert.equal(result.standard, undefined);
  }
  assert.equal(globalThis.acuPriceComparison({ payable: { model_multiplier: '0.1' } }).standard, undefined);
});

test('Opus 5.5 shows the canonical price once and is searchable by either spelling', () => {
  const canonical = { model_name: 'claude-opus-5-5', display_name: 'Claude Opus 5.5', payable: { output_cny_per_million: 9.436 } };
  const alias = { model_name: 'claude-opus-5.5', payable: { output_cny_per_million: 22.5 } };
  const fast = { model_name: 'claude-opus-5-5-fast' };
  const rows = globalThis.acuPriceCatalogRows;
  assert.deepEqual(rows([alias, fast, canonical], ''), [fast, canonical]);
  assert.deepEqual(rows([canonical, alias], '  CLAUDE-OPUS-5.5 '), [canonical]);
  assert.deepEqual(rows([alias, canonical], 'claude-opus-5-5'), [canonical]);
  assert.deepEqual(rows([alias], ''), [alias]);
  assert.equal(alias.payable.output_cny_per_million, 22.5);
});

test('flat ACU rates can be compared to each official tier without inventing a surcharge', () => {
  const ref = { ...reference, context_tiers: { thresholdTokens: 200000, longContext: { inputPricePerMillion: 134.8, outputPricePerMillion: 505.5 } } };
  const payable = { input_cny_per_million: 2, output_cny_per_million: 10 };
  const result = globalThis.acuPriceComparison({ payable, comparison_reference: ref });
  assert.equal(result.longThreshold, 200000);
  assert.equal(result.longInput, 2 / 134.8);
  assert.equal(result.longOutput, 10 / 505.5);
  assert.equal(result.long, undefined);
  assert.equal(globalThis.acuPriceComparison({ payable: { ...payable, context_tiers: { thresholdTokens: 272000 } }, comparison_reference: ref }).longInput, undefined);
});

test('official links come from published provenance and do not accept executable URLs', () => {
  const source = globalThis.acuPriceSourceURL;
  assert.equal(source('https://platform.claude.com/docs/en/about-claude/pricing; Text tariff'), 'https://platform.claude.com/docs/en/about-claude/pricing');
  assert.equal(source('javascript:alert(1)'), '');
  assert.equal(source('https://user@platform.claude.com/'), '');
});
