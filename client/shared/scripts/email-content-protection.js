// @ts-check

import { normalizeCidReference } from './attachment-content.js'

const TRACKER_URL_PATTERNS = [
  /(?:^|\.)click\.ngpvan\.com\//i,
  /(?:^|\.)bl-1\.com\//i,
  /mailstat\.us\/tr/i,
  /tracking\.cirrusinsight\.com/i,
  /(?:^|\.)t\.yesware\.com\//i,
  /app\.yesware\.com\//i,
  /mailfoogae\.appspot\.com/i,
  /launchbit\.com\/taz-pixel/i,
  /list-manage\.com\/track/i,
  /cmail1\.com\/t(?:[/?]|$)/i,
  /click\.icptrack\.com\/icp\//i,
  /infusionsoft\.com\/app\/emailopened/i,
  /via\.intercom\.io\/o(?:[/?]|$)/i,
  /mandrillapp\.com\/track/i,
  /(?:^|\.)t\.hsms06\.com\//i,
  /app\.relateiq\.com\/t\.png/i,
  /api\.mixpanel\.com\/track/i,
  /web\.frontapp\.com\/api/i,
  /mailtrack\.io\/trace/i,
  /sdr\.salesloft\.com\/email_trackers/i,
  /(?:^|\.)t\.(?:signaux|senal|sidekickopen|sigopn)(?:[./]|$)/i,
  /\/track\/open(?:[/?#]|$)/i,
]

const NETWORK_URL = /^https?:/i
const SAFE_LINK_URL = /^(?:https?:|mailto:|tel:|#|\/)/i
const REMOTE_ATTRIBUTES = ['src', 'background', 'poster', 'placeholder', 'icon']
const UNSAFE_ELEMENTS = 'script,iframe,frame,frameset,object,embed,applet,form,input,button,textarea,select,option'
const QUOTED_CONTENT = [
  'blockquote',
  '.gmail_quote',
  '.gmail_extra',
  '.yahoo_quoted',
  '.moz-cite-prefix',
  '[type="cite"]',
].join(',')

/**
 * Test whether a URL can make a network request from the email document.
 * Protocol-relative URLs are resolved as HTTPS for the purpose of the policy.
 *
 * @param {string} value
 * @returns {boolean}
 */
export function isRemoteContentUrl(value) {
  const url = String(value || '').trim()
  return NETWORK_URL.test(url) || url.startsWith('//')
}

/**
 * Recognize provider pixels that should never be loaded, even after a user
 * opts in to ordinary remote images.
 *
 * @param {string} value
 * @returns {boolean}
 */
export function isKnownTrackingUrl(value) {
  const normalized = String(value || '').trim().replace(/^\/\//, 'https://')
  if (!normalized) return false
  return TRACKER_URL_PATTERNS.some(pattern => pattern.test(normalized))
}

/**
 * Remove stylesheet imports and conditionally remove network-backed url().
 * Returning the count makes the privacy banner explain what was blocked.
 *
 * @param {string} css
 * @param {{ allowRemote?: boolean, cidSources?: Record<string, string>, remoteSources?: Record<string, string>, shouldRejectUrl?: (url: string) => boolean }} [options]
 * @returns {{ css: string, remoteCount: number, blockedCount: number, trackingCount: number, remoteUrls: string[] }}
 */
export function protectCss(css, options = {}) {
  const allowRemote = options.allowRemote === true
  const cidSources = options.cidSources || {}
  const remoteSources = options.remoteSources || {}
  const shouldRejectUrl = options.shouldRejectUrl || isKnownTrackingUrl
  let remoteCount = 0
  let blockedCount = 0
  let trackingCount = 0
  const remoteUrls = []
  let output = String(css || '')
    .replace(/@import\s+(?:url\(\s*)?(?:['"][^'"]+['"]|[^;\s)]+)\s*\)?\s*;?/gi, '')
    .replace(/@font-face\s*\{[^}]*\}/gi, '')

  output = output.replace(/url\(\s*(['"]?)([^'"\)]+)\1\s*\)/gi, (match, _quote, rawUrl) => {
    const url = String(rawUrl || '').trim()
    if (/^cid:/i.test(url)) {
      const source = cidSources[normalizeCidReference(url)]
      return isSafeEmbeddedSource(source) ? `url("${source}")` : 'none'
    }
    if (!isRemoteContentUrl(url)) return match
    const normalizedUrl = normalizeRemoteContentUrl(url)
    remoteCount += 1
    if (shouldRejectUrl(url)) {
      trackingCount += 1
      return 'none'
    }
    if (normalizedUrl && !remoteUrls.includes(normalizedUrl)) remoteUrls.push(normalizedUrl)
    if (!allowRemote) {
      blockedCount += 1
      return 'none'
    }
    const source = remoteSources[normalizedUrl]
    if (!isSafeEmbeddedSource(source)) {
      blockedCount += 1
      return 'none'
    }
    return `url("${source}")`
  })

  return { css: output, remoteCount, blockedCount, trackingCount, remoteUrls }
}

/**
 * Sanitize untrusted email HTML and apply the remote-content policy before it
 * ever enters an iframe. DOMParser works on a detached document and therefore
 * does not initiate image requests while the tree is being inspected.
 *
 * @param {string} html
 * @param {{ allowRemote?: boolean, cidSources?: Record<string, string>, remoteSources?: Record<string, string>, parser?: DOMParser }} [options]
 * @returns {{ html: string, remoteCount: number, blockedCount: number, trackingCount: number, quotedCount: number, suspiciousLinkCount: number, remoteUrls: string[] }}
 */
export function protectEmailHtml(html, options = {}) {
  const Parser = /** @type {typeof DOMParser | undefined} */ (globalThis.DOMParser)
  const parser = options.parser || (Parser ? new Parser() : null)
  if (!parser) {
    return {
      html: escapeHtml(String(html || '')),
      remoteCount: 0,
      blockedCount: 0,
      trackingCount: 0,
      quotedCount: 0,
      suspiciousLinkCount: 0,
      remoteUrls: [],
    }
  }

  const allowRemote = options.allowRemote === true
  const cidSources = options.cidSources || {}
  const remoteSources = options.remoteSources || {}
  const document = parser.parseFromString(String(html || ''), 'text/html')
  let remoteCount = 0
  let blockedCount = 0
  let trackingCount = 0
  let suspiciousLinkCount = 0
  const remoteUrls = []
  /** @param {string[]} urls */
  const rememberRemoteUrls = (urls) => {
    for (const url of urls) if (url && !remoteUrls.includes(url)) remoteUrls.push(url)
  }

  document.querySelectorAll(UNSAFE_ELEMENTS).forEach(element => element.remove())
  document.querySelectorAll('link,base,meta').forEach(element => element.remove())

  document.querySelectorAll('style').forEach(element => {
    const result = protectCss(element.textContent || '', { allowRemote, cidSources, remoteSources })
    remoteCount += result.remoteCount
    blockedCount += result.blockedCount
    trackingCount += result.trackingCount
    rememberRemoteUrls(result.remoteUrls)
    if (result.css.trim()) element.textContent = result.css
    else element.remove()
  })

  document.querySelectorAll('*').forEach(element => {
    for (const attribute of [...element.attributes]) {
      const name = attribute.name.toLowerCase()
      if (name.startsWith('on') || name === 'srcdoc' || name === 'formaction' || name.startsWith('data-caduceus-')) {
        element.removeAttribute(attribute.name)
      }
    }

    const href = element.getAttribute('href')
    if (href && !SAFE_LINK_URL.test(href.trim())) element.removeAttribute('href')
    if (element.localName === 'a' && element.hasAttribute('href')) {
      element.setAttribute('target', '_blank')
      element.setAttribute('rel', 'noopener noreferrer nofollow')
      const warning = analyzeLinkDestination(element.getAttribute('href') || '', element.textContent || '')
      if (warning.suspicious) {
        suspiciousLinkCount += 1
        element.classList.add('caduceus-suspicious-link')
        element.setAttribute('data-caduceus-link-warning', warning.reason)
        element.setAttribute('data-caduceus-link-target', warning.target)
        element.setAttribute('data-caduceus-link-host', warning.destinationHost)
        element.setAttribute('data-caduceus-link-label-host', warning.displayedHost)
        element.setAttribute('title', `Suspicious link: ${warning.reason}`)
      }
    }

    const style = element.getAttribute('style') || ''
    const candidateUrls = remoteUrlsFromElement(element, style)
    const tracking = candidateUrls.some(isKnownTrackingUrl) ||
      (candidateUrls.length > 0 && isInvisibleTrackingElement(element, style))
    if (tracking) {
      remoteCount += candidateUrls.length
      trackingCount += Math.max(candidateUrls.length, 1)
      element.remove()
      return
    }

    for (const name of REMOTE_ATTRIBUTES) {
      const value = element.getAttribute(name)
      if (value && /^cid:/i.test(value.trim())) {
        const source = cidSources[normalizeCidReference(value)]
        if (isSafeEmbeddedSource(source)) element.setAttribute(name, source)
        else element.removeAttribute(name)
        continue
      }
      if (!value || !isRemoteContentUrl(value)) continue
      const normalizedUrl = normalizeRemoteContentUrl(value)
      rememberRemoteUrls([normalizedUrl])
      remoteCount += 1
      const source = remoteSources[normalizedUrl]
      if (!allowRemote || !isSafeEmbeddedSource(source)) {
        element.removeAttribute(name)
        blockedCount += 1
      } else {
        element.setAttribute(name, source)
      }
    }

    const srcset = element.getAttribute('srcset')
    if (srcset) {
      const entries = srcset.split(',').map(entry => entry.trim()).filter(Boolean)
      const safeEntries = entries.flatMap(entry => {
        const [url, ...descriptor] = entry.split(/\s+/)
        if (!/^cid:/i.test(url || '')) return [entry]
        const source = cidSources[normalizeCidReference(url)]
        return isSafeEmbeddedSource(source) ? [`${source}${descriptor.length ? ` ${descriptor.join(' ')}` : ''}`] : []
      })
      const rewrittenEntries = []
      for (const entry of safeEntries) {
        const [url, ...descriptor] = entry.split(/\s+/)
        if (!isRemoteContentUrl(url || '')) {
          rewrittenEntries.push(entry)
          continue
        }
        const normalizedUrl = normalizeRemoteContentUrl(url)
        rememberRemoteUrls([normalizedUrl])
        remoteCount += 1
        const source = remoteSources[normalizedUrl]
        if (!allowRemote || !isSafeEmbeddedSource(source)) {
          blockedCount += 1
          continue
        }
        rewrittenEntries.push(`${source}${descriptor.length ? ` ${descriptor.join(' ')}` : ''}`)
      }
      if (rewrittenEntries.length) element.setAttribute('srcset', rewrittenEntries.join(', '))
      else element.removeAttribute('srcset')
    }

    if (style) {
      const result = protectCss(style, { allowRemote, cidSources, remoteSources })
      remoteCount += result.remoteCount
      blockedCount += result.blockedCount
      trackingCount += result.trackingCount
      rememberRemoteUrls(result.remoteUrls)
      if (result.css.trim()) element.setAttribute('style', result.css)
      else element.removeAttribute('style')
    }
  })

  const quoted = [...document.querySelectorAll(QUOTED_CONTENT)]
    .filter(element => !element.parentElement?.closest(QUOTED_CONTENT))
  quoted.forEach(element => element.setAttribute('data-caduceus-quoted', ''))

  return {
    html: document.body.innerHTML,
    remoteCount,
    blockedCount,
    trackingCount,
    quotedCount: quoted.length,
    suspiciousLinkCount,
    remoteUrls,
  }
}

/**
 * Build the complete sandbox document. CSP is a second boundary after HTML
 * rewriting: blocked mode cannot fetch network images, fonts, media, styles,
 * scripts, or forms even if a malformed token escaped the sanitizer.
 *
 * @param {string} html
 * @param {{ allowRemote?: boolean, cidSources?: Record<string, string>, remoteSources?: Record<string, string>, showQuoted?: boolean, colorMode?: 'light'|'dark', parser?: DOMParser }} [options]
 * @returns {{ documentHtml: string, html: string, remoteCount: number, blockedCount: number, trackingCount: number, quotedCount: number, suspiciousLinkCount: number, remoteUrls: string[] }}
 */
export function buildProtectedEmailDocument(html, options = {}) {
  const result = protectEmailHtml(html, options)
  const csp = `default-src 'none'; img-src data:; style-src 'unsafe-inline'; font-src data:; media-src 'none'; connect-src 'none'; frame-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; script-src 'none'`
  const quotedClass = options.showQuoted === true ? ' class="show-quoted"' : ''
  const palette = options.colorMode === 'dark'
    ? { scheme: 'dark', background: '#181a1b', foreground: '#f2f2f2', link: '#8ab4f8' }
    : { scheme: 'light', background: '#fff', foreground: '#111', link: '#0645ad' }
  const documentHtml = `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="${csp}"><style>html{color-scheme:${palette.scheme};background:${palette.background};color:${palette.foreground}}body{margin:0;padding:16px;font:14px/1.55 -apple-system,BlinkMacSystemFont,'Segoe UI',sans-serif;overflow-wrap:anywhere}a{color:${palette.link}}img{max-width:100%;height:auto}table{max-width:100%}[data-caduceus-quoted]{display:none!important}body.show-quoted [data-caduceus-quoted]{display:block!important}.caduceus-suspicious-link{outline:2px solid #b3261e;outline-offset:2px}.caduceus-suspicious-link::after{content:' ⚠';color:#b3261e;font-weight:700}</style></head><body${quotedClass}>${result.html}</body></html>`
  return { ...result, documentHtml }
}

/**
 * Detect a destination that is disguised by mismatched visible host text or
 * an internationalized/confusable hostname. The result is display-only until
 * the parent reader confirms the target again before opening it.
 *
 * @param {string} href
 * @param {string} label
 * @returns {{ suspicious: boolean, reason: string, target: string, destinationHost: string, displayedHost: string }}
 */
export function analyzeLinkDestination(href, label) {
  const target = safeExternalLinkTarget(href)
  if (!target) return { suspicious: false, reason: '', target: '', destinationHost: '', displayedHost: '' }
  const parsed = new URL(target)
  const destinationHost = normalizedDisplayHost(parsed.hostname)
  const displayedHost = visibleHostname(label)
  const rawAuthority = String(href).match(/^https?:\/\/([^/?#]+)/i)?.[1] || ''
  const rawHost = rawAuthority.replace(/^.*@/, '').replace(/:\d+$/, '')
  const hasIdn = destinationHost.split('.').some(labelPart => labelPart.startsWith('xn--'))
    || /[^\x00-\x7f]/.test(rawHost)
    || /[\u0370-\u03ff\u0400-\u052f]/u.test(rawHost)

  if (displayedHost && displayedHost !== destinationHost) {
    return {
      suspicious: true,
      reason: `The displayed host ${displayedHost} does not match ${destinationHost}.`,
      target,
      destinationHost,
      displayedHost,
    }
  }
  if (hasIdn) {
    return {
      suspicious: true,
      reason: `The destination ${destinationHost} uses internationalized characters that may imitate another domain.`,
      target,
      destinationHost,
      displayedHost,
    }
  }
  return { suspicious: false, reason: '', target, destinationHost, displayedHost }
}

/** @param {unknown} value */
export function safeExternalLinkTarget(value) {
  try {
    const parsed = new URL(String(value || '').trim())
    if (!['http:', 'https:'].includes(parsed.protocol) || parsed.username || parsed.password) return ''
    return parsed.href
  } catch {
    return ''
  }
}

/** @param {string} value */
function visibleHostname(value) {
  const label = String(value || '').trim()
  if (!label || /\s/.test(label)) return ''
  const candidate = /^[a-z][a-z0-9+.-]*:\/\//i.test(label) ? label : `https://${label}`
  try {
    return normalizedDisplayHost(new URL(candidate).hostname)
  } catch {
    return ''
  }
}

/** @param {string} value */
function normalizedDisplayHost(value) {
  return String(value || '').trim().toLowerCase().replace(/^www\./, '').replace(/\.$/, '')
}

/** @param {string} value */
function normalizeRemoteContentUrl(value) {
  try {
    return new URL(String(value || '').trim().startsWith('//') ? `https:${String(value).trim()}` : String(value || '').trim()).href
  } catch {
    return ''
  }
}

/** @param {Element} element @param {string} style */
function remoteUrlsFromElement(element, style) {
  const urls = []
  for (const name of REMOTE_ATTRIBUTES) {
    const value = element.getAttribute(name)
    if (value && isRemoteContentUrl(value)) urls.push(value)
  }
  const srcset = element.getAttribute('srcset') || ''
  for (const entry of srcset.split(',')) {
    const url = entry.trim().split(/\s+/)[0] || ''
    if (isRemoteContentUrl(url)) urls.push(url)
  }
  for (const match of style.matchAll(/url\(\s*['"]?([^'"\)]+)['"]?\s*\)/gi)) {
    if (isRemoteContentUrl(match[1])) urls.push(match[1])
  }
  return urls
}

/** @param {Element} element @param {string} style */
function isInvisibleTrackingElement(element, style) {
  const width = dimension(element.getAttribute('width'), style, 'width')
  const height = dimension(element.getAttribute('height'), style, 'height')
  if (width === 0 || height === 0) return true
  if (width !== null && height !== null && width <= 3 && height <= 3) return true
  return /(?:display\s*:\s*none|visibility\s*:\s*hidden|opacity\s*:\s*0(?:\D|$))/i.test(style)
}

/** @param {string | null} attribute @param {string} style @param {'width'|'height'} name */
function dimension(attribute, style, name) {
  const raw = String(attribute || '').trim()
  if (/^\d+(?:\.\d+)?(?:px)?$/i.test(raw)) return Number.parseFloat(raw)
  const match = style.match(new RegExp(`(?:^|;)\\s*${name}\\s*:\\s*(\\d+(?:\\.\\d+)?)px`, 'i'))
  return match ? Number.parseFloat(match[1]) : null
}

/** @param {string} value */
function escapeHtml(value) {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

/** Only attachment-policy-generated inert raster data URLs may replace CID references. */
function isSafeEmbeddedSource(value) {
  return /^data:image\/(?:png|jpeg|gif|webp|avif);base64,[A-Za-z0-9+/]+=*$/i.test(String(value || ''))
}
