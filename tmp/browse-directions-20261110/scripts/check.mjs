import { chromium } from 'playwright-core'
import { mkdir } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'

const base = process.env.PREVIEW_URL ?? 'http://127.0.0.1:4397'
const evidence = new URL('../evidence/', import.meta.url)
await mkdir(evidence, { recursive: true })

const browser = await chromium.launch({ channel: 'chrome', headless: true })
const context = await browser.newContext({ viewport: { width: 1320, height: 920 }, deviceScaleFactor: 1 })
const page = await context.newPage()
const errors = []
page.on('pageerror', (error) => errors.push(error.message))
page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()) })

let checks = 0
const ok = (condition, message) => {
  checks += 1
  if (!condition) throw new Error(message)
}

async function open(direction, width = 1320, height = 920) {
  await page.setViewportSize({ width, height })
  await page.goto(`${base}/?capture=${direction}`, { waitUntil: 'networkidle' })
  await page.waitForSelector('.poster')
  await page.evaluate(() => document.fonts.ready)
  await page.waitForFunction(() => [...document.querySelectorAll('img.poster')].every((image) => image.complete && image.naturalWidth > 0))
}

async function shot(name) {
  await page.waitForTimeout(280)
  await page.screenshot({ path: fileURLToPath(new URL(name, evidence)), fullPage: true })
}

async function count() {
  return page.locator('.nav-item small').innerText()
}

await open('shelf')
ok(await page.locator('.poster-card').count() === 19, 'shelf should show all 19 posters')
ok(await page.locator('main').getAttribute('aria-label') === '海报书架方向', 'shelf label missing')
await shot('shelf-desktop.png')

await page.locator('[data-target="anime-4031"]').click()
await page.waitForSelector('.detail-sheet')
ok(await page.locator('.detail-sheet h2').innerText() === '天是红河岸', 'detail title does not match poster')
ok(await page.evaluate(() => document.activeElement?.closest('[role="dialog"]') !== null), 'focus should enter the detail sheet')
await shot('shelf-detail.png')
await page.keyboard.press('Escape')
await page.waitForSelector('.detail-sheet', { state: 'detached' })
ok(await page.evaluate(() => document.activeElement?.getAttribute('data-target') === 'anime-4031'), 'focus should return to the poster')

await page.locator('[data-target="anime-4031"]').click()
await page.locator('[data-target="start-follow"]').click()
await page.waitForSelector('.version-picker')
ok(await page.locator('[data-target="confirm-follow"]').getAttribute('aria-disabled') === 'true', 'confirm should be disabled before a version is chosen')
await shot('shelf-version.png')
await page.locator('[data-target="version-0"]').click()
ok(await page.locator('[data-target="version-0"]').getAttribute('aria-checked') === 'true', 'version should be selectable')
ok(await page.locator('[data-target="confirm-follow"]').getAttribute('aria-disabled') === null, 'confirm should enable after a version is chosen')
await page.locator('[data-target="confirm-follow"]').click()
await page.waitForSelector('.study-toast')
ok((await page.locator('.study-toast').innerText()).includes('天是红河岸'), 'follow toast should name the title')
ok(await count() === '3', 'follow should increment the sidebar count')
await shot('shelf-followed.png')
await page.locator('.study-toast button', { hasText: '撤销' }).click()
ok(await count() === '2', 'undo should restore the sidebar count')
ok((await page.locator('.study-toast').innerText()).includes('已撤销'), 'undo should confirm the reversal')
await page.keyboard.press('Escape')
await page.waitForSelector('.detail-sheet', { state: 'detached' })

await page.locator('input[aria-label="搜索番剧"]').fill('天是红河岸')
ok(await page.locator('.poster-card').count() === 1, 'search should narrow the shelf')
await page.locator('button[aria-label="清空搜索"]').click()
ok(await page.locator('.poster-card').count() === 19, 'clearing search should restore the shelf')
await page.locator('input[aria-label="搜索番剧"]').fill('不存在的作品')
await page.waitForSelector('.empty-state')
ok((await page.locator('.empty-state').innerText()).includes('没找到'), 'empty search should explain the result')
await page.getByRole('button', { name: '查看全部番剧' }).click()
ok(await page.locator('.poster-card').count() === 19, 'empty-state reset should restore all posters')
await page.locator('[data-target="day-5"]').click()
await page.waitForSelector('.empty-state')
ok((await page.locator('.empty-state').innerText()).includes('暂时没有样本'), 'empty weekday should explain the result')

