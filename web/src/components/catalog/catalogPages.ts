// Keep navigation and optional preloading on the same Vite import identities.
export const catalogPages = {
  traffic: () => import('./PAYGPage.vue'),
  hourly: () => import('@/components/hourly/HourlyPage.vue'),
}

export async function preloadCatalogPages(): Promise<void> {
  const connection = (navigator as Navigator & { connection?: { saveData?: boolean; effectiveType?: string } }).connection
  if (connection?.saveData || ['slow-2g', '2g'].includes(connection?.effectiveType ?? '') || document.hidden) return
  // Individual failures are contained; foreground mounting keeps its existing
  // chunk-loading/recovery UI and can retry normally.
  await Promise.allSettled([catalogPages.traffic(), catalogPages.hourly()])
}
