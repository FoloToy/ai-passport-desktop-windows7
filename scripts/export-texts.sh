#!/bin/bash
# Refresh internal/i18n/web.json from the desktop app's frontend (the I18N table in
# app.js and the Object.assign(I18N.xx, …) additions in community.js and tools.js) so both
# clients use the same wording. Usage: scripts/export-texts.sh [path to ai-passport-desktop]
set -euo pipefail
cd "$(dirname "$0")/.."
desktop=${1:-../ai-passport-desktop}
node - "$desktop/frontend" internal/i18n/web.json <<'JS'
const fs = require('fs');
const [dir, out] = process.argv.slice(2);
// object returns the object literal that starts at the first "{" after from.
function object(code, from) {
  const start = code.indexOf('{', from);
  let depth = 0;
  for (let i = start; i < code.length; i++) {
    if (code[i] === '{') depth++;
    if (code[i] === '}' && --depth === 0) return new Function('return (' + code.slice(start, i + 1) + ')')();
  }
  throw new Error('unbalanced object');
}
const app = fs.readFileSync(dir + '/app.js', 'utf8');
const I18N = object(app, app.indexOf('const I18N'));
for (const f of ['community.js', 'tools.js']) {
  const code = fs.readFileSync(dir + '/' + f, 'utf8');
  for (const l of ['zh', 'en']) {
    const at = code.indexOf('Object.assign(I18N.' + l + ',');
    if (at >= 0) Object.assign(I18N[l], object(code, at));
  }
}
const texts = { zh: {}, en: {} };
for (const l of ['zh', 'en']) for (const [k, v] of Object.entries(I18N[l])) if (typeof v === 'string') texts[l][k] = v;
fs.writeFileSync(out, JSON.stringify(texts, null, 1));
console.log(`${Object.keys(texts.zh).length} texts written to ${out}`);
JS
