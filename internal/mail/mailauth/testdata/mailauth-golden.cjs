// Records what the `mailauth` npm package (the JavaScript CADUCEUS verifier)
// answers for signed and unsigned messages against a fixed DNS, so the Go
// port can be checked header for header.
// Usage: node mailauth-golden.cjs <repo>/node_modules <output.json>
const path = require('node:path')
const fs = require('node:fs')
const crypto = require('node:crypto')
const modules = process.argv[2]
const { authenticate } = require(path.join(modules, 'mailauth'))
const { dkimSign } = require(path.join(modules, 'mailauth/lib/dkim/sign'))

const rsa = crypto.generateKeyPairSync('rsa', { modulusLength: 2048 })
const weak = crypto.generateKeyPairSync('rsa', { modulusLength: 512 })
const ed = crypto.generateKeyPairSync('ed25519')
const spki = key => key.export({ type: 'spki', format: 'der' }).toString('base64')
const edRaw = key => key.export({ type: 'spki', format: 'der' }).subarray(12).toString('base64')

const dns = {
  'TXT sender.test': [['v=spf1 ip4:192.0.2.0/24 include:_spf.partner.test ~all']],
  'TXT _spf.partner.test': [['v=spf1 ip6:2001:db8::/32 a:mail.partner.test mx -all']],
  'A mail.partner.test': ['203.0.113.9'],
  'MX partner.test': [{ exchange: 'mx1.partner.test', priority: 10 }],
  'A mx1.partner.test': ['203.0.113.25'],
  'TXT _dmarc.sender.test': [['v=DMARC1; p=reject; sp=quarantine; adkim=r']],
  'TXT rsa._domainkey.sender.test': [['v=DKIM1; k=rsa; p=' + spki(rsa.publicKey)]],
  'TXT ed._domainkey.sender.test': [['v=DKIM1; k=ed25519; p=' + edRaw(ed.publicKey)]],
  'TXT weak._domainkey.sender.test': [['v=DKIM1; k=rsa; p=' + spki(weak.publicKey)]],
  'TXT other._domainkey.third.test': [['v=DKIM1; k=rsa; p=' + spki(rsa.publicKey)]],
  'TXT default._bimi.sender.test': [['v=BIMI1; l=https://sender.test/logo.svg']],
  'TXT strict.test': [['v=spf1 -all']],
  'TXT _dmarc.strict.test': [['v=DMARC1; p=none']],
  'TXT perm.test': [['v=spf1 ip4:192.0.2.1 bogus:x']],
  'TXT loop.test': [['v=spf1 redirect=loop.test']],
  'TXT sub.org.test': [],
  'TXT _dmarc.org.test': [['v=DMARC1; p=quarantine; sp=reject']],
}

const resolver = async (name, type) => {
  const key = `${type} ${name}`
  if (!(key in dns)) {
    const error = new Error(`queryTxt ENOTFOUND ${name}`)
    error.code = 'ENOTFOUND'
    throw error
  }
  const value = dns[key]
  if (!value.length) {
    const error = new Error(`queryTxt ENODATA ${name}`)
    error.code = 'ENODATA'
    throw error
  }
  return value
}

const base = [
  'From: Alice <alice@sender.test>',
  'To: bob@mx.test',
  'Subject: Quarterly   report',
  'Date: Sat, 10 Oct 2026 10:00:00 +0000',
  'Message-ID: <m1@sender.test>',
  '',
  'Hello Bob,  ',
  '',
  'Numbers\tattached. ',
  '',
  '',
].join('\r\n')

async function sign(message, entries) {
  const result = await dkimSign(Buffer.from(message), { signatureData: entries, signTime: new Date('2026-10-10T10:00:00Z') })
  if (result.errors?.length) throw result.errors[0]
  return result.signatures + message
}

