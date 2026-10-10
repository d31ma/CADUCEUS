// @ts-check

const STORAGE_KEY = 'caduceus-screenshot-mode'
const FRAME_STYLE_ID = 'caduceus-screenshot-mode-styles'
const FRAME_STYLE = `
  html body * {
    -webkit-text-security: disc !important;
    word-break: break-word !important;
    color: transparent !important;
    text-shadow: 0 0 7px rgba(15, 23, 42, 0.82) !important;
    background-image: none !important;
  }
  html body img,
  html body video,
  html body canvas,
  html body svg {
    filter: blur(18px) grayscale(1) !important;
  }
`

/** @type {MutationObserver | null} */
let observer = null
/** @type {string} */
let previousTitle = ''
/** @type {WeakSet<HTMLIFrameElement>} */
const boundFrames = new WeakSet()

/**
 * Whether visual privacy mode is active for this document.
 *
 * @param {Document} [doc]
 */
export function isScreenshotModeEnabled(doc = document) {
  if (doc.documentElement.getAttribute('data-screenshot-mode') === 'true') return true
  try { return doc.defaultView?.sessionStorage?.getItem(STORAGE_KEY) === 'on' } catch { return false }
}

/**
 * Restore tab-scoped privacy state before authenticated content is presented.
 * Session storage keeps the protection active across same-tab route loads but
 * naturally clears when the browsing session ends.
 *
 * @param {Document} [doc]
 */
export function restoreScreenshotMode(doc = document) {
  return setScreenshotMode(isScreenshotModeEnabled(doc), doc)
}

/**
 * Mask or restore every visually rendered value in the application and all
 * same-origin message-body frames. Underlying DOM text remains available to
 * assistive technology; this is a screenshot/demo privacy control, not an
 * authorization boundary.
 *
 * @param {boolean} enabled
 * @param {Document} [doc]
 */
export function setScreenshotMode(enabled, doc = document) {
  const root = doc.documentElement
  const storage = doc.defaultView?.sessionStorage
  if (enabled) {
    if (!previousTitle && doc.title !== 'CADUCEUS') previousTitle = doc.title
    root.setAttribute('data-screenshot-mode', 'true')
    doc.title = 'CADUCEUS'
    try { storage?.setItem(STORAGE_KEY, 'on') } catch {}
    for (const frame of doc.querySelectorAll('iframe')) applyFramePrivacy(frame, true)
    observeFrames(doc)
  } else {
    root.removeAttribute('data-screenshot-mode')
    try { storage?.removeItem(STORAGE_KEY) } catch {}
    for (const frame of doc.querySelectorAll('iframe')) applyFramePrivacy(frame, false)
    observer?.disconnect()
    observer = null
    if (previousTitle) doc.title = previousTitle
    previousTitle = ''
  }
  doc.defaultView?.dispatchEvent(new CustomEvent('caduceus:screenshot-mode', { detail: { enabled } }))
  return enabled
}

/** @param {Document} doc */
function observeFrames(doc) {
  observer?.disconnect()
  observer = new MutationObserver(records => {
    for (const record of records) {
      for (const node of record.addedNodes) {
        if (!(node instanceof Element)) continue
        if (node instanceof HTMLIFrameElement) applyFramePrivacy(node, true)
        for (const frame of node.querySelectorAll?.('iframe') || []) applyFramePrivacy(frame, true)
      }
    }
  })
  if (doc.body) observer.observe(doc.body, { childList: true, subtree: true })
}

/**
 * Apply the same visual masking inside a sandboxed same-origin `srcdoc`
 * message frame. Cross-origin frames fail closed at the parent CSS layer.
 *
 * @param {HTMLIFrameElement} frame
 * @param {boolean} enabled
 */
function applyFramePrivacy(frame, enabled) {
  const apply = () => {
    try {
      const frameDoc = frame.contentDocument
      if (!frameDoc?.head) return
      const existing = frameDoc.getElementById(FRAME_STYLE_ID)
      if (!enabled || !isScreenshotModeEnabled(frame.ownerDocument)) {
        existing?.remove()
        return
      }
      if (existing) return
      const style = frameDoc.createElement('style')
      style.id = FRAME_STYLE_ID
      style.textContent = FRAME_STYLE
      frameDoc.head.appendChild(style)
    } catch {
      // Parent-level blur still masks a frame whose document is inaccessible.
    }
  }
  apply()
  if (!boundFrames.has(frame)) {
    boundFrames.add(frame)
    frame.addEventListener('load', apply)
  }
}
