import { createApp, h, shallowRef } from 'vue'
import { createPinia } from 'pinia'
import { createRouter, createMemoryHistory } from 'vue-router'
import ui from '@nuxt/ui/vue-plugin'
import { api } from '@/api/client'
import type { BackupRecord, JobRecord } from '@/api/types'
import { featuresApi } from '@/api/features'
import { setExperience } from '@/utils/experience'
import { initializeBrowserTheme } from '@/browser/theme'
import { setLocale, t } from '@/i18n'
import BackupRecoveryAudit from './BackupRecoveryAudit.vue'
import '@/browser/styles.css'

const params = new URLSearchParams(location.search)
setExperience('browser')
initializeBrowserTheme()
setLocale(params.has('zh') ? 'zh-CN' : 'en')
const visible = shallowRef(true)
const createdAt = '2026-11-13T00:00:00Z'
let backups: BackupRecord[] = params.has('empty') ? [] : [{ id: 'constructed-backup', path: 'constructed.dump', sizeBytes: '1000', status: 'complete', error: '', createdAt, completedAt: createdAt }]
let jobs: JobRecord[] = params.has('empty') ? [] : [{ id: 'constructed-job', kind: 'constructed', payload: '{}', status: 'failed', attempts: 1, availableAt: createdAt, lastError: '', createdAt, updatedAt: createdAt }]
const state = { reads: { backups: 0, jobs: 0 }, deletions: [] as { kind: string; id: string }[], downloads: [] as string[], objectURLs: 0, pending: 0, recover: false }
const pending: Array<{ resolve: () => void; reject: (error: Error) => void }> = []
async function hold(): Promise<void> {
  if (!params.has('defer')) return
  state.pending++
  await new Promise<void>((resolve, reject) => { pending.push({ resolve, reject }) })
}
api.getAdminResource = async resource => {
  if (resource !== 'backups' && resource !== 'jobs') throw new Error('Unexpected constructed resource')
  state.reads[resource]++
  if (params.has('slow')) await new Promise(resolve => setTimeout(resolve, 1200))
  if ((params.has('error') && state.reads[resource] === 1) || (params.has('refresh-error') && state.deletions.length > 0 && !state.recover)) throw new Error('Constructed inventory read failure')
  return { items: structuredClone(resource === 'backups' ? backups : jobs), page: { nextCursor: null } } as never
}
featuresApi.deleteBackup = async id => {
  state.deletions.push({ kind: 'backup', id })
  await hold()
  backups = backups.filter(backup => backup.id !== id)
}
api.deleteAdminJob = async id => {
  state.deletions.push({ kind: 'job', id })
  await hold()
  jobs = jobs.filter(job => job.id !== id)
}
featuresApi.downloadBackup = async id => {
  state.downloads.push(id)
  await hold()
  return new Blob(['constructed archive'], { type: 'application/octet-stream' })
}
const createObjectURL = URL.createObjectURL.bind(URL)
URL.createObjectURL = value => { state.objectURLs++; return createObjectURL(value) }
Object.assign(window, { __backupRecoveryAudit: Object.assign(state, {
  retire: () => { visible.value = false },
  release: async (fail = false) => {
    for (const item of pending.splice(0)) { if (fail) item.reject(new Error('Constructed action refusal')); else item.resolve() }
    state.pending = 0
    await new Promise(resolve => setTimeout(resolve, 0))
  },
}) })
const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: BackupRecoveryAudit }] })
await router.push('/')
const app = createApp({ render: () => h(BackupRecoveryAudit, { visible: visible.value }) }).use(createPinia()).use(router).use(ui)
app.config.globalProperties.$t = t
app.mount('#app')
