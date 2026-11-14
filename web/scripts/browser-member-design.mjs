import assert from 'node:assert/strict'
import { inspectBetLabel, inspectHostName, inspectAllowanceLegend, settleCardEntry } from './browser-member-design-checks.mjs'

// Each case mounts real member components with isolated, constructed data.
const pages = [
  ['home', 'subscription-audit.html?mode=home&shell&around&history-shares', '.home-layout'],
  ['subscription', 'subscription-audit.html?compatibility&diagnostics', '.subscription-page__hosts'],
  ['explore-traffic', 'payg-audit.html?features', '.payg-page__grid--content'],
  ['explore-hourly', 'hourly-audit.html?active&catalog', '.catalog-experience'],
  ['connections', 'member-connections-audit.html', '.connections-page__workspace'],
  ['settings', 'settings-audit.html', '.settings-page'],
  ['diy', 'routing-audit.html?diy&populated&split&local-preview', '.diy-tabs'],
  ['community', 'community-audit.html?joined', '.community-rows'],
  ['activity', 'lucky-audit.html?bet&progress', '.page--activity .page-toolbar'],
  ['ip-lookup', 'ip-lookup-audit.html', '.ip-lookup-empty'],
  ['onboarding', 'onboarding-audit.html', '.agreement-callout'],
  ['payment-result', 'payment-return-audit.html?crypto&paid', '.payment-receipt__fields'],
  ['public-subscription', 'public-subscription-audit.html?diagnostics', '.subscription-facts'],
  ['emby', 'member-utility-audit.html?page=emby', '.emby-account-summary'],
  ['affiliates', 'member-utility-audit.html?page=affiliates', '.affiliate-referral-row'],
  ['admin-access', 'member-utility-audit.html?page=admin-access', '.admin-access-panel input[readonly]'],
  ['entry', 'member-utility-audit.html?page=entry', '.auth-screen'],
]

async function settled(page, selector) {
  await page.locator(selector).first().waitFor()
  await page.evaluate(async () => {
    await globalThis.document.fonts.ready
    await new Promise(resolve => globalThis.requestAnimationFrame(() => globalThis.requestAnimationFrame(resolve)))
  })
  assert.deepEqual(await page.evaluate(() => globalThis.__memberUtilityAudit?.unexpected ?? []), [], 'unexpected utility-fixture API request')
}

async function captureState(page, capture, label) {
  assert.equal(await page.evaluate(() => globalThis.document.documentElement.scrollWidth <= globalThis.innerWidth + 1), true, `${label}: document overflow`)
  await capture(label)
}

async function inspectSaveContrast(page) {
  const selector = '.diy-save button[data-haptic="confirm"]'
  await page.waitForFunction(selector => { const button = globalThis.document.querySelector(selector); return button && !button.disabled }, selector)
  const save = page.locator(selector)
  assert.equal(await save.isEnabled(), true)
  const colors = await save.evaluate(element => {
    const style = globalThis.getComputedStyle(element)
    const context = globalThis.document.createElement('canvas').getContext('2d', { willReadFrequently: true })
    const rgba = color => {
      context.clearRect(0, 0, 1, 1); context.fillStyle = color; context.fillRect(0, 0, 1, 1)
      return [...context.getImageData(0, 0, 1, 1).data]
    }
    return { foreground: rgba(style.color), background: rgba(style.backgroundColor), opacity: Number(style.opacity), image: style.backgroundImage }
  })
  assert.equal(colors.opacity, 1, 'enabled Save button is faded')
  assert.equal(colors.image, 'none', 'Save contrast requires a solid background')
  assert.equal(colors.background[3], 255, 'Save contrast requires an opaque background')
  const alpha = colors.foreground[3] / 255
  const foreground = colors.foreground.slice(0, 3).map((value, index) => value * alpha + colors.background[index] * (1 - alpha))
  const luminance = color => color.slice(0, 3).map(value => value / 255).map(value => value <= .04045 ? value / 12.92 : ((value + .055) / 1.055) ** 2.4).reduce((sum, value, index) => sum + value * [.2126, .7152, .0722][index], 0)
  const light = luminance(foreground), dark = luminance(colors.background)
  const ratio = (Math.max(light, dark) + .05) / (Math.min(light, dark) + .05)
  assert.ok(ratio >= 4.5, `enabled DIY Save contrast is ${ratio.toFixed(2)}:1; expected at least 4.5:1`)
}

