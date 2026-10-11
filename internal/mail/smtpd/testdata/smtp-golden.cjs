// Records how the Node smtp-server (the JavaScript CADUCEUS SMTP layer) answers
// scripted sessions, so the Go state machine can be checked against it.
// Usage: node smtp-golden.cjs <repo>/node_modules <output.json>
const path = require('node:path')
const net = require('node:net')
const fs = require('node:fs')
const modules = process.argv[2]
const { SMTPServer } = require(path.join(modules, 'smtp-server'))

const long = 'X'.repeat(5000)

const configs = {
  inbound: {
    name: 'mx.test',
    banner: 'mx.test ESMTP CADUCEUS',
    size: 64,
    authOptional: true,
    disabledCommands: ['AUTH'],
    hideSTARTTLS: true,
  },
  submission: {
    name: 'smtp.test',
    banner: 'smtp.test ESMTP CADUCEUS submission',
    size: 64,
    authMethods: ['PLAIN', 'LOGIN'],
    allowInsecureAuth: true,
  },
}

const scenarios = [
  { config: 'inbound', name: 'basics', steps: [
    'EHLO Client.Test', 'HELO', 'HELO client.test', 'EHLO a b', 'NOOP', 'HELP', 'VRFY someone',
    'RSET', 'FOO bar', 'WIZ', 'SHELL', 'WIZ secret', 'SHELL', 'KILL', 'AUTH PLAIN AGZvbwBiYXI=', 'QUIT',
  ] },
  { config: 'inbound', name: 'sequence', steps: [
    'MAIL FROM:<a@b.test>', 'RCPT TO:<a@b.test>', 'DATA', 'EHLO client.test',
    'RCPT TO:<a@b.test>', 'DATA', 'MAIL FROM:<bad>', 'MAIL FROM:a@b.test', 'MAIL TO:<a@b.test>',
    'MAIL FROM:<a@b.test> SIZE=100', 'MAIL FROM:<a@b.test> SIZE', 'RSET', 'MAIL FROM:<a@b.test> SIZE=abc', 'RSET',
    'MAIL FROM:<a@b.test> BODY=BINARY', 'RSET',
    'MAIL FROM:<a@b.test> SMTPUTF8=yes', 'MAIL FROM:<a@b.test> REQUIRETLS', 'MAIL FROM:<.a@b.test>',
    'MAIL FROM:<a..b@b.test>', 'MAIL FROM:<a@b..test>', 'MAIL FROM:<a@-b.test>', 'RSET', 'MAIL FROM:<a@b@c.test>',
    'MAIL FROM:<a@[1.2.3.999]>', 'MAIL FROM:<a@b.test> ENVID=x+0D+0Ay', 'MAIL FROM:<a@b_c.test>', 'MAIL FROM:<a b@c.test>',
    'MAIL FROM:<a@[IPv6:zz]>', 'MAIL FROM:<a@b.test> =x X=1=2', 'RSET',
    'MAIL FROM: <a@xn--bcher-kva.test> SIZE=10 BODY=8BITMIME SMTPUTF8', 'MAIL FROM:<x@y.test>',
    'RCPT TO:<>', 'RCPT TO:<c@[1.2.3.4]>', 'RCPT TO:<c@[IPv6:2001:db8:0::1]>', 'RCPT TO:<C@d.test>',
    'RCPT TO:<c@d.test>', 'RSET', 'DATA', 'QUIT',
  ] },
  { config: 'inbound', name: 'data', steps: [
    'EHLO client.test', 'MAIL FROM:<>', 'RCPT TO:<u@d.test>', 'DATA',
    { raw: 'Subject: hi\r\n\r\n..dot\r\n.\nbare\n.\r\nlast\r\n.\r\n' },
    'MAIL FROM:<s@d.test>', 'RCPT TO:<u@d.test>', 'DATA',
    { raw: 'Subject: big\r\n\r\n' + 'y'.repeat(100) + '\r\n.\r\n' },
    'MAIL FROM:<s@d.test>', 'RCPT TO:<u@d.test>', 'DATA', { raw: '.\r\n' }, 'QUIT',
  ] },
  { config: 'inbound', name: 'http', steps: ['GET / HTTP/1.1'] },
  { config: 'inbound', name: 'unrecognized', steps: Array.from({ length: 10 }, (_, i) => `NOPE${i}`) },
  { config: 'inbound', name: 'long-line', steps: [long] },
  { config: 'inbound', name: 'early', early: true, steps: ['EHLO client.test'] },
  { config: 'submission', name: 'auth', steps: [
    'AUTH PLAIN x', 'EHLO client.test', 'MAIL FROM:<a@b.test>', 'AUTH CRAM-MD5', 'AUTH XOAUTH2 x', 'AUTH',
    'AUTH PLAIN a b', 'AUTH PLAIN bm9udWxscw==', 'AUTH PLAIN AHVzZXIAcGFzcw==', 'AUTH PLAIN', '*', 'AUTH PLAIN',
    'AHVzZXIAcGFzcw==', 'AUTH LOGIN', 'dXNlcg==', 'cGFzcw==', 'AUTH LOGIN', '', 'AUTH LOGIN dXNlcg==', '*',
  ] },
  { config: 'submission', name: 'unauthenticated', steps: Array.from({ length: 10 }, () => 'NOOP') },
]

