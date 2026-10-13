// @ts-check

/** Revalidate server-derived GitHub targets at the final browser boundary. */
export function safeGitHubTarget(value) {
  const raw = String(value || '')
  if (!raw || raw.length > 2048 || /[\u0000-\u001f\u007f]/.test(raw)) return ''
  try {
    const url = new URL(raw)
    if (url.protocol !== 'https:' || url.username || url.password || url.port) return ''
    if (url.hostname !== 'github.com' && url.hostname !== 'www.github.com') return ''
    return url.href
  } catch {
    return ''
  }
}
