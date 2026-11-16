globalThis.acuFormatPriceMultiplier = value => typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value.toFixed(3) + '×' : '—';
globalThis.acuPriceComparison = model => {
  const reference = model.comparison_reference;
  const payable = model.payable;
  const valid = value => typeof value === 'number' && Number.isFinite(value) && value >= 0;
  const ratio = (value, base) => valid(value) && valid(base) && base > 0 ? value / base : undefined;
  const input = ratio(payable?.input_cny_per_million, reference?.input_cny_per_million);
  const output = ratio(payable?.output_cny_per_million, reference?.output_cny_per_million);
  const uniform = (left, right) => left !== undefined && right !== undefined && Math.abs(left - right) <= Math.max(1, left, right) * 1e-10 ? left : undefined;
  const standard = uniform(input, output);
  let long;
  const tiers = payable?.context_tiers;
  const refTiers = reference?.context_tiers;
  if (tiers?.thresholdTokens > 0 && refTiers?.thresholdTokens === tiers.thresholdTokens) {
    long = uniform(ratio(tiers.longContext?.inputPricePerMillion, refTiers.longContext?.inputPricePerMillion), ratio(tiers.longContext?.outputPricePerMillion, refTiers.longContext?.outputPricePerMillion));
  }
  return { standard, input, output, long, reference };
};
