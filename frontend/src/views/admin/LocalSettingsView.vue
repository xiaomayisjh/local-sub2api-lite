<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import {
  checkLocalPort,
  getLocalInfo,
  updateLocalPort,
  type LocalInfo
} from '@/api/admin/local'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const router = useRouter()
const info = ref<LocalInfo | null>(null)
const loading = ref(true)
const error = ref('')

const portInput = ref('8080')
const portChecking = ref(false)
const portSaving = ref(false)
const portCheckMessage = ref('')
const portCheckOk = ref<boolean | null>(null)
const portSaveMessage = ref('')

async function load() {
  loading.value = true
  error.value = ''
  try {
    info.value = await getLocalInfo()
    portInput.value = String(info.value.server_port || 8080)
    portCheckMessage.value = ''
    portCheckOk.value = null
    portSaveMessage.value = ''
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : '加载失败'
  } finally {
    loading.value = false
  }
}

async function copyText(text: string) {
  if (!text) return
  await navigator.clipboard.writeText(text)
}

function parsePort(): number | null {
  const port = Number.parseInt(portInput.value.trim(), 10)
  if (!Number.isFinite(port) || port < 1 || port > 65535) {
    return null
  }
  return port
}

async function handleCheckPort() {
  const port = parsePort()
  if (port === null) {
    portCheckOk.value = false
    portCheckMessage.value = '请输入 1–65535 之间的有效端口'
    return
  }
  portChecking.value = true
  portCheckMessage.value = ''
  portCheckOk.value = null
  try {
    const result = await checkLocalPort(port)
    portCheckOk.value = result.available
    if (result.available) {
      if (port === result.current_port) {
        portCheckMessage.value = '当前正在使用该端口（修改后需重启应用）'
      } else {
        portCheckMessage.value = '端口可用'
      }
    } else {
      portCheckMessage.value = result.message || '端口已被占用'
    }
  } catch (e: unknown) {
    portCheckOk.value = false
    portCheckMessage.value = e instanceof Error ? e.message : '检测失败'
  } finally {
    portChecking.value = false
  }
}

async function handleSavePort() {
  const port = parsePort()
  if (port === null) {
    portSaveMessage.value = '请输入有效端口后再保存'
    return
  }
  portSaving.value = true
  portSaveMessage.value = ''
  try {
    const result = await updateLocalPort(port)
    portSaveMessage.value = result.message
    if (info.value) {
      info.value.server_port = result.port
    }
    portInput.value = String(result.port)
  } catch (e: unknown) {
    const err = e as { response?: { data?: { message?: string } } }
    portSaveMessage.value = err.response?.data?.message || (e instanceof Error ? e.message : '保存失败')
  } finally {
    portSaving.value = false
  }
}

function leaveLocalSettings() {
  void router.push('/admin/dashboard')
}

onMounted(() => {
  if (authStore.isLocalMode) {
    void load()
  }
})
</script>

<template>
  <div class="max-w-3xl mx-auto p-6 space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h1 class="text-2xl font-semibold text-gray-900 dark:text-gray-100">本地设置</h1>
      <button
        type="button"
        class="px-3 py-2 text-sm rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-800"
        @click="leaveLocalSettings"
      >
        返回控制台
      </button>
    </div>
    <p v-if="!authStore.isLocalMode" class="text-sm text-gray-500">仅本地桌面模式可用。</p>
    <p v-else-if="loading" class="text-sm text-gray-500">加载中…</p>
    <p v-else-if="error" class="text-sm text-red-600">{{ error }}</p>
    <template v-else-if="info">
      <section class="rounded-lg border border-gray-200 dark:border-gray-700 p-4 space-y-4">
        <h2 class="font-medium">HTTP 端口</h2>
        <p class="text-sm text-gray-600 dark:text-gray-400">
          当前服务：<span class="font-mono">{{ info.server_host }}:{{ info.server_port }}</span>
        </p>
        <p class="text-sm text-gray-500">
          启动时若配置端口被占用，将自动在附近寻找可用端口并写入 config.yaml。
        </p>
        <div class="flex flex-wrap gap-2 items-end">
          <div>
            <label for="http-port" class="block text-xs text-gray-500 mb-1">监听端口</label>
            <input
              id="http-port"
              v-model="portInput"
              type="number"
              min="1"
              max="65535"
              class="w-32 rounded border border-gray-300 dark:border-gray-600 bg-white dark:bg-gray-900 px-3 py-2 text-sm"
            />
          </div>
          <button
            type="button"
            class="px-3 py-2 text-sm rounded border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-800 disabled:opacity-50"
            :disabled="portChecking"
            @click="handleCheckPort"
          >
            {{ portChecking ? '检测中…' : '检测占用' }}
          </button>
          <button
            type="button"
            class="px-3 py-2 text-sm rounded bg-primary-600 text-white disabled:opacity-50"
            :disabled="portSaving"
            @click="handleSavePort"
          >
            {{ portSaving ? '保存中…' : '保存端口' }}
          </button>
        </div>
        <p
          v-if="portCheckMessage"
          class="text-sm"
          :class="portCheckOk ? 'text-green-600 dark:text-green-400' : 'text-amber-600 dark:text-amber-400'"
        >
          {{ portCheckMessage }}
        </p>
        <p v-if="portSaveMessage" class="text-sm text-blue-600 dark:text-blue-400">
          {{ portSaveMessage }}
        </p>
        <p v-if="info.config_path" class="text-xs text-gray-500 break-all">
          配置文件：{{ info.config_path }}
        </p>
      </section>

      <section class="rounded-lg border border-gray-200 dark:border-gray-700 p-4 space-y-3">
        <h2 class="font-medium">数据目录</h2>
        <p class="text-sm break-all">{{ info.data_dir }}</p>
      </section>

      <section class="rounded-lg border border-gray-200 dark:border-gray-700 p-4 space-y-3">
        <h2 class="font-medium">默认 API Key</h2>
        <p class="text-sm text-gray-600 dark:text-gray-400">供 Claude Code / Codex CLI 等工具使用（Bearer）。</p>
        <div class="flex gap-2 items-center">
          <code class="flex-1 text-xs break-all bg-gray-100 dark:bg-gray-800 p-2 rounded">{{
            info.default_api_key || '（启动后自动生成，请刷新）'
          }}</code>
          <button
            v-if="info.default_api_key"
            type="button"
            class="px-3 py-1 text-sm rounded bg-primary-600 text-white"
            @click="copyText(info.default_api_key)"
          >
            复制
          </button>
        </div>
      </section>

      <section
        v-if="info.generated_admin_password"
        class="rounded-lg border border-amber-300 bg-amber-50 dark:bg-amber-900/20 p-4 space-y-2"
      >
        <h2 class="font-medium text-amber-800 dark:text-amber-200">首次管理员密码</h2>
        <p class="text-sm">请尽快登录后修改密码。此密码仅展示一次。</p>
        <code class="block text-sm">{{ info.generated_admin_password }}</code>
      </section>
    </template>
  </div>
</template>
