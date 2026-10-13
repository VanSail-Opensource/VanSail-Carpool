/* eslint-disable vue/one-component-per-file */

import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import type { DatabaseMutationReview, DatabaseRow, DatabaseTable } from '@/api/features'
import DatabaseRecordEditor from './DatabaseRecordEditor.vue'

const table: DatabaseTable = {
  name: 'users',
  columns: [
    { name: 'id', declaredType: 'TEXT', nullable: false, primaryKeyPosition: 1, editable: true, sensitive: false },
    { name: 'role', declaredType: 'TEXT', nullable: false, primaryKeyPosition: 0, editable: true, sensitive: false },
  ],
  highRisk: true,
  supportsRowId: false,
  warning: 'Direct edits bypass hooks.',
}
const review: DatabaseMutationReview = {
  action: 'insert', table: 'users', key: undefined, before: null,
  after: { id: 'user-1', role: 'admin' }, changedColumns: ['id', 'role'],
  reviewHash: 'review-1', requiredConfirmation: 'EDIT users', rescueBackupRequired: true,
  warning: 'Review this change.',
}
const row: DatabaseRow = { key: { id: 'user-1' }, values: { id: 'user-1', role: 'admin' }, recordHash: 'row-1' }
const deleteReview: DatabaseMutationReview = {
  action: 'delete', table: 'users', key: row.key, before: row.values, after: null,
  changedColumns: ['id', 'role'], reviewHash: 'review-delete', requiredConfirmation: 'DELETE users',
  rescueBackupRequired: true, warning: 'Review this deletion.',
}

const DrawerStub = defineComponent({
  props: {
    title: { type: String, default: '' }, description: { type: String, default: '' },
    ui: { type: Object, default: () => ({}) },
  },
  template: `<div class="drawer" :data-container-ui="ui.content" :data-body-ui="ui.body" :data-footer-ui="ui.footer"><slot name="close" /><h2>{{ title }}</h2><slot name="body" /><footer data-test="drawer-footer"><slot name="footer" /></footer></div>`,
})
const ButtonStub = defineComponent({
  inheritAttrs: false,
  props: {
    type: { type: String, default: 'button' }, form: { type: String, default: '' },
    label: { type: String, default: '' }, disabled: { type: Boolean, default: false },
  },
  emits: ['click'],
  template: `<div role="button" :data-action="type === 'submit' ? 'submit' : 'secondary'" :data-label="label" :data-form="form || undefined" :data-disabled="disabled || undefined" @click="$emit('click')">{{ label }}</div>`,
})
const InputStub = defineComponent({
  inheritAttrs: false,
  props: { modelValue: { type: String, default: '' } },
  emits: ['update:modelValue'],
  template: `<div role="textbox" :data-test="$attrs.autocomplete === 'off' ? 'confirmation' : 'field-input'" :data-value="modelValue" @click="$emit('update:modelValue', 'EDIT users')" />`,
})
const TextareaStub = defineComponent({
  inheritAttrs: false,
  props: { modelValue: { type: String, default: '' } },
  emits: ['update:modelValue'],
  template: `<div role="textbox" :data-test="$attrs.minlength ? 'reason' : 'blob-field'" :data-value="modelValue" @click="$emit('update:modelValue', 'Repair imported role')" />`,
})

function mountEditor(action: 'insert' | 'delete' = 'insert', currentRow?: DatabaseRow) {
  return mount(DatabaseRecordEditor, {
    props: { table, action, row: currentRow, review: null, busy: false },
    global: {
      stubs: {
        Slideover: DrawerStub, Button: ButtonStub, Input: InputStub, Textarea: TextareaStub,
        FormField: { template: '<div><slot /></div>' }, Alert: { template: '<div class="alert" />' },
        Checkbox: { template: '<div />' }, Icon: { template: '<span />' }, SwitchField: { template: '<div />' },
      },
    },
  })
}

describe('DatabaseRecordEditor', () => {
  it('submits a required-reason draft from the fixed footer', async () => {
    const wrapper = mountEditor()
    expect(wrapper.find('.database-record-fields').exists()).toBe(true)
    expect(wrapper.get('[data-action="submit"]').attributes('data-disabled')).toBe('true')
    await wrapper.get('[data-test="reason"]').trigger('click')
    await wrapper.find('form').trigger('submit')

    expect(wrapper.emitted('review')?.[0]?.[0]).toMatchObject({ action: 'insert', reason: 'Repair imported role' })
    expect(wrapper.get('[data-test="drawer-footer"] [data-action="submit"]').attributes('data-form')).toBe('database-record-form')
    expect(wrapper.get('.drawer').attributes('data-body-ui')).toBe('database-record-editor__scroll-body')
    expect(wrapper.get('.drawer').attributes('data-container-ui')).toBe('database-record-editor__content')
  })

  it('switches to review-only content and gates Apply by typed confirmation', async () => {
    const wrapper = mountEditor()
    await wrapper.get('[data-test="reason"]').trigger('click')
    await wrapper.setProps({ review })

    expect(wrapper.find('.database-record-fields').exists()).toBe(false)
    expect(wrapper.find('[data-test="reason"]').exists()).toBe(false)
    expect(wrapper.find('.alert').exists()).toBe(false)
    expect(wrapper.get('.database-review').text()).toContain('Before')
    expect(wrapper.get('.database-review').text()).toContain('After')
    expect(wrapper.get('[data-action="submit"]').attributes('data-disabled')).toBe('true')
    await wrapper.get('[data-test="confirmation"]').trigger('click')
    expect(wrapper.get('[data-action="submit"]').attributes('data-disabled')).toBeUndefined()
    await wrapper.find('form').trigger('submit')
    const invalidationsBeforeChange = wrapper.emitted('invalidate')?.length ?? 0
    await wrapper.get('[data-label="Change draft"]').trigger('click')

    expect(wrapper.emitted('apply')?.[0]?.[0]).toMatchObject({ confirmation: 'EDIT users', reason: 'Repair imported role' })
    expect(wrapper.emitted('invalidate')).toHaveLength(invalidationsBeforeChange + 1)
  })

  it('routes Delete through draft and review without applying immediately', async () => {
    const wrapper = mountEditor('delete', row)
    expect(wrapper.find('.database-delete-summary').exists()).toBe(true)
    await wrapper.get('[data-test="reason"]').trigger('click')
    await wrapper.find('form').trigger('submit')

    expect(wrapper.emitted('review')?.[0]?.[0]).toMatchObject({ action: 'delete', row, reason: 'Repair imported role' })
    expect(wrapper.emitted('apply')).toBeUndefined()
    await wrapper.setProps({ review: deleteReview })
    expect(wrapper.find('.database-delete-summary').exists()).toBe(false)
    expect(wrapper.find('.database-review').exists()).toBe(true)
  })
})
