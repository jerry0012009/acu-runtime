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
globalThis.acuPriceModelCategories = model => {
  const name = String(model?.model_name || '').toLowerCase();
  const vendor = String(model?.vendor_name || model?.owner_by || '').toLowerCase();
  const endpoints = Array.isArray(model?.supported_endpoint_types) ? model.supported_endpoint_types : [];
  const domestic = /^(deepseek|glm-|kimi-|qwen|tongyi|minimax|hunyuan|hy[34](?:-|$)|tencent-hy|mimo-|doubao|seed(?:ance|ream)|step(?:-|[0-9])|ling-|baichuan|yi-|internlm|telechat|skywork|aquila|pangu|chatglm|baidu-|ernie-)/i.test(name)
    || /(deepseek|zhipu|moonshot|kimi|alibaba|qwen|minimax|hunyuan|tencent|xiaomi|stepfun|bytedance|doubao|baichuan|01\\.ai)/i.test(vendor);
  const openSource = /^(deepseek|glm-|qwen|mimo-|mistral|llama|gemma|gpt-oss|nemotron|granite|yi-|step-|internlm|hunyuan|ling-|baichuan|solar-|falcon|olmo|phi-|command-r)/i.test(name);
  const image = endpoints.some(endpoint => /image-generation/i.test(endpoint)) || /(^|[-/])(gpt-image|dall-e|imagen|imagine-image|[^/]*-image(?:-|$))/i.test(name);
  const video = endpoints.some(endpoint => /video/i.test(endpoint)) || /(^|[-/])(sora|veo|kling|seedance|hailuo|runway|wanx?|luma|imagine-video)([-/]|$)/i.test(name);
  return {
    open_source: openSource,
    overseas: !domestic && !name.startsWith('acu-'),
    image,
    video,
    openai: /^(gpt-|gpt$|o[1-9](?:-|$)|chatgpt-|codex(?:-|$)|dall-e|gpt-image)/i.test(name) || /openai/i.test(vendor),
    claude: /^claude-/i.test(name) || /anthropic/i.test(vendor),
    gemini: /^gemini-/i.test(name) || /google/i.test(vendor),
    grok: /^grok-/i.test(name) || /x\\.ai|xai/i.test(vendor)
  };
};
globalThis.acuFilterPriceModels = (models, selected) => {
  const categories = globalThis.acuPriceModelCategories;
  const filters = new Set(selected || []);
  const groups = [
    ['open_source', 'overseas'],
    ['image', 'video'],
    ['openai', 'claude', 'gemini', 'grok']
  ];
  return models.filter(model => {
    const values = categories(model);
    return groups.every(group => {
      const active = group.filter(key => filters.has(key));
      return active.length === 0 || active.some(key => values[key]);
    });
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
