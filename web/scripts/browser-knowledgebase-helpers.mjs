import assert from 'node:assert/strict'

export const topics = ['subscription', 'networks', 'payment', 'sharing', 'rules', 'why', 'hourly', 'traffic', 'routing']
export const lessonSources = { subscription: 4, networks: 4, payment: 6, sharing: 3, rules: 0, why: 2 }
export const chapterCounts = { subscription: 6, networks: 8, payment: 8, sharing: 8, rules: 8, why: 8, hourly: 7, traffic: 6, routing: 4 }
export const card = (page, id) => page.locator(`[data-knowledge-topic="${id}"]`)
export const player = page => page.locator('.demo-player')
export const calls = page => page.evaluate(() => [...globalThis.__knowledgebaseAudit.calls])
export const frames = page => page.evaluate(() => new Promise(resolve => globalThis.requestAnimationFrame(() => globalThis.requestAnimationFrame(resolve))))

export async function settleShell(page) {
  await page.waitForFunction(() => {
    const state = globalThis.__knowledgebaseAudit
    return state?.instrumented && state.pending === 0
      && ['api.getCommunityAccess', 'api.getBalance', 'preferencesApi.get', 'ipLookupApi.state', 'displayCurrencyApi.get'].every(name => state.calls.includes(name))
  })
}

export async function settle(page) {
  await page.locator('.knowledge-card').first().waitFor()
  await settleShell(page)
  await page.evaluate(() => globalThis.document.fonts.ready)
  await frames(page)
  assert.equal(await page.locator('.knowledge-card').count(), topics.length)
  assert.equal(await page.locator('.knowledge-card').first().getAttribute('data-knowledge-topic'), 'subscription')
  assert.deepEqual(await page.evaluate(() => globalThis.__knowledgebaseAudit.unexpected), [])
}

export async function inspectDesktopKnowledgeLabel(page, size) {
  if (size.name !== 'desktop') return
  await page.evaluate(() => globalThis.document.fonts.ready)
  await frames(page)
  const label = page.locator('.side-rail a[href="/knowledgebase"] [data-slot="linkLabel"]')
  await label.waitFor()
  assert.equal(await label.evaluate(element => element.scrollWidth <= element.clientWidth + 1), true, 'desktop Knowledgebase label is truncated')
}

export async function inspectTransport(page, step, fraction) {
  await frames(page)
  const view = await page.evaluate(() => {
    const root = globalThis.document.querySelector('.demo-transport__seek')
    const track = root?.querySelector('[data-slot="track"]')?.getBoundingClientRect()
    const thumb = root?.querySelector('[data-slot="thumb"][role="slider"]')
    const range = root?.querySelector('[data-slot="range"]')?.getBoundingClientRect()
    if (!track || !thumb || !range) return null
    const bounds = thumb.getBoundingClientRect()
    const selected = selector => [...globalThis.document.querySelectorAll(`.demo-transport__chapters ${selector}`)].map(element => element.getAttribute('data-demo-step'))
    return {
      current: selected('[data-demo-step].is-current'), accessible: selected('[data-demo-step][aria-current="step"]'),
      value: thumb.getAttribute('aria-valuenow'), trackLeft: track.left, trackWidth: track.width,
      thumbCenter: bounds.left + bounds.width / 2, thumbWidth: bounds.width, rangeWidth: range.width,
    }
  })
  assert.ok(view, 'demo transport is missing its Nuxt UI slider slots')
  assert.deepEqual(view.current, [String(step)], 'visual chapter selection is stale or duplicated')
  assert.deepEqual(view.accessible, view.current, 'visual and accessible chapter selection disagree')
  const value = Math.round(fraction * 1000)
  assert.equal(view.value, String(value), 'slider value does not match the selected chapter')
  assert.ok(view.trackWidth > view.thumbWidth && view.thumbWidth > 0, 'slider geometry is empty')
  const expectedCenter = view.trackLeft + view.thumbWidth / 2 + value / 1000 * (view.trackWidth - view.thumbWidth)
  assert.ok(Math.abs(view.thumbCenter - expectedCenter) <= 3, 'slider thumb is stale')
  assert.ok(Math.abs(view.rangeWidth - value / 1000 * view.trackWidth) <= 3, 'slider range does not match the selected chapter')
}

