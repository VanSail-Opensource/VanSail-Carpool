import { computed, onScopeDispose, readonly, shallowRef, watch } from 'vue'
import { preferencesApi, type GroupMemberTag } from '@/api/preferences'
import { restoreRef } from '@/api/cache/restore'
import { localizedError } from '@/i18n'
import { useSessionStore } from '@/stores/session'

export function useSettings() {
  const session = useSessionStore()
  const tag = shallowRef<GroupMemberTag | null>(null)
  const tagLoading = shallowRef(true)
  const tagRefreshing = shallowRef(false)
  const tagReady = shallowRef(false)
  const tagSaving = shallowRef(false)
  const tagError = shallowRef<string | null>(null)
  let disposed = false
  let tagRevision = 0
  const tagBlocked = computed(() => !tagReady.value || tagRefreshing.value || tagSaving.value)

  async function loadTag(): Promise<void> {
    if (disposed || !session.user?.id || tagSaving.value) return
    const revision = ++tagRevision
    if (!tag.value) restoreRef('/api/v1/me/group-member-tag', tag, {}, 60_000)
    tagLoading.value = !tag.value
    tagRefreshing.value = true
    tagReady.value = false
    try {
      const next = await preferencesApi.getTag()
      if (!disposed && revision === tagRevision) { tag.value = next; tagReady.value = true; tagError.value = null }
    } catch (caught) {
      if (!disposed && revision === tagRevision) tagError.value = localizedError(caught, 'settings.tag.loadFailed')
    } finally {
      if (!disposed && revision === tagRevision) { tagLoading.value = false; tagRefreshing.value = false }
    }
  }
  async function saveTag(value: string): Promise<void> {
    if (disposed || !session.user?.id || tagBlocked.value || !tag.value?.groupJoined || !tag.value.editable) return
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
  watch(() => session.user?.id, () => {
    tagRevision++; tag.value = null; tagReady.value = false; tagRefreshing.value = false; tagLoading.value = false; tagSaving.value = false; tagError.value = null
    if (session.user?.id) void loadTag()
  }, { immediate: true, flush: 'sync' })
  onScopeDispose(() => { disposed = true; tagRevision++; tag.value = null; tagReady.value = false; tagRefreshing.value = false; tagLoading.value = false; tagSaving.value = false; tagError.value = null })
  return { tag: readonly(tag), tagLoading: readonly(tagLoading), tagBlocked, tagSaving: readonly(tagSaving), tagError: readonly(tagError), loadTag, saveTag }
}
