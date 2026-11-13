import assert from 'node:assert/strict'

export async function operationRecoveryScenarios(scenario, size) {
  await scenario(size, 'operation-domain-boundary', 'operation-resolution-audit.html?blocked', async page => {
    const dialog = page.getByRole('dialog')
    await dialog.waitFor()
    await dialog.getByText('Reconcile the owning workflow first. Its recorded item outcomes must support this receipt status.', { exact: true }).waitFor()
    assert.equal(await dialog.getByRole('combobox').isEnabled(), false)
    assert.equal(await dialog.getByRole('button', { name: 'Repair receipt', exact: true }).isEnabled(), false)
    assert.equal(await page.evaluate(() => globalThis.__operationResolutionAudit.calls), 0)
  })
  await scenario(size, 'operation-evidence-retry', 'operation-resolution-audit.html?error', async (page, capture) => {
    const dialog = page.getByRole('dialog')
    await dialog.waitFor()
    await dialog.getByRole('textbox').fill('Reviewed retained completed item evidence')
    const submit = dialog.getByRole('button', { name: 'Repair receipt', exact: true })
    assert.equal(await submit.isEnabled(), false, 'receipt repair defaults to a successful outcome')
    await dialog.getByRole('combobox').click()
    await page.getByRole('option', { name: 'Succeeded', exact: true }).click()
    assert.equal(await submit.isEnabled(), true)
    await submit.click()
    await dialog.getByText('Reconcile the owning workflow first. Its recorded item outcomes must support this receipt status.', { exact: true }).waitFor()
    await capture('reconciliation-required')
    assert.equal(await page.evaluate(() => globalThis.__operationResolutionAudit.calls), 1)
    await page.evaluate(() => { globalThis.__operationResolutionAudit.fail = false })
    await submit.click()
    await dialog.waitFor({ state: 'hidden' })
    assert.deepEqual(await page.evaluate(() => globalThis.__operationResolutionAudit.saved), { resolution: 'succeeded', reason: 'Reviewed retained completed item evidence' })
    assert.equal(await page.evaluate(() => globalThis.__operationResolutionAudit.calls), 2)
  })
  await scenario(size, 'session-service-retry', 'browser-auth-audit.html?recovery&session-unavailable', async page => {
    await page.getByText('Your session is temporarily unavailable. Please try again.', { exact: true }).waitFor()
    assert.equal(await page.evaluate(() => globalThis.__browserAuthAudit.exchanges), 0)
    await page.getByRole('button', { name: 'Try again', exact: true }).click()
    await page.waitForFunction(() => globalThis.__browserAuthAudit.sessionReads === 2)
    await page.getByText('Your session is temporarily unavailable. Please try again.', { exact: true }).waitFor()
    assert.equal(await page.evaluate(() => globalThis.__browserAuthAudit.exchanges), 0)
  })
}
