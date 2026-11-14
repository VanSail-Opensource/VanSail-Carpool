import assert from 'node:assert/strict'

export const topics = ['subscription', 'networks', 'payment', 'sharing', 'rules', 'why', 'hourly', 'traffic', 'routing']
export const lessonSources = { subscription: 4, networks: 4, payment: 7, sharing: 3, rules: 0, why: 2 }
export const chapterCounts = { subscription: 12, networks: 10, payment: 12, sharing: 10, rules: 10, why: 10, hourly: 10, traffic: 9, routing: 8 }
export const card = (page, id) => page.locator(`.knowledge-card[data-knowledge-topic="${id}"]`)
export const player = page => page.locator('.demo-player')
export const visual = page => page.locator('.knowledge-scene, .hourly-scene, .traffic-scene, .routing-scene')
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

export async function open(page, id) {
  const trigger = card(page, id).getByRole('button')
  await trigger.click()
  await player(page).waitFor()
  await visual(page).waitFor()
  assert.equal(await page.getByRole('dialog').count(), 1)
  assert.equal(await visual(page).count(), 1, `${id}: expected one illustrative scene`)
  assert.equal(await player(page).locator('canvas').count(), 0, `${id}: tutorial still needs a GPU canvas`)
  assert.ok(await page.locator('.demo-player__caption').innerText(), `${id}: missing scene caption`)
  assert.equal(await player(page).locator('details, summary').count(), 0, 'lesson content remains hidden in a disclosure')
  const duration = Number(await player(page).getAttribute('data-demo-duration'))
  const beat = duration / chapterCounts[id]
  assert.ok(beat >= 2400 && beat <= 3600, `${id}: chapters must stay short and readable (${beat}ms)`)
  return trigger
}

export async function close(page, trigger) {
  await page.keyboard.press('Escape')
  await page.getByRole('dialog').waitFor({ state: 'hidden' })
  assert.equal(await trigger.evaluate(element => element === globalThis.document.activeElement), true, 'closing the lesson lost trigger focus')
  assert.equal(await visual(page).count(), 0, 'closed lesson retained its visual scene')
}

export async function inspectVisibleChapter(page, locale) {
  const detail = page.locator('.demo-player__detail')
  assert.equal(await detail.isVisible(), true, 'chapter explanation is not visible in the animation')
  const copy = await detail.innerText()
  assert.ok(copy.trim().length > 5, 'chapter explanation is empty')
  assert.doesNotMatch(copy, /(?:knowledgeLessons|knowledgeFilm|payg\.demo|diy\.demo)\./)
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
  assert.equal(await visual(page).evaluate(element => element.scrollWidth <= element.clientWidth + 1), true, 'mock interface or flow overflows horizontally')
  assert.equal(await player(page).locator('.demo-player__film').evaluate(element => element.scrollWidth <= element.clientWidth + 1), true, 'tutorial film overflows horizontally')
  const clipped = await visual(page).locator('.demo-window__bar, .demo-window__body, .demo-window__footer, .demo-window__body > *, .payment-equation > span, .subscription-options > div, .subscription-clients > div, .network-checks > span, .sharing-ip, .sharing-note, .sharing-checks > span').evaluateAll(elements => elements
    .filter(element => element.scrollWidth > element.clientWidth + 1)
    .map(element => ({ class: element.className, visible: element.clientWidth, content: element.scrollWidth })))
  assert.deepEqual(clipped, [], 'mock interface clips content inside a rounded surface')
  return await player(page).innerText()
}

export async function inspectPaymentAmounts(page, mode) {
  const payment = page.locator('.knowledge-payment')
  assert.equal(await payment.getAttribute('data-payment-mode'), mode)
  const amounts = await payment.evaluate(element => Object.fromEntries(['amount', 'fee', 'transfer', 'received'].map(key => [key, element.getAttribute(`data-payment-${key}`)])))
  const units = value => BigInt(value.replace('.', ''))
  assert.equal(amounts.received, amounts.amount, 'recipient must receive the exact displayed order amount')
  assert.equal(units(amounts.transfer) - (mode === 'deducted' ? units(amounts.fee) : 0n), units(amounts.received), 'exchange fee arithmetic underpays or double-adds the fee')
  const copy = await payment.innerText()
  assert.ok(copy.includes(amounts.amount), 'exact order amount is not visible')
  if (mode === 'deducted') assert.ok(copy.includes(amounts.transfer), 'fee-adjusted transfer amount is not visible')
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