export async function open(page, id, status = 'ready') {
  const trigger = card(page, id).getByRole('button')
  await trigger.click()
  await player(page).waitFor()
  await page.locator(`.demo-three-scene[data-three-status="${status}"]`).waitFor()
  assert.equal(await page.getByRole('dialog').count(), 1)
  assert.equal(await page.locator('.demo-three-scene').count(), 1, `${id}: expected one shared 3D stage`)
  assert.ok(await page.locator('.demo-player__caption').innerText(), `${id}: missing scene caption`)
  assert.equal(await player(page).locator('details, summary').count(), 0, 'lesson content remains hidden in a disclosure')
  const duration = Number(await player(page).getAttribute('data-demo-duration'))
  const maximum = id === 'subscription' ? 21000 : id === 'routing' ? 15000 : id === 'hourly' ? 24000 : id === 'traffic' ? 21000 : 24000
  assert.ok(duration > 0 && duration <= maximum, `${id}: tutorial duration is not shortened (${duration}ms)`)
  return trigger
}

export async function close(page, trigger) {
  await page.keyboard.press('Escape')
  await page.getByRole('dialog').waitFor({ state: 'hidden' })
  assert.equal(await trigger.evaluate(element => element === globalThis.document.activeElement), true, 'closing the lesson lost trigger focus')
  assert.equal(await page.locator('.demo-three-scene').count(), 0, 'closed lesson retained its 3D stage')
}

export async function inspectVisibleChapter(page, locale) {
  const detail = page.locator('.demo-player__detail')
  assert.equal(await detail.isVisible(), true, 'chapter explanation is not visible in the animation')
  const copy = await detail.innerText()
  assert.ok(copy.trim().length > 15, 'chapter explanation is empty')
  assert.doesNotMatch(copy, /(?:knowledgeLessons|payg\.demo|diy\.demo)\./)
  if (locale === 'zh') assert.match(copy, /[\u3400-\u9fff]/)
  const points = page.locator('.demo-player__points > li')
  assert.equal(await points.count(), 3, 'chapter is missing its three icon-led details')
  for (const point of await points.all()) {
    const text = (await point.innerText()).trim()
    assert.ok(text.length > 0)
    assert.doesNotMatch(text, /(?:knowledgeLessons|payg\.demo|diy\.demo)\./)
    assert.equal(await point.locator('.iconify').count(), 1, 'chapter detail lacks an icon')
  }
  const overflow = await page.locator('.demo-player__detail, .demo-player__points > li').evaluateAll(elements => elements.some(element => element.scrollWidth > element.clientWidth + 1))
  assert.equal(overflow, false, 'visible tutorial details overflow at this viewport')
  const annotations = await page.locator('.knowledge-payment-equation, .subscription-action, .knowledge-scene__legend').evaluateAll(elements => elements.map(element => {
    const scene = element.closest('.knowledge-scene')?.querySelector('.demo-three-scene')?.getBoundingClientRect()
    const bounds = element.getBoundingClientRect()
    return {
      annotation: element.className,
      belowScene: Boolean(scene && bounds.top >= scene.bottom - 1),
      withinSceneWidth: Boolean(scene && bounds.left >= scene.left - 1 && bounds.right <= scene.right + 1),
      fitsContent: element.scrollWidth <= element.clientWidth + 1,
    }
  }))
  assert.deepEqual(annotations.filter(item => !item.belowScene || !item.withinSceneWidth || !item.fitsContent), [], 'tutorial annotation overlaps the 3D scene or overflows horizontally')
  return await player(page).innerText()
}

export async function inspectSources(page, id) {
  const links = page.locator('.knowledge-player__sources a')
  assert.equal(await links.count(), lessonSources[id])
  for (const source of await links.evaluateAll(elements => elements.map(element => ({ href: element.getAttribute('href'), rel: element.getAttribute('rel'), text: element.textContent })))) {
    assert.match(source.href, /^https:\/\//)
    assert.equal(source.rel, 'noopener noreferrer')
    assert.ok(source.text.trim().length > 4)
    assert.doesNotMatch(source.text, /knowledge\./)
  }
}
