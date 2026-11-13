import assert from 'node:assert/strict'

async function visibleFeedback(page, dialog, feedback, save) {
  await feedback.waitFor()
  const [modal, error, action] = await Promise.all([dialog.boundingBox(), feedback.boundingBox(), save.boundingBox()])
  assert.ok(modal && error && action)
  for (const bounds of [error, action]) {
    assert.ok(bounds.x >= modal.x && bounds.x + bounds.width <= modal.x + modal.width + 1, 'save feedback/action exceeds modal width')
    assert.ok(bounds.y >= Math.max(0, modal.y) && bounds.y + bounds.height <= Math.min(page.viewportSize().height, modal.y + modal.height) + 1, 'save feedback/action is outside the visible modal viewport')
  }
}

export async function xnodeCreationScenarios(scenario, size) {
  for (const mode of ['configuration', 'snippet']) {
    await scenario(size, `xnode-${mode}-creation-retry`, `xnode-audit.html?xnodeTab=configurations&creation-retry=${mode}`, async (page, capture) => {
      const add = mode === 'configuration' ? 'Add configuration' : 'Add snippet'
      await page.getByRole('button', { name: add, exact: true }).click()
      const dialog = page.getByRole('dialog')
      const name = dialog.getByLabel('Name', { exact: true })
      const save = dialog.getByRole('button', { name: 'Save', exact: true })
      await name.fill('Original creation')
      await save.click()
      await page.waitForFunction(() => globalThis.__catalogCreation.attempts.length === 1)
      const feedback = dialog.getByRole('alert')
      await feedback.filter({ hasText: 'Could not save these changes. Check the input and retry.' }).waitFor()
      await visibleFeedback(page, dialog, feedback, save)
      await capture('uncertain-create-retained')
      await name.fill('Changed after uncertainty')
      await name.press('Enter')
      await page.waitForFunction(() => globalThis.__catalogCreation.attempts.length === 2)
      await page.waitForFunction(() => !globalThis.document.querySelector('[role="dialog"] button[type="submit"]')?.disabled)
      const changed = await page.evaluate(() => globalThis.__catalogCreation.attempts)
      assert.equal(changed[1].id, changed[0].id)
      assert.equal(changed[1].revision, 0)
      assert.equal(await dialog.isVisible(), true)
      await feedback.filter({ hasText: 'This resource is still in use or has changed. Refresh and try again.' }).waitFor()
      await visibleFeedback(page, dialog, feedback, save)
      await capture('conflict-retains-changed-draft')
      await name.fill('Original creation')
      await save.click()
      await dialog.waitFor({ state: 'hidden' })
      const recovered = await page.evaluate(() => globalThis.__catalogCreation)
      assert.equal(recovered.attempts.length, 3)
      assert.equal(recovered.attempts[2].id, recovered.attempts[0].id)
      assert.equal(recovered.records.filter(item => item.id === recovered.attempts[0].id).length, 1)
      assert.equal(await page.getByText('Original creation', { exact: true }).isVisible(), true)
      await capture('matching-create-recovered')
      await page.getByRole('button', { name: add, exact: true }).click()
      await dialog.getByLabel('Name', { exact: true }).fill('Next creation')
      await dialog.getByRole('button', { name: 'Save', exact: true }).click()
      await dialog.waitFor({ state: 'hidden' })
      const final = await page.evaluate(() => globalThis.__catalogCreation.attempts)
      assert.notEqual(final[3].id, final[0].id)
    })
  }
}
