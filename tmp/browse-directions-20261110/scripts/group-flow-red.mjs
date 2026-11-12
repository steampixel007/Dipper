import { readFile, writeFile } from 'node:fs/promises'
import { spawnSync } from 'node:child_process'

const source = new URL('../src.tsx', import.meta.url)
const original = await readFile(source, 'utf8')
const anchor = '<ReadingTransition anime={selected} followed={isFollowed} />'
if (!original.includes(anchor)) throw new Error('detail mutation anchor missing')
try {
  await writeFile(source, original.replace(anchor, `${anchor}<button role="radio" aria-checked="false">提前选择字幕组</button>`))
  const result = spawnSync(process.execPath, ['scripts/discovery.mjs'], { encoding: 'utf8', timeout: 20000 })
  const output = `${result.stdout ?? ''}\n${result.stderr ?? ''}`
  if (result.status !== 1 || !output.includes('Error: opening a work must show its synopsis before subtitle groups or resources')) {
    throw new Error(`mutation failed for an unexpected reason:\n${output}`)
  }
  console.log('behavioral red proved: subtitle selection inside detail fails the synopsis boundary assertion')
} finally {
  await writeFile(source, original)
}
