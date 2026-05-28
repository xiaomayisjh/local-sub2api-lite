<template>
  <BaseDialog
    :show="show"
    :title="t('admin.proxies.dataImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="import-proxy-data-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.proxies.dataImportHint') }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-600 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {{ t('admin.proxies.dataImportWarning') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.proxies.dataImportFile') }}</label>
        <div
          class="flex items-center justify-between gap-3 rounded-lg border border-dashed border-gray-300 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-gray-700 dark:text-dark-200">
              {{ fileName || t('admin.proxies.dataImportSelectFile') }}
            </div>
            <div class="text-xs text-gray-500 dark:text-dark-400">JSON (.json)</div>
          </div>
          <button type="button" class="btn btn-secondary shrink-0" @click="openFilePicker">
            {{ t('common.chooseFile') }}
          </button>
        </div>
        <input
          ref="fileInput"
          type="file"
          class="hidden"
          accept="application/json,.json"
          multiple
          @change="handleFileChange"
        />
      </div>

      <div
        v-if="result"
        class="space-y-2 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <div class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.proxies.dataImportResult') }}
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.proxies.dataImportResultSummary', result) }}
        </div>

        <div v-if="errorItems.length" class="mt-2">
          <div class="text-sm font-medium text-red-600 dark:text-red-400">
            {{ t('admin.proxies.dataImportErrors') }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800"
          >
            <div v-for="(item, idx) in errorItems" :key="idx" class="whitespace-pre-wrap">
              {{ item.kind }} {{ item.name || item.proxy_key || '-' }} - {{ item.message }}
            </div>
          </div>
        </div>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="importing" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          form="import-proxy-data-form"
          :disabled="importing"
        >
          {{ importing ? t('admin.proxies.dataImporting') : t('admin.proxies.dataImportButton') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { AdminDataImportResult } from '@/types'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
  (e: 'imported', payload?: { close?: boolean }): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()

const importing = ref(false)
const files = ref<File[]>([])
const result = ref<AdminDataImportResult | null>(null)

const fileInput = ref<HTMLInputElement | null>(null)
const fileName = computed(() => {
  if (files.value.length === 0) return ''
  if (files.value.length === 1) return files.value[0].name
  return t('admin.proxies.dataImportSelectedFiles', { count: files.value.length })
})

const errorItems = computed(() => result.value?.errors || [])

watch(
  () => props.show,
  (open) => {
    if (open) {
      files.value = []
      result.value = null
      if (fileInput.value) {
        fileInput.value.value = ''
      }
    }
  }
)

const openFilePicker = () => {
  fileInput.value?.click()
}

const handleFileChange = (event: Event) => {
  const target = event.target as HTMLInputElement
  files.value = Array.from(target.files || [])
}

const handleClose = () => {
  if (importing.value) return
  emit('close')
}

const readFileAsText = async (sourceFile: File): Promise<string> => {
  if (typeof sourceFile.text === 'function') {
    return sourceFile.text()
  }

  if (typeof sourceFile.arrayBuffer === 'function') {
    const buffer = await sourceFile.arrayBuffer()
    return new TextDecoder().decode(buffer)
  }

  return await new Promise<string>((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result ?? ''))
    reader.onerror = () => reject(reader.error || new Error('Failed to read file'))
    reader.readAsText(sourceFile)
  })
}

const createEmptyResult = (): AdminDataImportResult => ({
  proxy_created: 0,
  proxy_reused: 0,
  proxy_failed: 0,
  account_created: 0,
  account_failed: 0,
  errors: []
})

const mergeResult = (
  target: AdminDataImportResult,
  source: AdminDataImportResult,
  sourceName: string
) => {
  target.proxy_created += source.proxy_created || 0
  target.proxy_reused += source.proxy_reused || 0
  target.proxy_failed += source.proxy_failed || 0
  target.account_created += source.account_created || 0
  target.account_failed += source.account_failed || 0

  for (const item of source.errors || []) {
    target.errors?.push({
      ...item,
      message: `${sourceName}: ${item.message}`
    })
  }
}

const handleImport = async () => {
  if (files.value.length === 0) {
    appStore.showError(t('admin.proxies.dataImportSelectFile'))
    return
  }

  importing.value = true
  const aggregate = createEmptyResult()
  let completedImports = 0
  let parseFailures = 0
  let requestFailures = 0

  try {
    for (const sourceFile of files.value) {
      let dataPayload: any
      try {
        const text = await readFileAsText(sourceFile)
        dataPayload = JSON.parse(text)
      } catch (error: any) {
        aggregate.proxy_failed += 1
        aggregate.errors?.push({
          kind: 'file',
          name: sourceFile.name,
          message: error instanceof SyntaxError
            ? t('admin.proxies.dataImportParseFailed')
            : (error?.message || t('admin.proxies.dataImportFailed'))
        })
        if (error instanceof SyntaxError) parseFailures += 1
        else requestFailures += 1
        continue
      }

      try {
        const res = await adminAPI.proxies.importData({ data: dataPayload })
        completedImports += 1
        mergeResult(aggregate, res, sourceFile.name)
      } catch (error: any) {
        requestFailures += 1
        aggregate.proxy_failed += 1
        aggregate.errors?.push({
          kind: 'file',
          name: sourceFile.name,
          message: error?.message || t('admin.proxies.dataImportFailed')
        })
      }
    }

    result.value = aggregate

    const msgParams: Record<string, unknown> = {
      proxy_created: aggregate.proxy_created,
      proxy_reused: aggregate.proxy_reused,
      proxy_failed: aggregate.proxy_failed
    }

    const hasFailures = aggregate.proxy_failed > 0 || parseFailures > 0 || requestFailures > 0
    const hasSuccessfulChanges = aggregate.proxy_created > 0 || aggregate.proxy_reused > 0

    if (completedImports === 0 && files.value.length === 1 && parseFailures === 1) {
      appStore.showError(t('admin.proxies.dataImportParseFailed'))
    } else if (hasFailures) {
      appStore.showError(t('admin.proxies.dataImportCompletedWithErrors', msgParams))
    } else {
      appStore.showSuccess(t('admin.proxies.dataImportSuccess', msgParams))
    }

    if (hasSuccessfulChanges) {
      emit('imported', { close: !hasFailures })
    }
  } catch (error: any) {
    if (error instanceof SyntaxError) {
      appStore.showError(t('admin.proxies.dataImportParseFailed'))
    } else {
      appStore.showError(error?.message || t('admin.proxies.dataImportFailed'))
    }
  } finally {
    importing.value = false
  }
}
</script>
