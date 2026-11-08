import { onMounted, onScopeDispose, readonly, shallowRef } from 'vue'
import { preferencesApi, type GroupMemberTag } from '@/api/preferences'
import { localizedError } from '@/i18n'

export function useSettings() {
  const tag = shallowRef<GroupMemberTag | null>(null)
  const tagLoading = shallowRef(true)
  const tagSaving = shallowRef(false)
  const tagError = shallowRef<string | null>(null)
  let disposed = false
  let tagRevision = 0

  async function loadTag(): Promise<void> {
    const revision = ++tagRevision
    tagLoading.value = true
    try {
      const next = await preferencesApi.getTag()
      if (!disposed && revision === tagRevision) { tag.value = next; tagError.value = null }
    } catch (caught) {
      if (!disposed && revision === tagRevision) tagError.value = localizedError(caught, 'settings.tag.loadFailed')
    } finally {
      if (!disposed && revision === tagRevision) tagLoading.value = false
    }
  }
  async function saveTag(value: string): Promise<void> {
    if (tagSaving.value) return
    const revision = ++tagRevision
    tagSaving.value = true
    tagError.value = null
    try {
      const next = await preferencesApi.updateTag(value)
      if (!disposed && revision === tagRevision) tag.value = next
    } catch (caught) {
      if (!disposed && revision === tagRevision) tagError.value = localizedError(caught, 'settings.tag.saveFailed')
    } finally {
      if (!disposed && revision === tagRevision) tagSaving.value = false
    }
  }
  onMounted(() => { void loadTag() })
  onScopeDispose(() => { disposed = true; tagRevision += 1 })
  return { tag: readonly(tag), tagLoading: readonly(tagLoading), tagSaving: readonly(tagSaving), tagError: readonly(tagError), loadTag, saveTag }
}
