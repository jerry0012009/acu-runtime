const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { chromium } = require(process.env.ACU_PLAYWRIGHT || 'playwright');

// Probe ordinary Google Chrome, never a user's profile. This does not implement
// or approve attaching debugger access to someone's existing browser session.
async function main() {
  const executablePath = process.env.ACU_CHROME || '/opt/google/chrome/chrome';
  const extension = process.env.TAPNOW_TEST_EXTENSION;
  if (!extension) throw Error('TAPNOW_TEST_EXTENSION is required');
  const manifest = JSON.parse(fs.readFileSync(path.join(extension, 'manifest.json')));
  const profiles = [];
  const results = [];
  try {
    for (const mode of ['load-extension', 'cdp', 'cdp-opt-in']) {
      const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'acu-chrome-install-probe-'));
      profiles.push(profile);
      const args = ['--no-sandbox', '--disable-background-networking'];
      if (mode === 'load-extension') args.push(`--load-extension=${extension}`);
      if (mode === 'cdp-opt-in') args.push('--enable-unsafe-extension-debugging');
      const context = await chromium.launchPersistentContext(profile, {
        executablePath, headless: true, args,
        ignoreDefaultArgs: ['--disable-extensions'],
      });
      try {
        const browser = context.browser();
        const session = await browser.newBrowserCDPSession();
        const version = await session.send('Browser.getVersion');
        let reply;
        if (mode === 'load-extension') {
          await new Promise(resolve => setTimeout(resolve, 1500));
          const targets = await session.send('Target.getTargets');
          reply = { loaded: targets.targetInfos.some(t => t.url.startsWith('chrome-extension://') && t.url.includes('/background.js')) };
        } else {
          try { reply = await session.send('Extensions.loadUnpacked', { path: path.resolve(extension) }); }
          catch (error) { reply = { error: error.message }; }
        }
        results.push({ mode, browser: version.product, extensionVersion: manifest.version, ...reply });
      } finally { await context.close(); }
    }
    assert.equal(results.length, 3);
    const output = path.resolve('.build/chrome-install-probe.json');
    fs.mkdirSync(path.dirname(output), { recursive: true });
    fs.writeFileSync(output, JSON.stringify(results, null, 2) + '\n');
    console.log(JSON.stringify(results, null, 2));
  } finally {
    for (const profile of profiles) fs.rmSync(profile, { recursive: true, force: true });
  }
}
main().catch(error => { console.error(error); process.exitCode = 1; });