async function inspectDesktopAddRoute(page) {
  const add = page.getByRole('button', { name: 'Add route', exact: true })
  await add.scrollIntoViewIfNeeded()
  const covered = await add.evaluate(element => {
    const button = element.getBoundingClientRect(), footer = globalThis.document.querySelector('.diy-save').getBoundingClientRect()
    const overlap = Math.min(button.right, footer.right) > Math.max(button.left, footer.left) && Math.min(button.bottom, footer.bottom) > Math.max(button.top, footer.top)
    const hit = globalThis.document.elementFromPoint(button.x + button.width / 2, button.y + button.height / 2)
    return overlap || !element.contains(hit)
  })
  assert.equal(covered, false, 'desktop Add route is covered by the Save footer or another element')
  await add.click({ trial: true })
}

async function inspectCatalogHeader(page) {
  const layout = await page.locator('[data-billing-demo]').evaluate(element => {
    const button = element.getBoundingClientRect(), modes = globalThis.document.querySelector('.catalog-experience__modes').getBoundingClientRect()
    return { width: button.width, height: button.height, centerDifference: Math.abs(button.y + button.height / 2 - modes.y - modes.height / 2), gap: button.left - modes.right, right: button.right, viewport: globalThis.innerWidth }
  })
  assert.ok(layout.width >= 44 && layout.height >= 44, 'billing demonstration tap target is smaller than 44px')
  assert.ok(layout.centerDifference <= 1 && layout.gap >= 0 && layout.right <= layout.viewport, 'billing demonstration icon left the mode selector row')
}

