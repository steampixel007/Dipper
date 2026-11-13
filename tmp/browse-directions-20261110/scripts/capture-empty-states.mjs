import { chromium } from 'playwright-core'
import { fileURLToPath } from 'node:url'

const base = process.env.PREVIEW_URL ?? 'http://127.0.0.1:4397'
const evidence = new URL('../evidence/', import.meta.url)
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const page = await browser.newPage({ viewport: { width: 1320, height: 920 }, deviceScaleFactor: 1 })

async function openShelf() {
  await page.goto(`${base}/?capture=shelf`, { waitUntil: 'networkidle' })
  await page.waitForSelector('.poster')
  await page.evaluate(() => document.fonts.ready)
  await page.waitForFunction(() => [...document.querySelectorAll('img.poster')].every((image) => image.complete && image.naturalWidth > 0))
}

async function shot(name) {
  await page.waitForTimeout(280)
  await page.screenshot({ path: fileURLToPath(new URL(name, evidence)), fullPage: true })
}

for (const [suffix, width, height] of [['desktop', 1320, 920], ['compact', 390, 920]]) {
  await page.setViewportSize({ width, height })
  await openShelf()
  await page.locator('input[aria-label="搜索番剧"]').fill('不存在的作品')
  await page.waitForSelector('.empty-state')
  await shot(`empty-search-${suffix}.png`)

  await page.locator('.empty-state [data-target="clear-search"]').click()
  await page.locator('[data-target="day-6"]').click()
  await page.waitForSelector('.empty-state')
  await shot(`empty-day-${suffix}.png`)

  await page.locator('[data-target="day--1"]').click()
  await page.locator('[data-target="anime-4063"]').click()
  await page.locator('[data-target="choose-group"]').click()
  await page.locator('[data-target="group-0"]').click()
  await page.locator('[data-target="confirm-follow"]').click()
  await page.locator('[data-target="close-detail"]').click()
  await page.getByRole('dialog').waitFor({ state: 'detached' })
  const dismissToast = page.locator('.study-toast [aria-label="关闭提示"]')
  if (await dismissToast.isVisible()) await dismissToast.click()
  await page.locator('.unfollowed-filter').click()
  await page.locator('[data-target="day-1"]').click()
  await page.waitForSelector('.empty-state')
  await shot(`empty-followed-${suffix}.png`)
}

await browser.close()
