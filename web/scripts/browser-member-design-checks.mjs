import assert from 'node:assert/strict'

async function inspectReadableCopy(control, sampleKind, description) {
  await control.waitFor()
  const metrics = await control.evaluate((element, kind) => {
    const label = element.querySelector('[data-slot="label"]') ?? element
    const style = globalThis.getComputedStyle(label), text = label.textContent.trim()
    const sample = kind === 'word' ? text.split(/\s+/).sort((left, right) => right.length - left.length)[0] : text.slice(0, 12)
    const context = globalThis.document.createElement('canvas').getContext('2d')
    context.font = style.font || `${style.fontWeight} ${style.fontSize} ${style.fontFamily}`
    const width = label.clientWidth - parseFloat(style.paddingLeft) - parseFloat(style.paddingRight)
    return { text, width, required: context.measureText(sample).width, overflow: label.scrollWidth > label.clientWidth + 1 || label.scrollHeight > label.clientHeight + 1 }
  }, sampleKind)
  assert.ok(metrics.text.length > 0, `${description}: missing text`)
  assert.equal(metrics.overflow, false, `${description}: label text is clipped`)
  assert.ok(metrics.width + 1 >= metrics.required, `${description}: ${metrics.width.toFixed(1)}px cannot fit a readable ${metrics.required.toFixed(1)}px text sample`)
}

export async function inspectBetLabel(page) {
  await inspectReadableCopy(page.locator('.bet-form__submit'), 'word', 'Confirm bet')
}

export async function inspectHostName(page) {
  const name = page.locator('.host-link__name').filter({ hasText: 'Shared transit with a deliberately long host name' })
  await inspectReadableCopy(name, 'characters', 'Long subscription host name')
}

export async function inspectAllowanceLegend(page) {
  const rows = await page.locator('.hourly-allowance__legend > div').evaluateAll(elements => elements.map(element => {
    const term = element.querySelector('dt'), value = element.querySelector('dd')
    const bounds = node => { const range = globalThis.document.createRange(); range.selectNodeContents(node); return range.getBoundingClientRect() }
    const left = bounds(term), right = bounds(value)
    const touching = left.right + 2 > right.left && right.right + 2 > left.left && left.bottom + 2 > right.top && right.bottom + 2 > left.top
    return { term: term.textContent.trim(), value: value.textContent.trim(), touching }
  }))
  assert.ok(rows.length >= 2, 'allowance legend is missing')
  for (const row of rows) assert.equal(row.touching, false, `allowance legend crowds ${row.term} and ${row.value}`)
}

export async function settleCardEntry(page) {
  await page.evaluate(async () => {
    for (const animation of globalThis.document.getAnimations()) {
      const css = animation instanceof globalThis.CSSAnimation || animation instanceof globalThis.CSSTransition
      if (css && Number.isFinite(animation.effect?.getComputedTiming().endTime)) animation.finish()
    }
    await new Promise(resolve => globalThis.requestAnimationFrame(() => globalThis.requestAnimationFrame(resolve)))
  })
}
