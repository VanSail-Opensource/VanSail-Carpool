import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { api } from '@/api/client'
import type { BackupRecord, JobRecord } from '@/api/types'
import { featuresApi } from '@/api/features'
import { t } from '@/i18n'
import AdminBackupsPanel from './AdminBackupsPanel.vue'
import BackupInventory from './backups/BackupInventory.vue'
import JobInventory from './backups/JobInventory.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'

vi.mock('@/api/cache/restore', () => ({ restoreCached: () => false }))
const createdAt = '2026-11-13T00:00:00Z'
const backup: BackupRecord = { id: 'backup-one', path: 'constructed.dump', sizeBytes: '1000', status: 'complete', error: '', createdAt, completedAt: createdAt }
const job: JobRecord = { id: 'job-one', kind: 'constructed', payload: '{}', status: 'failed', attempts: 1, availableAt: createdAt, lastError: '', createdAt, updatedAt: createdAt }
function render() {
  return mount(AdminBackupsPanel, { global: { stubs: {
    AdminSectionState: { template: '<div><slot /></div>' }, BackupInventory: true, JobInventory: true,
    MaintenanceTrigger: true, OperationStatusNotice: true,
    ConfirmDialog: { props: ['open', 'busy', 'error'], emits: ['confirm', 'update:open'], template: '<div />' },
  } } })
}
beforeEach(() => {
  vi.spyOn(api, 'getAdminResource').mockImplementation(async resource => ({ items: resource === 'backups' ? [backup] : [job], page: { nextCursor: null } }) as never)
})
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('reviewed backup and job actions', () => {
  it.each(['backup', 'job'] as const)('locks a pending %s deletion to its confirmed target and permits explicit retry', async kind => {
    let reject!: (error: Error) => void
    const remove = kind === 'backup' ? vi.spyOn(featuresApi, 'deleteBackup') : vi.spyOn(api, 'deleteAdminJob')
    remove.mockImplementationOnce(() => new Promise<void>((_resolve, fail) => { reject = fail })).mockResolvedValue(undefined)
    const wrapper = render()
    await flushPromises()
    const inventory = kind === 'backup' ? wrapper.findComponent(BackupInventory) : wrapper.findComponent(JobInventory)
    const target = kind === 'backup' ? backup : job
    inventory.vm.$emit('delete', target)
    await flushPromises()
    const dialog = wrapper.findAllComponents(ConfirmDialog).find(item => item.props('open'))!
    dialog.vm.$emit('confirm')
    dialog.vm.$emit('confirm')
    dialog.vm.$emit('update:open', false)
    inventory.vm.$emit('delete', { ...target, id: 'replacement' })
    await flushPromises()
    expect(remove.mock.calls).toEqual([[target.id]])
    expect(dialog.props('open')).toBe(true)
    expect(dialog.props('busy')).toBe(true)
    expect(inventory.props('busy')).toBe(true)
    reject(new Error('constructed refusal'))
    await flushPromises()
    expect(dialog.props('busy')).toBe(false)
    expect(dialog.props('error')).toBe(t(kind === 'backup' ? 'adminBackups.deleteFailed' : 'adminBackups.jobDeleteFailed'))
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(remove.mock.calls.map(call => call[0])).toEqual([target.id, target.id])
    expect(dialog.props('open')).toBe(false)
    wrapper.unmount()
  })

  it.each(['backup', 'job'] as const)('keeps a confirmed deleted %s absent when inventory refresh fails', async kind => {
    const remove = kind === 'backup' ? vi.spyOn(featuresApi, 'deleteBackup') : vi.spyOn(api, 'deleteAdminJob')
    remove.mockResolvedValue(undefined)
    const wrapper = render()
    await flushPromises()
    const inventory = kind === 'backup' ? wrapper.findComponent(BackupInventory) : wrapper.findComponent(JobInventory)
    const target = kind === 'backup' ? backup : job
    inventory.vm.$emit('delete', target)
    await flushPromises()
    const dialog = wrapper.findAllComponents(ConfirmDialog).find(item => item.props('open'))!
    vi.mocked(api.getAdminResource).mockRejectedValueOnce(new Error('read unavailable'))
    dialog.vm.$emit('confirm')
    await flushPromises()
    expect(remove.mock.calls).toEqual([[target.id]])
    expect(dialog.props('open')).toBe(false)
    expect(inventory.props('items')).toEqual([])
    inventory.vm.$emit('delete', target)
    await flushPromises()
    expect(dialog.props('open')).toBe(false)
    wrapper.unmount()
  })

  it('retires a pending download when the owner workspace unmounts', async () => {
    let resolve!: (value: Blob) => void
    const download = vi.spyOn(featuresApi, 'downloadBackup').mockReturnValue(new Promise(done => { resolve = done }))
    const createObjectURL = vi.fn(() => 'blob:constructed')
    vi.stubGlobal('URL', Object.assign(class extends URL {}, { createObjectURL, revokeObjectURL: vi.fn() }))
    const wrapper = render()
    await flushPromises()
    const inventory = wrapper.findComponent(BackupInventory)
    inventory.vm.$emit('download', backup)
    inventory.vm.$emit('download', backup)
    await flushPromises()
    expect(download).toHaveBeenCalledExactlyOnceWith(backup.id)
    wrapper.unmount()
    resolve(new Blob(['constructed']))
    await flushPromises()
    expect(createObjectURL).not.toHaveBeenCalled()
    expect(api.getAdminResource).toHaveBeenCalledTimes(2)
  })
})
