import assert from 'node:assert/strict'

export async function receiptRecoveryScenarios(scenario, size) {
  for (const status of ['pending', 'failed', 'refunded', 'paid']) {
    await scenario(size, `receipt-${status}`, `payment-return-audit.html?crypto&${status}`, async (page, capture) => {
      const label = status === 'paid' ? 'Balance credited' : 'Order value in TXB'
      await page.getByText('2.000001 USDT', { exact: true }).waitFor()
      assert.equal(await page.locator('.payment-receipt__label').innerText(), label)
      assert.equal(await page.getByText('Crypto amount', { exact: true }).isVisible(), true)
      assert.equal(await page.getByText('Settled amount', { exact: true }).count(), 0)
      if (status !== 'paid') assert.equal(await page.getByText('Balance credited', { exact: true }).count(), 0)
      await capture(status)
    })
  }
}
