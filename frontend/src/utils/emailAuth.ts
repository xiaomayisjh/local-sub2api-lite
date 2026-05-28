/** Standard email with at least one dot in the domain part. */
const STANDARD_EMAIL = /^[^\s@]+@[^\s@]+\.[^\s@]+$/

/** Desktop local admin addresses such as admin@localhost (no dot in host). */
const LOCAL_HOST_EMAIL = /^[^\s@]+@[a-zA-Z0-9_-]+$/

export function isValidAuthEmail(email: string, allowLocalHost = false): boolean {
  const trimmed = email.trim()
  if (!trimmed) {
    return false
  }
  if (STANDARD_EMAIL.test(trimmed)) {
    return true
  }
  return allowLocalHost && LOCAL_HOST_EMAIL.test(trimmed)
}
