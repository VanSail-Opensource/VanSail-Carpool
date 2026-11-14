import assert from 'node:assert/strict'
import { hourlyPreferenceScenarios } from './browser-hourly-preferences.mjs'

const conflict = 'The policy, price or account state changed. Refresh, review the current terms and try again.'
const mutations = page => page.evaluate(() => globalThis.__hourlyAudit.calls.filter(call => call.method !== 'GET'))

export async function hourlyRecoveryScenarios(scenario, size) {
  await hourlyPreferenceScenarios(scenario, size)
  await scenario(size, 'hourly-recovery-exhausted', 'hourly-audit.html?active&recovery-exhausted&extra', async (page, capture) => {
    const meter = page.locator('[data-hourly-throttle]')
    await meter.waitFor()
    const toggle = meter.getByRole('switch', { name: 'Buy extra full-speed traffic automatically', exact: true })
    assert.equal(await toggle.getAttribute('aria-checked'), 'true')
    assert.equal(await page.getByRole('progressbar').count(), 0)
    assert.equal(await page.getByText('109.81 MiB', { exact: true }).count(), 0)
    assert.equal(await page.locator('[data-hourly-boost]').count(), 0)
    assert.deepEqual(await page.evaluate(() => {
      const usage = globalThis.__hourlyAudit.fixture.view.usage
      return [usage.freeUsedBytes, usage.accounting.freeRecoveryUnknownBytes, usage.accounting.openReservedBytes]
    }), ['2032331516', '115152132', '0'])
    await capture('recovery-has-no-usable-credit')
    await toggle.click()
    await page.waitForFunction(() => globalThis.document.querySelector('[data-hourly-throttle] [role="switch"]')?.getAttribute('aria-checked') === 'false')
    assert.equal(await meter.isVisible(), true)
    assert.equal(await page.getByRole('dialog').count(), 0)
    const writes = await mutations(page)
    assert.equal(writes.length, 1)
    assert.deepEqual(writes[0].body, { enabled: false, policyVersion: 'fixture-policy-1' })
  })
  await scenario(size, 'hourly-open-credit-usable', 'hourly-audit.html?active&open-credit', async (page, capture) => {
    await page.getByRole('progressbar', { name: 'Full-speed traffic usage', exact: true }).waitFor()
    assert.equal(await page.locator('[data-hourly-throttle]').count(), 0)
    await page.locator('.hourly-allowance__legend').getByText('109.81 MiB', { exact: true }).waitFor()
    assert.deepEqual(await mutations(page), [])
    await capture('open-allocation-still-usable')
  })
  await scenario(size, 'hourly-clean-summary', 'hourly-audit.html?active&current-unknown&historical-unknown&historical-unsealed', async (page, capture) => {
    await page.locator('.hourly-summary').waitFor()
    assert.equal(await page.locator('[data-hourly-accounting]').count(), 0)
    assert.equal(await page.getByRole('switch', { name: 'Buy extra full-speed traffic automatically', exact: true }).count(), 0)
    assert.equal(await page.getByText(/approved price ceiling|reserved for open traffic|fully confirmed/).count(), 0)
    assert.deepEqual(await page.evaluate(() => {
      const usage = globalThis.__hourlyAudit.fixture.view.usage
      return [usage.accounting.openReservedBytes, usage.accounting.recoveryUnknownBytes, usage.accounting.historicalRecoveryUnknownBytes]
    }), ['40960', '131072', '262144'])
    assert.deepEqual(await mutations(page), [])
    await capture('quiet-summary')
  })
  await scenario(size, 'hourly-accounting-history-ignored-zh', 'hourly-audit.html?first&historical-unknown&historical-unsealed&zh', async (page, capture) => {
    await page.getByText('Tokyo Direct', { exact: true }).first().waitFor()
    assert.equal(await page.locator('[data-hourly-accounting]').count(), 0)
    assert.equal(await page.locator('.hourly-summary').count(), 0)
    assert.deepEqual(await page.evaluate(() => {
      const usage = globalThis.__hourlyAudit.fixture.view.usage
      return [usage.accountingPending, usage.accounting.historicalRecoveryUnknownBytes, usage.accounting.historicalUnsealedReservedBytes]
    }), [true, '262144', '524288'])
    assert.deepEqual(await mutations(page), [])
    await capture('historical-only-ignored')
  })
  await scenario(size, 'hourly-price-consent', 'hourly-audit.html?settings&active', async (page, capture) => {
    const toggle = page.getByRole('switch', { name: 'Buy extra full-speed traffic automatically', exact: true })
    await toggle.click()
    await page.getByRole('dialog').getByText(/automatically debit 1.25 TXB/).waitFor()
    await capture('reviewed')
    await page.evaluate(() => {
      const policy = globalThis.__hourlyAudit.fixture.view.policy
      policy.version = 'updated-policy'
      policy.policy.extraGBPriceTXBMinor = '250'
      globalThis.dispatchEvent(new globalThis.Event('focus'))
    })
    await page.getByRole('dialog').waitFor({ state: 'hidden' })
    await page.getByText(conflict, { exact: true }).waitFor()
    assert.deepEqual(await mutations(page), [])
    await capture('changed')
    await toggle.click()
    await page.getByRole('dialog').getByText(/automatically debit 2.50 TXB/).waitFor()
    await page.getByRole('button', { name: 'Enable automatic purchases', exact: true }).click()
    await page.waitForFunction(() => globalThis.__hourlyAudit.calls.some(call => call.method === 'PUT'))
    const writes = await mutations(page)
    assert.equal(writes.length, 1)
    assert.deepEqual(writes[0].body, { enabled: true, policyVersion: 'updated-policy', acceptedPriceTXBMinor: '250' })
  })
  await scenario(size, 'hourly-offer-consent', 'hourly-audit.html?activation', async (page, capture) => {
    await page.getByRole('button', { name: 'Enable', exact: true }).first().click()
    await page.getByLabel('Activation code', { exact: true }).fill('constructed-code')
    await capture('code')
    await page.evaluate(() => {
      globalThis.__hourlyAudit.fixture.view.squads[0].offer.revision = '2'
      globalThis.dispatchEvent(new globalThis.Event('focus'))
    })
    await page.getByRole('dialog').waitFor({ state: 'hidden' })
    assert.deepEqual(await mutations(page), [])
    await page.getByRole('button', { name: 'Enable', exact: true }).first().click()
    assert.equal(await page.getByLabel('Activation code', { exact: true }).inputValue(), '')
    await page.getByLabel('Activation code', { exact: true }).fill('reviewed-code')
    await page.getByRole('dialog').getByRole('button', { name: 'Continue', exact: true }).click()
    await page.waitForFunction(() => globalThis.__hourlyAudit.calls.some(call => call.path === '/api/v1/hourly/activate'))
    const writes = await mutations(page)
    assert.equal(writes.length, 1)
    assert.equal(writes[0].body.offerRevision, '2')
    assert.equal(writes[0].body.activationCode, 'reviewed-code')
  })
  await scenario(size, 'hourly-enrollment-consent', 'hourly-audit.html?active', async (page, capture) => {
    await page.getByRole('button', { name: 'Disable', exact: true }).click()
    await page.getByRole('dialog').waitFor()
    await capture('reviewed')
    await page.evaluate(() => {
      globalThis.__hourlyAudit.fixture.view.squads[0].enrollment.id = 'replacement-enrollment'
      globalThis.dispatchEvent(new globalThis.Event('focus'))
    })
    await page.getByRole('dialog').waitFor({ state: 'hidden' })
    assert.deepEqual(await mutations(page), [])
    await page.getByRole('button', { name: 'Disable', exact: true }).click()
    await page.getByRole('dialog').getByRole('button', { name: 'Disable', exact: true }).click()
    await page.waitForFunction(() => globalThis.__hourlyAudit.calls.some(call => call.method === 'POST'))
    const writes = await mutations(page)
    assert.equal(writes.length, 1)
    assert.equal(writes[0].path, '/api/v1/hourly/enrollments/replacement-enrollment/disable')
  })
  await scenario(size, 'hourly-retained-credit', 'hourly-audit.html?active&out-of-allowance&extra-off', async (page, capture) => {
    await page.getByRole('progressbar', { name: 'Full-speed traffic usage', exact: true }).waitFor()
    assert.equal(await page.locator('[data-hourly-throttle]').count(), 0)
    assert.equal(await page.locator('.hourly-allowance__segment--extra-left').isVisible(), true)
    await capture('funded')
    await page.evaluate(() => {
      const usage = globalThis.__hourlyAudit.fixture.view.usage
      usage.extraUsedBytes = usage.extraFundedBytes
      usage.extraReservedBytes = '0'
      globalThis.dispatchEvent(new globalThis.Event('focus'))
    })
    await page.locator('[data-hourly-throttle]').waitFor()
    assert.deepEqual(await mutations(page), [])
  })
  await scenario(size, 'hourly-zero-allowance', 'hourly-audit.html?active&allowance-zero-unused', async page => {
    await page.locator('[data-hourly-throttle]').waitFor()
    assert.equal(await page.getByRole('progressbar', { name: 'Full-speed traffic usage', exact: true }).count(), 0)
    assert.deepEqual(await mutations(page), [])
  })
}
