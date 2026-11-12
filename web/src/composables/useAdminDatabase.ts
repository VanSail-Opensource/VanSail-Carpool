import { restoreCached, restoreItems } from '@/api/cache/restore'
import { computed, onMounted, onScopeDispose, readonly, shallowRef } from 'vue'

import type { DatabaseMutationInput, DatabaseMutationReview, DatabaseQueryInput, DatabaseRow, DatabaseTable } from '@/api/features'
import { featuresApi } from '@/api/features'
import { ApiError } from '@/api/http'
import { localizedError, t } from '@/i18n'
import { createLatestRequest } from '@/utils/latestRequest'

export function useAdminDatabase() {
  const tables = shallowRef<DatabaseTable[]>([])
  const selectedTableName = shallowRef<string | null>(null)
  const rows = shallowRef<DatabaseRow[]>([])
  const nextCursor = shallowRef<string | null>(null)
  const loading = shallowRef(true)
  const querying = shallowRef(false)
  const mutating = shallowRef(false)
  const busy = computed(() => querying.value || mutating.value)
  const queries = createLatestRequest()
  let queryController: AbortController | undefined
  const error = shallowRef<string | null>(null)
  const review = shallowRef<DatabaseMutationReview | null>(null)
  const lastRescueBackupId = shallowRef<string | null>(null)
  const activeQuery = shallowRef<DatabaseQueryInput>({ filters: [], limit: 50 })

  const selectedTable = computed(() => tables.value.find((table) => table.name === selectedTableName.value) ?? null)

  async function loadTables(): Promise<void> {
    loading.value = !restoreItems('/api/v1/admin/database/tables', tables)
    if (!selectedTableName.value) selectedTableName.value = tables.value[0]?.name ?? null
    if (selectedTableName.value) restoreRows(selectedTableName.value, activeQuery.value)
    error.value = null
    try {
      const response = await featuresApi.getDatabaseTables()
      tables.value = response.items
      if (!selectedTableName.value || !tables.value.some((table) => table.name === selectedTableName.value)) {
        selectedTableName.value = tables.value[0]?.name ?? null
      }
      if (selectedTableName.value) await queryRows(selectedTableName.value, activeQuery.value)
    } catch (caught) {
      error.value = localizedError(caught, 'errors.databaseMetadata')
    } finally {
      loading.value = false
    }
  }

  async function loadRows(table: string, options: { append?: boolean } = {}): Promise<void> {
    return queryRows(table, activeQuery.value, options)
  }

  function restoreRows(table: string, input: DatabaseQueryInput): void {
    restoreCached<Awaited<ReturnType<typeof featuresApi.queryDatabaseRows>>>(
      `/api/v1/admin/database/tables/${encodeURIComponent(table)}/query`, page => {
        rows.value = page.items
        nextCursor.value = page.nextCursor
      }, { method: 'POST', body: { ...input, cursor: undefined } },
    )
  }

  async function queryRows(table: string, input: DatabaseQueryInput, options: { append?: boolean } = {}): Promise<void> {
    if (mutating.value || (options.append && querying.value)) return
    const ticket = queries.begin()
    queryController?.abort()
    queryController = new AbortController()
    querying.value = true
    error.value = null
    try {
      const base = { ...input, filters: input.filters.map((filter) => ({ ...filter })), limit: Math.min(200, Math.max(1, input.limit || 50)) }
      activeQuery.value = base
      if (!options.append) restoreRows(table, base)
      const response = await featuresApi.queryDatabaseRows(table, { ...base, cursor: options.append ? nextCursor.value ?? undefined : undefined }, queryController.signal)
      if (!queries.isCurrent(ticket) || selectedTableName.value !== table) return
      rows.value = options.append ? [...rows.value, ...response.items] : response.items
      nextCursor.value = response.nextCursor
    } catch (caught) {
      if (queries.isCurrent(ticket)) error.value = localizedError(caught, 'errors.databaseRows')
    } finally {
      if (queries.isCurrent(ticket)) querying.value = false
    }
  }

  async function selectTable(name: string): Promise<void> {
    selectedTableName.value = name
    rows.value = []
    nextCursor.value = null
    review.value = null
    activeQuery.value = { filters: [], limit: 50 }
    await queryRows(name, activeQuery.value)
  }

  async function reviewMutation(input: DatabaseMutationInput): Promise<boolean> {
    if (!selectedTable.value || busy.value) return false
    mutating.value = true
    error.value = null
    try {
      review.value = await featuresApi.reviewDatabaseMutation(input)
      return true
    } catch (caught) {
      error.value = mutationError(caught, 'errors.mutationReview')
      return false
    } finally {
      mutating.value = false
    }
  }

  async function applyMutation(input: DatabaseMutationInput, confirmation: string): Promise<boolean> {
    if (!review.value || busy.value) return false
    mutating.value = true
    error.value = null
    try {
      const response = await featuresApi.applyDatabaseMutation({ ...input, reviewHash: review.value.reviewHash, confirmation })
      if (response.row) {
        rows.value = input.action === 'insert'
          ? [response.row, ...rows.value]
          : rows.value.map((row) => row.recordHash === input.recordHash ? response.row! : row)
      } else if (response.deleted) {
        rows.value = rows.value.filter((row) => row.recordHash !== input.recordHash)
      }
      lastRescueBackupId.value = response.rescueBackupId
      review.value = null
      return true
    } catch (caught) {
      error.value = mutationError(caught, 'errors.mutationApply')
      return false
    } finally {
      mutating.value = false
    }
  }

  function clearReview(): void {
    review.value = null
  }

  onMounted(() => void loadTables())
  onScopeDispose(() => { queries.dispose(); queryController?.abort() })

  return {
    tables: readonly(tables),
    selectedTableName: readonly(selectedTableName),
    selectedTable,
    rows: readonly(rows),
    nextCursor: readonly(nextCursor),
    loading: readonly(loading),
    busy: readonly(busy),
    error: readonly(error),
    review: readonly(review),
    lastRescueBackupId: readonly(lastRescueBackupId),
    loadTables,
    loadRows,
    queryRows,
    selectTable,
    reviewMutation,
    applyMutation,
    clearReview,
  }
}

function mutationError(error: unknown, fallback: string): string {
  const reasons = ['financialEvidence', 'secrets', 'managedState', 'generated', 'primaryKey', 'protectedDependencies']
  if (error instanceof ApiError && error.code === 'DATABASE_PROTECTED' && reasons.includes(error.details?.reason ?? '')) return t(`databaseProtection.${error.details!.reason}`)
  return localizedError(error, fallback)
}
