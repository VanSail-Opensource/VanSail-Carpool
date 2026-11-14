import assert from 'node:assert/strict'

// Each case mounts real member components with isolated, constructed data.
const pages = [
  ['home', 'subscription-audit.html?mode=home&shell&around&history-shares', '.home-layout'],
  ['subscription', 'subscription-audit.html?compatibility&diagnostics', '.subscription-page__hosts'],
  ['explore-traffic', 'payg-audit.html?features', '.payg-page__grid--content'],
  ['explore-hourly', 'hourly-audit.html?active&catalog', '.catalog-experience'],
  ['connections', 'member-connections-audit.html', '.connections-page__workspace'],
  ['settings', 'settings-audit.html', '.settings-page'],
  ['diy', 'routing-audit.html?diy&populated&split&local-preview', '.diy-tabs'],
  ['community', 'community-audit.html?joined', '.community-page__workspace'],
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

export async function memberDesignScenarios(scenario, size) {
  for (const [name, path, selector] of pages) {
    await scenario(size, `design-${name}`, path, async (page, capture) => {
      await settled(page, selector)
      if (name === 'explore-hourly') {
        await page.getByRole('button', { name: 'Charge by hour', exact: true }).click()
        await settled(page, '.hourly-page__squads')
      }
      if (name === 'diy') {
        await captureState(page, capture, 'connection')
        for (const tab of ['XHTTP', 'Routing', 'Compatibility']) {
          await page.getByRole('tab', { name: tab, exact: true }).click()
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
  const narrow = { name: 'mobile-320-zh', viewport: { width: 320, height: 812 } }
  for (const [name, path, selector] of pages.filter(([name]) => ['home', 'subscription', 'explore-traffic', 'explore-hourly', 'connections', 'settings', 'diy', 'community', 'emby', 'affiliates', 'admin-access', 'entry'].includes(name))) {
    await scenario(narrow, `design-${name}`, `${path}${path.includes('?') ? '&' : '?'}zh`, async (page, capture) => {
      await settled(page, selector)
      if (name === 'explore-hourly') {
        await page.locator('.catalog-experience__modes button').nth(1).click()
        await settled(page, '.hourly-page__squads')
      }
      if (name === 'diy') {
        for (const index of [0, 1, 2, 3]) {
          await page.locator('.diy-tabs > [role="tablist"] [role="tab"]').nth(index).click()
          await captureState(page, capture, `tab-${index}`)
        }
      }
    })
  }
}
