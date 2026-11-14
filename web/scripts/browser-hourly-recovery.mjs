import assert from 'node:assert/strict'

const conflict = 'The policy, price or account state changed. Refresh, review the current terms and try again.'
const mutations = page => page.evaluate(() => globalThis.__hourlyAudit.calls.filter(call => call.method !== 'GET'))

export async function hourlyRecoveryScenarios(scenario, size) {
  await scenario(size, 'hourly-accounting-open', 'hourly-audit.html?active', async (page, capture) => {
    const open = page.locator('[data-hourly-accounting-kind="open"]')
    await open.waitFor()
    assert.equal(await open.textContent(), 'Today, 40 KiB is reserved for open traffic credits.')
    assert.equal(await page.locator('[data-hourly-accounting-kind]').count(), 1)
    assert.equal(await page.getByText(/still being reconciled/).count(), 0)
    assert.deepEqual(await mutations(page), [])
    await capture('normal-reservation')
  })
  await scenario(size, 'hourly-accounting-unknown', 'hourly-audit.html?active&current-unknown&historical-unknown&historical-unsealed', async (page, capture) => {
    const current = page.locator('[data-hourly-accounting-kind="recovery"]')
    const historical = page.locator('[data-hourly-accounting-kind="historical-recovery"]')
    const unsealed = page.locator('[data-hourly-accounting-kind="historical-unsealed"]')
    await current.waitFor()
    assert.match(await current.textContent(), /128 KiB remains reserved and unavailable/)
    assert.equal(await historical.count(), 0)
    assert.equal(await unsealed.count(), 0)
    assert.equal(await page.locator('[data-hourly-accounting-kind]').count(), 2)
    await capture('current-day-only')
    await page.evaluate(() => {
      const usage = globalThis.__hourlyAudit.fixture.view.usage
      usage.accounting.openReservedBytes = '0'
      usage.freeReservedBytes = usage.accounting.recoveryUnknownBytes
      usage.extraReservedBytes = '0'
      globalThis.dispatchEvent(new globalThis.Event('focus'))
    })
    await page.locator('[data-hourly-accounting-kind="open"]').waitFor({ state: 'hidden' })
    assert.equal(await page.locator('[data-hourly-accounting-kind]').count(), 1)
    assert.match(await current.textContent(), /128 KiB remains reserved and unavailable/)
    assert.equal(await historical.count(), 0)
    assert.equal(await unsealed.count(), 0)
    assert.deepEqual(await mutations(page), [])
    await capture('current-unknown-retained')
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
  await scenario(size, 'hourly-price-consent', 'hourly-audit.html?active', async (page, capture) => {
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
