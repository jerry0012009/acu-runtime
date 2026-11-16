const { test } = require('node:test');
const assert = require('node:assert/strict');

test('public benchmark parser preserves real variant scores and rejects missing data', async () => {
  const { parseLeaderboard } = await import('../scripts/refresh-model-benchmarks.mjs');
  const initialModels = Array.from({length:21}, (_, index) => ({release:{slug:`fixture-${index}`},name:`Fixture ${index}`,intelligenceIndex:index}));
  initialModels.push({release:{slug:'fixture-1'},name:'Fixture 1 High',intelligenceIndex:52.5});
  initialModels.push({release:{slug:'missing'},name:'Missing',intelligenceIndex:null});
  const text = '1:' + JSON.stringify({initialModels,benchmark:{name:'Artificial Analysis Intelligence Index v5.0.1'}}) + '\n';
  const html = `<script>self.__next_f.push(${JSON.stringify([1,text])})</script>`;
  const snapshot = parseLeaderboard(html,'2026-11-16');
  assert.deepEqual(snapshot.models['fixture-1'].map(v=>v.score),[52.5,1]);
  assert.equal(snapshot.models.missing,undefined);
  assert.equal(snapshot.observedAt,'2026-11-16');
  assert.throws(()=>parseLeaderboard('<html>unavailable</html>'),/missing/);
});
