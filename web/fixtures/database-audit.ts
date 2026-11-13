import { createApp, h, shallowRef } from 'vue'
import { createRouter, createMemoryHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import App from '@nuxt/ui/components/App.vue'
import { en, zh_cn } from '@nuxt/ui/locale'
import { featuresApi, type DatabaseMutationInput, type DatabaseRow, type DatabaseTable } from '@/api/features'
import { ApiError } from '@/api/http'
import { cacheGeneration, setCacheSession, writeResponseCache } from '@/api/cache/session'
import { responseCacheKey } from '@/api/cache/policy'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import AdminDatabasePanel from '@/components/admin/AdminDatabasePanel.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser'); initializeBrowserTheme(); setLocale(params.has('zh') ? 'zh-CN' : 'en')
const visible = shallowRef(true)
const trace = { writes: [] as unknown[], reviews: [] as DatabaseMutationInput[], applies: [] as (DatabaseMutationInput & { reviewHash: string; confirmation: string })[], metadataReads: 0, rowReads: 0, pendingReview: 0, pendingApply: 0, pendingMetadata: 0 }
const table: DatabaseTable = { name: 'operational_records', allowInsert: true, allowUpdate: true, allowDelete: true, highRisk: true, warning: '', columns: [
  { name: 'id', declaredType: 'text', nullable: false, primaryKeyPosition: 1, editable: false, insertable: true, sensitive: false, protectionReason: 'primaryKey' },
  { name: 'label', declaredType: 'text', nullable: false, primaryKeyPosition: 0, editable: true, insertable: true, sensitive: false },
  { name: 'amount', declaredType: 'numeric(30,0)', nullable: false, primaryKeyPosition: 0, editable: true, insertable: true, sensitive: false },
  { name: 'enabled', declaredType: 'BOOLEAN', nullable: true, primaryKeyPosition: 0, editable: true, insertable: true, sensitive: false },
  { name: 'updated_at', declaredType: 'text', nullable: false, primaryKeyPosition: 0, editable: false, insertable: false, sensitive: false, protectionReason: 'generated' },
] }
const protectedTable: DatabaseTable = { ...table, name: 'ledger_entries', allowInsert: false, allowUpdate: false, allowDelete: false, protectionReason: 'financialEvidence', columns: table.columns.map(column => ({ ...column, editable: false, insertable: false, protectionReason: 'financialEvidence' })) }
let rows: DatabaseRow[] = params.has('empty') ? [] : [{ key: { id: 'one' }, recordHash: 'before', values: { id: 'one', label: 'Operational label', amount: '900719925474099312345', enabled: null, updated_at: '2026-11-13T00:00:00Z' } }]
const pending: Array<{ kind: 'review' | 'apply'; resolve: () => void; reject: (error: Error) => void }> = []
let releaseMetadata: (() => void) | undefined
async function pause(): Promise<void> {
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 1600))
  if (params.has('error')) throw new ApiError(503, { code: 'HTTP_ERROR', message: '' })
}
async function hold(kind: 'review' | 'apply'): Promise<void> {
  if (!params.has('defer-' + kind)) return
  if (kind === 'review') trace.pendingReview++; else trace.pendingApply++
  await new Promise<void>((resolve, reject) => pending.push({ kind, resolve, reject }))
}
featuresApi.getDatabaseTables = async () => {
  trace.metadataReads++
  await pause()
  if (params.has('cached-metadata') && trace.metadataReads === 1) {
    trace.pendingMetadata = 1
    await new Promise<void>(resolve => { releaseMetadata = resolve })
    return { items: [protectedTable] }
  }
  return { items: [table, protectedTable] }
}
featuresApi.queryDatabaseRows = async () => { trace.rowReads++; await pause(); return { items: structuredClone(rows), nextCursor: null } }
featuresApi.reviewDatabaseMutation = async body => {
  trace.reviews.push(structuredClone(body)); trace.writes.push(structuredClone(body))
  await pause(); await hold('review')
  return { action: body.action, table: body.table, key: body.key, before: rows[0]?.values ?? null, after: body.action === 'delete' ? null : body.values, changedColumns: Object.keys(body.action === 'delete' ? rows[0]?.values ?? {} : body.values ?? {}), reviewHash: 'review', requiredConfirmation: (body.action === 'delete' ? 'DELETE ' : 'EDIT ') + body.table, rescueBackupRequired: true, warning: '' }
}
featuresApi.applyDatabaseMutation = async body => {
  trace.applies.push(structuredClone(body)); trace.writes.push(structuredClone(body))
  await hold('apply')
  if (params.has('backup-error')) throw new ApiError(503, { code: 'HTTP_ERROR', message: '' })
  if (body.action === 'delete') { rows = []; return { deleted: true, rescueBackupId: 'verified-rescue' } }
  const row = { key: { id: String(body.values?.id ?? body.key?.id ?? 'one') }, recordHash: 'after', values: { ...rows[0]?.values, ...body.values } }
  rows = [row]
  return { row, deleted: false, rescueBackupId: 'verified-rescue' }
}
if (params.has('cached-metadata')) {
  setCacheSession('constructed-database-admin')
  const tablesKey = responseCacheKey('/api/v1/admin/database/tables')
  const rowsKey = responseCacheKey('/api/v1/admin/database/tables/operational_records/query', { method: 'POST', body: { filters: [], limit: 50 } })
  if (tablesKey) writeResponseCache(tablesKey, { items: [table, protectedTable] }, cacheGeneration())
  if (rowsKey) writeResponseCache(rowsKey, { items: rows, nextCursor: null }, cacheGeneration())
}
Object.assign(window, { databaseAudit: Object.assign(trace, {
  retire: () => { visible.value = false },
  releaseMetadata: async () => { releaseMetadata?.(); trace.pendingMetadata = 0; await new Promise(resolve => setTimeout(resolve, 0)) },
  release: async (fail = false) => {
    for (const item of pending.splice(0)) { if (fail) item.reject(new Error('Constructed mutation refusal')); else item.resolve() }
    trace.pendingReview = 0; trace.pendingApply = 0
    await new Promise(resolve => setTimeout(resolve, 0))
  },
}) })
createApp({ render: () => h(App, { locale: params.has('zh') ? zh_cn : en }, () => h('main', { class: 'p-6 max-w-6xl mx-auto' }, visible.value ? h(AdminDatabasePanel) : h('p', { 'data-test': 'retired-database-workspace' }, t('browser.sessionExpired')))) })
  .use(createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: { render: () => null } }] })).use(ui).mount('#app')
