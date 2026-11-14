import assert from 'node:assert/strict'
import {
  topics, lessonSources, chapterCounts, player, calls, frames, settle, settleShell,
  open, close, inspectDesktopKnowledgeLabel, inspectTransport, inspectVisibleChapter, inspectSources,
} from './browser-knowledgebase-helpers.mjs'
import { knowledgebaseRenderingScenarios } from './browser-knowledgebase-rendering.mjs'

function inspectLessonCopy(copy, id, locale) {
  assert.doesNotMatch(copy, /knowledge(?:Lessons)?\./)
  if (id === 'subscription') {
    for (const pattern of locale === 'zh'
      ? [/余额/, /小时/, /流量/, /路由/, /FlClash/, /v2rayN/, /v2rayNG/, /Mihomo/, /Xray/, /订阅/, /节点/, /连接/]
      : [/balance/i, /hour/i, /traffic/i, /rout/i, /FlClash/, /v2rayN/, /v2rayNG/, /Mihomo/, /Xray/, /subscription/i, /node/i, /connect/i]) assert.match(copy, pattern)
  }
  if (id === 'payment') {
    for (const pattern of locale === 'zh'
      ? [/Gas|手续费/, /网络/, /金额/, /私钥|助记词/]
      : [/gas|fee/i, /network/i, /amount/i, /seed|private key/i]) assert.match(copy, pattern)
  }
  if (id === 'sharing') assert.match(copy, locale === 'zh' ? /风险|风控/ : /risk/i)
}

export async function knowledgebaseScenarios(scenario, size) {
  for (const locale of ['en', 'zh']) {
    await scenario(size, `knowledgebase-all-${locale}`, `knowledgebase-audit.html${locale === 'zh' ? '?zh' : ''}`, async (page, capture) => {
      await settle(page)
      assert.match(await page.locator('html').getAttribute('lang'), locale === 'zh' ? /^zh/ : /^en/)
      await inspectDesktopKnowledgeLabel(page, size)
      await capture('overview')
      const before = await calls(page)
      for (const id of topics) {
        const trigger = await open(page, id)
        assert.equal(await player(page).getAttribute('data-demo-playing'), 'false', 'reduced motion started automatic playback')
        assert.equal(await page.locator('.demo-pointer').count(), 0)
        const chapters = page.locator('[data-demo-step]')
        const count = await chapters.count()
        assert.equal(count, chapterCounts[id], `${id}: missing tutorial chapters`)
        const copy = []
        for (let index = 0; index < count; index++) {
          await chapters.nth(index).click()
          assert.equal(await chapters.nth(index).getAttribute('aria-current'), 'step')
          assert.equal(await player(page).getAttribute('data-demo-playing'), 'false')
          await inspectTransport(page, index, (index + .92) / count)
          copy.push(await inspectVisibleChapter(page, locale))
          if (id === 'subscription') await capture(`subscription-step-${index + 1}`)
          if (id === 'payment' && [2, 4].includes(index)) await capture(`payment-${index === 2 ? 'wallet-gas' : 'exchange-fee'}`)
        }
        inspectLessonCopy(copy.join('\n'), id, locale)
        if (Object.hasOwn(lessonSources, id)) await inspectSources(page, id)
        await capture(`${locale}-${id}`)
        await close(page, trigger)
      }
      assert.deepEqual(await calls(page), before, 'lesson playback called a member API')
      assert.deepEqual(await page.evaluate(() => globalThis.__knowledgebaseAudit.unexpected), [])
      assert.equal(await page.locator('.knowledge-card li').count(), topics.length * 3)
      assert.doesNotMatch(await page.locator('.page--knowledgebase').innerText(), /knowledge(?:Lessons)?\./)
    })
  }
  await scenario({ ...size, reducedMotion: 'no-preference' }, 'knowledgebase-controls', 'knowledgebase-audit.html', async (page, capture) => {
    await settle(page)
    const before = await calls(page)
    const trigger = await open(page, 'payment')
    const seek = page.locator('.demo-transport__seek [role="slider"]')
    await page.waitForFunction(() => Number(globalThis.document.querySelector('.demo-transport__seek [role="slider"]')?.getAttribute('aria-valuenow')) > 30)
    await page.getByRole('button', { name: 'Pause demonstration', exact: true }).click()
    assert.equal(await player(page).getAttribute('data-demo-playing'), 'false')
    const paused = await seek.getAttribute('aria-valuenow')
    await frames(page)
    assert.equal(await seek.getAttribute('aria-valuenow'), paused)
    await seek.focus(); await seek.press('End')
    assert.equal(await seek.getAttribute('aria-valuenow'), '1000')
    assert.equal(await page.locator('[data-demo-step="7"]').getAttribute('aria-current'), 'step')
    await inspectTransport(page, 7, 1)
    await inspectVisibleChapter(page, 'en')
    await capture('seek-end')
    await page.getByRole('button', { name: 'Replay demonstration', exact: true }).click()
    assert.equal(await player(page).getAttribute('data-demo-playing'), 'true')
    await page.getByRole('button', { name: 'Pause demonstration', exact: true }).click()
    await seek.focus(); await seek.press('Home')
    assert.equal(await seek.getAttribute('aria-valuenow'), '0')
    await close(page, trigger)
    assert.deepEqual(await calls(page), before, 'playback controls called a member API')
  })
  for (const locale of ['en', 'zh']) {
    await scenario(size, `knowledgebase-new-member-${locale}`, `knowledgebase-audit.html?mode=home&new-user${locale === 'zh' ? '&zh' : ''}`, async (page, capture) => {
      await page.locator('.knowledge-home').waitFor()
      await settleShell(page)
      const entrance = page.locator('.knowledge-home a[href="/knowledgebase"]')
      assert.equal(await entrance.isVisible(), true)
      assert.match(await entrance.innerText(), locale === 'zh' ? /教程/ : /tutorial/i)
      assert.match(await page.locator('.knowledge-home').innerText(), locale === 'zh' ? /新手|初次/ : /new/i)
      assert.equal(await page.locator('.side-rail__footer').count(), 0, 'new member received privileged shortcuts')
      if (size.name === 'desktop') assert.equal(await page.locator('.side-rail a[href="/knowledgebase"]').isVisible(), true)
      else assert.equal(await page.locator('.home-around').isVisible(), true)
      await inspectDesktopKnowledgeLabel(page, size)
      await capture('home-entrance')
      await entrance.focus(); await entrance.press('Enter')
      await settle(page)
      assert.equal(new globalThis.URL(page.url()).pathname, '/knowledgebase')
      if (size.name === 'mobile') assert.equal(await page.locator('.bottom-nav__item').count(), 2)
    })
  }
  await knowledgebaseRenderingScenarios(scenario, size)
}
