// Interpretable projection; geometry matches the Console ordinary radial scale.
globalThis.acuPreferenceChart = (profile, selectedId, esc) => {
  const rays = profile?.rays || [];
  if (!rays.length) return '';
  if (profile.schemaVersion && profile.schemaVersion !== 'acu-preference-v5-bipolar') return '<p role="status">暂不支持此图谱版本，偏好仍可在列表查看。</p>';
  const categories = [...new Set(rays.map(ray => ray.category))].sort((a,b) => a.localeCompare(b));
  const ordered = categories.flatMap(category => rays.filter(ray => ray.category === category));
  const scores = rays.map(ray => Number.isFinite(ray.score) ? ray.score : 50);
  const minimum = Math.min(...scores), maximum = Math.max(...scores), span = Math.max(20, maximum-minimum);
  const lower = Math.max(0, Math.min(100-span, (minimum+maximum)/2-span/2));
  const bipolar = profile.schemaVersion === 'acu-preference-v5-bipolar';
  const points = ordered.map((ray,index) => {
    let angle = index*Math.PI*2/ordered.length-Math.PI/2;
    let radius = 64 + ((Number.isFinite(ray.score) ? ray.score : 50)-lower)/span*186;
    if (bipolar && ray.bipolar) {
      let hash = 0;
      for (const character of ray.id) hash = (hash * 31 + character.charCodeAt(0)) >>> 0;
      const jitter = ((hash % 101) / 100 - .5) * .18;
      angle = (index + jitter) * Math.PI / ordered.length - Math.PI / 2;
      const value = ray.bipolar.position;
      radius = ray.bipolar.applicability ? 250*Math.abs((value-60)/60) : 0;
      if (value < 60) angle += Math.PI;
    }
    return {ray,angle,radius,x:320+Math.cos(angle)*radius,y:320+Math.sin(angle)*radius};
  });
  const selected = rays.find(ray => ray.id === selectedId) || rays[0];
  return `<div class="preference-chart-layout"><div class="preference-chart-viewport"><svg viewBox="-100 -100 840 840" role="group" aria-label="偏好向量海星图">
    ${[48,99,150,201,250].map(radius=>`<circle cx="320" cy="320" r="${radius}" fill="none" stroke="currentColor" opacity=".12"/>`).join('')}
    <circle class="preference-center-pulse" cx="320" cy="320" r="18" fill="currentColor" opacity=".4"/>
    ${points.map(({ray,angle,radius,x,y}) => {const back=radius-Math.min(8,radius*.45), half=Math.min(3.5,radius*.2);return `<g role="button" tabindex="0" aria-label="${esc(ray.label)}" aria-pressed="${ray.id===selected.id}" data-pref-ray="${esc(ray.id)}"><title>${esc(ray.summary)}</title><line x1="320" y1="320" x2="${x}" y2="${y}" stroke="currentColor" stroke-width="${ray.id===selected.id?2.5:1.5}" opacity="${ray.id===selected.id?1:.55}"/><line x1="320" y1="320" x2="${320+Math.cos(angle)*272}" y2="${320+Math.sin(angle)*272}" stroke="transparent" stroke-width="14"/>${radius>0?`<path d="M ${320+Math.cos(angle)*back-Math.sin(angle)*half} ${320+Math.sin(angle)*back+Math.cos(angle)*half} L ${x} ${y} L ${320+Math.cos(angle)*back+Math.sin(angle)*half} ${320+Math.sin(angle)*back-Math.cos(angle)*half}" fill="none" stroke="currentColor" stroke-width="2"/>`:''}</g>`;}).join('')}
    ${categories.map(category=>{const indices=ordered.map((ray,index)=>ray.category===category?index:-1).filter(index=>index>=0);const angle=((indices[0]+indices.at(-1))/2)*Math.PI*2/ordered.length-Math.PI/2;const lines=[...category].reduce((out,char,index)=>{const i=Math.floor(index/12);out[i]=(out[i]||'')+char;return out;},[]);return `<text x="${320+Math.cos(angle)*310}" y="${320+Math.sin(angle)*310}" text-anchor="middle" fill="currentColor" font-size="14">${lines.map((line,index)=>`<tspan x="${320+Math.cos(angle)*310}" dy="${index?18:0}">${esc(line)}</tspan>`).join('')}</text>`;}).join('')}
    </svg></div><aside class="preference-selected"><h2>${esc(selected.label)}</h2><p>${esc(selected.summary)}</p><p class="preference-muted">${esc(selected.scope)}</p><select aria-label="选择偏好" data-pref-ray-select>${rays.map(ray=>`<option value="${esc(ray.id)}" ${ray.id===selected.id?'selected':''}>${esc(ray.label)}</option>`).join('')}</select></aside></div>`;
};
