<template>
  <BaseDialog :show="show" :title="t('admin.accounts.findDuplicatesTitle')" size="lg" @close="$emit('close')">
    <div class="space-y-4">
      <!-- 加载中 -->
      <div v-if="loading" class="flex items-center justify-center py-10 text-sm text-gray-500 dark:text-gray-400">
        {{ t('common.loading') }}…
      </div>

      <!-- 加载失败 -->
      <div v-else-if="loadError" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200">
        {{ loadError }}
      </div>

      <!-- 没有重复 -->
      <div v-else-if="groups.length === 0" class="rounded-lg border border-green-200 bg-green-50 p-4 text-center text-sm text-green-800 dark:border-green-800 dark:bg-green-900/20 dark:text-green-200">
        {{ t('admin.accounts.findDuplicatesNone', { count: accountIds.length }) }}
      </div>

      <!-- 重复组列表 -->
      <div v-else class="space-y-4">
        <div class="rounded-lg border border-rose-200 bg-rose-50 p-3 text-sm text-rose-800 dark:border-rose-800 dark:bg-rose-900/20 dark:text-rose-200">
          {{ t('admin.accounts.findDuplicatesSummary', { groups: groups.length, dupes: totalDuplicates }) }}
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.accounts.findDuplicatesHint') }}
        </p>

        <div class="max-h-[26rem] space-y-3 overflow-y-auto pr-1">
          <div
            v-for="(group, gIdx) in groups"
            :key="gIdx"
            class="rounded-lg border border-gray-200 dark:border-gray-700"
          >
            <div class="flex items-center justify-between border-b border-gray-100 px-3 py-2 dark:border-gray-700">
              <span class="text-xs font-semibold uppercase tracking-wide text-gray-400 dark:text-gray-500">
                {{ t('admin.accounts.findDuplicatesGroup', { index: gIdx + 1, count: group.accounts.length }) }}
              </span>
              <span class="text-xs text-gray-400 dark:text-gray-500">
                {{ group.accounts[0].platform }} · {{ group.accounts[0].type }}
              </span>
            </div>
            <ul class="divide-y divide-gray-100 dark:divide-gray-800">
              <li
                v-for="acc in group.accounts"
                :key="acc.id"
                class="flex items-center gap-3 px-3 py-2"
                :class="toDelete.has(acc.id) ? 'bg-red-50/60 dark:bg-red-900/10' : ''"
              >
                <input
                  type="checkbox"
                  class="h-4 w-4 rounded border-gray-300 text-red-600 focus:ring-red-500 dark:border-gray-600"
                  :checked="toDelete.has(acc.id)"
                  @change="toggleDelete(acc.id)"
                />
                <div class="min-w-0 flex-1">
                  <div class="flex items-center gap-2">
                    <span class="truncate font-medium text-gray-900 dark:text-gray-100">{{ acc.name }}</span>
                    <span
                      v-if="acc.suggest_keep"
                      class="shrink-0 rounded-full bg-green-100 px-2 py-0.5 text-[11px] font-medium text-green-700 dark:bg-green-900/40 dark:text-green-300"
                    >
                      {{ t('admin.accounts.findDuplicatesSuggestKeep') }}
                    </span>
                  </div>
                  <div class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
                    #{{ acc.id }} · {{ t('admin.accounts.findDuplicatesCreatedAt') }} {{ formatTime(acc.created_at) }}
                    <span v-if="acc.last_used_at"> · {{ t('admin.accounts.findDuplicatesLastUsed') }} {{ formatTime(acc.last_used_at) }}</span>
                  </div>
                </div>
                <span
                  class="shrink-0 text-xs font-medium"
                  :class="toDelete.has(acc.id) ? 'text-red-600 dark:text-red-400' : 'text-gray-400 dark:text-gray-500'"
                >
                  {{ toDelete.has(acc.id) ? t('admin.accounts.findDuplicatesWillDelete') : t('admin.accounts.findDuplicatesWillKeep') }}
                </span>
              </li>
            </ul>
          </div>
        </div>

        <!-- 最后一次操作反馈 -->
        <div v-if="lastFeedback"
          class="rounded-lg border px-3 py-2 text-xs"
          :class="lastFeedback.kind === 'ok'
            ? 'border-blue-200 bg-blue-50 text-blue-800 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200'
            : 'border-red-200 bg-red-50 text-red-800 dark:border-red-800 dark:bg-red-900/20 dark:text-red-200'">
          {{ lastFeedback.text }}
        </div>
      </div>
    </div>

    <template #footer>
      <button class="btn btn-secondary" :disabled="deleting" @click="$emit('close')">
        {{ t('common.close') }}
      </button>
      <button
        v-if="groups.length > 0"
        class="btn btn-danger"
        :disabled="deleting || selectedDeleteCount === 0"
        @click="deleteSelected"
      >
        <span v-if="deleting">{{ t('common.loading') }}…</span>
        <span v-else>{{ t('admin.accounts.findDuplicatesDeleteSelected', { count: selectedDeleteCount }) }}</span>
      </button>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import type { DuplicateGroupResult } from '@/api/admin/accounts'

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  accountIds: number[]
}>()

