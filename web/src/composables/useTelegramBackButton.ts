import type { ComputedRef } from 'vue'
import { onMounted, onUnmounted, watch } from 'vue'

import { getTelegramWebApp, haptic, supportsTelegramVersion, tryTelegramCall } from '@/utils/telegram'

type BackAction = () => void | Promise<void>

interface BackOwner {
  id: number
  visible: ComputedRef<boolean>
  action: BackAction
}

const owners: BackOwner[] = []
let nextOwnerId = 0
let backButton: TelegramWebApp['BackButton']
let retry: number | undefined
let attempts = 0

function activeOwner(): BackOwner | undefined {
  return [...owners].reverse().find((owner) => owner.visible.value)
}

function handleBack(): void {
  const owner = activeOwner()
  if (!owner) return
  haptic('navigate')
  try {
    void Promise.resolve(owner.action()).catch(() => undefined)
  } catch {
    // Telegram can dispatch the event while the route component is tearing down.
  }
}

function sync(): void {
  if (retry !== undefined) window.clearTimeout(retry)
  retry = undefined
  const app = getTelegramWebApp()
  const next = owners.length > 0 && supportsTelegramVersion('6.1') ? app?.BackButton : undefined
  if (next !== backButton) {
    if (backButton) {
      tryTelegramCall(() => backButton?.offClick(handleBack))
      tryTelegramCall(() => backButton?.hide())
    }
    backButton = next
    if (backButton) tryTelegramCall(() => backButton?.onClick(handleBack))
  }
  if (backButton) tryTelegramCall(() => activeOwner() ? backButton?.show() : backButton?.hide())
  if (!backButton && owners.length > 0 && attempts < 30) {
    attempts += 1
    retry = window.setTimeout(sync, 100)
  }
}

export function useTelegramBackButton(visible: ComputedRef<boolean>, onBack: BackAction): void {
  const owner: BackOwner = { id: nextOwnerId, visible, action: onBack }
  nextOwnerId += 1

  onMounted(() => {
    owners.push(owner)
    attempts = 0
    sync()
  })

  watch(visible, sync, { immediate: true })

  onUnmounted(() => {
    const index = owners.findIndex((candidate) => candidate.id === owner.id)
    if (index >= 0) owners.splice(index, 1)
    sync()
  })
}
