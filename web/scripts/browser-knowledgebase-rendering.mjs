import assert from 'node:assert/strict'
import { topics, chapterCounts, card, calls, frames, settle, open, close, inspectVisibleChapter } from './browser-knowledgebase-helpers.mjs'

export async function knowledgebaseRenderingScenarios(scenario, size) {
  await scenario({ ...size, reducedMotion: 'no-preference' }, 'knowledgebase-three-pause-seek', 'knowledgebase-audit.html', async (page, capture) => {
    await settle(page)
    const before = await calls(page)
    const trigger = await open(page, 'networks')
    await page.getByRole('button', { name: 'Pause demonstration', exact: true }).click()
    await page.locator('[data-demo-step="3"]').click()
    await frames(page)
    const stage = page.locator('.demo-three-scene')
    const canvas = stage.locator('canvas.demo-three-scene__canvas')
    const pose = await stage.getAttribute('data-three-frame')
    assert.ok(pose, '3D scene has no deterministic frame signature')
    const first = await canvas.screenshot()
    await frames(page)
    assert.equal(await stage.getAttribute('data-three-frame'), pose, 'paused Three.js state continued advancing')
    assert.deepEqual(await canvas.screenshot(), first, 'paused Three.js canvas continued animating')
    await page.locator('[data-demo-step="5"]').click()
    await frames(page)
    assert.notEqual(await stage.getAttribute('data-three-frame'), pose, 'seeking did not redraw the 3D scene')
    assert.notDeepEqual(await canvas.screenshot(), first, 'different chapters rendered the same 3D scene')
    await capture('paused-three-scene')
    await close(page, trigger)
    assert.deepEqual(await calls(page), before, '3D rendering called a member API')
  })
  await scenario(size, 'knowledgebase-webgl-unavailable', 'knowledgebase-audit.html', async (page, capture) => {
    await settle(page)
    const before = await calls(page)
    // Simulate a device/browser that cannot provide WebGL while preserving 2D APIs.
    await page.evaluate(() => {
      const original = globalThis.HTMLCanvasElement.prototype.getContext
      globalThis.HTMLCanvasElement.prototype.getContext = function (type, ...args) {
        return type.startsWith('webgl') || type === 'experimental-webgl' ? null : Reflect.apply(original, this, [type, ...args])
      }
    })
    for (const id of topics) {
      const trigger = await open(page, id, 'fallback')
      assert.equal(await page.locator('.demo-three-scene__fallback').isVisible(), true, `${id}: no meaningful fallback illustration`)
      await page.locator(`[data-demo-step="${chapterCounts[id] - 1}"]`).click()
      await inspectVisibleChapter(page, 'en')
      if (id === 'subscription') await capture('subscription-fallback')
      await close(page, trigger)
    }
    assert.deepEqual(await calls(page), before, 'fallback rendering called a member API')
  })
  await scenario(size, 'knowledgebase-webgl-context-loss', 'knowledgebase-audit.html', async (page, capture) => {
    await settle(page)
    const trigger = await open(page, 'subscription')
    const canvas = page.locator('canvas.demo-three-scene__canvas')
    assert.equal(await canvas.evaluate(element => {
      const extension = element.getContext('webgl2')?.getExtension('WEBGL_lose_context')
      if (!extension) return false
      globalThis.__restoreTutorialContext = () => extension.restoreContext()
      extension.loseContext()
      return true
    }), true, 'browser cannot exercise actual WebGL context loss')
    await page.locator('.demo-three-scene[data-three-status="fallback"]').waitFor()
    assert.equal(await page.locator('.demo-three-scene__fallback').isVisible(), true)
    await page.locator('[data-demo-step="4"]').click()
    await inspectVisibleChapter(page, 'en')
    await capture('context-lost-readable')
    await page.evaluate(() => globalThis.__restoreTutorialContext())
    await page.locator('.demo-three-scene[data-three-status="ready"]').waitFor()
    assert.equal(await canvas.evaluate(element => element.getContext('webgl2')?.isContextLost()), false)
    await capture('context-restored')
    await close(page, trigger)
  })
  await scenario(size, 'knowledgebase-close-during-three-load', 'knowledgebase-audit.html', async (page, capture) => {
    await settle(page)
    let release
    const gate = new Promise(resolve => { release = resolve })
    await page.route('**/src/components/common/demoThreeRenderer.ts*', async route => {
      await gate
      await route.continue()
    })
    try {
      const requested = page.waitForRequest(request => request.url().includes('/src/components/common/demoThreeRenderer.ts'))
      const trigger = card(page, 'subscription').getByRole('button')
      await trigger.click()
      await requested
      await page.locator('.demo-three-scene[data-three-status="loading"]').waitFor()
      assert.equal(await page.locator('.demo-three-scene__fallback').isVisible(), true)
      await capture('loading-fallback')
      await close(page, trigger)
      const loaded = page.waitForResponse(response => response.url().includes('/src/components/common/demoThreeRenderer.ts'))
      release()
      await loaded
      await frames(page)
      assert.equal(await page.locator('.demo-three-scene').count(), 0, 'late import mounted a closed tutorial')
      const reopened = await open(page, 'subscription')
      await close(page, reopened)
    } finally { release() }
  })
  if (size.name !== 'mobile') return
  for (const locale of ['en', 'zh']) {
    await scenario({ ...size, name: 'mobile-narrow', viewport: { width: 320, height: 740 } }, `knowledgebase-all-${locale}`, `knowledgebase-audit.html${locale === 'zh' ? '?zh' : ''}`, async (page, capture) => {
      await settle(page)
      const before = await calls(page)
      for (const id of topics) {
        const trigger = await open(page, id)
        const chapters = page.locator('[data-demo-step]')
        for (let index = 0; index < chapterCounts[id]; index++) {
          await chapters.nth(index).click()
          await inspectVisibleChapter(page, locale)
        }
        assert.equal(await page.getByRole('dialog').evaluate(element => element.getBoundingClientRect().width <= globalThis.innerWidth), true, 'dialog exceeds the narrow viewport')
        await capture(`${id}-narrow`)
        await close(page, trigger)
      }
      assert.deepEqual(await calls(page), before, 'narrow tutorial playback called a member API')
    })
  }
}
