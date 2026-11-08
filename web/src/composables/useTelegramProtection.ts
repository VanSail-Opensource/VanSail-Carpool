import type { Ref } from 'vue'
import { onMounted, onUnmounted, watch } from 'vue'

import { getTelegramWebApp, supportsTelegramVersion, tryTelegramCall } from '@/utils/telegram'
import { isBrowserExperience } from '@/utils/experience'

const owners = new Map<number, Ref<boolean>>()
let nextOwner = 0
let closingProtected = false
let protectedApp: TelegramWebApp | undefined

function syncProtection(): void {
  const active = [...owners.values()].some((state) => state.value)
  const app = getTelegramWebApp()
  if (app !== protectedApp) {
    protectedApp = app
    closingProtected = false
  }
  if (app && supportsTelegramVersion('6.2') && active !== closingProtected) {
    const method = active ? app.enableClosingConfirmation : app.disableClosingConfirmation
    const changed = typeof method === 'function' && tryTelegramCall(() => method.call(app))
    if (changed) closingProtected = active
  }
}

// Confirm closing busy/destructive workflows without changing host gestures.
export function useTelegramProtection(active: Ref<boolean>): void {
  if (isBrowserExperience()) return
  const owner = nextOwner
  nextOwner += 1

  onMounted(() => {
    owners.set(owner, active)
    syncProtection()
  })
  watch(active, syncProtection)
  onUnmounted(() => {
    owners.delete(owner)
    syncProtection()
  })
}
