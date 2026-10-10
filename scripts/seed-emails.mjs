import { createHmac, randomUUID } from 'node:crypto'

const API = process.env.SEED_API || 'http://127.0.0.1:9876'
const INBOUND_SECRET = process.env.INBOUND_WEBHOOK_SECRET || 'caduceus-e2e-inbound-secret'
const DOMAIN = 'caduceus.com'
const ADMIN = `admin@${DOMAIN}`

async function api(path, opts = {}) {
  const res = await fetch(`${API}${path}`, opts)
  const text = await res.text()
  let body
  try { body = JSON.parse(text) } catch { body = text }
  if (!res.ok) throw new Error(`${opts.method || 'GET'} ${path} failed (${res.status}): ${text}`)
  return { status: res.status, body }
}

function requireField(result, field, operation) {
  const value = result.body?.[field]
  if (!value) throw new Error(`${operation} did not return ${field}`)
  return value
}

async function inboundIngest(msg, recipient) {
  const body = { recipient, ...msg }
  const json = JSON.stringify(body)
  const sig = createHmac('sha256', INBOUND_SECRET).update(json).digest('hex')
  return api('/inbound/webhook', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', 'X-Caduceus-Signature': sig },
    body: json,
  })
}

function mimeWithAttachment(text, filename, contentType, content) {
  const boundary = `caduceus-seed-${randomUUID()}`
  return [
    'MIME-Version: 1.0',
    `Content-Type: multipart/mixed; boundary="${boundary}"`,
    '',
    `--${boundary}`,
    'Content-Type: text/plain; charset=utf-8',
    'Content-Transfer-Encoding: 8bit',
    '',
    text,
    `--${boundary}`,
    `Content-Type: ${contentType}; name="${filename}"`,
    'Content-Transfer-Encoding: base64',
    `Content-Disposition: attachment; filename="${filename}"`,
    '',
    Buffer.from(content).toString('base64'),
    `--${boundary}--`,
    '',
  ].join('\r\n')
}

console.log('Resetting...')
await api('/test/reset', { method: 'DELETE' })

console.log('Seeding admin user...')
const seedRes = await api('/test/seed/user', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ email: ADMIN, domains: [DOMAIN], role: 'admin' }),
})
const TOKEN = requireField(seedRes, 'token', 'Admin seed')
const auth = { Authorization: `Bearer ${TOKEN}` }

console.log('Creating caduceus.com domain...')
const domainRes = await api('/domains', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json', ...auth },
  body: JSON.stringify({
    domain: DOMAIN,
    inboundEnabled: true,
    routes: [{ id: `store-${DOMAIN}`, match: `*@${DOMAIN}`, action: { type: 'store' }, enabled: true }],
  }),
})
requireField(domainRes, 'domain', 'Domain seed')

const inboxMessages = [
  { sender: 'alice@partner.io',     subject: 'Welcome to Caduceus',                body: 'Thanks for signing up — this is your shiny new inbox.' },
  { sender: 'bob@vendor.co',        subject: 'Invoice #4821 attached',             body: mimeWithAttachment('Hi Admin,\n\nPlease find invoice #4821 attached. Net 30.\n\n— Bob', 'invoice-4821.pdf', 'application/pdf', '%PDF-1.4\nCADUCEUS seed invoice #4821\n%%EOF') },
  { sender: 'newsletter@stripe.dev',subject: 'Weekly digest: 5 new docs you might like', body: 'This week in API land: webhooks, idempotency keys, and a brand-new SDK.' },
  { sender: 'team@github.com',      subject: '[caduceus/caduceus] PR #128 needs review', body: 'd31ma opened a new pull request. Two files changed.' },
  { sender: 'security@auth0.com',   subject: 'New sign-in from Edinburgh',         body: 'A new sign-in was detected at 09:14 UTC from Edinburgh, UK.' },
  { sender: 'noreply@calendar.app', subject: 'Standup tomorrow at 10:00',          body: 'Reminder: Engineering standup tomorrow at 10:00 AM.' },
  { sender: 'support@figma.com',    subject: 'Your design file was duplicated',    body: 'Someone duplicated your file "Caduceus / Inbox".' },
  { sender: 'careers@anthropic.com',subject: 'We received your application',       body: 'Thanks for applying to the Forward Deployed Engineer role.' },
  { sender: 'mom@family.net',       subject: 'Re: Dinner Sunday?',                 body: 'Sounds good — see you at 6pm. Bring wine.' },
  { sender: 'billing@hosting.example', subject: 'Your hosting bill is ready',       body: mimeWithAttachment('Your monthly bill for May is $12.42. Itemised breakdown attached.', 'hosting-bill-may-2026.pdf', 'application/pdf', '%PDF-1.4\nCADUCEUS seed hosting bill\n%%EOF') },
]

