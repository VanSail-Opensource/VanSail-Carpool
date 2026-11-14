import assert from 'node:assert/strict'

const label = 'Buy extra full-speed traffic automatically'
const writes = page => page.evaluate(() => globalThis.__hourlyAudit.calls.filter(call => call.method !== 'GET'))

export async function hourlyPreferenceScenarios(scenario, size) {
  await scenario(size, 'hourly-inline-auto-buy', 'hourly-audit.html?active&credit-exhausted', async (page, capture) => {
    const meter = page.locator('[data-hourly-throttle]')
    const toggle = meter.getByRole('switch', { name: label, exact: true })
    await toggle.waitFor()
    assert.equal(await toggle.getAttribute('aria-checked'), 'false')
    await capture('exhausted-switch')
    await toggle.click()
    await page.getByRole('dialog').waitFor()
    assert.deepEqual(await writes(page), [])
    // The open modal hides background controls from the accessibility tree.
    assert.equal(await meter.locator('[role="switch"]').getAttribute('aria-checked'), 'false')
    await page.getByRole('button', { name: 'Enable automatic purchases', exact: true }).click()
    await page.waitForFunction(() => globalThis.__hourlyAudit.fixture.view.extraPreference.extraEnabled)
    await page.locator('[data-hourly-boost]').waitFor()
    await capture('confirmed-boost')
    await meter.waitFor({ state: 'hidden' })
    assert.equal(await page.getByRole('switch', { name: label, exact: true }).count(), 0)
    const mutations = await writes(page)
    assert.equal(mutations.length, 1)
    assert.deepEqual(mutations[0].body, { enabled: true, policyVersion: 'fixture-policy-1', acceptedPriceTXBMinor: '125' })
  })
  await scenario(size, 'hourly-settings-disable', 'hourly-audit.html?settings&active&credit-exhausted&extra', async (page, capture) => {
    const toggle = page.getByRole('switch', { name: label, exact: true })
    await toggle.waitFor()
    assert.equal(await toggle.getAttribute('aria-checked'), 'true')
    await toggle.click()
    await page.waitForFunction(() => !globalThis.__hourlyAudit.fixture.view.extraPreference.extraEnabled)
    await page.waitForFunction(() => globalThis.document.querySelector('[role="switch"]')?.getAttribute('aria-checked') === 'false')
    assert.equal(await toggle.isVisible(), true)
    assert.equal(await page.getByRole('dialog').count(), 0)
    assert.deepEqual((await writes(page))[0].body, { enabled: false, policyVersion: 'fixture-policy-1' })
    await capture('settings-off')
  })
  await scenario(size, 'hourly-settings-save-failure', 'hourly-audit.html?settings&active&conflict', async (page, capture) => {
    const toggle = page.getByRole('switch', { name: label, exact: true })
    await toggle.click()
    await page.getByRole('button', { name: 'Enable automatic purchases', exact: true }).click()
    await page.getByRole('dialog').getByText('The policy, price or account state changed. Refresh, review the current terms and try again.', { exact: true }).waitFor()
    // Read the retained state behind the modal without interacting with it.
    assert.equal(await page.locator('[data-settings-hourly] [role="switch"]').getAttribute('aria-checked'), 'false')
    assert.equal(await page.evaluate(() => globalThis.__hourlyAudit.fixture.view.extraPreference.extraEnabled), false)
    assert.equal((await writes(page)).length, 1)
    await page.evaluate(() => new Promise(resolve => globalThis.requestAnimationFrame(() => globalThis.requestAnimationFrame(resolve))))
    await page.waitForFunction(() => {
      const dialog = globalThis.document.querySelector('[role="dialog"]'), title = dialog?.querySelector('.dialog-title')
      return title && Number(globalThis.getComputedStyle(title).opacity) >= .999 && dialog.getAnimations({ subtree: true }).every(animation => !Number.isFinite(animation.effect?.getComputedTiming().endTime) || animation.playState === 'finished')
    })
    await capture('saved-preference-retained')
  })
  await scenario(size, 'hourly-zero-unavailable', 'hourly-audit.html?active&allowance-zero-unused&no-extra', async (page, capture) => {
    await page.locator('[data-hourly-throttle]').waitFor()
    assert.equal(await page.getByRole('switch', { name: label, exact: true }).count(), 0)
    await page.getByText('Full-speed credit is exhausted. Extra traffic purchases are unavailable.', { exact: true }).waitFor()
    assert.deepEqual(await writes(page), [])
    await capture('unavailable')
  })
}
