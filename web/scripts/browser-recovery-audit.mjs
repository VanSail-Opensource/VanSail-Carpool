import assert from 'node:assert/strict'
import { mkdir, writeFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { chromium } from 'playwright'
import { createServer } from 'vite'
import { browserRecoverySuites, parseBrowserScopes } from './browser-recovery-scopes.mjs'

// Hosted PR browser validation only; AGENTS.md prohibits local automated suites.
if (globalThis.process.env.GITHUB_EVENT_NAME !== 'pull_request') throw new Error('Browser recovery is pull-request-only')
const scopes = parseBrowserScopes(globalThis.process.env.BROWSER_AUDIT_SCOPES)
const output = resolve(globalThis.process.env.BROWSER_AUDIT_OUTPUT ?? 'browser-audit-artifacts')
await mkdir(output, { recursive: true })
await writeFile(resolve(output, 'scope.json'), JSON.stringify({
  pullRequest: globalThis.process.env.BROWSER_AUDIT_PR,
  commit: globalThis.process.env.GITHUB_SHA,
  scopes,
}, null, 2))
const server = await createServer({ server: { host: '127.0.0.1', port: 4173, strictPort: true } })
const results = []
let browser
try {
  await server.listen()
  // Hosted runners have no hardware GPU; exercise actual WebGL with a stable
  // software renderer. Fixture requests remain restricted to the local server.
  browser = await chromium.launch({ args: ['--use-gl=angle', '--use-angle=swiftshader', '--enable-unsafe-swiftshader'] })
  async function scenario(scope, size, name, path, inspect) {
    const context = await browser.newContext({ viewport: size.viewport, reducedMotion: size.reducedMotion ?? 'reduce', colorScheme: 'dark', hasTouch: size.name.startsWith('mobile') })
    const page = await context.newPage()
    page.setDefaultTimeout(30_000)
    const issues = []
    const label = `${size.name}-${name}`
    const result = { scope, scenario: label, passed: false, issues }
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
    for (const scope of scopes) {
      const suite = browserRecoverySuites[scope]
      const module = await import(suite.module)
      const before = results.length
      await module[suite.export]((...args) => scenario(scope, ...args), size)
      assert.ok(results.length > before, `${scope} did not execute a ${size.name} scenario`)
    }
  }
  globalThis.console.log(JSON.stringify(results, null, 2))
  if (results.some(result => !result.passed)) globalThis.process.exitCode = 1
} finally {
  await browser?.close()
  await server.close()
}
