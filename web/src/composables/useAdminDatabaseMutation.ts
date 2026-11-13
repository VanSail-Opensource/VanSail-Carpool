import { onScopeDispose, shallowRef, type ShallowRef } from 'vue'
import { featuresApi, type DatabaseMutationInput, type DatabaseMutationReview, type DatabaseRow, type DatabaseTable } from '@/api/features'
import { ApiError } from '@/api/http'
import { localizedError, t } from '@/i18n'

function snapshot(input: DatabaseMutationInput): DatabaseMutationInput {
  return {
    ...input, key: input.key ? { ...input.key } : undefined,
    values: input.values ? Object.fromEntries(Object.entries(input.values).map(([key, value]) => [key, value && typeof value === 'object' ? { ...value } : value])) : undefined,
  }
}
function identity(input: DatabaseMutationInput): string {
  const sorted = (value?: Record<string, unknown>) => value ? Object.entries(value).sort(([left], [right]) => left < right ? -1 : left > right ? 1 : 0) : null
  return JSON.stringify([input.action, input.table, sorted(input.key), sorted(input.values), input.recordHash ?? null, input.reason])
}
function mutationError(error: unknown, fallback: string): string {
  const reasons = ['financialEvidence', 'secrets', 'managedState', 'generated', 'primaryKey', 'protectedDependencies']
  if (error instanceof ApiError && error.code === 'DATABASE_PROTECTED' && reasons.includes(error.details?.reason ?? '')) return t('databaseProtection.' + error.details!.reason)
  return localizedError(error, fallback)
}

export function useAdminDatabaseMutation(state: {
  table: () => DatabaseTable | null
  blocked: () => boolean
  beforeStart: () => void
  rows: ShallowRef<DatabaseRow[]>
  error: ShallowRef<string | null>
}) {
  const mutating = shallowRef(false)
  const review = shallowRef<DatabaseMutationReview | null>(null)
  const lastRescueBackupId = shallowRef<string | null>(null)
  let disposed = false
  let draftVersion = 0
  let reviewedInput: DatabaseMutationInput | null = null

  function clearReview(): void { draftVersion++; review.value = null; reviewedInput = null }
  function canStart(input: DatabaseMutationInput): boolean {
    return !disposed && !mutating.value && !state.blocked() && state.table()?.name === input.table
  }

  async function reviewMutation(input: DatabaseMutationInput): Promise<boolean> {
    if (!canStart(input)) return false
    clearReview()
    const version = draftVersion
    const target = snapshot(input)
    state.beforeStart()
    mutating.value = true
    state.error.value = null
    try {
      const result = await featuresApi.reviewDatabaseMutation(target)
      if (disposed || version !== draftVersion || state.table()?.name !== target.table) return false
      if (result.action !== target.action || result.table !== target.table) { state.error.value = t('errors.mutationReview'); return false }
      reviewedInput = target
      review.value = result
      return true
    } catch (caught) {
      if (!disposed && version === draftVersion) state.error.value = mutationError(caught, 'errors.mutationReview')
      return false
    } finally {
      if (!disposed) mutating.value = false
    }
  }

  async function applyMutation(input: DatabaseMutationInput, confirmation: string): Promise<boolean> {
    if (!canStart(input) || !review.value || !reviewedInput) return false
    if (identity(input) !== identity(reviewedInput)) {
      clearReview()
      state.error.value = t('errors.mutationReview')
      return false
    }
    if (confirmation !== review.value.requiredConfirmation) return false
    const target = snapshot(reviewedInput)
    const reviewHash = review.value.reviewHash
    state.beforeStart()
    mutating.value = true
    state.error.value = null
    try {
      const response = await featuresApi.applyDatabaseMutation({ ...target, reviewHash, confirmation })
      if (disposed || state.table()?.name !== target.table) return false
      if (response.row) {
        state.rows.value = target.action === 'insert'
          ? [response.row, ...state.rows.value]
          : state.rows.value.map(row => row.recordHash === target.recordHash ? response.row! : row)
      } else if (response.deleted) state.rows.value = state.rows.value.filter(row => row.recordHash !== target.recordHash)
      lastRescueBackupId.value = response.rescueBackupId
      clearReview()
      return true
    } catch (caught) {
      if (!disposed) state.error.value = mutationError(caught, 'errors.mutationApply')
      return false
    } finally {
      if (!disposed) mutating.value = false
    }
  }

  onScopeDispose(() => { disposed = true; clearReview() })
  return { mutating, review, lastRescueBackupId, reviewMutation, applyMutation, clearReview }
}
