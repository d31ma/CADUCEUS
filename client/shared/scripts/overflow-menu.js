/**
 * CADUCEUS overflow-menu helper.
 *
 * NB: children of `[data-overflow-menu]` containers must NOT shrink (default
 * `flex-shrink: 1` lets them ellipsize to fit instead of overflowing — which
 * defeats the overflow-detection in measure()). The companion CSS rule in
 * shared/assets/styles.css applies `flex-shrink: 0` to all children of
 * any `[data-overflow-menu]` so this works without per-component tweaking.
 *
 * Attach to any flex container by adding the `data-overflow-menu` attribute:
 *
 *   <div class="email-actions" data-overflow-menu>
 *     <w-btn variant="text" @click="..."> Mark unread </w-btn>
 *     <w-btn variant="text" @click="..."> Star </w-btn>
 *     …
 *   </div>
 *
 * On wide layouts, trailing items collapse into a "More ▾" dropdown menu.
 * At 1024px and below, every item stays visible and the row scrolls
 * horizontally with touch, trackpad, mouse wheel, or keyboard. Clicking a
 * More-menu entry on wide layouts re-dispatches a click
 * to the original (hidden) element so the existing `@click` handlers fire
 * exactly as if the user had clicked the item directly — no need to wire
 * the overflow items separately.
 *
 * Works alongside Tachyon's morph re-renderer:
 *   - Listens to ResizeObserver and MutationObserver so the menu rebuilds
 *     itself whenever the container's width or children change.
 *   - The injected More button + menu are tagged `data-overflow-injected`
 *     so we can recognise (and skip) them when re-measuring.
 *
 * @module overflow-menu
 */
