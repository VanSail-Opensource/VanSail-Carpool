import { afterEach, describe, expect, it } from 'vitest'
import { clearCatalogDraft, readCatalogDraft, writeCatalogDraft } from './catalogDraft'

afterEach(() => sessionStorage.clear())

describe('catalog draft recovery', () => {
  it.each(['null', '[]', '"draft"', '{"squadIds":"old-format"}', '{"squadIds":[null]}', '{"comboId":42}', '{'])('ignores malformed persisted input %s', raw => {
    sessionStorage.setItem('txc-catalog-draft:member-1', raw)
    expect(readCatalogDraft('member-1')).toBeNull()
  })

  it('restores a valid draft only for its owning account', () => {
    const draft = { comboId: 'combo-1', squadIds: ['squad-1'], couponGrantId: null }
    writeCatalogDraft('member-1', draft)
    expect(readCatalogDraft('member-1')).toEqual(draft)
    expect(readCatalogDraft('member-2')).toBeNull()
    clearCatalogDraft('member-1')
    expect(readCatalogDraft('member-1')).toBeNull()
  })
})
