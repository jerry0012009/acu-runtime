// Adapt defaults only; decoding, sampling and MCP tools remain upstream-owned.
const { spawn } = require('node:child_process');
const { createInterface } = require('node:readline');
const path = require('node:path');
const root = __dirname;
process.env.PATH = path.join(root, 'bin') + path.delimiter + process.env.PATH;
process.env.HOME = path.join(root, 'home');
process.env.USERPROFILE = process.env.HOME;
const child = spawn(process.execPath, [path.join(root, 'node_modules/claude-video-vision/dist/index.js')], {
  env: process.env, stdio: ['pipe', 'pipe', 'inherit'], windowsHide: true,
});
const reads = new Set(['video_watch', 'video_info', 'video_analyze', 'video_detail']);
createInterface({ input: process.stdin }).on('line', line => {
  try {
    const msg = JSON.parse(line);
    if (msg.method === 'tools/call') {
      if (!reads.has(msg.params?.name)) {
        process.stdout.write(JSON.stringify({ jsonrpc: '2.0', id: msg.id,
          error: { code: -32602, message: 'Only video reading tools are enabled.' } }) + '\n');
        return;
      }
      if (msg.params.name === 'video_watch') {
        msg.params.arguments = { ...msg.params.arguments, skip_audio: true };
      }
      if (msg.params.name === 'video_analyze') {
        msg.params.arguments = { ...msg.params.arguments, filters: {
          ...msg.params.arguments?.filters, transcription: false, silence: false, loudness: false,
        } };
      }
    }
    child.stdin.write(JSON.stringify(msg) + '\n');
  } catch { child.stdin.write(line + '\n'); }
}).on('close', () => { child.stdin.end(); child.kill(); });
createInterface({ input: child.stdout }).on('line', line => {
  try {
    const msg = JSON.parse(line);
    if (Array.isArray(msg.result?.tools)) {
      msg.result.tools = msg.result.tools.filter(t => reads.has(t.name));
      for (const tool of msg.result.tools) {
        tool.description += ' ACU visual-only mode: audio is not analyzed. Frames are samples, not exhaustive defect detection. Use video_detail for a closer look. Requires an image-capable model or image interpretation tool.';
        if (tool.inputSchema?.properties?.skip_audio) tool.inputSchema.properties.skip_audio.default = true;
      }
    }
    process.stdout.write(JSON.stringify(msg) + '\n');
  } catch { process.stdout.write(line + '\n'); }
});
child.on('error', () => process.exit(1));
child.stdin.on('error', () => {});
child.on('close', code => process.exit(code ?? 1));
process.on('SIGTERM', () => child.kill());
process.on('SIGINT', () => child.kill());
