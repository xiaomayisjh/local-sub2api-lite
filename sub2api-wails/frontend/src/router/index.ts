import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'
import { resolveDocumentTitle } from './title'

const routes: RouteRecordRaw[] = [
  {
    path: '/',
    redirect: '/admin/dashboard'
  },
  {
    path: '/home',
    redirect: '/admin/dashboard'
  },
  {
    path: '/login',
    redirect: '/admin/dashboard'
  },
  {
    path: '/key-usage',
    name: 'KeyUsage',
    component: () => import('@/views/KeyUsageView.vue'),
    meta: { title: 'Key Usage' }
  },

  // ==================== Admin Routes ====================
  {
    path: '/admin',
    redirect: '/admin/dashboard'
  },
  {
    path: '/admin/dashboard',
    name: 'AdminDashboard',
    component: () => import('@/views/admin/DashboardView.vue'),
    meta: { title: 'Admin Dashboard', titleKey: 'admin.dashboard.title' }
  },
  {
    path: '/admin/ops',
    name: 'AdminOps',
    component: () => import('@/views/admin/ops/OpsDashboard.vue'),
    meta: { title: 'Ops Monitoring', titleKey: 'admin.ops.title' }
  },
  {
    path: '/admin/groups',
    name: 'AdminGroups',
    component: () => import('@/views/admin/GroupsView.vue'),
    meta: { title: 'Group Management', titleKey: 'admin.groups.title' }
  },
  {
    path: '/admin/channels',
    redirect: '/admin/channels/pricing'
  },
  {
    path: '/admin/channels/pricing',
    name: 'AdminChannels',
    component: () => import('@/views/admin/ChannelsView.vue'),
    meta: { title: 'Channel Management', titleKey: 'admin.channels.title' }
  },
  {
    path: '/admin/channels/monitor',
    name: 'AdminChannelMonitor',
    component: () => import('@/views/admin/ChannelMonitorView.vue'),
    meta: { title: 'Channel Monitor', titleKey: 'admin.channelMonitor.title' }
  },
  {
    path: '/admin/accounts',
    name: 'AdminAccounts',
    component: () => import('@/views/admin/AccountsView.vue'),
    meta: { title: 'Account Management', titleKey: 'admin.accounts.title' }
  },
  {
    path: '/admin/announcements',
    name: 'AdminAnnouncements',
    component: () => import('@/views/admin/AnnouncementsView.vue'),
    meta: { title: 'Announcements', titleKey: 'admin.announcements.title' }
  },
  {
    path: '/admin/proxies',
    name: 'AdminProxies',
    component: () => import('@/views/admin/ProxiesView.vue'),
    meta: { title: 'Proxy Management', titleKey: 'admin.proxies.title' }
  },
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('@/views/admin/SettingsView.vue'),
    meta: { title: 'System Settings', titleKey: 'admin.settings.title' }
  },
  {
    path: '/admin/risk-control',
    name: 'AdminRiskControl',
    component: () => import('@/views/admin/RiskControlView.vue'),
    meta: { title: 'Risk Control', titleKey: 'admin.riskControl.title' }
  },
  {
    path: '/admin/usage',
    name: 'AdminUsage',
    component: () => import('@/views/admin/UsageView.vue'),
    meta: { title: 'Usage Records', titleKey: 'admin.usage.title' }
  },

  // ==================== 404 Not Found ====================
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/NotFoundView.vue'),
    meta: { title: '404 Not Found' }
  }
]

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior(_to, _from, savedPosition) {
    if (savedPosition) return savedPosition
    return { top: 0 }
  }
})

router.beforeEach((to, _from, next) => {
  document.title = resolveDocumentTitle(to.meta.title as string, 'Sub2API', to.meta.titleKey as string)
  next()
})

router.onError((error) => {
  console.error('Router error:', error)
  const isChunkLoadError =
    error.message?.includes('Failed to fetch dynamically imported module') ||
    error.message?.includes('Loading chunk')
  if (isChunkLoadError) {
    const reloadKey = 'chunk_reload_attempted'
    const lastReload = sessionStorage.getItem(reloadKey)
    const now = Date.now()
    if (!lastReload || now - parseInt(lastReload) > 10000) {
      sessionStorage.setItem(reloadKey, now.toString())
      window.location.reload()
    }
  }
})

export default router
