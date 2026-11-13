import { defineComponent, h, type ShallowRef } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { featuresApi, type DatabaseMutationInput, type DatabaseMutationReview, type DatabaseMutationResult, type DatabaseRowsPage, type DatabaseTable } from '@/api/features'
import { t } from '@/i18n'
import { useAdminDatabase } from './useAdminDatabase'

const cache = vi.hoisted(() => ({ restoreItems: vi.fn(), restoreCached: vi.fn() }))
vi.mock('@/api/cache/restore', () => cache)
const tables: DatabaseTable[] = ['first', 'second'].map(name => ({ name, columns: [], highRisk: false, warning: '' }))
const page: DatabaseRowsPage = { items: [{ key: { id: 'one' }, values: { label: 'Before' }, recordHash: 'before' }], nextCursor: null }
const input: DatabaseMutationInput = { action: 'update', table: 'first', key: { id: 'one' }, recordHash: 'before', values: { label: 'Reviewed' }, reason: 'Repair the label' }
const result: DatabaseMutationResult = { row: { key: { id: 'one' }, values: { label: 'Reviewed' }, recordHash: 'after' }, deleted: false, rescueBackupId: 'verified-rescue' }
function reviewFor(body: DatabaseMutationInput): DatabaseMutationReview {
  return { action: body.action, table: body.table, key: body.key, before: page.items[0]!.values, after: body.values, changedColumns: Object.keys(body.values ?? {}), reviewHash: 'reviewed-input', requiredConfirmation: 'EDIT first', rescueBackupRequired: true, warning: '' }
}
function setup() {
  let state!: ReturnType<typeof useAdminDatabase>
  const wrapper = mount(defineComponent({ setup() { state = useAdminDatabase(); return () => h('div') } }))
  return { state, wrapper }
}
beforeEach(() => {
  cache.restoreItems.mockReset().mockReturnValue(false)
  cache.restoreCached.mockReset().mockReturnValue(false)
  vi.spyOn(featuresApi, 'getDatabaseTables').mockResolvedValue({ items: tables })
  vi.spyOn(featuresApi, 'queryDatabaseRows').mockResolvedValue(page)
  vi.spyOn(featuresApi, 'reviewDatabaseMutation').mockImplementation(async body => reviewFor(body))
  vi.spyOn(featuresApi, 'applyDatabaseMutation').mockResolvedValue(result)
})
afterEach(() => vi.restoreAllMocks())

