import assert from 'node:assert/strict'

const chapter = (page, index) => page.locator(`[data-demo-step="${index}"]`).click()
const writes = page => page.evaluate(() => globalThis.__hourlyAudit.calls.filter(call => call.method !== 'GET'))

async function openBilling(page, mode) {
  if (mode === 'hourly') await page.getByRole('button', { name: 'Charge by hour', exact: true }).click()
  await page.locator('[data-billing-demo]').click()
  await page.locator(`.billing-demo[data-billing-mode="${mode}"]`).waitFor()
}

async function pauseAndInspectPointer(page, size, root, pose) {
  await page.waitForFunction(() => Number(globalThis.document.querySelector('.demo-transport__seek [role="slider"]')?.getAttribute('aria-valuenow')) > 30)
  await page.getByRole('button', { name: 'Pause demonstration', exact: true }).click()
  assert.equal(await page.locator(root).getAttribute('data-demo-playing'), 'false')
  const seek = page.locator('.demo-transport__seek [role="slider"]')
  await seek.focus(); await seek.press('Home')
  for (let index = 0; index < Math.floor(pose / 10); index++) await seek.press('PageUp')
  for (let index = 0; index < pose % 10; index++) await seek.press('ArrowRight')
  assert.equal(await seek.getAttribute('aria-valuenow'), String(pose))
  assert.ok(await page.locator('.demo-pointer').evaluate(element => Number(globalThis.getComputedStyle(element).opacity) > .9), 'click cue is faded')
  const cursor = size.name === 'desktop' ? '.demo-pointer__cursor' : '.demo-pointer__touch'
  const other = size.name === 'desktop' ? '.demo-pointer__touch' : '.demo-pointer__cursor'
  assert.equal(await page.locator(cursor).isVisible(), true)
  assert.equal(await page.locator(other).isVisible(), false)
  const value = await seek.getAttribute('aria-valuenow')
  await page.evaluate(() => new Promise(resolve => globalThis.requestAnimationFrame(() => globalThis.requestAnimationFrame(resolve))))
  assert.equal(await seek.getAttribute('aria-valuenow'), value, 'paused clock continued advancing')
}

async function capturePaused(page, capture, label) {
  assert.equal(await page.locator('.billing-demo, .routing-demo').getAttribute('data-demo-playing'), 'false')
  const seek = page.locator('.demo-transport__seek [role="slider"]')
  const position = await seek.getAttribute('aria-valuenow')
  await page.evaluate(async () => {
    for (const animation of globalThis.document.getAnimations()) {
      const css = animation instanceof globalThis.CSSTransition || animation instanceof globalThis.CSSAnimation
      if (css && animation.playState !== 'finished' && Number.isFinite(animation.effect?.getComputedTiming().endTime)) animation.finish()
    }
    await new Promise(resolve => globalThis.requestAnimationFrame(() => globalThis.requestAnimationFrame(resolve)))
  })
  await capture(label)
  assert.equal(await seek.getAttribute('aria-valuenow'), position, 'settling CSS advanced the paused demo pose')
}

export async function demonstrationScenarios(scenario, size) {
  const motion = { ...size, reducedMotion: 'no-preference' }
  await scenario(motion, 'demo-traffic-timeline', 'hourly-audit.html?catalog&active', async (page, capture) => {
    await openBilling(page, 'traffic')
    await pauseAndInspectPointer(page, size, '.billing-demo', 55)
    await capturePaused(page, capture, 'pointer-paused')
    await chapter(page, 1)
    assert.equal(await page.locator('.traffic-scene').getAttribute('data-demo-blocks'), '1')
    assert.equal(await page.locator('.traffic-scene').getAttribute('data-demo-balance'), '9.00')
    await chapter(page, 4)
    assert.equal(await page.locator('.traffic-scene').getAttribute('data-demo-balance'), '8.00')
    await capturePaused(page, capture, 'reconnected-credit')
    const seek = page.locator('.demo-transport__seek [role="slider"]')
    await seek.focus(); await seek.press('End')
    assert.equal(await seek.getAttribute('aria-valuenow'), '1000')
    assert.equal(await page.locator('.traffic-scene').getAttribute('data-demo-balance'), '8.00')
    await page.getByRole('button', { name: 'Replay demonstration', exact: true }).click()
    assert.equal(await page.locator('.billing-demo').getAttribute('data-demo-playing'), 'true')
    assert.deepEqual(await writes(page), [])
    await page.keyboard.press('Escape')
    await page.getByRole('dialog').waitFor({ state: 'hidden' })
    assert.equal(await page.locator('[data-billing-demo]').evaluate(element => element === globalThis.document.activeElement), true)
  })
  await scenario(size, 'demo-hourly-reduced-motion', 'hourly-audit.html?catalog&active', async (page, capture) => {
    await openBilling(page, 'hourly')
    assert.equal(await page.locator('.billing-demo').getAttribute('data-demo-playing'), 'false')
    assert.equal(await page.locator('.demo-pointer').count(), 0)
    await chapter(page, 0)
    assert.equal(await page.locator('.hourly-scene').getAttribute('data-demo-balance'), '9.50')
    await chapter(page, 3)
    assert.equal(await page.locator('.hourly-scene').getAttribute('data-demo-auto-buy'), 'false')
    await capturePaused(page, capture, 'no-automatic-debit')
    await chapter(page, 4)
    assert.equal(await page.locator('.hourly-scene').getAttribute('data-demo-auto-buy'), 'true')
    assert.equal(await page.locator('.hourly-scene').getAttribute('data-demo-balance'), '8.30')
    await chapter(page, 6)
    assert.equal(await page.locator('.hourly-scene').getAttribute('data-demo-enabled'), 'false')
    assert.deepEqual(await writes(page), [])
  })
  await scenario(motion, 'demo-routing-timeline', 'routing-audit.html?diy&populated&split&local-preview', async (page, capture) => {
    await page.getByRole('tab', { name: 'Routing', exact: true }).click()
    const trigger = page.getByRole('button', { name: 'See routing', exact: true })
    await trigger.click()
    await page.locator('.routing-demo').waitFor()
    await pauseAndInspectPointer(page, size, '.routing-demo', 64)
    await capturePaused(page, capture, 'pointer-paused')
    await chapter(page, 2)
    assert.equal(await page.locator('.routing-scene').getAttribute('data-demo-applied'), 'true')
    await page.getByRole('tab', { name: 'No match', exact: true }).click()
    assert.equal(await page.locator('.routing-scene').getAttribute('data-demo-match'), 'false')
    await capturePaused(page, capture, 'default-route')
    await page.getByRole('tab', { name: 'Match', exact: true }).click()
    assert.equal(await page.locator('.routing-scene').getAttribute('data-demo-match'), 'true')
    assert.deepEqual(await page.evaluate(() => globalThis.__routingAudit.writes), [])
    await page.keyboard.press('Escape')
    await page.getByRole('dialog').waitFor({ state: 'hidden' })
    assert.equal(await trigger.evaluate(element => element === globalThis.document.activeElement), true)
  })
}