console.log('Seeding inbox emails...')
const inboxIds = []
for (const m of inboxMessages) {
  const r = await inboundIngest(m, ADMIN)
  inboxIds.push(requireField(r, 'emailId', `Inbox seed: ${m.subject}`))
}

console.log('Moving emails into other folders...')
const moves = [
  { idx: 0, folder: 'archive' },
  { idx: 1, folder: 'snoozed' },
  { idx: 2, folder: 'trash' },
  { idx: 3, folder: 'spam' },
]
for (const { idx, folder } of moves) {
  const id = inboxIds[idx]
  if (!id) continue
  await api(`/inbox/${id}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json', ...auth },
    body: JSON.stringify({ folder }),
  })
}

await api(`/inbox/${inboxIds[4]}`, {
  method: 'PUT',
  headers: { 'Content-Type': 'application/json', ...auth },
  body: JSON.stringify({ read: true, starred: true }),
})
await api(`/inbox/${inboxIds[5]}`, {
  method: 'PUT',
  headers: { 'Content-Type': 'application/json', ...auth },
  body: JSON.stringify({ read: true }),
})

const draftMessages = [
  { sender: ADMIN, subject: 'Draft: Q3 planning notes',     body: 'Some thoughts on Q3 priorities...\n\n[Incomplete]' },
  { sender: ADMIN, subject: 'Re: Onboarding doc feedback',  body: 'Adding a few inline comments before sending back.' },
  { sender: ADMIN, subject: '',                             body: 'Untitled draft body...' },
]

console.log('Seeding drafts...')
for (const m of draftMessages) {
  const r = await inboundIngest(m, ADMIN)
  const id = r.body?.emailId
  if (id) {
    await api(`/inbox/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json', ...auth },
      body: JSON.stringify({ folder: 'drafts' }),
    })
  }
}

const sentMessages = [
  { to: ['ops@partner.io'],   subject: 'Deployment summary — May 17',     text: 'Production deploy went smoothly. See dashboard for details.' },
  {
    to: ['legal@vendor.co'],
    subject: 'Re: NDA draft',
    text: 'Two small redlines attached. Otherwise looks good.',
    attachments: [{
      filename: 'nda-redlines.txt',
      contentType: 'text/plain',
      contentBase64: Buffer.from('Section 4: clarify the thirty-day notice period.').toString('base64'),
    }],
  },
  { to: ['alice@partner.io'], subject: 'Welcome aboard!',                  text: 'Excited to have you on the team.' },
  { to: ['team@github.com'],  subject: 'Re: [caduceus/caduceus] PR #128',  text: 'Approving — thanks for the quick turnaround.' },
]

console.log('Seeding sent emails...')
let trackId = 0
for (const m of sentMessages) {
  const result = await api('/send', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...auth },
    body: JSON.stringify({ ...m, trackingId: `seed-track-${++trackId}` }),
  })
  requireField(result, 'messageId', `Sent seed: ${m.subject}`)
}

const tomorrow = new Date(Date.now() + 26 * 60 * 60 * 1000).toISOString()
const nextWeek = new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString()
const scheduledMessages = [
  { to: ['client@partner.io'], subject: 'Monthly status report',  text: 'Auto-generated monthly status...', sendAt: tomorrow },
  { to: ['team@caduceus.com'], subject: 'Weekly OKRs reminder',   text: 'Remember to update your OKRs in Linear.', sendAt: nextWeek },
]

console.log('Seeding scheduled emails...')
for (const m of scheduledMessages) {
  const result = await api('/send', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...auth },
    body: JSON.stringify(m),
  })
  requireField(result, 'id', `Scheduled seed: ${m.subject}`)
}

console.log('\n✓ Seed complete')
console.log(`  Admin: ${ADMIN}`)
