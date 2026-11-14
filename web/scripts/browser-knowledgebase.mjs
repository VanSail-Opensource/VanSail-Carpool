import assert from 'node:assert/strict'

const topics = ['networks', 'payment', 'sharing', 'rules', 'why', 'hourly', 'traffic', 'routing']
const lessonSources = { networks: 4, payment: 6, sharing: 3, rules: 0, why: 2 }
const card = (page, id) => page.locator(`[data-knowledge-topic="${id}"]`)
const player = page => page.locator('.demo-player')
const calls = page => page.evaluate(() => [...globalThis.__knowledgebaseAudit.calls])

async function settleShell(page) {
  await page.waitForFunction(() => {
    const state = globalThis.__knowledgebaseAudit
    return state?.instrumented && state.pending === 0
      && ['api.getCommunityAccess', 'api.getBalance', 'preferencesApi.get', 'ipLookupApi.state', 'displayCurrencyApi.get'].every(name => state.calls.includes(name))
  })
}

async function settle(page) {
  await page.locator('.knowledge-card').first().waitFor()
  await settleShell(page)
  await page.evaluate(async () => {
    await globalThis.document.fonts.ready
    await new Promise(resolve => globalThis.requestAnimationFrame(() => globalThis.requestAnimationFrame(resolve)))
  })
  assert.equal(await page.locator('.knowledge-card').count(), 8)
  assert.deepEqual(await page.evaluate(() => globalThis.__knowledgebaseAudit.unexpected), [])
}

async function inspectDesktopKnowledgeLabel(page, size) {
  if (size.name !== 'desktop') return
  await page.evaluate(async () => {
    await globalThis.document.fonts.ready
    await new Promise(resolve => globalThis.requestAnimationFrame(() => globalThis.requestAnimationFrame(resolve)))
  })
  const label = page.locator('.side-rail a[href="/knowledgebase"] [data-slot="linkLabel"]')
  await label.waitFor()
  assert.equal(await label.evaluate(element => element.scrollWidth <= element.clientWidth + 1), true, 'desktop Knowledgebase label is truncated')
}

