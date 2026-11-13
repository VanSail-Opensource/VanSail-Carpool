import { restoreCached, restoreItems } from '@/api/cache/restore'
import { computed, onMounted, onScopeDispose, readonly, shallowRef } from 'vue'
import type { DatabaseQueryInput, DatabaseRow, DatabaseTable } from '@/api/features'
import { featuresApi } from '@/api/features'
import { localizedError } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'
import { useAdminDatabaseMutation } from './useAdminDatabaseMutation'

export function useAdminDatabase() {
  const tables = shallowRef<DatabaseTable[]>([])
  const selectedTableName = shallowRef<string | null>(null)
  const rows = shallowRef<DatabaseRow[]>([])
  const nextCursor = shallowRef<string | null>(null)
  const loading = shallowRef(true)
  const querying = shallowRef(false)
  const error = shallowRef<string | null>(null)
  const activeQuery = shallowRef<DatabaseQueryInput>({ filters: [], limit: 50 })
  const queries = createLatestRequest()
  const metadata = createLatestRequest()
  let queryController: AbortController | undefined
  let disposed = false
  let hydrated = false
  let allowSnapshots = true
  const selectedTable = computed(() => tables.value.find(table => table.name === selectedTableName.value) ?? null)
  const { mutating, review, lastRescueBackupId, reviewMutation, applyMutation, clearReview } = useAdminDatabaseMutation({
    table: () => selectedTable.value, rows, error,
    blocked: () => querying.value || loading.value,
    beforeStart: () => { metadata.invalidate(); loading.value = false; allowSnapshots = false },
  })
  const busy = computed(() => querying.value || mutating.value)

  async function loadTables(): Promise<void> {
    if (disposed || mutating.value) return
    const ticket = metadata.begin()
    if (!hydrated) {
      loading.value = !restoreItems('/api/v1/admin/database/tables', tables)
      hydrated = true
      if (!selectedTableName.value) selectedTableName.value = tables.value[0]?.name ?? null
      if (selectedTableName.value) restoreRows(selectedTableName.value, activeQuery.value)
    }
    error.value = null
    try {
      const response = await featuresApi.getDatabaseTables()
      if (!metadata.isCurrent(ticket)) return
      tables.value = response.items
      if (!selectedTableName.value || !tables.value.some(table => table.name === selectedTableName.value)) {
        selectedTableName.value = tables.value[0]?.name ?? null
        rows.value = []
        nextCursor.value = null
        activeQuery.value = { filters: [], limit: 50 }
        clearReview()
      }
      if (selectedTableName.value) await queryRows(selectedTableName.value, activeQuery.value)
    } catch (caught) {
      if (metadata.isCurrent(ticket)) error.value = localizedError(caught, 'errors.databaseMetadata')
    } finally {
      if (metadata.isCurrent(ticket)) loading.value = false
    }
  }

  function restoreRows(table: string, input: DatabaseQueryInput): void {
    if (!allowSnapshots) return
    restoreCached<Awaited<ReturnType<typeof featuresApi.queryDatabaseRows>>>(
      '/api/v1/admin/database/tables/' + encodeURIComponent(table) + '/query', page => {
        rows.value = page.items
        nextCursor.value = page.nextCursor
      }, { method: 'POST', body: { ...input, cursor: undefined } },
    )
  }

  async function queryRows(table: string, input: DatabaseQueryInput, options: { append?: boolean } = {}): Promise<void> {
    if (disposed || selectedTableName.value !== table || mutating.value || (options.append && (querying.value || !nextCursor.value))) return
    const ticket = queries.begin()
    queryController?.abort()
    queryController = new AbortController()
    querying.value = true
    error.value = null
    try {
      const base = { ...input, filters: input.filters.map(filter => ({ ...filter })), limit: Math.min(200, Math.max(1, input.limit || 50)) }
      activeQuery.value = base
      if (!options.append) restoreRows(table, base)
      const response = await featuresApi.queryDatabaseRows(table, { ...base, cursor: options.append ? nextCursor.value ?? undefined : undefined }, queryController.signal)
      if (!queries.isCurrent(ticket) || selectedTableName.value !== table) return
      rows.value = options.append ? [...rows.value, ...response.items] : response.items
      nextCursor.value = response.nextCursor
    } catch (caught) {
      if (queries.isCurrent(ticket) && selectedTableName.value === table) error.value = localizedError(caught, 'errors.databaseRows')
    } finally {
      if (queries.isCurrent(ticket)) querying.value = false
    }
  }

  async function loadRows(table: string, options: { append?: boolean } = {}): Promise<void> {
    await queryRows(table, activeQuery.value, options)
  }

  async function selectTable(name: string): Promise<void> {
    if (disposed || mutating.value || !tables.value.some(table => table.name === name)) return
    selectedTableName.value = name
    rows.value = []
    nextCursor.value = null
    clearReview()
    activeQuery.value = { filters: [], limit: 50 }
    await queryRows(name, activeQuery.value)
  }

  onMounted(() => void loadTables())
  onScopeDispose(() => { disposed = true; metadata.dispose(); queries.dispose(); queryController?.abort() })
  return {
    tables: readonly(tables), selectedTableName: readonly(selectedTableName), selectedTable,
    rows: readonly(rows), nextCursor: readonly(nextCursor), loading: readonly(loading), busy: readonly(busy),
    error: readonly(error), review: readonly(review), lastRescueBackupId: readonly(lastRescueBackupId),
    loadTables, loadRows, queryRows, selectTable, reviewMutation, applyMutation, clearReview,
  }
}
