// Send one test email to a running CADUCEUS inbound MX — e.g. the docker compose
// stack on localhost:25 — to exercise the standalone mail server end to end.
//
// Usage:
//   bun scripts/send-test-mail.mjs [to] [host] [port]
//   bun scripts/send-test-mail.mjs admin@caduceus.com localhost 25
import nodemailer from 'nodemailer'

const [to = 'admin@caduceus.com', host = 'localhost', port = '25'] = process.argv.slice(2)

const transport = nodemailer.createTransport({
  host,
  port: Number(port),
  secure: false,
  ignoreTLS: true,
  tls: { rejectUnauthorized: false },
})

const info = await transport.sendMail({
  from: 'Test Sender <tester@example.com>',
  to,
  subject: `Test message ${new Date().toISOString()}`,
  text: 'Hello from send-test-mail.mjs over the standalone MX.',
  html: '<p>Hello from <b>send-test-mail.mjs</b> over the standalone MX.</p>',
})

console.log(`→ ${to} via ${host}:${port}`)
console.log('response:', info.response)
console.log('accepted:', info.accepted)
