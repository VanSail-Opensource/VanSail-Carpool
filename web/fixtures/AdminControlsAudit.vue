<script setup lang="ts">
import { shallowRef } from 'vue'
import { useI18n } from '@/i18n'
const { t } = useI18n()
const enabled = shallowRef(true)
const checked = shallowRef(false)
const open = shallowRef(false)
</script>

<template>
  <div class="browser-console admin-form form-stack admin-editor-form">
    <UFormField :label="t('common.name')"><UInput :model-value="t('xnode.title')" /></UFormField>
    <USwitch v-model="enabled" name="enabled" :label="t('common.enabled')" />
    <UCheckbox v-model="checked" name="checked" :label="t('common.enabled')" />
    <UButton :label="t('common.edit')" class="justify-self-start" @click="open = true" />
    <UModal v-model:open="open" :title="t('common.edit')">
      <template #body>
        <form class="admin-form form-stack admin-editor-form">
          <UFormField :label="t('common.name')"><UInput :model-value="t('xnode.title')" /></UFormField>
          <USwitch v-model="enabled" name="enabled" :label="t('common.enabled')" />
          <UCheckbox v-model="checked" name="checked" :label="t('common.enabled')" />
        </form>
      </template>
    </UModal>
  </div>
</template>
