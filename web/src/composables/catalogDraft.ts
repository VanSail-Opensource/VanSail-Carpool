import { z } from 'zod'

export interface CatalogDraft {
  comboId?: string
  squadIds?: string[]
  couponGrantId?: string | null
}

const draftSchema = z.object({
  comboId: z.string().optional(),
  squadIds: z.array(z.string()).optional(),
  couponGrantId: z.string().nullable().optional(),
})

function storageKey(userID: string | null | undefined): string | null {
  return userID ? `txc-catalog-draft:${userID}` : null
}

export function readCatalogDraft(userID: string | null | undefined): CatalogDraft | null {
  const key = storageKey(userID)
  if (!key) return null
  try {
    const raw = globalThis.sessionStorage?.getItem(key)
    if (!raw) return null
    const parsed = draftSchema.safeParse(JSON.parse(raw))
    return parsed.success ? parsed.data : null
  } catch {
    return null
  }
}

export function writeCatalogDraft(userID: string | null | undefined, draft: CatalogDraft): void {
  const key = storageKey(userID)
  if (!key) return
  try { globalThis.sessionStorage?.setItem(key, JSON.stringify(draft)) } catch { /* Storage is optional in Telegram WebViews. */ }
}

export function clearCatalogDraft(userID: string | null | undefined): void {
  const key = storageKey(userID)
  if (!key) return
  try { globalThis.sessionStorage?.removeItem(key) } catch { /* Ignore unavailable browser storage. */ }
}
