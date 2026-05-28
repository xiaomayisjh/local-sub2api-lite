<script setup lang="ts">
import { RouterView, useRouter, useRoute } from 'vue-router'
import { computed, onMounted, onBeforeUnmount, watch } from 'vue'
import Toast from '@/components/common/Toast.vue'
import NavigationProgress from '@/components/common/NavigationProgress.vue'
import { resolveDocumentTitle } from '@/router/title'
import AnnouncementPopup from '@/components/common/AnnouncementPopup.vue'
import { useAppStore, useAuthStore, useSubscriptionStore, useAnnouncementStore } from '@/stores'
import { getSetupStatus } from '@/api/setup'

const router = useRouter()
const route = useRoute()
const appStore = useAppStore()
const authStore = useAuthStore()
const subscriptionStore = useSubscriptionStore()
const announcementStore = useAnnouncementStore()
const userSelfServiceFeaturesEnabled = computed(() =>
  authStore.isAuthenticated && !authStore.isSimpleMode && !appStore.backendModeEnabled
)
let visibilityListenerRegistered = false
let delayedAnnouncementFetch: ReturnType<typeof setTimeout> | null = null

/**
 * Update favicon dynamically
 * @param logoUrl - URL of the logo to use as favicon
 */
function updateFavicon(logoUrl: string) {
  // Find existing favicon link or create new one
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }
  link.type = logoUrl.endsWith('.svg') ? 'image/svg+xml' : 'image/x-icon'
  link.href = logoUrl
}

// Watch for site settings changes and update favicon/title
watch(
  () => appStore.siteLogo,
  (newLogo) => {
    if (newLogo) {
      updateFavicon(newLogo)
    }
  },
  { immediate: true }
)

// Watch for authentication state and manage subscription data + announcements
function onVisibilityChange() {
  if (document.visibilityState === 'visible' && userSelfServiceFeaturesEnabled.value) {
    announcementStore.fetchAnnouncements()
  }
}

function addVisibilityListener() {
  if (visibilityListenerRegistered) return
  document.addEventListener('visibilitychange', onVisibilityChange)
  visibilityListenerRegistered = true
}

function removeVisibilityListener() {
  if (!visibilityListenerRegistered) return
  document.removeEventListener('visibilitychange', onVisibilityChange)
  visibilityListenerRegistered = false
}

function clearDelayedAnnouncementFetch() {
  if (!delayedAnnouncementFetch) return
  clearTimeout(delayedAnnouncementFetch)
  delayedAnnouncementFetch = null
}

watch(
  () => userSelfServiceFeaturesEnabled.value,
  (enabled, oldValue) => {
    clearDelayedAnnouncementFetch()

    if (!enabled) {
      subscriptionStore.clear()
      announcementStore.reset()
      removeVisibilityListener()
      return
    }

    subscriptionStore.fetchActiveSubscriptions().catch((error) => {
      console.error('Failed to preload subscriptions:', error)
    })
    subscriptionStore.startPolling()

    // Announcements: new login vs page refresh restore
    if (oldValue === false) {
      delayedAnnouncementFetch = setTimeout(() => {
        delayedAnnouncementFetch = null
        if (userSelfServiceFeaturesEnabled.value) {
          announcementStore.fetchAnnouncements(true)
        }
      }, 3000)
    } else {
      announcementStore.fetchAnnouncements()
    }

    addVisibilityListener()
  },
  { immediate: true }
)

// Route change trigger (throttled by store)
router.afterEach(() => {
  if (userSelfServiceFeaturesEnabled.value) {
    announcementStore.fetchAnnouncements()
  }
})

onBeforeUnmount(() => {
  clearDelayedAnnouncementFetch()
  removeVisibilityListener()
})

onMounted(async () => {
  // Check if setup is needed
  try {
    const status = await getSetupStatus()
    if (status.needs_setup && route.path !== '/setup') {
      router.replace('/setup')
      return
    }
  } catch {
    // If setup endpoint fails, assume normal mode and continue
  }

  // Load public settings into appStore (will be cached for other components)
  await appStore.fetchPublicSettings()

  // Re-resolve document title now that siteName is available
  document.title = resolveDocumentTitle(route.meta.title, appStore.siteName, route.meta.titleKey as string)
})
</script>

<template>
  <NavigationProgress />
  <RouterView />
  <Toast />
  <AnnouncementPopup />
</template>
