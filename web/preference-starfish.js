// Adapted from acu-frontend aa68dd7: preference-starfish.tsx and its two
// geometry helpers. Auth and source records remain in Runtime's native boundary.
globalThis.ACUStarfishGeometry = {
  preferenceRayLengths(scores) {
    if (!scores.length) return [];
    const minimum = Math.min(...scores), maximum = Math.max(...scores);
    const span = Math.max(20, maximum - minimum), midpoint = (minimum + maximum) / 2;
    const lower = Math.max(0, Math.min(100 - span, midpoint - span / 2));
    return scores.map(score => 64 + (score - lower) / span * 186);
  },
  bipolarRayGeometry(id, index, count, position) {
    let hash = 0;
    for (const character of id) hash = (hash * 31 + character.charCodeAt(0)) >>> 0;
    const jitter = ((hash % 101) / 100 - 0.5) * 0.18;
    return { angle: (index + jitter) * Math.PI / count - Math.PI / 2 + (position < 60 ? Math.PI : 0), radius: 250 * Math.abs((position - 60) / 60) };
  },
};

globalThis.createACUStarfish = ({ escapeHTML: esc, sourceList, onSelect }) => {
  let pinned = '', preview = '', zoom = 1, rays = [], profile = null, bipolar = false, valid = false;
  const finite = (n, max) => typeof n === 'number' && Number.isFinite(n) && n >= 0 && n <= max;
  const origin = { authorized_initialization: '初始设置', learned: '来自交流', user_explicit: '个人设置' };
  function reset() { pinned = ''; preview = ''; zoom = 1; rays = []; profile = null; }
  function selected() { return rays.find(ray => ray.id === (preview || pinned)) || rays[0]; }
  function detail() {
    const ray = selected();
    if (!ray) return '';
    const b = ray.bipolar;
    return `<article class="starfish-detail-content"><p class="preference-muted">${esc(ray.category)}</p><h3>${esc(ray.label)}</h3><p>${esc(ray.summary || '暂无摘要。')}</p>${ray.scope ? `<p class="preference-muted">适用范围：${esc(ray.scope)}</p>` : ''}${bipolar && b ? `<div class="starfish-poles"><span>${esc(b.end_a)}</span><span>${esc(b.end_b)}</span></div><p>${esc(origin[b.origin] || '')}</p>${b.applicability === false ? '<p class="preference-note">当前场景不适用</p>' : b.position === 60 ? '<p class="preference-muted">无明显倾向</p>' : ''}${b.scope ? `<p class="preference-muted">${[['project','项目'],['task','任务'],['subtype','子类型']].filter(([key]) => b.scope[key]).map(([key,label]) => `${label}：${esc(b.scope[key])}`).join(' · ')}</p>` : ''}` : ''}${sourceList(ray)}${profile?.acceptanceConditions?.length ? `<details><summary>适用条件</summary><ul>${profile.acceptanceConditions.map(condition => `<li>${esc(condition)}</li>`).join('')}</ul></details>` : ''}</article>`;
  }
  function render(view) {
    profile = view?.profile;
    rays = Array.isArray(profile?.rays) ? profile.rays : [];
    if (!rays.length) return '';
    bipolar = profile.schemaVersion === 'acu-preference-v5-bipolar';
    const schemaOK = !profile.schemaVersion || bipolar || /^acu-preference-v[1-4]$/.test(profile.schemaVersion);
    valid = schemaOK && rays.every(ray => typeof ray.id === 'string' && typeof ray.category === 'string' && (bipolar ? ray.bipolar?.schema_version === 'acu-preference-v5-bipolar' && finite(ray.bipolar.position,120) && typeof ray.bipolar.applicability === 'boolean' : finite(ray.score,100) && finite(ray.confidence,100)));
    if (!valid) return '<section class="panel starfish-unsupported" role="status">图谱暂不可用，请更新 Runtime。</section>';
    if (!rays.some(ray => ray.id === pinned)) pinned = rays[0].id;
    if (!rays.some(ray => ray.id === preview)) preview = '';
    const groups = [...new Set(rays.map(ray => ray.category))].sort((a,b) => a.localeCompare(b)).map(category => ({ category, rays: rays.filter(ray => ray.category === category) }));
    const ordered = groups.flatMap(group => group.rays);
    const lengths = globalThis.ACUStarfishGeometry.preferenceRayLengths(ordered.map(ray => ray.score));
    const points = ordered.map((ray,index) => {
      const geometry = bipolar ? globalThis.ACUStarfishGeometry.bipolarRayGeometry(ray.id,index,ordered.length,ray.bipolar.position) : {angle:index * Math.PI * 2 / ordered.length - Math.PI/2,radius:lengths[index]};
      const {angle} = geometry, radius = bipolar && !ray.bipolar.applicability ? 0 : geometry.radius;
      const arrowLength = Math.min(8,radius * .45), half = Math.min(3.5,arrowLength * .44);
      return {ray,angle,radius,x:320+Math.cos(angle)*radius,y:320+Math.sin(angle)*radius,bx:320+Math.cos(angle)*(radius-arrowLength),by:320+Math.sin(angle)*(radius-arrowLength),half};
    });
    const labels = groups.map((group,index) => {
      const contained = points.filter(point => point.ray.category === group.category);
      const angle = bipolar ? index*Math.PI*2/groups.length-Math.PI/2 : (contained[0].angle+contained.at(-1).angle)/2;
      const x=320+Math.cos(angle)*310,y=320+Math.sin(angle)*310;
      const chars=[...group.category], lines=[]; for(let i=0;i<chars.length;i+=14) lines.push(chars.slice(i,i+14).join(''));
      return `<text x="${x}" y="${y}" text-anchor="middle" font-size="14" fill="currentColor" opacity=".72" role="button" tabindex="0" data-star-domain="${esc(group.category)}" aria-label="领域：${esc(group.category)}">${lines.map((line,i) => `<tspan x="${x}" dy="${i ? 18 : -(lines.length-1)*9}">${esc(line)}</tspan>`).join('')}</text>`;
    }).join('');
    return `<section class="panel starfish-panel" aria-label="偏好向量"><header class="starfish-header"><div><h2>偏好向量</h2>${view.status==='error' ? '<p class="preference-muted">刷新失败 · 上次更新 '+esc(new Date(view.generatedAt).toLocaleString('zh-CN'))+'</p>' : ['queued','generating'].includes(view.status) ? '<p class="preference-muted">更新中…</p>' : ''}</div><div class="starfish-zoom"><button class="button secondary" data-star-zoom="out" aria-label="缩小图谱" ${zoom<=.5?'disabled':''}>−</button><button class="button secondary" data-star-zoom="in" aria-label="放大图谱" ${zoom>=2.5?'disabled':''}>＋</button></div></header><div class="starfish-layout"><div class="starfish-viewport" data-testid="preference-chart-viewport"><svg class="starfish-svg" viewBox="${320-420/Math.min(zoom,1)} ${320-420/Math.min(zoom,1)} ${840/Math.min(zoom,1)} ${840/Math.min(zoom,1)}" style="width:${Math.max(zoom,1)*100}%;height:${Math.max(zoom,1)*100}%" role="group" aria-label="动态偏好向量海星图" data-mode="${bipolar?'bipolar':'ordinary'}"><defs><filter id="starfish-glow" x="-100%" y="-100%" width="300%" height="300%"><feGaussianBlur stdDeviation="4" result="blur"/><feMerge><feMergeNode in="blur"/><feMergeNode in="SourceGraphic"/></feMerge></filter><radialGradient id="starfish-core-gradient"><stop offset="0%" stop-color="currentColor" stop-opacity=".95"/><stop offset="42%" stop-color="currentColor" stop-opacity=".7"/><stop offset="100%" stop-color="currentColor" stop-opacity="0"/></radialGradient></defs>${[48,99,150,201,250].map(r => `<circle cx="320" cy="320" r="${r}" fill="none" stroke="currentColor" opacity="${r===250?.16:.08}"/>`).join('')}<circle cx="320" cy="320" r="28" fill="none" stroke="currentColor" stroke-opacity=".15" class="starfish-pulse"/><circle cx="320" cy="320" r="12" fill="url(#starfish-core-gradient)" filter="url(#starfish-glow)" class="starfish-core"/><circle cx="320" cy="320" r="3" fill="currentColor" filter="url(#starfish-glow)"/><circle cx="320" cy="320" r="250" fill="none" stroke="currentColor" stroke-width="1.5" class="starfish-scan"/>${labels}${points.filter(p => p.radius>0).map(p => `<g role="button" tabindex="0" data-star-ray="${esc(p.ray.id)}" aria-label="${esc(p.ray.label)}" aria-pressed="${selected()?.id===p.ray.id}" class="starfish-ray-group" style="--ray-opacity:${.42+(bipolar?p.ray.bipolar.confidence/120:p.ray.confidence/100)*.58}"><title>${esc(p.ray.label)}：${esc(p.ray.summary)}</title><line x1="320" y1="320" x2="${p.x}" y2="${p.y}" class="starfish-ray"/><line x1="320" y1="320" x2="${320+Math.cos(p.angle)*272}" y2="${320+Math.sin(p.angle)*272}" stroke="transparent" stroke-width="14"/><path d="M ${p.bx-Math.sin(p.angle)*p.half} ${p.by+Math.cos(p.angle)*p.half} L ${p.x} ${p.y} L ${p.bx+Math.sin(p.angle)*p.half} ${p.by-Math.cos(p.angle)*p.half}${bipolar?' Z':''}" fill="${bipolar?'currentColor':'none'}" class="starfish-arrow"/></g>`).join('')}</svg></div><aside class="starfish-detail" aria-label="所选偏好详情">${detail()}</aside></div><details class="starfish-list"><summary>全部偏好方向（${rays.length}）</summary><div class="starfish-domains">${groups.map(group => `<button class="text-button" data-star-domain="${esc(group.category)}">${esc(group.category)} · ${group.rays.length}</button>`).join('')}</div><div class="starfish-ray-list">${rays.map(ray => `<button class="button secondary" data-star-ray="${esc(ray.id)}" aria-pressed="${selected()?.id===ray.id}">${esc(ray.label)}</button>`).join('')}</div></details></section>`;
  }
  function bind(root) {
    const panel=root.querySelector('.starfish-panel'); if(!panel||!valid)return;
    let detailID=selected()?.id;
    function paint() {
      const id=selected()?.id;
      panel.querySelectorAll('[data-star-ray]').forEach(node => {node.setAttribute('aria-pressed',String(node.dataset.starRay===id));node.classList.toggle('is-muted',!!preview&&node.dataset.starRay!==id);});
      if(detailID!==id){panel.querySelector('.starfish-detail').innerHTML=detail();detailID=id;}
    }
    panel.addEventListener('click',event => {
      const ray=event.target.closest('[data-star-ray]'), domain=event.target.closest('[data-star-domain]'), control=event.target.closest('[data-star-zoom]');
      if(ray){pinned=ray.dataset.starRay;preview='';onSelect?.(pinned);paint();}
      if(domain){pinned=rays.find(ray=>ray.category===domain.dataset.starDomain)?.id||pinned;preview='';onSelect?.(pinned);paint();}
      if(control){zoom=Math.max(.5,Math.min(2.5,zoom+(control.dataset.starZoom==='in'?.5:-.5)));const svg=panel.querySelector('svg');svg.setAttribute('viewBox',`${320-420/Math.min(zoom,1)} ${320-420/Math.min(zoom,1)} ${840/Math.min(zoom,1)} ${840/Math.min(zoom,1)}`);svg.style.width=svg.style.height=`${Math.max(zoom,1)*100}%`;panel.querySelector('[data-star-zoom="out"]').disabled=zoom<=.5;panel.querySelector('[data-star-zoom="in"]').disabled=zoom>=2.5;}
    });
    panel.addEventListener('keydown',event=>{const node=event.target.closest('svg [data-star-ray],svg [data-star-domain]');if(node&&(event.key==='Enter'||event.key===' ')){event.preventDefault();node.dispatchEvent(new MouseEvent('click',{bubbles:true}));}});
    for(const [enter,leave]of[['mouseover','mouseout'],['focusin','focusout']]){
      panel.addEventListener(enter,event=>{const node=event.target.closest('svg [data-star-ray]');if(node&&!node.contains(event.relatedTarget)){preview=node.dataset.starRay;paint();}});
      panel.addEventListener(leave,event=>{const node=event.target.closest('svg [data-star-ray]');if(node&&!node.contains(event.relatedTarget)){preview='';onSelect?.(pinned);paint();}});
    }
  }
  return {render,bind,reset};
};
