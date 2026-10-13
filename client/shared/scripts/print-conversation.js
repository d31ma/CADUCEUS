// @ts-check

import { formatPreferenceDate } from './mail-preferences.js'
import { formatNumber } from './localization.js'

/**
 * Load the complete authorized conversation for a message. Printing remains
 * available offline or during a transient API failure by falling back to the
 * open message.
 *
 * @param {PrintableEmail} currentEmail
 * @param {((path: string) => Promise<Response | null>) | null | undefined} apiFetch
 * @returns {Promise<PrintableConversation>}
 */
export async function loadPrintableConversation(currentEmail, apiFetch) {
  const fallback = singleMessageConversation(currentEmail)
  if (!currentEmail?.id || !apiFetch) return fallback

  try {
    const response = await apiFetch(`/threads?emailId=${encodeURIComponent(currentEmail.id)}`)
    if (!response?.ok) return fallback
    const threads = await response.json()
    const thread = Array.isArray(threads) ? threads[0] : null
    if (!thread || !Array.isArray(thread.emails)) return fallback
    if (!thread.emails.some(email => email?.id === currentEmail.id)) return fallback
    return {
      subject: text(thread.subject) || text(currentEmail.subject),
      count: thread.emails.length,
      latestEmail: thread.latestEmail || thread.emails[thread.emails.length - 1],
      emails: thread.emails,
    }
  } catch {
    return fallback
  }
}

/**
 * Choose the complete authorized conversation for a message when the reader
 * already holds it, and fall back to the open message alone.
 *
 * @param {PrintableEmail} currentEmail
 * @param {PrintableEmail[]} conversation
 * @returns {PrintableConversation}
 */
export function printableConversation(currentEmail, conversation) {
  const emails = Array.isArray(conversation) ? conversation : []
  if (!emails.some(email => email?.id === currentEmail?.id)) return singleMessageConversation(currentEmail)
  return {
    subject: text(currentEmail?.subject),
    count: emails.length,
    latestEmail: emails[emails.length - 1],
    emails,
  }
}

/**
 * Render a standalone, script-free print document. Email HTML is converted to
 * inert text when no plain-text body exists. Remote images, hidden headers,
 * tracking identifiers, authentication state, and application chrome never
 * enter the document.
 *
 * @param {PrintableConversation} conversation
 * @param {{ account?: string }} [options]
 * @returns {string}
 */
export function renderPrintDocument(conversation, options = {}) {
  const messages = Array.isArray(conversation?.emails)
    ? conversation.emails.filter(message => message && typeof message === 'object')
      .sort(compareReceivedAt)
    : []
  const subject = text(conversation?.subject) || text(messages[0]?.subject) || '(No subject)'
  const participants = collectParticipants(messages)
  const account = text(options.account)
  const messageCount = messages.length

  return `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src 'self'; img-src 'none'; font-src 'none'; base-uri 'none'; form-action 'none'">
  <title>${escapeHtml(subject)} — CADUCEUS</title>
  <link rel="stylesheet" href="/shared/assets/print.css">
</head>
<body>
  <main class="caduceus-print">
    <header class="print-conversation-header">
      <p class="print-product">CADUCEUS conversation</p>
      <h1>${escapeHtml(subject)}</h1>
      <dl class="print-summary">
        ${account ? `<div><dt>Mailbox</dt><dd>${escapeHtml(account)}</dd></div>` : ''}
        <div><dt>Participants</dt><dd>${escapeHtml(participants.join(', ') || 'Unknown')}</dd></div>
        <div><dt>Messages</dt><dd>${messageCount} ${messageCount === 1 ? 'message' : 'messages'}</dd></div>
      </dl>
      <p class="print-privacy-note">Privacy-safe print view: remote images, tracking metadata, and hidden application data are omitted.</p>
    </header>
    <section class="print-messages" aria-label="Conversation messages">
      ${messages.map(renderMessage).join('\n')}
    </section>
  </main>
</body>
</html>`
}

/**
 * Open the rendered document in a short-lived same-origin window and invoke
 * the browser's native print dialog (which also provides Save as PDF).
 *
 * @param {string} documentHtml
 * @param {{ open: (url?: string, target?: string, features?: string) => Window | null }} [opener]
 * @param {Window | null} [reservedWindow]
 * @returns {Window}
 */