const emit = defineEmits<{
  close: []
  // 删除完成后通知父组件刷新列表；payload 为实际删除的账号 ID。
  deleted: [ids: number[]]
}>()

const loading = ref(false)
const loadError = ref<string | null>(null)
const groups = ref<DuplicateGroupResult[]>([])
const totalDuplicates = ref(0)
const deleting = ref(false)
const lastFeedback = ref<{ kind: 'ok' | 'err'; text: string } | null>(null)

// 待删除的账号 ID 集合。默认：每组里"建议保留"的不勾选删除，其余勾选删除。
const toDelete = ref<Set<number>>(new Set())

const selectedDeleteCount = computed(() => toDelete.value.size)

const toggleDelete = (id: number) => {
  const next = new Set(toDelete.value)
  if (next.has(id)) {
    next.delete(id)
  } else {
    next.add(id)
  }
  toDelete.value = next
}

const formatTime = (iso: string) => {
  if (!iso) return '-'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return '-'
  return d.toLocaleString()
}

const load = async () => {
  loading.value = true
  loadError.value = null
  lastFeedback.value = null
  groups.value = []
  totalDuplicates.value = 0
  toDelete.value = new Set()
  try {
    const res = await adminAPI.accounts.findDuplicates(props.accountIds)
    groups.value = res.groups ?? []
    totalDuplicates.value = res.total_duplicates ?? 0
    // 预选：组内非"建议保留"的默认勾选为待删。
    const preset = new Set<number>()
    for (const g of groups.value) {
      for (const acc of g.accounts) {
        if (!acc.suggest_keep) preset.add(acc.id)
      }
    }
    toDelete.value = preset
  } catch (e: any) {
    loadError.value = e?.message || t('admin.accounts.findDuplicatesLoadFailed')
  } finally {
    loading.value = false
  }
}

const deleteSelected = async () => {
  const ids = Array.from(toDelete.value)
  if (ids.length === 0 || deleting.value) return
  deleting.value = true
  lastFeedback.value = null

  // 防呆：若某一组的账号被全部勾选删除，提示用户至少保留一个（避免整组清空）。
  for (const g of groups.value) {
    const groupIds = g.accounts.map(a => a.id)
    const remaining = groupIds.filter(id => !toDelete.value.has(id))
    if (groupIds.length > 0 && remaining.length === 0) {
      deleting.value = false
      lastFeedback.value = { kind: 'err', text: t('admin.accounts.findDuplicatesAllSelectedWarning') }
      return
    }
  }

  let ok = 0
  let failed = 0
  const deletedIds: number[] = []
  for (const id of ids) {
    try {
      await adminAPI.accounts.delete(id)
      ok++
      deletedIds.push(id)
    } catch {
      failed++
    }
  }

  if (failed === 0) {
    lastFeedback.value = { kind: 'ok', text: t('admin.accounts.findDuplicatesDeleteOk', { count: ok }) }
  } else {
    lastFeedback.value = { kind: 'err', text: t('admin.accounts.findDuplicatesDeletePartial', { ok, failed }) }
  }

  deleting.value = false
  if (deletedIds.length > 0) {
    emit('deleted', deletedIds)
    // 删除后重新拉取，刷新剩余重复组（可能整组已清干净）。
    await load()
  }
}

// 打开时加载；关闭时清理。
watch(
  () => props.show,
  (show) => {
    if (show) {
      load()
    } else {
      groups.value = []
      toDelete.value = new Set()
      lastFeedback.value = null
      loadError.value = null
    }
  }
)
</script>
