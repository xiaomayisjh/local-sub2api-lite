<template>
  <BaseDialog :show="show" :title="t('admin.accounts.batchTest')" size="lg" @close="$emit('close')">
    <div class="space-y-4">
      <!-- 配置阶段 -->
      <div v-if="!running && progress.length === 0" class="space-y-4">
        <div class="rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
          {{ t('admin.accounts.batchTestAutoMode', { count: accountIds.length }) }}
        </div>

        <div>
          <label class="mb-2 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.accounts.batchTestConcurrency') }}
          </label>
          <input
            type="number"
            v-model.number="concurrency"
            min="1"
            max="20"
            class="w-full rounded-lg border border-gray-300 px-3 py-2 focus:border-primary-500 focus:ring-primary-500 dark:border-gray-600 dark:bg-gray-800"
          />
        </div>
      </div>

      <!-- 运行/完成阶段 -->
      <div v-else class="space-y-4">
        <div class="flex items-center justify-between text-sm">
          <span class="text-gray-600 dark:text-gray-400">
            {{ t('admin.accounts.batchTestProgress', { current: progress.length, total: accountIds.length }) }}
          </span>
          <span v-if="running" class="text-primary-600 dark:text-primary-400">
            {{ t('admin.accounts.batchTestRunning') }}
          </span>
          <span v-else class="text-green-600 dark:text-green-400">
            {{ t('admin.accounts.batchTestCompleted') }}
          </span>
        </div>

        <div class="h-2 overflow-hidden rounded-full bg-gray-200 dark:bg-gray-700">
          <div
            class="h-full bg-primary-600 transition-all duration-300"
            :style="{ width: `${accountIds.length > 0 ? (progress.length / accountIds.length) * 100 : 0}%` }"
          ></div>
        </div>

        <div class="max-h-96 space-y-2 overflow-y-auto">
          <div
            v-for="(result, idx) in progress"
            :key="idx"
            class="rounded-lg border px-3 py-2 text-sm"
            :class="{
              'border-green-200 bg-green-50 dark:border-green-800 dark:bg-green-900/20': resultState(result) === 'success',
              'border-amber-200 bg-amber-50 dark:border-amber-800 dark:bg-amber-900/20': resultState(result) === 'unavailable',
              'border-red-200 bg-red-50 dark:border-red-800 dark:bg-red-900/20': resultState(result) === 'failed',
            }"
          >
            <div class="flex items-start justify-between gap-2">
              <div class="flex-1 min-w-0">
                <div
                  class="font-medium"
                  :class="{
                    'text-green-900 dark:text-green-100': resultState(result) === 'success',
                    'text-amber-900 dark:text-amber-100': resultState(result) === 'unavailable',
                    'text-red-900 dark:text-red-100': resultState(result) === 'failed',
                  }"
                >
                  {{ getAccountName(result.accountId) }}
                  <span v-if="result.model" class="ml-1 text-xs font-normal opacity-70">· {{ result.model }}</span>
                  <span
                    v-if="result.modelSource === 'fallback'"
                    class="ml-1 inline-block rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
                    :title="t('admin.accounts.batchTestModelSourceFallbackHint')"
                  >{{ t('admin.accounts.batchTestModelSourceFallback') }}</span>
                  <span
                    v-if="resultState(result) === 'unavailable'"
                    class="ml-1 inline-block rounded bg-amber-100 px-1.5 py-0.5 text-[10px] font-medium text-amber-700 dark:bg-amber-900/30 dark:text-amber-300"
                    :title="t('admin.accounts.batchTestUnavailableHint')"
                  >{{ t('admin.accounts.batchTestUnavailableBadge') }}</span>
                </div>
                <div v-if="result.attempts && result.attempts > 1" class="mt-0.5 text-xs opacity-60">
                  {{ t('admin.accounts.batchTestAttempts', { count: result.attempts }) }}<span v-if="result.fellBack"> · {{ t('admin.accounts.batchTestFellBack', { mode: result.fellBack }) }}</span>
                </div>
                <div
                  v-if="result.error"
                  class="mt-1 break-words text-xs"
                  :class="resultState(result) === 'unavailable' ? 'text-amber-600 dark:text-amber-400' : 'text-red-600 dark:text-red-400'"
                >
                  {{ result.error }}
                </div>
              </div>
              <Icon
                :name="resultState(result) === 'success' ? 'check' : resultState(result) === 'unavailable' ? 'exclamationTriangle' : 'x'"
                size="sm"
                :class="{
                  'text-green-600': resultState(result) === 'success',
                  'text-amber-600': resultState(result) === 'unavailable',
                  'text-red-600': resultState(result) === 'failed',
                }"
              />
            </div>
          </div>
        </div>

        <div v-if="!running && progress.length > 0" class="space-y-3">
          <!-- 成功组操作 -->
          <div v-if="successCount > 0" class="rounded-lg border border-green-200 bg-green-50 p-3 dark:border-green-800 dark:bg-green-900/20">
            <div class="mb-2 flex items-center justify-between text-sm">
              <span class="font-medium text-green-900 dark:text-green-100">
                {{ t('admin.accounts.batchTestSuccessSummary', { count: successCount }) }}
              </span>
              <span v-if="bulkBusy === 'success'" class="text-xs text-gray-500">{{ t('common.loading') }}…</span>
            </div>
            <div class="flex flex-wrap gap-2">
              <button class="btn btn-secondary text-xs" :disabled="bulkBusy !== null"
                @click="bulkApply('success', { schedulable: true })">
                {{ t('admin.accounts.batchTestEnableSuccess') }}
              </button>
              <button class="btn btn-secondary text-xs" :disabled="bulkBusy !== null"
                @click="bulkApply('success', { schedulable: false })">
                {{ t('admin.accounts.batchTestDisableSuccess') }}
              </button>
              <button class="btn btn-secondary text-xs" :disabled="bulkBusy !== null"
                @click="bulkApply('success', { status: 'active' })">
                {{ t('admin.accounts.batchTestMarkSuccessActive') }}
              </button>
            </div>
          </div>

          <!-- 暂不可用组：上游池子/网关临时问题，账号未被证伪。默认不禁用，避免误伤。 -->
          <div v-if="unavailableResults.length > 0" class="rounded-lg border border-amber-200 bg-amber-50 p-3 dark:border-amber-800 dark:bg-amber-900/20">
            <div class="mb-2 flex items-center justify-between text-sm">
              <span class="font-medium text-amber-900 dark:text-amber-100">
                {{ t('admin.accounts.batchTestUnavailableSummary', { count: unavailableResults.length }) }}
              </span>
              <span v-if="bulkBusy === 'unavailable'" class="text-xs text-gray-500">{{ t('common.loading') }}…</span>
            </div>
            <p class="mb-2 text-xs text-amber-700 dark:text-amber-300">
              {{ t('admin.accounts.batchTestUnavailableNote') }}
            </p>
            <div class="flex flex-wrap gap-2">
              <button class="btn btn-secondary text-xs" :disabled="bulkBusy !== null"
                @click="bulkApply('unavailable', { schedulable: true })">
                {{ t('admin.accounts.batchTestKeepUnavailableEnabled') }}
              </button>
              <button class="btn btn-secondary text-xs" :disabled="bulkBusy !== null"
                @click="bulkApply('unavailable', { schedulable: false })">
                {{ t('admin.accounts.batchTestDisableUnavailable') }}
              </button>
            </div>
          </div>

          <!-- 失败组操作 -->
          <div v-if="failedResults.length > 0" class="rounded-lg border border-red-200 bg-red-50 p-3 dark:border-red-800 dark:bg-red-900/20">
            <div class="mb-2 flex items-center justify-between text-sm">
              <span class="font-medium text-red-900 dark:text-red-100">
                {{ t('admin.accounts.batchTestFailedSummary', { count: failedResults.length }) }}
              </span>
              <span v-if="bulkBusy === 'failed'" class="text-xs text-gray-500">{{ t('common.loading') }}…</span>
            </div>
            <div class="flex flex-wrap gap-2">
              <button class="btn btn-secondary text-xs" :disabled="bulkBusy !== null"
                @click="bulkApply('failed', { schedulable: true })">
                {{ t('admin.accounts.batchTestEnableFailed') }}
              </button>
              <button class="btn btn-secondary text-xs" :disabled="bulkBusy !== null"
                @click="bulkApply('failed', { schedulable: false })">
                {{ t('admin.accounts.batchTestDisableFailed') }}
              </button>
              <button class="btn btn-secondary text-xs" :disabled="bulkBusy !== null"
                @click="bulkApply('failed', { status: 'error' })">
                {{ t('admin.accounts.batchTestMarkFailedAsError') }}
              </button>
            </div>
          </div>

          <!-- 最后一次操作的反馈 -->
          <div v-if="lastActionFeedback"
            class="rounded-lg border px-3 py-2 text-xs"
            :class="lastActionFeedback.kind === 'ok'
              ? 'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200'
              : 'border-red-200 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200'">
            {{ lastActionFeedback.text }}
          </div>
        </div>
      </div>
    </div>

    <template #footer>
      <button
        v-if="!running && progress.length === 0"
        class="btn btn-secondary"
        @click="$emit('close')"
      >
        {{ t('common.cancel') }}
      </button>
      <button
        v-if="!running && progress.length === 0"
        class="btn btn-primary"
        @click="handleStart"
      >
        {{ t('admin.accounts.batchTestStart') }}
      </button>
      <button
        v-if="!running && progress.length > 0"
        class="btn btn-primary"
        @click="$emit('close')"
      >
        {{ t('common.close') }}
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'

