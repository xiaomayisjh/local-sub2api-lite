<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.dataImportTitle')"
    width="normal"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="import-data-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.dataImportHint') }}
      </div>
      <div
        class="rounded-lg border border-emerald-200 bg-emerald-50 p-3 text-xs text-emerald-700 dark:border-emerald-800 dark:bg-emerald-900/20 dark:text-emerald-300"
      >
        {{ t('admin.accounts.dataImportDedupHint') }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-600 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-400"
      >
        {{ t('admin.accounts.dataImportWarning') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.dataImportFile') }}</label>
        <div
          class="flex items-center justify-between gap-3 rounded-lg border border-dashed border-gray-300 bg-gray-50 px-4 py-3 dark:border-dark-600 dark:bg-dark-800"
        >
          <div class="min-w-0">
            <div class="truncate text-sm text-gray-700 dark:text-dark-200">
              {{ fileName || t('admin.accounts.dataImportSelectFile') }}
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
          {{ t('admin.accounts.dataImportResult') }}
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.accounts.dataImportResultSummary', result) }}
        </div>

        <div v-if="errorItems.length" class="mt-2">
          <div class="text-sm font-medium text-red-600 dark:text-red-400">
            {{ t('admin.accounts.dataImportErrors') }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-lg bg-gray-50 p-3 font-mono text-xs dark:bg-dark-800"
          >
            <div v-for="(item, idx) in errorItems" :key="idx" class="whitespace-pre-wrap">
              {{ item.kind }} {{ item.name || item.proxy_key || '-' }} - {{ item.message }}
            </div>
          </div>
        </div>

        <div v-if="skippedItems.length" class="mt-2">
          <div class="text-sm font-medium text-amber-600 dark:text-amber-400">
            {{ t('admin.accounts.dataImportSkippedTitle', { count: skippedItems.length }) }}
          </div>
          <div
            class="mt-2 max-h-48 overflow-auto rounded-lg bg-amber-50 p-3 text-xs dark:bg-amber-900/20"
          >
            <div v-for="(item, idx) in skippedItems" :key="idx" class="whitespace-pre-wrap text-amber-700 dark:text-amber-300">
              {{ item.name || '-' }} — {{ item.reason }}
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
          form="import-data-form"
          :disabled="importing"
        >
          {{ importing ? t('admin.accounts.dataImporting') : t('admin.accounts.dataImportButton') }}
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
  return t('admin.accounts.dataImportSelectedFiles', { count: files.value.length })
})

const errorItems = computed(() => result.value?.errors || [])
const skippedItems = computed(() => result.value?.skipped || [])

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

// 解包导出文件：兼容两种格式——
// 1) 裸 payload：{ proxies, accounts, ... }（本应用前端导出后的形态，已被响应拦截器解包）
// 2) API 信封：{ code, message, data: { proxies, accounts } }（直接保存接口原始响应的文件，
//    例如从外部 sub2api 实例 curl 下来的备份）。此时真实 payload 在 .data 里。
const unwrapDataPayload = (raw: any): any => {
  if (
    raw &&
    typeof raw === 'object' &&
    'code' in raw &&
    'data' in raw &&
    raw.data &&
    typeof raw.data === 'object' &&
    !Array.isArray(raw.data)
  ) {
    return raw.data
  }
  return raw
}

const createEmptyResult = (): AdminDataImportResult => ({  proxy_created: 0,
  proxy_reused: 0,
  proxy_failed: 0,
  account_created: 0,
  account_failed: 0,
  account_skipped: 0,
  errors: [],
  skipped: []
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
  target.account_skipped = (target.account_skipped || 0) + (source.account_skipped || 0)

  for (const item of source.errors || []) {
    target.errors?.push({
      ...item,
      message: `${sourceName}: ${item.message}`
    })
  }
  for (const item of source.skipped || []) {
    target.skipped?.push({ ...item })
  }
}

const handleImport = async () => {
  if (files.value.length === 0) {
    appStore.showError(t('admin.accounts.dataImportSelectFile'))
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
        dataPayload = unwrapDataPayload(JSON.parse(text))
      } catch (error: any) {
        aggregate.account_failed += 1
        aggregate.errors?.push({
          kind: 'file',
          name: sourceFile.name,
          message: error instanceof SyntaxError
            ? t('admin.accounts.dataImportParseFailed')
            : (error?.message || t('admin.accounts.dataImportFailed'))
        })
        if (error instanceof SyntaxError) parseFailures += 1
        else requestFailures += 1
        continue
      }

      try {
        const res = await adminAPI.accounts.importData({
          data: dataPayload,
          skip_default_group_bind: false
        })
        completedImports += 1
        mergeResult(aggregate, res, sourceFile.name)
      } catch (error: any) {
        requestFailures += 1
        aggregate.account_failed += 1
        aggregate.errors?.push({
          kind: 'file',
          name: sourceFile.name,
          message: error?.message || t('admin.accounts.dataImportFailed')
        })
      }
    }

    result.value = aggregate

    const msgParams: Record<string, unknown> = {
      account_created: aggregate.account_created,
      account_failed: aggregate.account_failed,
      account_skipped: aggregate.account_skipped || 0,
      proxy_created: aggregate.proxy_created,
      proxy_reused: aggregate.proxy_reused,
      proxy_failed: aggregate.proxy_failed,
    }

    const hasFailures =
      aggregate.account_failed > 0 ||
      aggregate.proxy_failed > 0 ||
      parseFailures > 0 ||
      requestFailures > 0
    const hasSuccessfulChanges =
      aggregate.account_created > 0 ||
      aggregate.proxy_created > 0 ||
      aggregate.proxy_reused > 0

    if (completedImports === 0 && files.value.length === 1 && parseFailures === 1) {
      appStore.showError(t('admin.accounts.dataImportParseFailed'))
    } else if (hasFailures) {
      appStore.showError(t('admin.accounts.dataImportCompletedWithErrors', msgParams))
    } else {
      appStore.showSuccess(t('admin.accounts.dataImportSuccess', msgParams))
    }

    if (hasSuccessfulChanges) {
      emit('imported', { close: !hasFailures })
    }
  } catch (error: any) {
    if (error instanceof SyntaxError) {
      appStore.showError(t('admin.accounts.dataImportParseFailed'))
    } else {
      appStore.showError(error?.message || t('admin.accounts.dataImportFailed'))
    }
  } finally {
    importing.value = false
  }
}
</script>
