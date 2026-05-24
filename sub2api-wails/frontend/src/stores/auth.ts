import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

const AUTH_TOKEN_KEY = 'auth_token'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem(AUTH_TOKEN_KEY) || 'local-admin')
  const user = ref({
    id: 1,
    email: 'admin@localhost',
    role: 'admin' as const,
    is_admin: true,
    status: 'active' as const
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
    logout,
    getAuthHeaders
  }
})
