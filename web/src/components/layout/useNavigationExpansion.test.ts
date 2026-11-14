import { computed, effectScope, shallowRef } from 'vue'
import { describe, expect, it } from 'vitest'
import { desktopNavigationItems } from './navigation'
import { useNavigationExpansion } from './useNavigationExpansion'

function setup() {
  const scope = effectScope()
  const access = shallowRef(false), around = shallowRef(false), activity = shallowRef(false), locale = shallowRef('en')
  const items = computed(() => desktopNavigationItems(key => `${locale.value}:${key}`, false, around.value, activity.value, false, access.value))
  const expanded = scope.run(() => useNavigationExpansion(() => items.value))!
  return { scope, access, around, activity, locale, expanded }
}

describe('asynchronous member navigation', () => {
  it('opens education immediately and adds quick actions when access arrives', () => {
    const state = setup()
    try {
      expect(state.expanded.value).toEqual(['around-tx'])
      state.access.value = true
      expect(state.expanded.value).toEqual(['around-tx', 'home'])
      state.around.value = true
      expect(state.expanded.value).toEqual(['around-tx', 'home'])
    } finally { state.scope.stop() }
  })

  it('keeps manually collapsed groups collapsed across locale and menu changes', () => {
    const state = setup()
    try {
      state.access.value = true
      state.around.value = true
      state.expanded.value = ['home']
      state.locale.value = 'zh-CN'
      state.activity.value = true
      expect(state.expanded.value).toEqual(['home'])
      state.expanded.value = ['around-tx']
      state.activity.value = false
      expect(state.expanded.value).toEqual(['around-tx'])
    } finally { state.scope.stop() }
  })

  it('retires paid quick actions while preserving education and explicit group choices', () => {
    const state = setup()
    try {
      state.access.value = true
      state.around.value = true
      state.access.value = false
      expect(state.expanded.value).toEqual(['around-tx'])
      state.around.value = false
      expect(state.expanded.value).toEqual(['around-tx'])
      state.expanded.value = []
      state.around.value = true
      expect(state.expanded.value).toEqual([])
      state.access.value = true
      expect(state.expanded.value).toEqual(['home'])
    } finally { state.scope.stop() }
  })
})