;(function () {
  'use strict'

  // Debug telemetry — surfaced on window so we can confirm the IIFE ran.
  window._overflowMenuLoaded = (window._overflowMenuLoaded || 0) + 1

  /** @type {WeakMap<HTMLElement, { measure: () => void, ro: ResizeObserver, mo: MutationObserver }>} */
  var instances = new WeakMap()
  var compactQuery = window.matchMedia('(max-width: 1024px)')

  /**
   * Initialise the overflow behaviour on a single container.
   * Safe to call repeatedly — subsequent calls are no-ops.
   *
   * @param {HTMLElement} container
   */
  function setup(container) {
    if (instances.has(container)) return

    // Create the More button + dropdown panel. Both live inside the container
    // so they participate in the same flex flow as the real items.
    var moreBtn = document.createElement('button')
    moreBtn.type = 'button'
    moreBtn.className = 'overflow-more-btn'
    moreBtn.dataset.overflowInjected = '1'
    moreBtn.setAttribute('aria-haspopup', 'menu')
    moreBtn.setAttribute('aria-expanded', 'false')
    moreBtn.innerHTML = 'More <span aria-hidden="true" style="margin-left:0.25rem;">▾</span>'
    moreBtn.style.cssText = 'display:none;flex-shrink:0;font:inherit;color:inherit;background:transparent;border:1px solid var(--ms-divider,#d8e0de);border-radius:var(--ms-shape-sharp,4px);padding:0.25rem 0.75rem;cursor:pointer;'

    var menu = document.createElement('div')
    menu.className = 'overflow-more-menu'
    menu.dataset.overflowInjected = '1'
    menu.setAttribute('role', 'menu')
    // NB: inline `display:flex` would override the user-agent [hidden]
    // { display:none } rule and leave the empty menu visible as a stray
    // box. Start with display:none and let openMenu()/closeMenu() toggle
    // both `hidden` and `display` together to keep state consistent.
    menu.hidden = true
    // `position:fixed` so the menu is positioned in VIEWPORT coords, matching
    // what `moreBtn.getBoundingClientRect()` returns. Using `position:absolute`
    // would offset relative to the container's positioned ancestor and push
    // the menu wildly off (the bug we're fixing).
    menu.style.cssText = 'position:fixed;z-index:50;min-width:160px;margin-top:0.25rem;padding:0.25rem;background:var(--ms-surface-container,#edf3f1);border:1px solid var(--ms-divider,#d8e0de);border-radius:var(--ms-shape-sharp,4px);box-shadow:0 4px 12px rgba(0,0,0,0.12);display:none;flex-direction:column;gap:0.125rem;'

    container.appendChild(moreBtn)
    container.appendChild(menu)

    function openMenu() {
      menu.hidden = false
      menu.style.display = 'flex'
      moreBtn.setAttribute('aria-expanded', 'true')
      positionMenu()
    }

    function closeMenu(m, btn) {
      m.hidden = true
      m.style.display = 'none'
      if (btn) btn.setAttribute('aria-expanded', 'false')
    }

    moreBtn.addEventListener('click', function (e) {
      e.stopPropagation()
      var open = !menu.hidden
      closeAllMenus()
      if (!open) openMenu()
    })

    function positionMenu() {
      // Menu is `position:fixed`, so DON'T add window.scrollY/scrollX —
      // getBoundingClientRect() already returns viewport-relative coords.
      var rect = moreBtn.getBoundingClientRect()
      menu.style.top = rect.bottom + 'px'
      menu.style.left = Math.max(8, rect.right - 160) + 'px'
    }

    /** Hide every overflow menu on the page. */
    function closeAllMenus() {
      var openMenus = document.querySelectorAll('.overflow-more-menu')
      for (var i = 0; i < openMenus.length; i++) {
        var m = /** @type {HTMLElement} */ (openMenus[i])
        var btn = m.previousElementSibling
        var btnEl = (btn && btn instanceof HTMLElement && btn.classList.contains('overflow-more-btn')) ? btn : null
        closeMenu(m, btnEl)
      }
    }

    // Click outside dismisses
    document.addEventListener('click', function (e) {
      if (menu.hidden) return
      if (!(e.target instanceof Node)) return
      if (menu.contains(e.target) || moreBtn.contains(e.target)) return
      closeAllMenus()
    })

    /**
     * Re-measure the container and decide which items overflow.
     *
     * Algorithm:
     *   1. Show every real (non-injected) item to measure natural widths.
     *   2. If everything fits, leave the More button hidden and return.
     *   3. Otherwise, show the More button to reserve space for itself,
     *      then hide items from the end one at a time until the container
     *      no longer overflows. Hidden items are mirrored into the menu
     *      as buttons that re-dispatch click events to the originals.
     */
    function measure() {
      // Move More button to the end of the flow before measuring.
      if (moreBtn.nextElementSibling !== menu) container.appendChild(moreBtn)
      if (menu.parentElement !== container) container.appendChild(menu)

      var items = Array.from(container.children).filter(function (c) {
        return c instanceof HTMLElement && !c.dataset.overflowInjected
      })

      // Reset: every item visible, More button hidden, menu empty.
      for (var i = 0; i < items.length; i++) /** @type {HTMLElement} */ (items[i]).style.removeProperty('display')
      moreBtn.style.display = 'none'
      menu.innerHTML = ''

      // Compact layouts are easier to scan and swipe when the full row stays
      // visible. Bypass menu injection and let CSS provide native scrolling.
      if (compactQuery.matches) {
        container.dataset.overflowScroll = '1'
        moreBtn.remove()
        menu.remove()
        return
      }
      delete container.dataset.overflowScroll

      // If the container fits in one row, we're done.
      // Use scrollWidth vs clientWidth to detect overflow.
      if (container.scrollWidth <= container.clientWidth + 1) return

      // Show More button so its width is reserved while we hide items.
      moreBtn.style.display = ''

      // Walk from the end, hiding items until the container fits.
      var hidden = []
      for (var j = items.length - 1; j >= 0; j--) {
        var item = /** @type {HTMLElement} */ (items[j])
        item.style.display = 'none'
        hidden.unshift(item)
        if (container.scrollWidth <= container.clientWidth + 1) break
      }

      // Build menu entries — each is a button that proxies a click to the
      // original element so the existing @click handler runs unchanged.
      for (var k = 0; k < hidden.length; k++) (function (orig) {
        var entry = document.createElement('button')
        entry.type = 'button'
        entry.dataset.overflowInjected = '1'
        entry.setAttribute('role', 'menuitem')
        entry.style.cssText = 'font:inherit;color:inherit;text-align:left;background:transparent;border:0;padding:0.5rem 0.75rem;border-radius:4px;cursor:pointer;white-space:nowrap;'
        entry.addEventListener('mouseenter', function () { entry.style.background = 'var(--ms-hover-bg,#eaf1ef)' })
        entry.addEventListener('mouseleave', function () { entry.style.background = 'transparent' })
        // Use the item's text content as the label. DuVay's light-DOM w-*
        // components keep their text in the light DOM, so textContent works;
        // fall back to `label` / `aria-label` for any icon-only control.
        var label = (orig.textContent || '').trim()
        if (!label) label = orig.getAttribute('label') || ''
        if (!label) label = orig.getAttribute('aria-label') || ''
        entry.textContent = label.trim()
        entry.addEventListener('click', function (e) {
          e.stopPropagation()
          closeAllMenus()
          // Re-dispatch a real click on the original so Tachyon's delegated
          // listener picks it up. The original is display:none but still in
          // the DOM, so it has an id and an `@click` attribute.
          orig.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }))
        })
        menu.appendChild(entry)
      })(hidden[k])
    }

    var ro = new ResizeObserver(function () { measure() })
    ro.observe(container)
    // Re-measure when Tachyon morphs new children in/out.
    var mo = new MutationObserver(function (records) {
      // Skip mutations our own measurement caused (style / display flips).
      var meaningful = records.some(function (r) {
        if (r.type === 'childList') {
          // Children added/removed and we didn't inject them.
          for (var i = 0; i < r.addedNodes.length; i++) {
            var n = r.addedNodes[i]
            if (n instanceof HTMLElement && n.dataset.overflowInjected) continue
            return true
          }
          for (var j = 0; j < r.removedNodes.length; j++) {
            var n2 = r.removedNodes[j]
            if (n2 instanceof HTMLElement && n2.dataset.overflowInjected) continue
            return true
          }
        }
        return false
      })
      if (meaningful) measure()
    })
    mo.observe(container, { childList: true })

    instances.set(container, { measure: measure, ro: ro, mo: mo })
    // First measurement.
    measure()
  }

  /**
   * Scan the document for new overflow-menu containers, set up new ones, and
   * re-measure existing ones (in case the child sizes changed without a
   * mutation event — e.g. custom-element upgrades that fire later than
   * ResizeObserver/MutationObserver pick up).
   */
  function scan() {
    var els = document.querySelectorAll('[data-overflow-menu]')
    for (var i = 0; i < els.length; i++) {
      var el = els[i]
      if (!(el instanceof HTMLElement)) continue
      if (instances.has(el)) {
        instances.get(el)?.measure()
      } else {
        setup(el)
      }
    }
  }

  // Initial scan once DOM ready (handle both pre- and post-load cases).
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', scan)
  } else {
    scan()
  }

  // Tachyon re-renders the body on navigation; pick up new containers each time.
  window.addEventListener('tachyon:navigate', scan)
  compactQuery.addEventListener?.('change', scan)

  // Watch for new `[data-overflow-menu]` elements added anywhere — handles
  // Tac component mounts inside a stable page.
  var rootObs = new MutationObserver(function (records) {
    for (var i = 0; i < records.length; i++) {
      var added = records[i].addedNodes
      for (var j = 0; j < added.length; j++) {
        var n = added[j]
        if (!(n instanceof HTMLElement)) continue
        if (n.matches && n.matches('[data-overflow-menu]')) setup(/** @type {HTMLElement} */ (n))
        if (n.querySelectorAll) {
          var nested = n.querySelectorAll('[data-overflow-menu]')
          for (var k = 0; k < nested.length; k++) setup(/** @type {HTMLElement} */ (nested[k]))
        }
      }
      // Also handle the case where Tachyon's morph adds the data-overflow-menu
      // attribute to an existing element (rather than swapping the node).
      if (records[i].type === 'attributes' && records[i].target instanceof HTMLElement) {
        var t = /** @type {HTMLElement} */ (records[i].target)
        if (t.matches('[data-overflow-menu]')) setup(t)
      }
    }
  })
  rootObs.observe(document.body, { childList: true, subtree: true, attributes: true, attributeFilter: ['data-overflow-menu'] })

  // Backstop: scan periodically for any containers we missed (e.g. an SPA
  // route change that doesn't dispatch `tachyon:navigate`). Idempotent —
  // setup() returns early for containers it's already initialised.
  setInterval(scan, 1500)
})()
