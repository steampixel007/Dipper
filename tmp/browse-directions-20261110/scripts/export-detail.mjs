import { build } from 'esbuild'
import { readFile, writeFile } from 'node:fs/promises'
import { dirname, resolve } from 'node:path'

const project = resolve(import.meta.dirname, '..')
const shelfReview = process.argv.includes('--shelf')
const sourcePath = resolve(project, 'src.tsx')
const original = await readFile(sourcePath, 'utf8')
const pinnedDeclaration = original.match(/const PINNED = (\[[^\n]+\])/)[1]
const pinned = new Set([...pinnedDeclaration.matchAll(/'([^']+)'/g)].map(match => match[1]))
const catalog = JSON.parse(await readFile(resolve(project, 'public/art/catalog.json'), 'utf8')).filter(anime => pinned.has(anime.id))
const artwork = Object.fromEntries(await Promise.all(catalog.map(async anime => [
  anime.cover,
  `data:image/jpeg;base64,${(await readFile(resolve(project, 'public/art', anime.cover))).toString('base64')}`,
])))
const logo = `data:image/svg+xml;base64,${(await readFile(resolve(project, 'public/art/dipper.svg'))).toString('base64')}`
const source = `const REVIEW_CATALOG = ${JSON.stringify(catalog)};\nconst REVIEW_ART = ${JSON.stringify(artwork)};\n${original}`
  .replaceAll('src={`/art/${anime.cover}`}', 'src={REVIEW_ART[anime.cover]}')
  .replaceAll('src={`/art/${current.cover}`}', 'src={REVIEW_ART[current.cover]}')
  .replaceAll('src="/art/dipper.svg"', `src=${JSON.stringify(logo)}`)
  .replace("fetch('/art/catalog.json')", 'Promise.resolve({ ok: true, json: async () => REVIEW_CATALOG })')
  .replaceAll('new URLSearchParams(location.search)', `new URLSearchParams('${shelfReview ? '?capture=shelf' : '?capture=shelf&anime=4031'}')`)
  .replace('width={1144} height={744} compact={{ below: 800,', 'width={960} height={688} compact={{ below: 520,')
  .replace("createRoot(document.getElementById('root')!).render(<Study />)", `
const reviewRoot = createRoot(document.getElementById('root')!)
let reviewGeneration = 0
const restartReview = () => reviewRoot.render(<Study key={reviewGeneration++} />)
document.getElementById('review-reset')!.addEventListener('click', restartReview)
restartReview()
`)

const result = await build({
  absWorkingDir: project,
  entryPoints: ['src.tsx'],
  outfile: 'detail-review-bundle.js',
  bundle: true,
  jsx: 'automatic',
  minify: true,
  write: false,
  format: 'iife',
  define: { 'process.env.NODE_ENV': '"production"' },
  loader: { '.svg': 'dataurl' },
  plugins: [{
    name: 'standalone-detail',
    setup(builder) {
      builder.onLoad({ filter: /[\\/]src\.tsx$/ }, () => ({ contents: source, loader: 'tsx', resolveDir: project }))
      builder.onLoad({ filter: /[\\/]brands[\\/]index\.ts$/ }, async args => {
        let contents = await readFile(args.path, 'utf8')
        const assets = [...contents.matchAll(/new URL\('([^']+)', import\.meta\.url\)\.href/g)]
        const replacements = await Promise.all(assets.map(async match => [
          match[0],
          JSON.stringify(`data:image/svg+xml;base64,${(await readFile(resolve(dirname(args.path), match[1]))).toString('base64')}`),
        ]))
        for (const [reference, replacement] of replacements) contents = contents.replaceAll(reference, replacement)
        return { contents, loader: 'ts', resolveDir: dirname(args.path) }
      })
    },
  }],
})
const javascript = result.outputFiles.find(file => file.path.endsWith('.js')).text.replaceAll('</script', '<\\/script')
const stylesheet = result.outputFiles.find(file => file.path.endsWith('.css')).text.replaceAll('100vh', '10000px')
const html = `<!doctype html>
<html lang="zh-CN">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>${shelfReview ? '首页字体与颜色交互原型' : '番剧详情交互原型'}</title>
<style>${stylesheet}
html:root, body { background: transparent; height: auto; }
body { margin: 0; }
.capture-host { padding: 0; max-width: none; }
.review-controls { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; color: var(--muted-foreground, #666); font-family: var(--font-sans, "Segoe UI", "Microsoft YaHei UI", sans-serif); font-size: 12px; }
.review-controls button { margin-left: auto; padding: 6px 8px; font: inherit; white-space: nowrap; color: var(--foreground, #333); border: 1px solid var(--border, #ddd); border-radius: var(--radius, 6px); background: var(--secondary, transparent); }
.review-controls button:focus-visible { outline-color: var(--ring); }
</style>
<div class="review-controls"><span>可直接悬停、点击 · 演示不会影响真实片单</span><button id="review-reset" type="button">重新开始</button></div>
<div id="root"></div>
<script>${javascript}</script>
</html>`
const output = resolve(project, shelfReview ? 'shelf-interactive.html' : 'detail-interactive.html')
await writeFile(output, html, 'utf8')
console.log(JSON.stringify({ output, bytes: Buffer.byteLength(html), covers: catalog.length }))