;(async () => {
  const cases = []
  const add = async (name, raw, options) => {
    const result = await authenticate(Buffer.from(raw), { mta: 'mx.test', resolver, ...options, disableArc: false })
    cases.push({
      name, raw, options,
      headers: result.headers,
      spf: result.spf?.status?.result,
      dmarc: result.dmarc ? result.dmarc.status.result : null,
      dkim: result.dkim.results.map(r => r.status.result),
    })
  }
  const pass = { ip: '192.0.2.10', helo: 'mail.sender.test', sender: 'bounce@sender.test' }
  await add('unsigned spf pass', base, pass)
  await add('unsigned spf softfail', base, { ip: '198.51.100.7', helo: 'x.example', sender: 'bounce@sender.test' })
  await add('include a mechanism', base, { ip: '203.0.113.9', helo: 'mail.partner.test', sender: 'bounce@sender.test' })
  await add('include mx mechanism', base, { ip: '203.0.113.25', helo: 'mx1.partner.test', sender: 'bounce@sender.test' })
  await add('include ip6', base, { ip: '2001:db8::5', helo: 'v6.partner.test', sender: 'bounce@sender.test' })
  await add('relaxed rsa', await sign(base, [{ signingDomain: 'sender.test', selector: 'rsa', privateKey: rsa.privateKey.export({ type: 'pkcs8', format: 'pem' }), algorithm: 'rsa-sha256', canonicalization: 'relaxed/relaxed' }]), { ip: '198.51.100.7', helo: 'x.example', sender: 'bounce@sender.test' })
  await add('simple rsa sha1', await sign(base, [{ signingDomain: 'sender.test', selector: 'rsa', privateKey: rsa.privateKey.export({ type: 'pkcs8', format: 'pem' }), algorithm: 'rsa-sha1', canonicalization: 'simple/simple' }]), pass)
  await add('ed25519 relaxed/simple', await sign(base, [{ signingDomain: 'sender.test', selector: 'ed', privateKey: ed.privateKey.export({ type: 'pkcs8', format: 'pem' }), algorithm: 'ed25519-sha256', canonicalization: 'relaxed/simple' }]), pass)
  await add('body length limit', await sign(base, [{ signingDomain: 'sender.test', selector: 'rsa', privateKey: rsa.privateKey.export({ type: 'pkcs8', format: 'pem' }), algorithm: 'rsa-sha256', canonicalization: 'relaxed/relaxed', maxBodyLength: 12 }]), pass)
  const tampered = (await sign(base, [{ signingDomain: 'sender.test', selector: 'rsa', privateKey: rsa.privateKey.export({ type: 'pkcs8', format: 'pem' }), algorithm: 'rsa-sha256', canonicalization: 'relaxed/relaxed' }]))
  await add('tampered body', tampered.replace('Numbers', 'Nambers'), pass)
  await add('tampered header', tampered.replace('Quarterly', 'Annual'), pass)
  await add('weak key and foreign signer', await sign(base, [
    { signingDomain: 'sender.test', selector: 'weak', privateKey: weak.privateKey.export({ type: 'pkcs8', format: 'pem' }), algorithm: 'rsa-sha256', canonicalization: 'relaxed/relaxed' },
    { signingDomain: 'third.test', selector: 'other', privateKey: rsa.privateKey.export({ type: 'pkcs8', format: 'pem' }), algorithm: 'rsa-sha256', canonicalization: 'relaxed/relaxed' },
  ]), { ip: '198.51.100.7', helo: 'x.example', sender: 'bounce@sender.test' })
  await add('missing key', await sign(base, [{ signingDomain: 'sender.test', selector: 'gone', privateKey: rsa.privateKey.export({ type: 'pkcs8', format: 'pem' }), algorithm: 'rsa-sha256', canonicalization: 'relaxed/relaxed' }]), pass)
  await add('spf fail dmarc none policy', base.replace('alice@sender.test', 'alice@strict.test'), { ip: '192.0.2.10', helo: 'mail.strict.test', sender: 'bounce@strict.test' })
  await add('spf permerror', base, { ip: '192.0.2.1', helo: 'h.perm.test', sender: 'x@perm.test' })
  await add('spf redirect loop', base, { ip: '192.0.2.1', helo: 'h.loop.test', sender: 'x@loop.test' })
  await add('null sender uses helo', base, { ip: '192.0.2.10', helo: 'sender.test', sender: '' })
  await add('no spf record org dmarc', base.replace('alice@sender.test', 'alice@sub.org.test'), { ip: '192.0.2.10', helo: 'sub.org.test', sender: 'x@sub.org.test' })
  await add('ip helo', base, { ip: '192.0.2.10', helo: '', sender: 'bounce@sender.test' })
  await add('two from addresses', base.replace('From: Alice <alice@sender.test>', 'From: alice@sender.test, carol@sender.test'), pass)
  fs.writeFileSync(process.argv[3], JSON.stringify({ dns, cases }, null, 1) + '\n')
})().catch(error => { console.error(error); process.exit(1) })