export async function memberDesignScenarios(scenario, size) {
  for (const [name, path, selector] of pages) {
    await scenario(size, `design-${name}`, path, async (page, capture) => {
      await settled(page, selector)
      if (name === 'home') {
        assert.equal(await page.locator('.subscription-entrance').isVisible(), size.name === 'mobile', 'Home subscription shortcut visibility differs from the sidebar layout')
        assert.equal(await page.locator('.side-rail__member').count(), 0, 'desktop member identity remained visible')
        if (size.name === 'desktop') assert.equal(await page.locator('.side-rail a[href="/subscription"]').isVisible(), true)
      }
      if (name === 'explore-traffic') assert.equal(await page.locator('.payg-summary').count(), 0, 'traffic page repeated account totals')
      if (name === 'activity') await inspectBetLabel(page)
      if (name === 'subscription') await inspectHostName(page)
      if (name === 'explore-hourly') {
        await page.getByRole('button', { name: 'Charge by hour', exact: true }).click()
        await settled(page, '.hourly-page__squads')
        assert.equal(await page.locator('.hourly-summary__wallet').count(), 0, 'hourly page repeated the wallet balance')
      }
      if (name === 'diy') {
        await inspectSaveContrast(page)
        await captureState(page, capture, 'connection')
        for (const tab of ['XHTTP', 'Routing', 'Compatibility']) {
          await page.getByRole('tab', { name: tab, exact: true }).click()
          if (tab === 'Routing' && size.name === 'desktop') await inspectDesktopAddRoute(page)
          await captureState(page, capture, tab.toLowerCase())
        }
      }
      if (name === 'connections') {
        const search = page.getByRole('searchbox')
        await search.fill('2001:4860::8888')
        await page.waitForFunction(() => globalThis.document.querySelectorAll('.connection-list__row').length === 1)
        await capture('filtered')
        await search.clear()
        await page.waitForFunction(() => globalThis.document.querySelectorAll('.connection-list__row').length === 25)
      }
      assert.ok((await page.locator('body').innerText()).trim().length > 20, 'member page is empty')
    })
  }
  await scenario({ ...size, reducedMotion: 'no-preference' }, 'design-card-effects', 'hourly-audit.html?catalog&active', async (page, capture) => {
    await settled(page, '.squad-pricing-card')
    if (size.name === 'mobile') {
      await page.getByRole('button', { name: 'Charge by hour', exact: true }).click()
      await settled(page, '.hourly-squad')
    }
    const card = page.locator('.squad-pricing-card').first()
    await settleCardEntry(page)
    if (size.name === 'desktop') await card.hover()
    else await card.locator('.hourly-squad__toggle').focus()
    // Hold finite decoration at its midpoint only for the visual evidence.
    await card.evaluate(element => {
      for (const animation of element.getAnimations({ subtree: true })) {
        const end = animation.effect?.getComputedTiming().endTime
        if (animation instanceof globalThis.CSSAnimation && animation.animationName === 'member-card-sweep' && Number.isFinite(end) && end > 0) { animation.pause(); animation.currentTime = end / 2 }
      }
    })
    await capture(size.name === 'desktop' ? 'hover' : 'focus')
    await card.evaluate(element => { for (const animation of element.getAnimations({ subtree: true })) if (animation instanceof globalThis.CSSAnimation && animation.animationName === 'member-card-sweep' && animation.playState === 'paused') animation.play() })
    assert.deepEqual(await page.evaluate(() => globalThis.__hourlyAudit.calls.filter(call => call.method !== 'GET')), [])
  })
  await scenario(size, 'design-loading', 'member-connections-audit.html?slow', async (page, capture) => {
    await page.locator('.connections-page__skeleton').waitFor()
    await capture('loading')
    await settled(page, '.connections-page__workspace')
  })
  await scenario(size, 'design-empty', 'member-connections-audit.html?empty', async page => {
    await settled(page, '.connections-page__empty')
    assert.equal(await page.getByRole('searchbox').isEnabled(), true)
  })
  await scenario(size, 'design-error', 'hourly-audit.html?error', async page => {
    await page.getByRole('button', { name: 'Try again', exact: true }).waitFor()
  })
  await scenario(size, 'design-entry-loading', 'member-utility-audit.html?page=entry&loading', async page => {
    await settled(page, '.session-loading')
  })
  await scenario(size, 'design-emby-setup', 'member-utility-audit.html?page=emby&empty', async page => {
    await settled(page, '.emby-form h2')
  })
  await scenario(size, 'design-affiliates-error', 'member-utility-audit.html?page=affiliates&error', async page => {
    await settled(page, '.affiliate-page .error-state')
  })
  if (size.name !== 'mobile') return
  for (const [name, path, selector] of pages.filter(([name]) => ['home', 'explore-traffic', 'explore-hourly'].includes(name))) {
    await scenario({ name: 'mobile-320-en', viewport: { width: 320, height: 812 } }, `design-${name}`, path, async page => {
      await settled(page, selector)
      assert.match(await page.locator('html').getAttribute('lang'), /^en/)
      if (name === 'explore-hourly') { await page.getByRole('button', { name: 'Charge by hour', exact: true }).click(); await settled(page, '.hourly-page__squads') }
      if (name === 'explore-hourly') await inspectAllowanceLegend(page)
      if (name.startsWith('explore-')) await inspectCatalogHeader(page)
    })
  }
  const narrow = { name: 'mobile-320-zh', viewport: { width: 320, height: 812 } }
  for (const [name, path, selector] of pages.filter(([name]) => ['home', 'subscription', 'explore-traffic', 'explore-hourly', 'connections', 'settings', 'diy', 'community', 'emby', 'affiliates', 'admin-access', 'entry'].includes(name))) {
    await scenario(narrow, `design-${name}`, `${path}${path.includes('?') ? '&' : '?'}zh`, async (page, capture) => {
      await settled(page, selector)
      assert.match(await page.locator('html').getAttribute('lang'), /^zh/, `${name}: Chinese fixture locale was not applied`)
      if (name === 'subscription') await inspectHostName(page)
      if (name === 'explore-hourly') {
        await page.locator('.catalog-experience__modes button').nth(1).click()
        await settled(page, '.hourly-page__squads')
        await inspectAllowanceLegend(page)
      }
      if (name.startsWith('explore-')) await inspectCatalogHeader(page)
      if (name === 'diy') {
        await inspectSaveContrast(page)
        for (const index of [0, 1, 2, 3]) {
          await page.locator('.diy-tabs > [role="tablist"] [role="tab"]').nth(index).click()
          await captureState(page, capture, `tab-${index}`)
        }
      }
    })
  }
}
