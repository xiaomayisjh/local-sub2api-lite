import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import api from '@/api/client'

const AUTH_TOKEN_KEY = 'auth_token'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem(AUTH_TOKEN_KEY) || 'local-admin')
  const user = ref({
    id: 1,
    email: 'admin@localhost',
    username: 'admin',
    role: 'admin' as const,
    is_admin: true,
    status: 'active' as const,
    balance: 0,
    concurrency: 999,
    allowed_groups: null as number[] | null,
    balance_notify_enabled: false,
    balance_notify_threshold: null as number | null,
    balance_notify_extra_emails: [] as any[],
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
  })

  const isAuthenticated = computed(() => true)
  const isAdmin = computed(() => true)
  const isSimpleMode = computed(() => true)
  const hasPendingAuthSession = computed(() => false)

  function checkAuth() {
    if (!token.value) {
      token.value = 'local-admin'
      localStorage.setItem(AUTH_TOKEN_KEY, 'local-admin')
    }
  }

  function setToken(newToken: string) {
    token.value = newToken
    localStorage.setItem(AUTH_TOKEN_KEY, newToken)
  }

  function setPendingAuthSession() {}
  function clearPendingAuthSession() {}

  async function login() {
    setToken('local-admin')
    return true
  }

  async function login2FA() {
    setToken('local-admin')
    return true
  }

  async function register() {
    setToken('local-admin')
    return true
  }

  async function refreshUser() {
    try {
      const resp = await api.get('/api/admin/profile')
      if (resp.data?.data) {
        user.value = { ...user.value, ...resp.data.data }
      }
    } catch {
      // ignore
    }
  }

  function logout() {
    token.value = 'local-admin'
    localStorage.setItem(AUTH_TOKEN_KEY, 'local-admin')
  }

  function getAuthHeaders() {
    return { Authorization: `Bearer ${token.value}` }
  }

  return {
    token,
    user,
    isAuthenticated,
    isAdmin,
    isSimpleMode,
    hasPendingAuthSession,
    checkAuth,
    setToken,
    setPendingAuthSession,
    clearPendingAuthSession,
    login,
    login2FA,
    register,
    refreshUser,
    logout,
    getAuthHeaders
  }
})
