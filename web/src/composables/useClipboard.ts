import { onScopeDispose, shallowRef } from 'vue'

import { copyText } from '@/utils/browserCompatibility'

export function useClipboard() {
  const copied = shallowRef(false)
  let resetTimer: ReturnType<typeof setTimeout> | undefined
  let generation = 0
  let active = true

  function reset(): void {
    generation++
    if (resetTimer !== undefined) clearTimeout(resetTimer)
    resetTimer = undefined
    copied.value = false
  }
  async function copy(value: string): Promise<boolean> {
    if (!active) return false
    reset()
    const attempt = generation
    try {
      const success = await copyText(value)
      if (!active || attempt !== generation) return false
      if (!success) return false
      copied.value = true
      resetTimer = setTimeout(() => { copied.value = false; resetTimer = undefined }, 1800)
      return true
    } catch {
      if (active && attempt === generation) copied.value = false
      return false
    }
  }
  onScopeDispose(() => { active = false; reset() })
  return { copied, copy, reset }
}
