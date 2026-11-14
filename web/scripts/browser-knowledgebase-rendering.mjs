import assert from 'node:assert/strict'
import { topics, chapterCounts, card, player, visual, calls, frames, settle, open, close, inspectVisibleChapter, inspectPaymentAmounts } from './browser-knowledgebase-helpers.mjs'

export async function knowledgebaseRenderingScenarios(scenario, size) {
  await scenario({ ...size, reducedMotion: 'no-preference' }, 'knowledgebase-interface-pause-seek', 'knowledgebase-audit.html', async (page, capture) => {
    await settle(page)
    const before = await calls(page)
    const trigger = await open(page, 'networks')
    const moving = await visual(page).getAttribute('data-demo-motion')
    await page.waitForFunction(previous => globalThis.document.querySelector('.knowledge-scene')?.getAttribute('data-demo-motion') !== previous, moving)
    await page.getByRole('button', { name: 'Pause demonstration', exact: true }).click()
    await page.locator('[data-demo-step="3"]').click()
    await frames(page)
    const pose = await visual(page).innerHTML()
    const first = await visual(page).screenshot()
    await frames(page)
    assert.equal(await visual(page).innerHTML(), pose, 'paused interface state continued advancing')
    assert.deepEqual(await visual(page).screenshot(), first, 'paused interface still animates')
    await page.locator('[data-demo-step="5"]').click()
    await frames(page)
    assert.notEqual(await visual(page).innerHTML(), pose, 'seeking did not update the visual state')
    assert.notDeepEqual(await visual(page).screenshot(), first, 'different chapters rendered the same interface')
    await page.locator('[data-demo-step="3"]').click()
    await frames(page)
    assert.equal(await visual(page).innerHTML(), pose, 'rewinding did not reproduce the same visual state')
    await capture('paused-network-flow')
    await close(page, trigger)
    assert.deepEqual(await calls(page), before, 'interface rendering called a member API')
  })
  await scenario(size, 'knowledgebase-gpu-independent', 'knowledgebase-audit.html', async (page, capture) => {
    await settle(page)
    const before = await calls(page)
    await page.evaluate(() => {
      globalThis.__tutorialWebGLRequests = 0
      const original = globalThis.HTMLCanvasElement.prototype.getContext
      globalThis.HTMLCanvasElement.prototype.getContext = function (type, ...args) {
        if (type.startsWith('webgl') || type === 'experimental-webgl') { globalThis.__tutorialWebGLRequests++; return null }
        return Reflect.apply(original, this, [type, ...args])
      }
    })
    for (const id of topics) {
      const trigger = await open(page, id)
      await page.locator(`[data-demo-step="${chapterCounts[id] - 1}"]`).click()
      await inspectVisibleChapter(page, 'en')
      assert.ok(await visual(page).locator('.iconify').count(), `${id}: visual scene has no icons`)
      await close(page, trigger)
    }
    assert.equal(await page.evaluate(() => globalThis.__tutorialWebGLRequests), 0, 'tutorials still attempt to create a WebGL renderer')
    const resources = await page.evaluate(() => globalThis.performance.getEntriesByType('resource').map(entry => entry.name))
    assert.deepEqual(resources.filter(url => /(?:demoThree|DemoThree|\/three[./])/u.test(url)), [], 'tutorials still request Three.js resources')
    assert.deepEqual(await calls(page), before, 'GPU-independent rendering called a member API')
    await capture('all-guides-without-gpu')
  })
  await scenario(size, 'knowledgebase-payment-fee-modes', 'knowledgebase-audit.html', async (page, capture) => {
    await settle(page)
    const before = await calls(page)
    const trigger = await open(page, 'payment')
    for (const [step, mode] of [[4, 'deducted'], [3, 'separate'], [6, 'wallet']]) {
      await page.locator(`[data-demo-step="${step}"]`).click()
      await inspectPaymentAmounts(page, mode)
      await capture(`payment-${mode}`)
    }
    await close(page, trigger)
    assert.deepEqual(await calls(page), before, 'payment illustration performed an account action')
  })
  await scenario(size, 'knowledgebase-superseded-player-load', 'knowledgebase-audit.html', async (page, capture) => {
    await settle(page)
    const before = await calls(page)
    let release
    const gate = new Promise(resolve => { release = resolve })
    await page.route('**/src/components/knowledge/KnowledgePlayer.vue*', async route => { await gate; await route.continue() })
    try {
      const requested = page.waitForRequest(request => request.url().includes('/src/components/knowledge/KnowledgePlayer.vue'))
      const pending = card(page, 'subscription').getByRole('button')
      await pending.click()
      await requested
      assert.equal(await pending.getAttribute('aria-busy'), 'true', 'delayed guide did not announce loading')
      assert.equal(await player(page).count(), 0, 'incomplete guide opened an empty player')
      const trigger = await open(page, 'traffic')
      const loaded = page.waitForResponse(response => response.url().includes('/src/components/knowledge/KnowledgePlayer.vue'))
      release()
      await loaded
      await frames(page)
      assert.equal(await page.getByRole('dialog').count(), 1, 'late import opened a second dialog')
      assert.equal(await player(page).getAttribute('data-billing-mode'), 'traffic', 'late import replaced the newly selected guide')
      await capture('newer-guide-retained')
      await close(page, trigger)
      const reopened = await open(page, 'subscription')
      await close(page, reopened)
    } finally { release() }
    assert.deepEqual(await calls(page), before, 'lazy guide loading called a member API')
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
          if (id === 'payment' && [3, 4, 6].includes(index)) {
            const mode = index === 3 ? 'separate' : index === 4 ? 'deducted' : 'wallet'
            await inspectPaymentAmounts(page, mode)
            await capture(`payment-${mode}-narrow`)
          }
        }
        assert.equal(await page.getByRole('dialog').evaluate(element => element.getBoundingClientRect().width <= globalThis.innerWidth), true, 'dialog exceeds the narrow viewport')
        await capture(`${id}-narrow`)
        await close(page, trigger)
      }
      assert.deepEqual(await calls(page), before, 'narrow tutorial playback called a member API')
    })
  }
}
