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
