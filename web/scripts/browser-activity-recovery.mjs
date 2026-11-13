import assert from 'node:assert/strict'

export async function activityRecoveryScenarios(scenario, size) {
  await scenario(size, 'activity-disabled-game', 'lucky-audit.html?bet&progress', async (page, capture) => {
    await page.getByRole('button', { name: 'Confirm bet', exact: true }).waitFor()
    const progress = page.getByRole('progressbar', { name: 'Group-message reward progress', exact: true })
    assert.equal(await progress.getAttribute('aria-valuenow'), '5')
    assert.equal(await progress.getAttribute('aria-valuemax'), '5')
    await capture('available')
    await page.evaluate(() => { globalThis.__audit.overview.games[0].enabled = false })
    await page.getByRole('button', { name: 'Check again', exact: true }).click()
    await page.getByText('No bet games', { exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: 'Confirm bet', exact: true }).count(), 0)
    assert.equal(await page.evaluate(() => globalThis.__audit.bets), 0)
  })
  await scenario(size, 'activity-bet-retry', 'lucky-audit.html?bet', async (page, capture) => {
    const confirm = page.getByRole('button', { name: 'Confirm bet', exact: true })
    await confirm.waitFor()
    await page.evaluate(() => { globalThis.__audit.failNext = true })
    await confirm.click()
    await page.getByRole('button', { name: 'Resolving securely', exact: true }).waitFor()
    await capture('pending')
    await page.getByText('The activity could not be completed.', { exact: true }).waitFor()
    await capture('error')
    await confirm.click()
    await page.getByRole('dialog').waitFor()
    const state = await page.evaluate(() => ({ bets: globalThis.__audit.bets, keys: globalThis.__audit.betKeys }))
    assert.equal(state.bets, 2)
    assert.equal(state.keys.length, 2)
    assert.ok(state.keys[0])
    assert.equal(state.keys[0], state.keys[1], 'uncertain bet retry changed its idempotency key')
  })
}
