#!/bin/bash
# Refresh internal/i18n/web.json from the desktop app's frontend/app.js so both
# clients use the same wording. Usage: scripts/export-texts.sh [path to ai-passport-desktop]
set -euo pipefail
cd "$(dirname "$0")/.."
desktop=${1:-../ai-passport-desktop}
node - "$desktop/frontend/app.js" internal/i18n/web.json <<'JS'
const fs = require('fs');
const [src, out] = process.argv.slice(2);
const code = fs.readFileSync(src, 'utf8');
const start = code.indexOf('{', code.indexOf('const I18N'));
let depth = 0, end = -1;
for (let i = start; i < code.length; i++) {
  if (code[i] === '{') depth++;
  if (code[i] === '}' && --depth === 0) { end = i + 1; break; }
}
const I18N = new Function('return (' + code.slice(start, end) + ')')();
const texts = { zh: {}, en: {} };
for (const l of ['zh', 'en']) for (const [k, v] of Object.entries(I18N[l])) if (typeof v === 'string') texts[l][k] = v;
fs.writeFileSync(out, JSON.stringify(texts, null, 1));
console.log(`${Object.keys(texts.zh).length} texts written to ${out}`);
JS
