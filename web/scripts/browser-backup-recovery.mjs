import assert from 'node:assert/strict'

export async function backupRecoveryScenarios(scenario, size) {
  for (const kind of ['backup', 'job']) {
    await scenario(size, 'admin-' + kind + '-delete-recovery', 'backup-recovery-audit.html?defer&slow&refresh-error', async (page, capture) => {
      await capture('loading')
      if (kind === 'job') await page.getByRole('tab', { name: 'Synchronization jobs', exact: true }).click()
      const trigger = page.getByRole('button', { name: kind === 'backup' ? 'Delete' : 'Delete constructed job', exact: true })
      await trigger.waitFor()
      await trigger.click()
      const dialog = page.getByRole('dialog')
      await capture('review')
      await dialog.getByRole('button', { name: kind === 'backup' ? 'Delete backup' : 'Delete job', exact: true }).click()
      await page.waitForFunction(() => globalThis.__backupRecoveryAudit.pending === 1)
      assert.equal(await dialog.getByRole('button', { name: 'Cancel', exact: true }).isEnabled(), false)
      assert.equal(await trigger.isEnabled(), false)
      await page.keyboard.press('Escape')
      assert.equal(await dialog.isVisible(), true)
      await capture('pending')
      await page.evaluate(() => globalThis.__backupRecoveryAudit.release(true))
      await dialog.getByRole('alert').filter({ hasText: kind === 'backup' ? 'Backup deletion failed.' : 'Job deletion failed.' }).waitFor()
      await capture('error')
      await dialog.getByRole('button', { name: kind === 'backup' ? 'Delete backup' : 'Delete job', exact: true }).click()
      await page.waitForFunction(() => globalThis.__backupRecoveryAudit.pending === 1)
      await page.evaluate(() => globalThis.__backupRecoveryAudit.release())
      await dialog.waitFor({ state: 'hidden' })
      await page.getByText(kind === 'backup' ? 'No backups yet' : 'No synchronization jobs', { exact: true }).waitFor()
      await page.getByText('Admin data could not be loaded.', { exact: true }).waitFor()
      await capture('confirmed-with-read-error')
      await page.evaluate(() => { globalThis.__backupRecoveryAudit.recover = true })
      await page.getByRole('button', { name: 'Retry', exact: true }).click()
      await page.getByText('Admin data could not be loaded.', { exact: true }).waitFor({ state: 'hidden' })
      assert.equal(await trigger.count(), 0)
      assert.deepEqual(await page.evaluate(() => globalThis.__backupRecoveryAudit.deletions), [
        { kind, id: 'constructed-' + kind }, { kind, id: 'constructed-' + kind },
      ])
    })
  }
  await scenario(size, 'admin-backup-retired-download', 'backup-recovery-audit.html?defer', async (page, capture) => {
    await page.getByRole('button', { name: 'Download', exact: true }).click()
    await page.waitForFunction(() => globalThis.__backupRecoveryAudit.pending === 1)
    assert.equal(await page.getByRole('button', { name: 'Download', exact: true }).isEnabled(), false)
    await capture('pending')
    await page.evaluate(() => globalThis.__backupRecoveryAudit.retire())
    await page.locator('[data-test="retired-backup-workspace"]').waitFor()
    await page.evaluate(() => globalThis.__backupRecoveryAudit.release())
    const state = await page.evaluate(() => globalThis.__backupRecoveryAudit)
    assert.deepEqual(state.downloads, ['constructed-backup'])
    assert.equal(state.objectURLs, 0, 'retired owner received a downloaded archive')
  })
}
