import assert from 'node:assert/strict'
import nodemailer from 'nodemailer'
import { simpleParser } from 'mailparser'
import { SMTPServer } from 'smtp-server'
import tlsOptions from 'smtp-server/lib/tls-options.js'
import { SmtpDeliveryError, SmtpDirectAdapter } from '../server/features/delivery/services/smtp.js'

const deliveries = []
const { key, cert } = tlsOptions({})
const server = new SMTPServer({
  key,
  cert,
  disabledCommands: ['AUTH'],
  onRcptTo(address, _session, callback) {
    if (address.address === 'reject@wire.test') {
      callback(Object.assign(new Error('mailbox unavailable'), { responseCode: 550 }))
      return
    }
    callback()
  },
  onData(stream, session, callback) {
    const chunks = []
    stream.on('data', chunk => chunks.push(Buffer.from(chunk)))
    stream.once('error', callback)
    stream.once('end', () => {
      deliveries.push({
        raw: Buffer.concat(chunks),
        secure: session.secure,
        recipients: session.envelope.rcptTo.map(recipient => recipient.address),
      })
      callback()
    })
  },
})

/** @returns {Promise<number>} */
function listen() {
  return new Promise((resolve, reject) => {
    const failed = error => reject(error)
    server.once('error', failed)
    server.listen(0, '127.0.0.1', () => {
      server.removeListener('error', failed)
      const address = server.server.address()
      if (!address || typeof address === 'string') {
        reject(new Error('SMTP fixture did not expose a TCP port'))
        return
      }
      resolve(address.port)
    })
  })
}

/** @returns {Promise<void>} */
function close() {
  return new Promise(resolve => server.close(() => resolve()))
}

try {
  const port = await listen()
  const adapter = new SmtpDirectAdapter({
    resolveMx: async () => ['wire.test'],
    createTransport: options => nodemailer.createTransport({ ...options, host: '127.0.0.1', port }),
  })

  const result = await adapter.sendEmail('sender@local.test', {
    to: ['accept@wire.test'],
    subject: 'Unicode ✓ interoperability',
    text: 'first line\n.leading dot\nlast line',
    attachments: [{
      filename: 'résumé.txt',
      contentType: 'text/plain; charset=utf-8',
      contentBase64: Buffer.from('attachment ✓').toString('base64'),
    }],
  })
  assert.match(result.messageId, /^<.+>$/)
  assert.equal(deliveries.length, 1)
  assert.equal(deliveries[0].secure, true)
  assert.deepEqual(deliveries[0].recipients, ['accept@wire.test'])

  const parsed = await simpleParser(deliveries[0].raw)
  assert.equal(parsed.subject, 'Unicode ✓ interoperability')
  assert.match(parsed.text || '', /\.leading dot/)
  assert.equal(parsed.attachments[0].filename, 'résumé.txt')
  assert.equal(parsed.attachments[0].content.toString(), 'attachment ✓')

  let failure
  try {
    await adapter.sendEmail('sender@local.test', {
      to: ['accept@wire.test', 'reject@wire.test'],
      subject: 'Partial recipient result',
      text: 'accepted recipients still receive DATA',
    })
  } catch (error) {
    failure = error
  }
  assert.ok(failure instanceof SmtpDeliveryError)
  assert.deepEqual(failure.deliveredRecipients, ['accept@wire.test'])
  assert.deepEqual(failure.failedRecipients, ['reject@wire.test'])
  assert.equal(failure.responseCode, 550)
  assert.equal(deliveries.length, 2)

  process.stdout.write(`${JSON.stringify({
    protocol: 'ESMTP',
    starttls: true,
    mime: true,
    unicode: true,
    attachment: true,
    partialRecipientFailure: true,
  })}\n`)
} finally {
  await close()
}
