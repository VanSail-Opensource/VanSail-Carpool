import assert from 'node:assert/strict'

export async function purchaseRecoveryScenarios(scenario, size) {
  await scenario(size, 'queued-target-replaced', 'purchase-recovery-audit.html', async page => {
    await page.getByRole('button', { name: 'Cancel queued ride', exact: true }).click()
    await page.getByRole('dialog').waitFor()
    await page.evaluate(() => globalThis.__purchaseRecoveryAudit.replace())
    await page.getByRole('dialog').waitFor({ state: 'hidden' })
    assert.deepEqual(await page.evaluate(() => globalThis.__purchaseRecoveryAudit.state.cancels), [])
    assert.equal(await page.getByRole('button', { name: 'Cancel queued ride', exact: true }).isEnabled(), true)
  })
  await scenario(size, 'queued-pending-replaced', 'purchase-recovery-audit.html?defer', async (page, capture) => {
    await page.getByRole('button', { name: 'Cancel queued ride', exact: true }).click()
    await page.getByRole('button', { name: 'Cancel and refund', exact: true }).click()
    await page.waitForFunction(() => globalThis.__purchaseRecoveryAudit.state.cancels.length === 1)
    await capture('cancelling')
    await page.evaluate(() => { globalThis.__purchaseRecoveryAudit.replace(); globalThis.__purchaseRecoveryAudit.release() })
    await page.getByRole('dialog').waitFor({ state: 'hidden' })
    await page.waitForFunction(() => globalThis.__purchaseRecoveryAudit.state.finished === 1)
    assert.deepEqual(await page.evaluate(() => globalThis.__purchaseRecoveryAudit.state.cancels), ['queued-1'])
    assert.equal(await page.evaluate(() => globalThis.__purchaseRecoveryAudit.state.changed), 0)
  })
  await scenario(size, 'queued-error-retry', 'purchase-recovery-audit.html?error', async (page, capture) => {
    await page.getByRole('button', { name: 'Cancel queued ride', exact: true }).click()
    const dialog = page.getByRole('dialog')
    const confirm = dialog.getByRole('button', { name: 'Cancel and refund', exact: true })
    await confirm.click()
    await dialog.getByRole('alert').getByText('Queued ride cancellation failed.', { exact: true }).waitFor()
    await capture('error')
    await page.evaluate(() => { globalThis.__purchaseRecoveryAudit.state.fail = false })
    await confirm.click()
    await dialog.waitFor({ state: 'hidden' })
    assert.deepEqual(await page.evaluate(() => globalThis.__purchaseRecoveryAudit.state.cancels), ['queued-1', 'queued-1'])
    assert.equal(await page.evaluate(() => globalThis.__purchaseRecoveryAudit.state.changed), 1)
  })
}
