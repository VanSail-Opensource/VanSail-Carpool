import { defineComponent, h } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { featuresApi, type DatabaseRowsPage } from '@/api/features'
import { clearResponseCache } from '@/api/cache/session'
import { useAdminDatabase } from './useAdminDatabase'

const page = (id: string): DatabaseRowsPage => ({ items: [{ key: { id }, values: { id }, recordHash: id }], nextCursor: null })
function pendingPage() {
  let resolve!: (value: DatabaseRowsPage) => void
  const promise = new Promise<DatabaseRowsPage>(done => { resolve = done })
  return { promise, resolve }
}
function setup() {
  let state!: ReturnType<typeof useAdminDatabase>
  const wrapper = mount(defineComponent({ setup() { state = useAdminDatabase(); return () => h('div') } }))
  return { state, wrapper }
}

beforeEach(() => {
  clearResponseCache()
  vi.spyOn(featuresApi, 'getDatabaseTables').mockResolvedValue({ items: ['first', 'second'].map(name => ({ name, columns: [], highRisk: false, warning: '' })) })
  vi.spyOn(featuresApi, 'queryDatabaseRows').mockResolvedValue(page('initial'))
})
afterEach(() => vi.restoreAllMocks())

describe('database query recovery', () => {
  it('runs a new filter while the previous request is pending and ignores the older response', async () => {
    const { state, wrapper } = setup()
    await flushPromises()
    const first = pendingPage(), second = pendingPage()
    vi.mocked(featuresApi.queryDatabaseRows).mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise)
    const older = state.queryRows('first', { search: 'older', filters: [], limit: 50 })
    const latest = state.queryRows('first', { search: 'latest', filters: [], limit: 50 })
    expect(featuresApi.queryDatabaseRows).toHaveBeenLastCalledWith('first', expect.objectContaining({ search: 'latest' }), expect.any(AbortSignal))
    second.resolve(page('latest'))
    await latest
    first.resolve(page('older'))
    await older
    expect(state.rows.value[0]?.recordHash).toBe('latest')
    expect(state.busy.value).toBe(false)
    wrapper.unmount()
  })

  it('does not apply a previous table response after switching tables', async () => {
    const { state, wrapper } = setup()
    await flushPromises()
    const first = pendingPage()
    vi.mocked(featuresApi.queryDatabaseRows).mockReturnValueOnce(first.promise).mockResolvedValueOnce(page('second'))
    const older = state.queryRows('first', { filters: [], limit: 50 })
    await state.selectTable('second')
    first.resolve(page('first'))
    await older
    expect(state.selectedTableName.value).toBe('second')
    expect(state.rows.value[0]?.recordHash).toBe('second')
    wrapper.unmount()
  })
})
