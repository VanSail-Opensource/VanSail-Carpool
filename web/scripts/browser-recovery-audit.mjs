import assert from 'node:assert/strict'
import { mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { chromium } from 'playwright'
import { createServer } from 'vite'
import { memberRecoveryScenarios } from './browser-member-recovery.mjs'

// Hosted browser validation only; AGENTS.md prohibits running suites locally.
const output = resolve(globalThis.process.env.BROWSER_AUDIT_OUTPUT ?? 'browser-audit-artifacts')
await mkdir(output, { recursive: true })
const server = await createServer({ server: { host: '127.0.0.1', port: 4173, strictPort: true } })
const results = []
let browser
try {
  await server.listen()
  browser = await chromium.launch()
  async function scenario(size, name, path, inspect) {
    const context = await browser.newContext({ viewport: size.viewport, reducedMotion: 'reduce', colorScheme: 'dark' })
    const page = await context.newPage()
    page.setDefaultTimeout(30_000)
    const issues = []
    const label = `${size.name}-${name}`
    const result = { scenario: label, passed: false, issues }
    page.on('pageerror', error => issues.push(`pageerror: ${error.message}`))
    page.on('console', message => { if (message.type() === 'error') issues.push(`console: ${message.text()}`) })
    page.on('requestfailed', request => {
      if (request.failure()?.errorText !== 'net::ERR_ABORTED') issues.push(`request: ${request.url()} ${request.failure()?.errorText}`)
    })
    await page.route('**/*', route => {
      const url = new globalThis.URL(route.request().url())
      if (url.origin !== 'http://127.0.0.1:4173' || url.pathname.startsWith('/api/')) {
        issues.push(`unexpected network: ${url.href}`)
        return route.abort()
      }
      return route.continue()
    })
    const capture = suffix => page.screenshot({ path: resolve(output, `${label}-${suffix}.png`), fullPage: true })
    try {
      await page.goto(`http://127.0.0.1:4173/fixtures/${path}`, { waitUntil: 'domcontentloaded' })
      await inspect(page, capture)
      assert.equal(await page.evaluate(() => globalThis.document.documentElement.scrollWidth <= globalThis.innerWidth + 1), true, 'horizontal document overflow')
      await capture('complete')
      assert.deepEqual(issues, [], 'unexpected console or network failures')
      result.passed = true
    } catch (error) {
      result.error = error.stack ?? String(error)
      await capture('failed').catch(() => undefined)
    } finally {
      results.push(result)
      await writeFile(resolve(output, 'results.json'), JSON.stringify(results, null, 2))
      await context.close()
    }
  }
  for (const size of [
    { name: 'desktop', viewport: { width: 1440, height: 1000 } },
    { name: 'mobile', viewport: { width: 390, height: 844 } },
  ]) {
    await scenario(size, 'currency-race', 'recovery-audit.html?currency-slow', async (page, capture) => {
      await page.locator('[data-test="currency-value"]').waitFor()
      assert.equal(await page.locator('[data-test="currency-loading"]').innerText(), 'true')
      await capture('loading')
      await page.getByRole('button', { name: 'Currency', exact: true }).click()
      await page.getByRole('button', { name: 'USD', exact: true }).click()
      await page.waitForFunction(() => globalThis.__recoveryAudit.snapshot().currency === 'USD')
      await page.evaluate(() => globalThis.__recoveryAudit.releaseCurrency('CNY'))
      assert.equal(await page.locator('[data-test="currency-value"]').innerText(), 'USD')
    })
    await scenario(size, 'currency-owner', 'recovery-audit.html?currency-slow', async (page, capture) => {
      await page.locator('[data-test="currency-value"]').waitFor()
      await page.evaluate(() => globalThis.__recoveryAudit.switchOwner())
      await page.waitForFunction(() => globalThis.__recoveryAudit.snapshot().reads === 2)
      await page.evaluate(() => globalThis.__recoveryAudit.releaseCurrency('CNY'))
      assert.equal(await page.locator('[data-test="currency-value"]').innerText(), 'USD')
      assert.equal(await page.locator('[data-test="currency-loading"]').innerText(), 'true')
      await page.evaluate(() => globalThis.__recoveryAudit.releaseCurrency('USD'))
      await page.waitForFunction(() => !globalThis.__recoveryAudit.snapshot().loading)
      await capture('new-owner')
      await page.locator('[data-test="refresh-currency"]').click()
      await page.waitForFunction(() => globalThis.__recoveryAudit.snapshot().loading)
      await page.locator('[data-test="clear-session"]').click()
      await page.waitForFunction(() => globalThis.__recoveryAudit.snapshot().user === null)
      await page.evaluate(() => globalThis.__recoveryAudit.releaseCurrency('CNY'))
      const value = await page.evaluate(() => globalThis.__recoveryAudit.snapshot())
      assert.equal(value.currency, 'TXB')
      assert.deepEqual(value.rates, { cnyTxbPerUnit: null, usdTxbPerUnit: null })
    })
    await scenario(size, 'currency-error', 'recovery-audit.html?currency-error', async (page, capture) => {
      await page.getByRole('button', { name: 'Currency', exact: true }).click()
      await page.getByRole('button', { name: 'USD', exact: true }).click()
      await page.getByText('This currency rate is unavailable right now.', { exact: true }).waitFor()
      assert.equal(await page.locator('[data-test="currency-value"]').innerText(), 'TXB')
      await page.waitForFunction(() => {
        const bounds = globalThis.document.querySelector('.currency-control__menu')?.getBoundingClientRect()
        return bounds && bounds.left >= 8 && bounds.right <= globalThis.innerWidth - 8 && bounds.top >= 8 && bounds.bottom <= globalThis.innerHeight - 8
      })
      const errorFits = await page.locator('.currency-control__error').evaluate(element => element.scrollWidth <= element.clientWidth)
      assert.equal(errorFits, true, 'currency error text is clipped within the popover')
      await capture('error')
      await page.evaluate(() => { globalThis.__recoveryAudit.state.recover = true })
      await page.getByRole('button', { name: 'USD', exact: true }).click()
      await page.waitForFunction(() => globalThis.__recoveryAudit.snapshot().currency === 'USD')
    })
    for (const outcome of ['failed', 'succeeded']) {
      await scenario(size, `cancel-${outcome}`, `recovery-audit.html?cancel-${outcome}`, async (page, capture) => {
        await page.locator('[data-test="open-checkout"]').click()
        await page.getByText('Waiting for payment', { exact: true }).waitFor()
        await page.getByRole('button', { name: 'Cancel order', exact: true }).click()
        await page.getByText('Cancelling order', { exact: true }).waitFor()
        await capture('cancelling')
        await page.getByText('Waiting for payment', { exact: true }).waitFor()
        assert.equal(await page.getByRole('button', { name: 'Cancel order', exact: true }).isEnabled(), true)
        assert.equal(await page.evaluate(() => globalThis.__recoveryAudit.snapshot().cancels), 1)
        await capture('pending')
        await page.evaluate(() => globalThis.__recoveryAudit.pay())
        await page.getByRole('dialog').waitFor({ state: 'hidden' })
        assert.ok(await page.evaluate(() => globalThis.__recoveryAudit.snapshot().orderReads >= 2), 'pending checkout did not resume authoritative polling')
      })
    }
    await scenario(size, 'payment-empty', 'recovery-audit.html?empty', async (page, capture) => {
      await page.locator('[data-test="open-checkout"]').click()
      await page.getByText('No payment methods are available right now. Please try again later.', { exact: true }).waitFor()
      assert.equal(await page.locator('[data-test="choose-channel"]').count(), 0)
      assert.equal(await page.locator('.provider-option').count(), 0)
      const close = page.getByRole('dialog').getByRole('button', { name: 'Close', exact: true })
      assert.equal(await close.isVisible(), true)
      await capture('empty')
      await close.click()
      await page.getByRole('dialog').waitFor({ state: 'hidden' })
    })
    await scenario(size, 'auth-recovery', 'browser-auth-audit.html?exchange&invalid&slow=1500#token=constructed-invalid', async (page, capture) => {
      await page.getByText('Signing you in', { exact: true }).waitFor()
      await capture('loading')
      await page.getByText('Generate a new access link', { exact: true }).waitFor()
      assert.equal(new globalThis.URL(page.url()).hash, '')
      assert.equal(await page.evaluate(() => globalThis.__browserAuthAudit.exchanges), 1)
      assert.equal(await page.getByRole('link', { name: 'Back to entrance', exact: true }).isVisible(), true)
    })
    await scenario(size, 'auth-success', 'browser-auth-audit.html?exchange#token=constructed-success', async page => {
      await page.getByText('Admin console', { exact: true }).waitFor()
      assert.equal(new globalThis.URL(page.url()).pathname, '/audit-complete')
      assert.equal(new globalThis.URL(page.url()).hash, '')
      assert.equal(await page.evaluate(() => globalThis.__browserAuthAudit.exchanges), 1)
    })
    await scenario(size, 'auth-service-retry', 'browser-auth-audit.html?exchange&unavailable#token=constructed-retry', async (page, capture) => {
      await page.getByText('Sign-in is temporarily unavailable. Please try again.', { exact: true }).waitFor()
      assert.equal(new globalThis.URL(page.url()).hash, '')
      assert.equal(await page.evaluate(() => globalThis.__browserAuthAudit.exchanges), 1)
      assert.equal(await page.getByText('Generate a new access link', { exact: true }).count(), 0)
      await capture('service-error')
      await page.getByRole('button', { name: 'Try again', exact: true }).click()
      await page.getByText('Admin console', { exact: true }).waitFor()
      assert.equal(new globalThis.URL(page.url()).pathname, '/audit-complete')
      assert.equal(await page.evaluate(() => globalThis.__browserAuthAudit.exchanges), 2)
    })
    await memberRecoveryScenarios(scenario, size)
  }
  globalThis.console.log(JSON.stringify(results, null, 2))
  if (results.some(result => !result.passed)) globalThis.process.exitCode = 1
} finally {
  await browser?.close()
  await server.close()
}
