globalThis.acuFormatPriceMultiplier = value => typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value.toFixed(3) + '×' : '—';
// Only confirmed aliases share a price row. Never merge by punctuation or name similarity.
globalThis.acuPriceCatalogRows = (models, query) => {
  const canonical = 'claude-opus-5-5';
  const alias = 'claude-opus-5.5';
  const hasCanonical = models.some(model => model.model_name === canonical && model.payable);
  return models.filter(model => {
    if (model.model_name === 'gemini-4-carbon' || (hasCanonical && model.model_name === alias)) return false;
    const alternate = hasCanonical && model.model_name === canonical ? alias : '';
    return `${model.model_name} ${model.display_name || ''} ${alternate}`.toLowerCase().includes(query.trim().toLowerCase());
  });
};
globalThis.acuPriceComparison = model => {
  const reference = model.comparison_reference;
  const payable = model.payable;
  const valid = value => typeof value === 'number' && Number.isFinite(value) && value >= 0;
  const ratio = (value, base) => valid(value) && valid(base) && base > 0 ? value / base : undefined;
  const input = ratio(payable?.input_cny_per_million, reference?.input_cny_per_million);
  const output = ratio(payable?.output_cny_per_million, reference?.output_cny_per_million);
  const uniform = (left, right) => left !== undefined && right !== undefined && Math.abs(left - right) <= Math.max(1, left, right) * 1e-10 ? left : undefined;
  const standard = uniform(input, output);
  const tiers = payable?.context_tiers;
  const refTiers = reference?.context_tiers;
  let longThreshold, longInput, longOutput;
  if (refTiers?.thresholdTokens > 0 && (!tiers || refTiers.thresholdTokens === tiers.thresholdTokens)) {
    longThreshold = refTiers.thresholdTokens;
    longInput = ratio(tiers ? tiers.longContext?.inputPricePerMillion : payable?.input_cny_per_million, refTiers.longContext?.inputPricePerMillion);
    longOutput = ratio(tiers ? tiers.longContext?.outputPricePerMillion : payable?.output_cny_per_million, refTiers.longContext?.outputPricePerMillion);
  }
  return { standard, input, output, long: uniform(longInput, longOutput), longInput, longOutput, longThreshold, reference };
};

globalThis.acuPriceSourceURL = source => {
  try {
    const url = new URL(typeof source === 'string' ? source.split(';', 1)[0].trim() : '');
    return url.protocol === 'https:' && !url.username && !url.password ? url.href : '';
  } catch { return ''; }
};