function serverFor(name, messages) {
  return new SMTPServer({
    ...configs[name],
    disableReverseLookup: true,
    logger: false,
    onAuth(auth, session, callback) {
      callback(new Error(`refused ${auth.method} ${auth.username} ${auth.password}`))
    },
    onData(stream, session, callback) {
      const chunks = []
      stream.on('data', c => chunks.push(c))
      stream.on('end', () => {
        const raw = Buffer.concat(chunks)
        messages.push(raw.toString('base64'))
        if (stream.sizeExceeded) {
          const error = new Error('Message exceeds maximum permitted size')
          error.responseCode = 552
          return callback(error)
        }
        callback()
      })
    },
  })
}

function run(server, scenario) {
  return new Promise((resolve, reject) => {
    server.listen(0, '127.0.0.1', () => {
      const port = server.server.address().port
      const socket = net.connect(port, '127.0.0.1')
      let buffer = ''
      const replies = []
      let waiting = null
      let closed = false
      const take = () => {
        if (!waiting) return
        const lines = buffer.split('\r\n')
        for (let i = 0; i < lines.length - 1; i++) {
          if (/^\d{3}( |$)/.test(lines[i])) {
            const reply = lines.slice(0, i + 1)
            buffer = lines.slice(i + 1).join('\r\n')
            const done = waiting
            waiting = null
            done(reply)
            return
          }
        }
        if (closed) {
          const done = waiting
          waiting = null
          done(buffer ? [buffer, '<closed>'] : ['<closed>'])
          buffer = ''
        }
      }
      const next = () => new Promise(r => { waiting = r; take() })
      socket.on('data', d => { buffer += d.toString('binary'); take() })
      socket.on('close', () => { closed = true; take() })
      socket.on('error', () => {})
      ;(async () => {
        if (scenario.early) {
          socket.write(scenario.steps[0] + '\r\n')
          replies.push({ send: scenario.steps[0], reply: await next() })
          replies.push({ send: null, reply: await next() })
        } else {
          replies.push({ send: null, reply: await next() })
          for (const step of scenario.steps) {
            if (closed && !buffer) break
            const text = typeof step === 'string' ? step + '\r\n' : step.raw
            socket.write(text)
            const reply = await next()
            replies.push({ send: typeof step === 'string' ? step : { raw: step.raw }, reply })
            if (reply[reply.length - 1] === '<closed>') break
          }
        }
        socket.destroy()
        server.close(() => resolve(replies))
      })().catch(reject)
    })
  })
}

;(async () => {
  const out = []
  for (const scenario of scenarios) {
    const messages = []
    const server = serverFor(scenario.config, messages)
    const replies = await run(server, scenario)
    out.push({ config: scenario.config, name: scenario.name, early: !!scenario.early, exchanges: replies, messages })
  }
  fs.writeFileSync(process.argv[3], JSON.stringify(out, null, 2) + '\n')
})()
