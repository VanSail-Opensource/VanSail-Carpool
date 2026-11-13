import assert from 'node:assert/strict'

export async function memberRecoveryScenarios(scenario, size) {
  await scenario(size, 'onboarding-intro-retry', 'onboarding-audit.html?intro&advance-error', async (page, capture) => {
    await page.getByText('Something went wrong. Please try again.', { exact: true }).waitFor()
    assert.equal(await page.evaluate(() => globalThis.__onboardingAudit.calls.filter(call => call === 'continue').length), 1)
    await capture('advance-error')
    await page.getByRole('button', { name: 'Try again', exact: true }).click()
    await page.getByRole('checkbox').waitFor()
    assert.equal(await page.evaluate(() => globalThis.__onboardingAudit.calls.filter(call => call === 'continue').length), 2)
  })
  await scenario(size, 'onboarding-revision-consent', 'onboarding-audit.html?slow', async (page, capture) => {
    await page.getByRole('progressbar').waitFor()
    await capture('loading')
    const checkbox = page.getByRole('checkbox')
    await checkbox.check()
    await capture('reviewed')
    await page.evaluate(() => { globalThis.__onboardingAudit.revision++; globalThis.__onboardingAudit.locale('zh-CN') })
    await page.waitForFunction(() => globalThis.__onboardingAudit.calls.filter(call => call === 'content').length === 2)
    await page.waitForFunction(() => globalThis.document.querySelector('[role="checkbox"]')?.getAttribute('aria-checked') === 'false')
    assert.equal(await checkbox.isChecked(), false)
    assert.deepEqual(await page.evaluate(() => globalThis.__onboardingAudit.writes), [])
  })
  await scenario(size, 'onboarding-owner', 'onboarding-audit.html?intro&defer-advance', async page => {
    await page.waitForFunction(() => globalThis.__onboardingAudit?.advancePending())
    await page.evaluate(() => { globalThis.__onboardingAudit.clearOwner(); globalThis.__onboardingAudit.releaseAdvance() })
    await page.waitForFunction(() => globalThis.__onboardingAudit.user() === null)
    assert.equal(await page.getByRole('checkbox').count(), 0)
    assert.equal(new globalThis.URL(page.url()).pathname, '/fixtures/onboarding-audit.html')
  })
  await scenario(size, 'community-loading', 'community-audit.html?slow', async (page, capture) => {
    await page.getByText('Checking community access', { exact: true }).waitFor()
    await capture('loading')
    await page.getByRole('button', { name: 'Join', exact: true }).first().waitFor()
    assert.equal(await page.getByRole('button', { name: 'Join', exact: true }).count(), 2)
  })
  await scenario(size, 'community-error', 'community-audit.html?error', async page => {
    await page.getByText('We could not refresh community access right now.', { exact: true }).waitFor()
    assert.equal(await page.getByRole('button', { name: 'Join', exact: true }).count(), 0)
    assert.equal(await page.getByRole('button', { name: 'Refresh', exact: true }).isEnabled(), true)
  })
  await scenario(size, 'community-membership-owner', 'community-audit.html?defer-membership', async page => {
    await page.getByText('Checking community access', { exact: true }).waitFor()
    await page.evaluate(() => { globalThis.__communityAudit.clearOwner(); globalThis.__communityAudit.releaseMembership() })
    await page.waitForFunction(() => globalThis.__communityAudit.state.membershipFinished === 1)
    assert.equal(await page.evaluate(() => globalThis.__communityAudit.user()), null)
    assert.equal(await page.getByRole('button', { name: 'Join', exact: true }).count(), 0)
  })
  await scenario(size, 'community-invite-owner', 'community-audit.html?defer-invite', async (page, capture) => {
    await page.getByRole('button', { name: 'Join', exact: true }).first().click()
    await page.getByRole('button', { name: 'Opening', exact: true }).waitFor()
    await capture('invite-pending')
    await page.evaluate(() => { globalThis.__communityAudit.clearOwner(); globalThis.__communityAudit.releaseInvite() })
    await page.waitForFunction(() => globalThis.__communityAudit.state.inviteFinished === 1)
    assert.equal(await page.evaluate(() => globalThis.__communityAudit.user()), null)
    assert.deepEqual(await page.evaluate(() => globalThis.__communityAudit.state.opened), [])
  })
}
