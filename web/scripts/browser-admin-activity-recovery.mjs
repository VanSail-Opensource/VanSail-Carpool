import assert from 'node:assert/strict'

export async function adminActivityRecoveryScenarios(scenario, size) {
  await scenario(size, 'admin-game-save-recovery', 'lucky-admin-audit.html?defer-save&read-after-save-error&dark', async (page, capture) => {
    await page.getByRole('button', { name: 'Edit Retained coin', exact: true }).click()
    const dialog = page.getByRole('dialog')
    const name = dialog.getByLabel('Name', { exact: true })
    await name.fill('Confirmed game')
    await dialog.getByRole('button', { name: 'Save game', exact: true }).click()
    await page.waitForFunction(() => globalThis.__audit.pendingSaves === 1)
    assert.equal(await name.isEditable(), false)
    assert.equal(await dialog.getByRole('button', { name: 'Cancel', exact: true }).isEnabled(), false)
    await page.keyboard.press('Escape')
    assert.equal(await dialog.isVisible(), true)
    await capture('pending')
    await page.evaluate(() => globalThis.__audit.releaseSave(true))
    await dialog.getByText('The game could not be saved.', { exact: true }).waitFor()
    await capture('save-error')
    await dialog.getByRole('button', { name: 'Save game', exact: true }).click()
    await page.waitForFunction(() => globalThis.__audit.pendingSaves === 1)
    await page.evaluate(() => globalThis.__audit.releaseSave())
    await dialog.waitFor({ state: 'hidden' })
    await page.getByRole('alert').filter({ hasText: 'Activity configuration could not be loaded.' }).waitFor()
    assert.equal(await page.getByRole('button', { name: 'Edit Confirmed game', exact: true }).isVisible(), true)
    assert.equal(await page.getByText('The game could not be saved.', { exact: true }).count(), 0)
    await capture('refresh-error')
    await page.evaluate(() => { globalThis.__audit.recover = true })
    await page.getByRole('button', { name: 'Refresh', exact: true }).click()
    await page.getByRole('alert').waitFor({ state: 'hidden' })
    const writes = await page.evaluate(() => globalThis.__audit.gameWrites)
    assert.equal(writes.length, 2)
    assert.deepEqual(writes.map(write => [write.id, write.body.name]), [['game-audit', 'Confirmed game'], ['game-audit', 'Confirmed game']])
  })
  await scenario(size, 'admin-draw-retired-inventory', 'lucky-admin-audit.html?stale-load&dark', async (page, capture) => {
    await page.waitForFunction(() => globalThis.__audit.pendingReads === 2)
    await page.getByRole('tab', { name: 'Lucky draws', exact: true }).click()
    await page.getByRole('button', { name: 'Edit Weekend rewards', exact: true }).click()
    const dialog = page.getByRole('dialog')
    await dialog.getByLabel('Name', { exact: true }).fill('Confirmed draw')
    await dialog.getByRole('button', { name: 'Save draw', exact: true }).click()
    await page.waitForFunction(() => globalThis.__audit.drawWrites.length === 1)
    await page.waitForFunction(() => {
      const fieldset = globalThis.document.querySelector('.draw-editor')
      return fieldset && !fieldset.disabled
    })
    await page.evaluate(() => globalThis.__audit.releaseReads())
    assert.equal(await dialog.getByLabel('Name', { exact: true }).inputValue(), 'Confirmed draw')
    await capture('confirmed')
    await dialog.getByRole('button', { name: 'Cancel', exact: true }).click()
    await dialog.waitFor({ state: 'hidden' })
    assert.equal(await page.getByRole('button', { name: 'Edit Confirmed draw', exact: true }).isVisible(), true)
    assert.deepEqual(await page.evaluate(() => globalThis.__audit.drawWrites.map(write => write.id)), ['instant-audit'])
  })
  for (const action of ['game', 'raffle', 'publish']) {
    await scenario(size, 'admin-activity-' + action + '-refresh-error', 'lucky-admin-audit.html?after-action-read-error&dark', async (page, capture) => {
      if (action !== 'game') await page.getByRole('tab', { name: 'Lucky draws', exact: true }).click()
      const name = action === 'game' ? 'Retained coin' : 'Community weekend draw'
      if (action === 'publish') await page.getByRole('button', { name: 'Publish to group', exact: true }).click()
      else {
        await page.getByRole('button', { name: 'Remove ' + name, exact: true }).click()
        await page.getByRole('dialog').getByRole('button', { name: 'Remove', exact: true }).click()
        await page.getByRole('dialog').waitFor({ state: 'hidden' })
      }
      await page.getByRole('alert').filter({ hasText: 'Activity configuration could not be loaded.' }).waitFor()
      if (action === 'game') assert.equal(await page.getByRole('button', { name: 'Edit Retained coin', exact: true }).count(), 0)
      else if (action === 'raffle') {
        await page.getByText('Cancelled', { exact: true }).waitFor()
        assert.equal(await page.getByRole('button', { name: 'Remove ' + name, exact: true }).isEnabled(), false)
      } else {
        await page.getByText('Publishing', { exact: true }).waitFor()
        assert.equal(await page.getByRole('button', { name: 'Publish to group', exact: true }).count(), 0)
      }
      await capture('confirmed-with-read-error')
      await page.evaluate(() => { globalThis.__audit.recover = true })
      await page.getByRole('button', { name: 'Refresh', exact: true }).click()
      await page.getByRole('alert').waitFor({ state: 'hidden' })
      const state = await page.evaluate(() => globalThis.__audit)
      assert.equal(action === 'publish' ? state.publications.length : state.deletes.length, 1)
    })
  }
  await scenario(size, 'admin-draw-forecast-retry', 'lucky-admin-audit.html?forecast-error&dark', async (page, capture) => {
    await page.getByRole('tab', { name: 'Lucky draws', exact: true }).click()
    await page.getByRole('button', { name: 'Edit Weekend rewards', exact: true }).click()
    const dialog = page.getByRole('dialog')
    await dialog.getByText('Forecast could not be loaded.', { exact: true }).waitFor()
    await capture('error')
    await page.evaluate(() => { globalThis.__audit.recover = true })
    await dialog.getByRole('button', { name: 'Try again', exact: true }).click()
    await dialog.locator('.draw-forecast__numbers').waitFor()
    assert.equal(await dialog.locator('.draw-forecast__numbers dd').first().innerText(), '125.00 TXB')
  })
}