async function inspectTransport(page, step, fraction) {
  const view = await page.evaluate(async () => {
    await new Promise(resolve => globalThis.requestAnimationFrame(() => globalThis.requestAnimationFrame(resolve)))
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
  // Reka contains the thumb inside the track instead of placing its centre on each edge.
  const expectedCenter = view.trackLeft + view.thumbWidth / 2 + value / 1000 * (view.trackWidth - view.thumbWidth)
  assert.ok(Math.abs(view.thumbCenter - expectedCenter) <= 3, `slider thumb is stale: expected ${expectedCenter}px, received ${view.thumbCenter}px`)
  assert.ok(Math.abs(view.rangeWidth - value / 1000 * view.trackWidth) <= 3, 'slider range does not match the selected chapter')
}

async function open(page, id) {
  const trigger = card(page, id).getByRole('button')
  await trigger.click()
  await player(page).waitFor()
  assert.equal(await page.getByRole('dialog').count(), 1)
  assert.ok(await page.locator('.demo-player__caption').innerText(), `${id}: missing scene caption`)
  return trigger
}

async function close(page, trigger) {
  await page.keyboard.press('Escape')
  await page.getByRole('dialog').waitFor({ state: 'hidden' })
  assert.equal(await trigger.evaluate(element => element === globalThis.document.activeElement), true, 'closing the lesson lost trigger focus')
}

async function sourceDetails(page, id, locale) {
  const details = page.locator('.knowledge-player__details')
  await details.locator('summary').click()
  assert.equal(await details.locator('ol > li').count(), 4)
  const links = details.locator('.knowledge-player__sources a')
  assert.equal(await links.count(), lessonSources[id])
  for (const source of await links.evaluateAll(elements => elements.map(element => ({ href: element.getAttribute('href'), rel: element.getAttribute('rel'), text: element.textContent })))) {
    assert.match(source.href, /^https:\/\//)
    assert.equal(source.rel, 'noopener noreferrer')
    assert.ok(source.text.trim().length > 4)
    assert.doesNotMatch(source.text, /knowledge\./)
  }
  const copy = await details.innerText()
  if (locale === 'zh') assert.match(copy, /[\u3400-\u9fff]/)
  if (id === 'payment') assert.match(copy, locale === 'zh' ? /Gas|手续费/ : /gas|fee/i)
  if (id === 'sharing') assert.match(copy, locale === 'zh' ? /风险|风控/ : /risk/i)
}

export async function knowledgebaseScenarios(scenario, size) {
  for (const locale of ['en', 'zh']) {
    await scenario(size, `knowledgebase-all-${locale}`, `knowledgebase-audit.html${locale === 'zh' ? '?zh' : ''}`, async (page, capture) => {
      await settle(page)
      assert.match(await page.locator('html').getAttribute('lang'), locale === 'zh' ? /^zh/ : /^en/)
      await inspectDesktopKnowledgeLabel(page, size)
      await capture('overview')
      const before = await calls(page)
      for (const id of topics) {
        const trigger = await open(page, id)
        assert.equal(await player(page).getAttribute('data-demo-playing'), 'false', 'reduced motion started automatic playback')
        assert.equal(await page.locator('.demo-pointer').count(), 0)
        const chapters = page.locator('[data-demo-step]')
        const count = await chapters.count()
        assert.ok(count >= 4)
        for (let index = 0; index < count; index++) {
          await chapters.nth(index).click()
          assert.equal(await chapters.nth(index).getAttribute('aria-current'), 'step')
          assert.equal(await player(page).getAttribute('data-demo-playing'), 'false')
          await inspectTransport(page, index, (index + .92) / count)
          if (id === 'payment' && (index === 1 || index === 2)) await capture(`payment-${index === 1 ? 'wallet-gas' : 'exchange-fee'}`)
        }
        if (Object.hasOwn(lessonSources, id)) await sourceDetails(page, id, locale)
        if (['networks', 'payment', 'sharing'].includes(id)) await capture(`${locale}-${id}`)
        await close(page, trigger)
      }
      assert.deepEqual(await calls(page), before, 'lesson playback called a member API')
      assert.deepEqual(await page.evaluate(() => globalThis.__knowledgebaseAudit.unexpected), [])
      assert.equal(await page.locator('.knowledge-card li').count(), topics.length * 3)
      assert.doesNotMatch(await page.locator('.page--knowledgebase').innerText(), /knowledge(?:Lessons)?\./)
    })
  }
  await scenario({ ...size, reducedMotion: 'no-preference' }, 'knowledgebase-controls', 'knowledgebase-audit.html', async (page, capture) => {
    await settle(page)
    const before = await calls(page)
    const trigger = await open(page, 'payment')
    const seek = page.locator('.demo-transport__seek [role="slider"]')
    await page.waitForFunction(() => Number(globalThis.document.querySelector('.demo-transport__seek [role="slider"]')?.getAttribute('aria-valuenow')) > 30)
    await page.getByRole('button', { name: 'Pause demonstration', exact: true }).click()
    assert.equal(await player(page).getAttribute('data-demo-playing'), 'false')
    const paused = await seek.getAttribute('aria-valuenow')
    await page.evaluate(() => new Promise(resolve => globalThis.requestAnimationFrame(() => globalThis.requestAnimationFrame(resolve))))
    assert.equal(await seek.getAttribute('aria-valuenow'), paused)
    await seek.focus(); await seek.press('End')
    assert.equal(await seek.getAttribute('aria-valuenow'), '1000')
    assert.equal(await page.locator('[data-demo-step="3"]').getAttribute('aria-current'), 'step')
    await inspectTransport(page, 3, 1)
    await capture('seek-end')
    await page.getByRole('button', { name: 'Replay demonstration', exact: true }).click()
    assert.equal(await player(page).getAttribute('data-demo-playing'), 'true')
    await page.getByRole('button', { name: 'Pause demonstration', exact: true }).click()
    await seek.focus(); await seek.press('Home')
    assert.equal(await seek.getAttribute('aria-valuenow'), '0')
    await close(page, trigger)
    assert.deepEqual(await calls(page), before, 'playback controls called a member API')
  })
  for (const locale of ['en', 'zh']) {
    await scenario(size, `knowledgebase-new-member-${locale}`, `knowledgebase-audit.html?mode=home&new-user${locale === 'zh' ? '&zh' : ''}`, async (page, capture) => {
      await page.locator('.knowledge-home').waitFor()
      await settleShell(page)
      const entrance = page.locator('.knowledge-home a[href="/knowledgebase"]')
      assert.equal(await entrance.isVisible(), true)
      assert.equal(await page.locator('.side-rail__footer').count(), 0, 'new member received privileged shortcuts')
      if (size.name === 'desktop') assert.equal(await page.locator('.side-rail a[href="/knowledgebase"]').isVisible(), true)
      else assert.equal(await page.locator('.home-around').isVisible(), true)
      await inspectDesktopKnowledgeLabel(page, size)
      await capture('home-entrance')
      await entrance.focus(); await entrance.press('Enter')
      await settle(page)
      assert.equal(new globalThis.URL(page.url()).pathname, '/knowledgebase')
      if (size.name === 'mobile') assert.equal(await page.locator('.bottom-nav__item').count(), 2)
    })
  }
}
