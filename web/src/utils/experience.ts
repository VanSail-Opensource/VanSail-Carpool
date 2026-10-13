export type Experience = 'miniapp' | 'browser'

export function setExperience(value: Experience): void {
  document.documentElement.dataset.experience = value
}

export function isBrowserExperience(): boolean {
  return typeof document !== 'undefined' && document.documentElement.dataset.experience === 'browser'
}