describe('reviewed database mutation lifetime', () => {
  it('does not start a row request from metadata that resolves after disposal', async () => {
    let release!: (value: { items: DatabaseTable[] }) => void
    vi.mocked(featuresApi.getDatabaseTables).mockReturnValueOnce(new Promise(done => { release = done }))
    const { state, wrapper } = setup()
    wrapper.unmount()
    release({ items: tables })
    await flushPromises()
    await state.loadTables()
    expect(featuresApi.getDatabaseTables).toHaveBeenCalledOnce()
    expect(featuresApi.queryDatabaseRows).not.toHaveBeenCalled()
  })

  it('retires cached metadata refresh before a reviewed mutation starts', async () => {
    cache.restoreItems.mockImplementation((_path: string, target: ShallowRef<DatabaseTable[]>) => { target.value = tables; return true })
    cache.restoreCached.mockImplementation((_path: string, apply: (value: DatabaseRowsPage) => void) => { apply(page); return true })
    let release!: (value: { items: DatabaseTable[] }) => void
    vi.mocked(featuresApi.getDatabaseTables).mockReturnValueOnce(new Promise(done => { release = done }))
    const { state, wrapper } = setup()
    expect(await state.reviewMutation(input)).toBe(true)
    release({ items: [tables[1]!] })
    await flushPromises()
    expect(state.selectedTableName.value).toBe('first')
    expect(state.review.value?.table).toBe('first')
    expect(featuresApi.queryDatabaseRows).not.toHaveBeenCalled()
    expect(await state.applyMutation(input, 'EDIT first')).toBe(true)
    expect(state.rows.value[0]?.recordHash).toBe('after')
    wrapper.unmount()
  })

  it('discards a delayed review after draft invalidation and blocks duplicate reviews', async () => {
    let release!: (value: DatabaseMutationReview) => void
    vi.mocked(featuresApi.reviewDatabaseMutation).mockReturnValueOnce(new Promise(done => { release = done }))
    const { state, wrapper } = setup()
    await flushPromises()
    const pending = state.reviewMutation(input)
    expect(await state.reviewMutation(input)).toBe(false)
    state.clearReview()
    release(reviewFor(input))
    expect(await pending).toBe(false)
    expect(state.review.value).toBeNull()
    expect(state.busy.value).toBe(false)
    expect(await state.applyMutation(input, 'EDIT first')).toBe(false)
    expect(featuresApi.applyDatabaseMutation).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('snapshots exact reviewed values and refuses a different draft before sending Apply', async () => {
    const { state, wrapper } = setup()
    await flushPromises()
    const draft = { ...input, values: { label: 'Reviewed', amount: '900719925474099312345' } }
    const pending = state.reviewMutation(draft)
    draft.values.amount = '1'
    expect(await pending).toBe(true)
    expect(featuresApi.reviewDatabaseMutation).toHaveBeenCalledWith(expect.objectContaining({ values: { label: 'Reviewed', amount: '900719925474099312345' } }))
    expect(await state.applyMutation(draft, 'EDIT first')).toBe(false)
    expect(featuresApi.applyDatabaseMutation).not.toHaveBeenCalled()
    expect(state.review.value).toBeNull()
    expect(state.error.value).toBe(t('errors.mutationReview'))
    wrapper.unmount()
  })

  it('locks table selection and duplicate Apply, then retains its returned row through read failure', async () => {
    let release!: (value: DatabaseMutationResult) => void
    vi.mocked(featuresApi.applyDatabaseMutation).mockReturnValueOnce(new Promise(done => { release = done }))
    const { state, wrapper } = setup()
    await flushPromises()
    await state.reviewMutation(input)
    const pending = state.applyMutation(input, 'EDIT first')
    expect(await state.applyMutation(input, 'EDIT first')).toBe(false)
    await state.selectTable('second')
    await state.loadTables()
    expect(state.selectedTableName.value).toBe('first')
    expect(featuresApi.getDatabaseTables).toHaveBeenCalledOnce()
    release(result)
    expect(await pending).toBe(true)
    expect(featuresApi.applyDatabaseMutation).toHaveBeenCalledExactlyOnceWith({ ...input, reviewHash: 'reviewed-input', confirmation: 'EDIT first' })
    expect(state.lastRescueBackupId.value).toBe('verified-rescue')
    cache.restoreCached.mockImplementation((_path: string, apply: (value: DatabaseRowsPage) => void) => { apply(page); return true })
    vi.mocked(featuresApi.queryDatabaseRows).mockRejectedValueOnce(new Error('read unavailable'))
    await state.queryRows('first', { filters: [], limit: 50 })
    expect(state.rows.value[0]?.recordHash).toBe('after')
    expect(state.error.value).toBe(t('errors.databaseRows'))
    wrapper.unmount()
  })

  it('requires the reviewed confirmation and does not append a page without a cursor', async () => {
    const { state, wrapper } = setup()
    await flushPromises()
    await state.reviewMutation(input)
    expect(await state.applyMutation(input, 'EDIT second')).toBe(false)
    expect(featuresApi.applyDatabaseMutation).not.toHaveBeenCalled()
    await state.queryRows('first', { filters: [], limit: 50 }, { append: true })
    expect(featuresApi.queryDatabaseRows).toHaveBeenCalledOnce()
    wrapper.unmount()
  })

  it('retires a pending Apply without presenting it as another workspace result', async () => {
    let release!: (value: DatabaseMutationResult) => void
    vi.mocked(featuresApi.applyDatabaseMutation).mockReturnValueOnce(new Promise(done => { release = done }))
    const { state, wrapper } = setup()
    await flushPromises()
    await state.reviewMutation(input)
    const pending = state.applyMutation(input, 'EDIT first')
    wrapper.unmount()
    release(result)
    expect(await pending).toBe(false)
    expect(state.rows.value).toEqual(page.items)
    expect(state.lastRescueBackupId.value).toBeNull()
    expect(state.review.value).toBeNull()
    await state.loadRows('first')
    expect(featuresApi.queryDatabaseRows).toHaveBeenCalledOnce()
  })
})
