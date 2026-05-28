import type { Router } from 'vue-router'

/**
 * Resolve where to send the user after a successful login.
 * Desktop/local installs always land on the admin dashboard for admin users.
 */
export function resolvePostLoginPath(
  router: Pick<Router, 'currentRoute'>,
  options: { isAdmin: boolean; isLocalMode: boolean; backendModeEnabled: boolean }
): string {
  const queryRedirect = router.currentRoute.value.query.redirect
  const raw =
    (typeof queryRedirect === 'string' && queryRedirect.trim()) ||
    (Array.isArray(queryRedirect) && typeof queryRedirect[0] === 'string'
      ? queryRedirect[0].trim()
      : '')

  const adminHome = '/admin/dashboard'
  const userHome = '/dashboard'

  if (options.isAdmin && (options.isLocalMode || options.backendModeEnabled)) {
    if (!raw || raw === '/dashboard' || raw === '/home' || raw === '/') {
      return adminHome
    }
    return raw
  }

  if (options.isAdmin) {
    return raw || adminHome
  }

  // Non-admin users must not follow admin redirect targets from the desktop shell.
  if (raw.startsWith('/admin')) {
    return userHome
  }

  return raw || userHome
}
