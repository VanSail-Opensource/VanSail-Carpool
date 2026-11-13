import { mount, flushPromises } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ApiError } from '@/api/http'
import type { XNodeConfiguration, XNodeSnippet } from '@/api/xnode'
import XNodeConfigurations from './XNodeConfigurations.vue'

const api = vi.hoisted(() => ({ saveConfiguration: vi.fn(), configurations: vi.fn(), saveSnippet: vi.fn(), snippets: vi.fn(), validate: vi.fn() }))
vi.mock('@/api/xnode', () => ({ xnodeApi: api }))
vi.mock('@/i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const Modal = { props: { open: Boolean }, template: '<section v-if="open"><slot name="body" /></section>' }
const Button = { props: { label: String, disabled: Boolean, type: String }, emits: ['click'], template: '<button :type="type ?? \'button\'" :disabled="disabled" @click="$emit(\'click\')">{{ label }}</button>' }
const Input = { props: { modelValue: String, disabled: Boolean }, emits: ['update:modelValue'], template: '<input :value="modelValue" :disabled="disabled" @input="$emit(\'update:modelValue\', $event.target.value)" />' }
const Field = { template: '<div><slot /></div>' }
const Json = { props: { modelValue: [Object, Array], disabled: Boolean }, emits: ['update:modelValue', 'valid'], template: '<div />' }
const base = { Modal, Button, Input, FormField: Field, Select: true, SelectMenu: true, Alert: true, Progress: true }
const stubs = { ...base, ...Object.fromEntries(Object.entries(base).map(([key, value]) => ['U' + key, value])), JsonDocumentField: Json, XNodeJsonEditor: Json }
beforeEach(() => {
  vi.resetAllMocks(); vi.useFakeTimers({ toFake: ['setTimeout', 'clearTimeout'] })
  api.validate.mockResolvedValue({ valid: true, needsAgentValidation: false, errorCode: '' })
})
afterEach(() => { vi.useRealTimers() })

describe('native configuration and snippet creation identity', () => {
  it.each(['configuration', 'snippet'] as const)('retains one %s identity through uncertain creation, changed drafts and matching recovery', async mode => {
    const save = mode === 'configuration' ? api.saveConfiguration : api.saveSnippet
    const read = mode === 'configuration' ? api.configurations : api.snippets
    const stored = new Map<string, XNodeConfiguration | XNodeSnippet>()
    let loseResponse = true
    save.mockImplementation(async (input: XNodeConfiguration | XNodeSnippet) => {
      if (stored.has(input.id)) throw new ApiError(409, { code: 'XNODE_CONFLICT', message: 'Conflict' })
      const saved = { ...input, revision: 1 }
      stored.set(input.id, saved)
      if (loseResponse) { loseResponse = false; throw new TypeError('Response lost after commit') }
      return saved
    })
    read.mockImplementation(() => Promise.resolve({ items: [...stored.values()] })).mockRejectedValueOnce(new Error('Recovery read unavailable'))
    const perform = async (action: () => Promise<unknown>): Promise<boolean> => { try { await action(); return true } catch { return false } }
    const wrapper = mount(XNodeConfigurations, { props: { configurations: [], snippets: [], busy: false, perform }, global: { stubs } })
    const add = async () => {
      const key = mode === 'configuration' ? 'xnode.addConfiguration' : 'xnode.addSnippet'
      await wrapper.findAll('button').find(button => button.text() === key)!.trigger('click')
      await nextTick(); await vi.advanceTimersByTimeAsync(350); await flushPromises()
    }
    const submit = async () => { await wrapper.get('form').trigger('submit'); await flushPromises() }
    await add()
    await wrapper.get('input').setValue('Original')
    await submit()
    expect(wrapper.find('form').exists()).toBe(true)
    const first = save.mock.calls[0]![0] as XNodeConfiguration | XNodeSnippet
    expect(first.id).toMatch(/^[\da-f]{8}-[\da-f]{4}-4[\da-f]{3}-[89ab][\da-f]{3}-[\da-f]{12}$/i)
    expect(stored.size).toBe(1)

    await wrapper.get('input').setValue('Changed after uncertainty')
    await submit()
    expect(save.mock.calls[1]![0]).toMatchObject({ id: first.id, name: 'Changed after uncertainty', revision: 0 })
    expect(wrapper.find('form').exists()).toBe(true)
    expect(stored.get(first.id)?.name).toBe('Original')
    expect(stored.size).toBe(1)

    await wrapper.get('input').setValue('Original')
    await submit()
    expect(save.mock.calls[2]![0]).toEqual(first)
    expect(wrapper.find('form').exists()).toBe(false)
    await add()
    await wrapper.get('input').setValue('New editor')
    await submit()
    expect(save.mock.calls[3]![0].id).not.toBe(first.id)
    expect(stored.size).toBe(2)
    wrapper.unmount()
  })
})
