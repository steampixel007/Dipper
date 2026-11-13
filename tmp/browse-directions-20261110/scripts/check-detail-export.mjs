import assert from 'node:assert/strict'
import { resolve } from 'node:path'
import { pathToFileURL } from 'node:url'
import { chromium } from 'playwright-core'

const browser = await chromium.launch({ channel: 'chrome', headless: true })
const results = []
try {
  for (const width of [728, 390]) {
    const page = await browser.newPage({ viewport: { width, height: 1000 } })
    const errors = []
    const external = []
    page.on('pageerror', error => errors.push(error.message))
    page.on('console', message => { if (message.type() === 'error') errors.push(message.text()) })
    page.on('request', request => { if (/^https?:/.test(request.url())) external.push(request.url()) })
    await page.goto(pathToFileURL(resolve(import.meta.dirname, '../detail-interactive.html')).href)
    const primary = page.locator('[data-target="choose-group"]')
    await primary.waitFor({ state: 'visible', timeout: 10000 })
    assert.equal(await page.locator('.sheet-toolbar > span').innerText(), '番剧详情')
    assert.equal(await page.locator('.detail-navigation, [data-target="detail-prev"], [data-target="detail-next"]').count(), 0)
    assert.equal(await page.locator('.sheet-toolbar').innerText(), '番剧详情')
    assert.equal(await page.locator('[data-target="close-detail"]').getAttribute('title'), '关闭详情（Esc）')
    await page.mouse.move(0, 0)
    const idle = await primary.evaluate(button => getComputedStyle(button).backgroundColor)
    await primary.hover()
    await page.waitForFunction(() => {
      const style = getComputedStyle(document.querySelector('[data-target="choose-group"]'))
      return style.backgroundColor === 'rgb(35, 35, 35)' && style.transform === 'matrix(1, 0, 0, 1, 0, -1)'
    })
    const hover = await primary.evaluate(button => ({
      background: getComputedStyle(button).backgroundColor,
      transform: getComputedStyle(button).transform,
      shadow: getComputedStyle(button).boxShadow,
    }))
    assert.notEqual(hover.background, idle)
    assert.notEqual(hover.shadow, 'none')
    await page.screenshot({ path: resolve(import.meta.dirname, `../evidence/detail-interactive-${width}.png`), fullPage: true })
    await page.mouse.down()
    await page.waitForFunction(() => getComputedStyle(document.querySelector('[data-target="choose-group"]')).transform.startsWith('matrix(0.97,'))
    await page.mouse.up()
    await page.locator('[data-target="group-0"]').click()
    await page.locator('[data-target="confirm-follow"]').click()
    await page.locator('[data-target="undo-follow"][data-visible]').waitFor({ state: 'visible' })
    await primary.waitFor({ state: 'visible' })
    assert.ok((await primary.innerText()).includes('追番设置'))
    await page.locator('[data-target="undo-follow"]').click()
    await page.waitForFunction(() => !document.querySelector('[data-target="undo-follow"][data-visible]'))
    assert.ok((await primary.innerText()).includes('选择字幕组'))
    await page.locator('#review-reset').click()
    await primary.waitFor({ state: 'visible' })
    assert.ok((await primary.innerText()).includes('选择字幕组'))
    await page.keyboard.press('Escape')
    await page.locator('.sheet-layer').waitFor({ state: 'detached' })
    await page.locator('#review-reset').click()
    await primary.waitFor({ state: 'visible' })
    await page.locator('[data-target="close-detail"]').click()
    await page.locator('.sheet-layer').waitFor({ state: 'detached' })
    await page.locator('#review-reset').click()
    await primary.waitFor({ state: 'visible' })
    await page.waitForFunction(() => [...document.images].every(image => image.complete && image.naturalWidth > 0))
    assert.deepEqual(external, [])
    assert.deepEqual(errors, [])
    results.push({ width, idle, hover, flow: 'group, confirmation, undo, reset passed', errors, external })
    await page.close()
  }
  console.log(JSON.stringify(results))
} finally {
  await browser.close()
}