interface Account {
  id: number
  name: string
}

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  accountIds: number[]
  accounts: Account[]
  progress: { accountId: number; model: string; modelSource?: string; outcome?: string; success: boolean; error?: string; attempts?: number; fellBack?: string }[]
  running: boolean
}>()

const emit = defineEmits<{
  close: []
  start: [concurrency: number]
  applied: []  // 批量操作完成后通知父组件刷新账号列表
}>()

const concurrency = ref(5)

// 三态归类：成功 / 失败（账号凭证被证伪）/ 暂不可用（上游池子/网关临时问题，账号未被证伪）。
// 后端 outcome 缺省时回退到 success 布尔，兼容旧版本。
type TestState = 'success' | 'failed' | 'unavailable'
const resultState = (r: { outcome?: string; success: boolean }): TestState => {
  if (r.outcome === 'failed' || r.outcome === 'unavailable' || r.outcome === 'success') {
    return r.outcome
  }
  return r.success ? 'success' : 'failed'
}

const successResults = computed(() => props.progress.filter(r => resultState(r) === 'success'))
const failedResults = computed(() => props.progress.filter(r => resultState(r) === 'failed'))
const unavailableResults = computed(() => props.progress.filter(r => resultState(r) === 'unavailable'))
const successCount = computed(() => successResults.value.length)