await open('daily')
ok(await page.locator('.schedule-row').count() === 8, 'Tuesday should show eight rows')
ok((await page.locator('.daily-detail h2').innerText()) === '天是红河岸', 'daily detail should start with the first Tuesday title')
await page.locator('.schedule-row').nth(1).click()
ok((await page.locator('.daily-detail h2').innerText()) === '弹珠汽水瓶里的千岁同学 第2部分', 'daily detail should follow the chosen row')
await shot('daily-desktop.png')
await page.locator('.daily-detail [data-target="start-follow"]').click()
await page.waitForSelector('.version-dialog')
await page.keyboard.press('Tab')
ok(await page.evaluate(() => document.activeElement?.closest('[role="dialog"]') !== null), 'Tab should stay inside the version dialog')
await page.locator('[data-target="version-1"]').click()
await page.locator('[data-target="confirm-follow"]').click()
await page.waitForSelector('.study-toast')
ok(await count() === '3', 'daily follow should increment the sidebar count')
await page.locator('.study-toast button', { hasText: '撤销' }).click()
ok(await count() === '2', 'daily undo should restore the sidebar count')

await open('cinema')
const first = await page.locator('.cinema-copy h2').innerText()
await page.getByRole('button', { name: '下一部' }).click()
const second = await page.locator('.cinema-copy h2').innerText()
ok(first !== second, 'next should change the featured title')
await page.getByRole('button', { name: '上一部' }).click()
ok(await page.locator('.cinema-copy h2').innerText() === first, 'previous should return to the first title')
await page.locator('.cinema-rail .rail-card').nth(2).click()
ok((await page.locator('.cinema-copy h2').innerText()) === '飙马野郎 JOJO的奇妙冒险 第一赛段', 'rail selection should change the feature')
await shot('cinema-desktop.png')
await page.locator('.cinema-hero [data-target="start-follow"]').click()
await page.waitForSelector('.version-dialog')
await page.keyboard.press('Escape')
await page.waitForSelector('.version-dialog', { state: 'detached' })
ok(await page.evaluate(() => document.activeElement?.getAttribute('data-target') === 'start-follow'), 'closing the version dialog should restore the opener')

for (const direction of ['shelf', 'daily', 'cinema']) {
  await open(direction, 390, 900)
  const layout = await page.locator('.ss-stage').getAttribute('data-layout')
  ok(layout === 'compact', `${direction} should use its compact layout`)
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth)
  ok(overflow <= 0, `${direction} compact page should not scroll sideways`)
  if (direction === 'cinema') {
    const follow = page.locator('.cinema-hero [data-target="start-follow"]')
    ok(await follow.isVisible(), 'compact cinema should expose its primary follow action')
    const compactTitle = await page.locator('.cinema-copy h2').innerText()
    await page.getByRole('button', { name: '下一部' }).click()
    ok(await page.locator('.cinema-copy h2').innerText() !== compactTitle, 'compact cinema next control should change the feature')
    await follow.click()
    await page.waitForSelector('.version-dialog')
    ok(await page.locator('.version-dialog').isVisible(), 'compact cinema follow should open version selection')
    await page.keyboard.press('Escape')
    await page.waitForSelector('.version-dialog', { state: 'detached' })
    ok(await page.evaluate(() => {
      const content = document.querySelector('.cinema-content')
      const footer = document.querySelector('.product-footer')
      return getComputedStyle(content).overflowY === 'auto' && content.getBoundingClientRect().bottom <= footer.getBoundingClientRect().top
    }), 'compact cinema should scroll inside its own pane above the footer')
  }
  await shot(`${direction}-390.png`)
}

const reduced = await browser.newContext({ viewport: { width: 1320, height: 920 }, reducedMotion: 'reduce' })
const reducedPage = await reduced.newPage()
await reducedPage.goto(`${base}/?capture=shelf`, { waitUntil: 'networkidle' })
await reducedPage.locator('[data-target="anime-4031"]').click()
await reducedPage.waitForSelector('.detail-sheet')
ok(await reducedPage.locator('.detail-sheet').isVisible(), 'reduced motion should keep the detail usable')
await reducedPage.screenshot({ path: fileURLToPath(new URL('shelf-reduced.png', evidence)), fullPage: true })
await reduced.close()

ok(errors.length === 0, `browser errors: ${errors.join(' | ')}`)
await browser.close()
console.log(`passed ${checks} checks`)