export function openPrintConversation(documentHtml, opener = window, reservedWindow = null) {
  const popup = reservedWindow || opener.open('', '_blank', 'popup')
  if (!popup) throw new Error('The print window was blocked')

  try { popup.opener = null } catch { /* the new document remains script-free */ }
  const address = URL.createObjectURL(new Blob([documentHtml], { type: 'text/html' }))
  popup.addEventListener('load', () => {
    popup.focus()
    popup.print()
    URL.revokeObjectURL(address)
  }, { once: true })
  popup.location.replace(address)
  return popup
}

/** @param {PrintableEmail} email @returns {PrintableConversation} */
function singleMessageConversation(email) {
  return {
    subject: text(email?.subject),
    count: 1,
    latestEmail: email,
    emails: [email],
  }
}

/** @param {PrintableEmail} left @param {PrintableEmail} right */
function compareReceivedAt(left, right) {
  return text(left.receivedAt).localeCompare(text(right.receivedAt))
}

/** @param {PrintableEmail[]} messages */
function collectParticipants(messages) {
  const participants = new Set()
  for (const message of messages) {
    for (const value of [message.sender, message.recipient]) {
      const participant = text(value).trim()
      if (participant) participants.add(participant)
    }
  }
  return [...participants]
}

/** @param {PrintableEmail} message */
function renderMessage(message) {
  const sender = text(message.sender) || 'Unknown sender'
  const recipient = text(message.recipient) || 'Unknown recipient'
  const body = messageBody(message) || '(No body)'
  const attachments = Array.isArray(message.attachments) ? message.attachments : []
  return `<article class="print-message">
    <header class="print-message-header">
      <h2>${escapeHtml(text(message.subject) || '(No subject)')}</h2>
      <dl>
        <div><dt>From</dt><dd>${escapeHtml(sender)}</dd></div>
        <div><dt>To</dt><dd>${escapeHtml(recipient)}</dd></div>
        <div><dt>Date</dt><dd>${escapeHtml(formatDate(message.receivedAt))}</dd></div>
      </dl>
    </header>
    <div class="print-message-body">${escapeHtml(body)}</div>
    ${attachments.length ? `<section class="print-attachments" aria-label="Attachments">
      <h3>Attachments</h3>
      <ul>${attachments.map(attachment => `<li>${escapeHtml(text(attachment?.filename) || 'attachment')} <span>(${escapeHtml(bytesLabel(attachment?.size))})</span></li>`).join('')}</ul>
    </section>` : ''}
  </article>`
}

/** @param {PrintableEmail} message */
function messageBody(message) {
  const body = text(message.body)
  if (body.trim()) return body
  return htmlToText(text(message.htmlBody))
}

/** @param {string} html */
function htmlToText(html) {
  return html
    .replace(/<(script|style|head|template|noscript)\b[^>]*>[\s\S]*?<\/\1\s*>/gi, ' ')
    .replace(/<(br|hr)\b[^>]*\/?>/gi, '\n')
    .replace(/<\/(p|div|li|tr|h[1-6])\s*>/gi, '\n')
    .replace(/<[^>]+>/g, '')
    .replace(/&nbsp;|&#160;/gi, ' ')
    .replace(/&amp;/gi, '&')
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&quot;|&#34;/gi, '"')
    .replace(/&#39;|&apos;/gi, "'")
    .replace(/\n[ \t]+/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim()
}

/** @param {unknown} value */
function formatDate(value) {
  const raw = text(value)
  return formatPreferenceDate(raw) || raw || 'Unknown date'
}

/** @param {unknown} value */
function bytesLabel(value) {
  const bytes = typeof value === 'number' && Number.isFinite(value) && value >= 0 ? value : 0
  if (bytes < 1024) return `${formatNumber(bytes)} B`
  if (bytes < 1024 * 1024) return `${formatNumber(Math.round(bytes / 1024))} KB`
  return `${formatNumber(Math.round((bytes / (1024 * 1024)) * 10) / 10, { maximumFractionDigits: 1 })} MB`
}

/** @param {unknown} value */
function text(value) {
  return typeof value === 'string' ? value : ''
}

/** @param {string} value */
function escapeHtml(value) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}
