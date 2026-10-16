// @ts-check
/** @import { SafeHtmlFramePackage } from '../../models/SafeHtmlFramePackage.js' */
/** @import { ComposeAttachment } from '/models/ComposeAttachment.js' */
/** @import { QuoteProof } from '/models/QuoteProof.js' */
/** @import { MatchedQuote } from '/models/MatchedQuote.js' */
import { proofFromTrustedQuote, matchQuoteProof } from './composer-quote-proof.js'
import { buildSafeHtmlFrame, safeHtmlFrameContentKey } from './safe-html-frame.js'
import { normalizeTemplateVariables } from './composer-templates.js'
import { sanitizeSignatureHTML } from './composer-signatures.js'

/** @import { ComposeEditorSnapshot } from '/models/ComposeEditorSnapshot.js' */

const WRAPPERS = /** @type {const} */ (['section', 'signature'])
const MISSING_IMAGE = 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///ywAAAAAAQABAAACAUwAOw=='
const IMAGE_TYPES = new Set(['image/png', 'image/jpeg', 'image/gif', 'image/webp', 'image/avif'])

/** @param {string} html @returns {HTMLElement} */
function parsedBody(html) {
  return new DOMParser().parseFromString(html, 'text/html').body
}

/** Canonical content includes structure, safe URLs, text and retained style, never raw metadata. @param {string} html @returns {string} */
function contentKey(html) {
  return safeHtmlFrameContentKey(html, WRAPPERS)
}

/** These keys are accepted only from a catalog-backed TemplateInsertRequested. @param {string} html @returns {string[]} */
export function templateVariableKeys(html) {
  const body = parsedBody(normalizeTemplateVariables(html))
  return Array.from(body.querySelectorAll('code[data-template-var]'), node => contentKey(node.innerHTML))
}

/**
 * Reconstruct bounded application metadata without copying raw class/id/data attributes.
 * Raw body CID references identify image sources; only matching typed attachment bytes
 * can authorize attachment controls. A missing CID keeps its reference and inert placeholder.
 * Signature identity alone grants nothing: canonical catalog content must also match.
 * A separate app-owned quote record authorizes only a canonical matching section.
 * Body markers alone and mailto launch data grant no quote affordance.
 *
 * @param {ComposeEditorSnapshot} snapshot
 * @param {Array<{id:string,text:string}>} signatures
 * @param {Set<string>} templateVariables
 * @param {MatchedQuote|null} [verified]
 * @returns {SafeHtmlFramePackage}
 */
export function composerFrame(snapshot, signatures, templateVariables, verified) {
  const quote = parsedBody(snapshot.quote).querySelector('section')
  const mode = verified?.proof.mode || (verified === undefined ? quote?.getAttribute('data-caduceus-quote') || '' : '')
  const quoteKey = verified?.key || (verified === undefined && quote && ['reply', 'forward'].includes(mode) ? contentKey(quote.innerHTML) : null)
  const originals = Array.from(parsedBody(snapshot.body).querySelectorAll('img'))
  const catalog = new Map(signatures.map(signature => [signature.id, contentKey(sanitizeSignatureHTML(signature.text))]))
  return buildSafeHtmlFrame(snapshot.document, snapshot.revision, {
    wrappers: WRAPPERS,
    metadataAttributes: {
      section: ['data-caduceus-quote'],
      signature: ['class', 'data-signature-id', 'contenteditable'],
      div: ['class', 'data-signature-id', 'contenteditable'],
      code: ['class', 'data-template-var'],
      img: ['data-caduceus-cid', 'data-caduceus-attachment-id', 'data-caduceus-inline-image'],
    },
    /** @param {Document} document @returns {void} */
    prepare(document) {
      // Go’s compatibility document labels missing CID images with a fragment.
      // The qualified sanitizer accepts raster data URLs without fragments.
      for (const image of document.body.querySelectorAll('img')) {
        if (image.getAttribute('src') === `${MISSING_IMAGE}#missing-inline-image`) image.setAttribute('src', MISSING_IMAGE)
      }
    },
    /** @param {Element} element @returns {Record<string,string>|null} */
    authorize(element) {
      const tag = element.localName
      if (tag === 'section' && quoteKey !== null && quoteKey === contentKey(element.innerHTML)) return { 'data-caduceus-quote': mode }
      if (tag === 'signature' || tag === 'div') {
        const id = element.getAttribute('data-signature-id') || (tag === 'signature' ? element.getAttribute('id') : '') || ''
        if (id && catalog.has(id) && catalog.get(id) === contentKey(element.innerHTML)) {
          return { class: 'caduceus-signature', 'data-signature-id': id, contenteditable: 'false' }
        }
      }
      if (tag === 'code' && (element.classList.contains('var') || element.hasAttribute('data-template-var')) && templateVariables.has(contentKey(element.innerHTML))) {
        const empty = !(element.textContent || '').trim()
        return { class: empty ? 'var empty' : 'var', 'data-template-var': 'true' }
      }
      if (tag === 'img') {
        const images = Array.from(element.ownerDocument.body.querySelectorAll('img'))
        const original = originals[images.findIndex(image => image === element)]
        const source = original?.getAttribute('src') || ''
        if (!/^cid:/i.test(source)) return null
        const cid = source.slice(4).replace(/^<|>$/g, '')
        const attachment = snapshot.attachments.find(item => (item.contentId || '').toLowerCase() === cid.toLowerCase() && IMAGE_TYPES.has(item.contentType.toLowerCase()))
        const expected = attachment ? `data:${attachment.contentType};base64,${attachment.contentBase64}` : MISSING_IMAGE
        if (element.getAttribute('src') !== expected) return null
        if (!attachment) return { 'data-caduceus-cid': cid }
        return {
          'data-caduceus-cid': attachment.contentId,
          'data-caduceus-attachment-id': attachment.id,
          'data-caduceus-inline-image': '',
        }
      }
      return null
    },
  })
}

/** Prepare complete metadata before publishing a frame revision; no late same-revision rewrite.
 * @param {ComposeEditorSnapshot} snapshot
 * @param {Array<{id:string,text:string}>} signatures
 * @param {Set<string>} templateVariables
 * @returns {Promise<{frame:SafeHtmlFramePackage,quoteProof:QuoteProof|null}>}
 */
export async function prepareComposerFrame(snapshot, signatures, templateVariables) {
  const authority = snapshot.proof || await proofFromTrustedQuote(snapshot.quote)
  const verified = await matchQuoteProof(snapshot.document, authority)
  return {frame:composerFrame(snapshot, signatures, templateVariables, verified),quoteProof:verified?.proof || null}
}