const bulkBusy = ref<'success' | 'failed' | 'unavailable' | null>(null)
const lastActionFeedback = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)

const getAccountName = (accountId: number) => {
  return props.accounts.find(a => a.id === accountId)?.name ?? `Account ${accountId}`
}

const handleStart = () => {
  emit('start', concurrency.value)
}

const targetIds = (group: 'success' | 'failed' | 'unavailable'): number[] => {
  const set = new Set<number>()
  for (const r of props.progress) {
    if (resultState(r) === group) set.add(r.accountId)
  }
  return Array.from(set)
}

const bulkApply = async (
  group: 'success' | 'failed' | 'unavailable',
  updates: Record<string, unknown>
) => {
  const ids = targetIds(group)
  if (ids.length === 0) return
  bulkBusy.value = group
  lastActionFeedback.value = null
  try {
    const result = await adminAPI.accounts.bulkUpdate(ids, updates)
    const failedCount = result.failed ?? 0
    const successCnt = result.success ?? (ids.length - failedCount)
    lastActionFeedback.value = {
      kind: failedCount === 0 ? 'ok' : 'err',
      text: t('admin.accounts.batchTestBulkActionDone', {
        ok: successCnt,
        fail: failedCount,
        total: ids.length
      })
    }
    emit('applied')
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e)
    lastActionFeedback.value = {
      kind: 'err',
      text: t('admin.accounts.batchTestBulkActionFailed', { error: msg })
    }
  } finally {
    bulkBusy.value = null
  }
}

// 重置 concurrency 默认值
watch(() => props.show, (visible) => {
  if (visible) {
    concurrency.value = 5
    bulkBusy.value = null
    lastActionFeedback.value = null
  }
})
</script>
