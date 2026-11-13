import { chromium } from 'playwright-core'

const browser = await chromium.launch({ channel: 'chrome' })
const page = await browser.newPage({ viewport: { width: 1144, height: 920 } })
const errors = []
page.on('pageerror', (error) => errors.push(error.message))

try {
  await page.goto('http://127.0.0.1:4397/?capture=shelf', { waitUntil: 'networkidle' })
  await page.locator('.poster-card').first().waitFor()
  await page.screenshot({ path: 'evidence/shelf-result-toolbar-1144.png', fullPage: true })
  await page.locator('.poster-card').nth(2).hover()
  await page.waitForTimeout(320)
  await page.screenshot({ path: 'evidence/shelf-poster-action-1144.png', fullPage: true })

  await page.setViewportSize({ width: 390, height: 920 })
  await page.reload({ waitUntil: 'networkidle' })
  await page.locator('.poster-card').first().waitFor()
  await page.screenshot({ path: 'evidence/shelf-result-toolbar-390.png', fullPage: true })
  await page.locator('.poster-card').nth(2).hover()
  await page.waitForTimeout(320)
  await page.screenshot({ path: 'evidence/shelf-poster-action-390.png', fullPage: true })

  console.log(JSON.stringify({ errors }))
} finally {
  await browser.close()
}
