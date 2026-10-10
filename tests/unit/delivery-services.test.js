import { describe, it, expect, beforeEach, afterEach } from 'bun:test'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { join } from 'node:path'
import { tmpdir } from 'node:os'
import { SmtpDirectAdapter } from '@/shared/delivery/SmtpDirectAdapter.js'
import { ConsoleSmtpAdapter } from '@/shared/delivery/ConsoleSmtpAdapter.js'
import { SmtpRules } from '@/shared/delivery/SmtpRules.js'
import { SmtpDeliveryError } from '@/shared/errors/SmtpDeliveryError.js'
import { AcmeService } from '../../background/AcmeService.js'
import { TrackingTokens } from '@/shared/delivery/TrackingTokens.js'
import { Security } from '@/shared/security/Security.js'
import { RuleValidation } from '@/shared/domains/RuleValidation.js'
import { PushService } from '@/features/notifications/services/PushService.js'
import { PushRules } from '@/shared/notifications/PushRules.js'
import tlsOptions from 'smtp-server/lib/tls-options.js'

const MSG = { to: ['bob@other.com'], subject: 'Hi', text: 'body' }

describe('createSmtpAdapter', () => {
  const original = process.env.SMTP_ADAPTER
  afterEach(() => {
    if (original === undefined) delete process.env.SMTP_ADAPTER
    else process.env.SMTP_ADAPTER = original
  })

  it('selects the adapter named in the environment', () => {
    process.env.SMTP_ADAPTER = 'console'
    expect(SmtpRules.createSmtpAdapter()).toBeInstanceOf(ConsoleSmtpAdapter)
    process.env.SMTP_ADAPTER = 'direct'
    expect(SmtpRules.createSmtpAdapter()).toBeInstanceOf(SmtpDirectAdapter)
  })

  it('rejects an unknown adapter', () => {
    process.env.SMTP_ADAPTER = 'sendgrid'
    expect(() => SmtpRules.createSmtpAdapter()).toThrow(/Unsupported SMTP_ADAPTER/)
  })
})

describe('ConsoleSmtpAdapter', () => {
  it('returns a message id for both entry points', async () => {
    expect((await new ConsoleSmtpAdapter().send(MSG)).messageId).toMatch(/^console-/)
    expect((await new ConsoleSmtpAdapter().sendEmail('a@b.com', MSG)).messageId).toMatch(/^console-/)
  })
})

describe('groupRecipientsByDomain', () => {
  it('groups case-insensitively and skips malformed addresses', () => {
    const grouped = SmtpRules.groupRecipientsByDomain(['a@X.com', 'b@x.com', 'c@y.com', 'nope'])
    expect([...grouped.keys()]).toEqual(['x.com', 'y.com'])
    expect(grouped.get('x.com')).toEqual(['a@X.com', 'b@x.com'])
  })
})

describe('buildSmtpMessage', () => {
  it('joins address lists and decodes attachments', () => {
    const built = SmtpRules.buildSmtpMessage('me@x.com', {
      ...MSG,
      cc: ['c@x.com'],
      bcc: ['d@x.com'],
      replyTo: ['r@x.com'],
      attachments: [{ filename: 'a.txt', contentType: 'text/plain', contentBase64: Buffer.from('hi').toString('base64'), contentId: 'cid1' }],
    })
    expect(built.cc).toBe('c@x.com')
    expect(built.bcc).toBe('d@x.com')
    expect(built.replyTo).toBe('r@x.com')
    expect(built.attachments[0].content.toString()).toBe('hi')
    expect(built.attachments[0].cid).toBe('cid1')
  })

  it('leaves optional lists undefined', () => {
    const built = SmtpRules.buildSmtpMessage('me@x.com', MSG)
    expect(built.cc).toBeUndefined()
    expect(built.attachments).toBeUndefined()
  })

  it('adds encrypted recipient-level tracking only to the delivery copy', () => {
    const previousSecret = process.env.TRACKING_SECRET
    const previousUrl = process.env.CADUCEUS_PUBLIC_URL
    process.env.TRACKING_SECRET = 'smtp-tracking-secret-0123456789abcdef'
    process.env.CADUCEUS_PUBLIC_URL = 'https://mail.example.test'
    try {
      const source = '<p><a href="https://example.org/x">Visit</a></p>'
      const built = SmtpRules.buildSmtpMessage('me@x.com', {
        ...MSG,
        html: source,
        trackingId: 'smtp-tracking-000001',
        openTracking: true,
        linkTracking: true,
      }, 'bob@other.com')
      expect(source).not.toContain('/track/')
      expect(built.html).toContain('/track/click?t=')
      expect(built.html).toContain('/track/open?t=')
      const token = new URL(built.html.match(/href="([^"]+)/)[1]).searchParams.get('t')
      expect(TrackingTokens.readTrackingToken(token, { expectedType: 'click' })).toMatchObject({
        recipient: 'bob@other.com',
        url: 'https://example.org/x',
      })
    } finally {
      if (previousSecret === undefined) delete process.env.TRACKING_SECRET
      else process.env.TRACKING_SECRET = previousSecret
      if (previousUrl === undefined) delete process.env.CADUCEUS_PUBLIC_URL
      else process.env.CADUCEUS_PUBLIC_URL = previousUrl
    }
  })
})

describe('resolveMailExchangers', () => {
  it('falls back to the domain itself when there is no MX record', async () => {
    expect(await SmtpRules.resolveMailExchangers('invalid.invalid')).toEqual(['invalid.invalid'])
  })

	it('sorts MX preference, rejects null MX, and preserves transient DNS failures', async () => {
		expect(await SmtpRules.resolveMailExchangers('example.test', async () => [
			{ priority: 20, exchange: 'mx2.example.test' }, { priority: 10, exchange: 'mx1.example.test' },
		])).toEqual(['mx1.example.test', 'mx2.example.test'])
		await expect(SmtpRules.resolveMailExchangers('none.test', async () => [{ priority: 0, exchange: '.' }])).rejects.toMatchObject({ responseCode: 556 })
		await expect(SmtpRules.resolveMailExchangers('later.test', async () => { throw Object.assign(new Error('servfail'), { code: 'ESERVFAIL' }) })).rejects.toMatchObject({ responseCode: 451 })
	})
})

describe('outbound SMTP TLS policy', () => {
	const previous = process.env.SMTP_TLS_POLICY
	afterEach(() => {
		if (previous === undefined) delete process.env.SMTP_TLS_POLICY
		else process.env.SMTP_TLS_POLICY = previous
	})

	it('uses explicit opportunistic TLS by default and validated mandatory TLS when required', () => {
		delete process.env.SMTP_TLS_POLICY
		expect(SmtpRules.transportOptions('mx.example.test', 'mail.example.test')).toMatchObject({ requireTLS: false, opportunisticTLS: true, tls: { servername: 'mx.example.test', rejectUnauthorized: false } })
		process.env.SMTP_TLS_POLICY = 'require'
		expect(SmtpRules.transportOptions('mx.example.test', 'mail.example.test')).toMatchObject({ requireTLS: true, opportunisticTLS: false, tls: { rejectUnauthorized: true } })
		process.env.SMTP_TLS_POLICY = 'invalid'
		expect(() => SmtpRules.transportOptions('mx.example.test', 'mail.example.test')).toThrow(/SMTP_TLS_POLICY/)
	})
})

describe('loadDkim', () => {
  let root
  beforeEach(() => { root = mkdtempSync(join(tmpdir(), 'caduceus-dkim-')) })
  afterEach(() => {
    rmSync(root, { recursive: true, force: true })
    for (const key of ['DKIM_DOMAIN', 'DKIM_SELECTOR', 'DKIM_PRIVATE_KEY', 'DKIM_PRIVATE_KEY_FILE']) delete process.env[key]
  })

  it('returns undefined until every part is present', () => {
    expect(SmtpRules.loadDkim()).toBeUndefined()
    process.env.DKIM_DOMAIN = 'example.com'
    process.env.DKIM_SELECTOR = 'mail'
    expect(SmtpRules.loadDkim()).toBeUndefined()
  })

  it('reads an inline key', () => {
    process.env.DKIM_DOMAIN = 'example.com'
    process.env.DKIM_SELECTOR = 'mail'
    process.env.DKIM_PRIVATE_KEY = 'PEM'
    expect(SmtpRules.loadDkim()).toEqual({ domainName: 'example.com', keySelector: 'mail', privateKey: 'PEM' })
  })

  it('reads a key file and tolerates a missing one', () => {
    process.env.DKIM_DOMAIN = 'example.com'
    process.env.DKIM_SELECTOR = 'mail'
    const path = join(root, 'dkim.pem')
    writeFileSync(path, 'FILE-PEM')
    process.env.DKIM_PRIVATE_KEY_FILE = path
    expect(SmtpRules.loadDkim().privateKey).toBe('FILE-PEM')

    process.env.DKIM_PRIVATE_KEY_FILE = join(root, 'missing.pem')
    expect(SmtpRules.loadDkim()).toBeUndefined()
  })
})

describe('SmtpDirectAdapter.send', () => {
  const adapter = (over = {}) => new SmtpDirectAdapter({
    resolveMx: async domain => [`mx1.${domain}`, `mx2.${domain}`],
    createTransport: () => ({ sendMail: async message => ({ messageId: 'mid-1', accepted: message.envelope.to, response: '250 OK' }) }),
    ...over,
  })

  it('delivers to the first MX that accepts', async () => {
    const res = await adapter().send(MSG)
    expect(res.messageId).toBe('mid-1')
    expect(res.results).toHaveLength(1)
    expect(res.results[0].host).toBe('mx1.other.com')
  })

  it('falls through to the next MX after a failure', async () => {
    let attempt = 0
    const res = await adapter({
      createTransport: () => ({
        sendMail: async () => {
          if (++attempt === 1) throw new Error('connection refused')
          return { messageId: 'mid-2' }
        },
      }),
    }).send(MSG)
    expect(res.results[0].host).toBe('mx2.other.com')
  })

  it('throws once every MX for every domain has failed', async () => {
    const failing = adapter({ createTransport: () => ({ sendMail: async () => { throw new Error('nope') } }) })
    await expect(failing.send(MSG)).rejects.toThrow(/Direct SMTP delivery failed — other.com: nope/)
  })

  it('reports a non-Error failure generically', async () => {
    const failing = adapter({ createTransport: () => ({ sendMail: async () => { throw 'string throw' } }) })
    await expect(failing.send(MSG)).rejects.toThrow(/delivery failed/)
  })

  it('reports accepted and failed domains without treating partial delivery as success', async () => {
		let error
		try { await adapter({
      createTransport: ({ host }) => ({
        sendMail: async () => {
					if (host.endsWith('bad.com')) throw Object.assign(new Error('refused'), { responseCode: 550 })
          return { messageId: 'mid-3' }
        },
      }),
		}).send({ ...MSG, to: ['a@good.com'], cc: ['b@bad.com'] }) } catch (caught) { error = caught }
		expect(error).toBeInstanceOf(SmtpDeliveryError)
		expect(error.deliveredRecipients).toEqual(['a@good.com'])
		expect(error.failedRecipients).toEqual(['b@bad.com'])
		expect(error.responseCode).toBe(550)
  })

  it('accounts for a partial RCPT result from one remote host', async () => {
    const partial = adapter({ createTransport: () => ({ sendMail: async () => ({
      messageId: 'partial', accepted: ['a@other.com'], rejected: ['b@other.com'],
      rejectedErrors: [Object.assign(new Error('unknown user'), { recipient: 'b@other.com', responseCode: 550 })],
    }) }) })
    await expect(partial.send({ ...MSG, to: ['a@other.com', 'b@other.com'] })).rejects.toMatchObject({
      deliveredRecipients: ['a@other.com'], failedRecipients: ['b@other.com'], responseCode: 550,
    })
  })

  it('preserves per-recipient reply classes for mixed RCPT failures on one host', async () => {
    const partial = adapter({ createTransport: () => ({ sendMail: async () => ({
      messageId: 'mixed', accepted: ['a@other.com'], rejected: ['later@other.com', 'gone@other.com'],
      rejectedErrors: [
        Object.assign(new Error('greylisted'), { recipient: 'later@other.com', responseCode: 451 }),
        Object.assign(new Error('unknown user'), { recipient: 'gone@other.com', responseCode: 550 }),
      ],
    }) }) })
    await expect(partial.send({ ...MSG, to: ['a@other.com', 'later@other.com', 'gone@other.com'] })).rejects.toMatchObject({
      deliveredRecipients: ['a@other.com'],
      failedRecipients: ['later@other.com', 'gone@other.com'],
      responseCode: 451,
      deliveryResults: [{
        recipientFailures: [
          { recipient: 'later@other.com', responseCode: 451 },
          { recipient: 'gone@other.com', responseCode: 550 },
        ],
      }],
    })
  })

  it('sets the reply-to from the sender on sendEmail', async () => {
    let sent
    await adapter({ createTransport: () => ({ sendMail: async m => { sent = m; return { messageId: 'x' } } }) })
      .sendEmail('alice@example.com', MSG)
    expect(sent.replyTo).toBe('alice@example.com')
    expect(sent.envelope.from).toBe('alice@example.com')
  })

  it('falls back to a generated message id', async () => {
    const res = await adapter({ createTransport: () => ({ sendMail: async () => ({}) }) }).send(MSG)
    expect(res.messageId).toMatch(/^direct-/)
  })

  it('delivers tracked mail once per recipient with distinct capabilities', async () => {
    const previousSecret = process.env.TRACKING_SECRET
    const previousUrl = process.env.CADUCEUS_PUBLIC_URL
    process.env.TRACKING_SECRET = 'smtp-tracking-secret-0123456789abcdef'
    process.env.CADUCEUS_PUBLIC_URL = 'https://mail.example.test'
    const deliveries = []
    try {
      await adapter({
        createTransport: () => ({
          sendMail: async message => {
            deliveries.push(message)
            return { messageId: 'tracked-mid', accepted: message.envelope.to }
          },
        }),
      }).send({
        to: ['one@other.com', 'two@other.com'],
        cc: ['three@elsewhere.com'],
        subject: 'Tracked',
        html: '<a href="https://example.org/x">X</a>',
        trackingId: 'smtp-tracking-000002',
        openTracking: true,
        linkTracking: true,
      })
      expect(deliveries).toHaveLength(3)
      expect(deliveries.map(item => item.envelope.to)).toEqual([
        ['one@other.com'], ['two@other.com'], ['three@elsewhere.com'],
      ])
      const recipients = deliveries.map(item => {
        const token = new URL(item.html.match(/href="([^"]+)/)[1]).searchParams.get('t')
        return TrackingTokens.readTrackingToken(token, { expectedType: 'click' }).recipient
      })
      expect(recipients).toEqual(['one@other.com', 'two@other.com', 'three@elsewhere.com'])
      expect(new Set(deliveries.map(item => item.html)).size).toBe(3)
      expect(deliveries.every(item => item.to === 'one@other.com, two@other.com')).toBe(true)
    } finally {
      if (previousSecret === undefined) delete process.env.TRACKING_SECRET
      else process.env.TRACKING_SECRET = previousSecret
      if (previousUrl === undefined) delete process.env.CADUCEUS_PUBLIC_URL
      else process.env.CADUCEUS_PUBLIC_URL = previousUrl
    }
  })
})

describe('Security.assertSafeWebhookUrl', () => {
  it('accepts a public https target', () => {
    expect(Security.assertSafeWebhookUrl('https://hooks.example.com/x')).toEqual({ ok: true, host: 'hooks.example.com' })
  })

  it('rejects a non-http protocol', () => {
    expect(Security.assertSafeWebhookUrl('ftp://example.com').error).toBe('Unsafe protocol')
  })

  it('rejects loopback in every spelling', () => {
    for (const url of ['http://localhost/x', 'http://127.0.0.1/x', 'http://[::1]/x', 'http://0.0.0.0/x']) {
      expect(Security.assertSafeWebhookUrl(url).error).toBe('Webhook URL must not be localhost')
    }
  })

  it('rejects RFC 1918 addresses', () => {
    for (const host of ['10.1.2.3', '192.168.1.1', '172.16.0.1', '172.31.255.255']) {
      expect(Security.assertSafeWebhookUrl(`http://${host}/x`).error).toBe('Webhook URL must not be a private address')
    }
  })

  it('allows public addresses that merely look similar', () => {
    expect(Security.assertSafeWebhookUrl('http://172.32.0.1/x').ok).toBe(true)
    expect(Security.assertSafeWebhookUrl('http://11.0.0.1/x').ok).toBe(true)
  })

  it('rejects an unparseable URL', () => {
    expect(Security.assertSafeWebhookUrl('not a url').ok).toBe(false)
  })
})

describe('RuleValidation.validateRouteAction', () => {
  it('requires an action object', () => {
    expect(RuleValidation.validateRouteAction(null).error).toBe('Action is required')
    expect(RuleValidation.validateRouteAction('drop').error).toBe('Action is required')
  })

  it('validates a webhook action', () => {
    expect(RuleValidation.validateRouteAction({ type: 'webhook' }).error).toBe('Webhook URL is required')
    expect(RuleValidation.validateRouteAction({ type: 'webhook', url: 'http://127.0.0.1' }).error).toMatch(/localhost/)
    expect(RuleValidation.validateRouteAction({ type: 'webhook', url: 'https://hooks.example.com' }).valid).toBe(true)
  })

  it('validates a forward action', () => {
    expect(RuleValidation.validateRouteAction({ type: 'forward' }).error).toBe('Forward target is required')
    expect(RuleValidation.validateRouteAction({ type: 'forward', to: 'a@b.com' }).valid).toBe(true)
  })

  it('accepts store and drop, and rejects anything else', () => {
    expect(RuleValidation.validateRouteAction({ type: 'store' }).valid).toBe(true)
    expect(RuleValidation.validateRouteAction({ type: 'drop' }).valid).toBe(true)
    expect(RuleValidation.validateRouteAction({ type: 'quarantine' }).error).toBe('Unknown action type')
  })
})

describe('RuleValidation.validateRouteRules', () => {
  it('requires an array', () => {
    expect(RuleValidation.validateRouteRules('nope').error).toBe('Routes must be an array')
  })

  it('requires a unique id and a match on each rule', () => {
    expect(RuleValidation.validateRouteRules([{ match: '*' }]).error).toMatch(/must have an id/)
    expect(RuleValidation.validateRouteRules([{ id: 'a', match: '*', action: { type: 'store' } }, { id: 'a', match: '*' }]).error).toMatch(/Duplicate route id/)
    expect(RuleValidation.validateRouteRules([{ id: 'a', match: '  ' }]).error).toMatch(/match is required/)
  })

  it('reports the failing rule id for a bad action', () => {
    expect(RuleValidation.validateRouteRules([{ id: 'a', match: '*', action: { type: 'nope' } }]).error).toBe('Route a: Unknown action type')
  })

  it('accepts a valid rule set', () => {
    expect(RuleValidation.validateRouteRules([{ id: 'a', match: '*', action: { type: 'drop' }, enabled: true }]).valid).toBe(true)
  })
})

describe('RuleValidation.validateInboxRuleActions', () => {
  it('requires at least one action', () => {
    expect(RuleValidation.validateInboxRuleActions([]).error).toMatch(/At least one action/)
    expect(RuleValidation.validateInboxRuleActions('nope').error).toMatch(/At least one action/)
  })

  it('requires a type', () => {
    expect(RuleValidation.validateInboxRuleActions([{}]).error).toBe('Action type is required')
  })

  it('validates a folder action', () => {
    expect(RuleValidation.validateInboxRuleActions([{ type: 'folder' }]).error).toMatch(/requires a folder name/)
    expect(RuleValidation.validateInboxRuleActions([{ type: 'folder', folder: '  ' }]).error).toMatch(/requires a folder name/)
    expect(RuleValidation.validateInboxRuleActions([{ type: 'folder', folder: 'x'.repeat(61) }]).error).toMatch(/too long/)
    expect(RuleValidation.validateInboxRuleActions([{ type: 'folder', folder: 'bad folder!' }]).error).toMatch(/lowercase letters/)
    expect(RuleValidation.validateInboxRuleActions([{ type: 'folder', folder: 'Team/Reports_1' }]).valid).toBe(true)
  })

  it('validates a forward action and rejects unknown types', () => {
    expect(RuleValidation.validateInboxRuleActions([{ type: 'forward' }]).error).toMatch(/requires a to address/)
    expect(RuleValidation.validateInboxRuleActions([{ type: 'forward', to: 'a@b.com' }]).valid).toBe(true)
    expect(RuleValidation.validateInboxRuleActions([{ type: 'delete' }]).valid).toBe(true)
    expect(RuleValidation.validateInboxRuleActions([{ type: 'shred' }]).error).toMatch(/Unknown action type/)
  })
})

describe('PushService', () => {
  const service = (over = {}) => Object.assign(new PushService({ listForAddress: async () => [] }, { findByEmail: async () => [null, null] }), over)
  const email = { id: 'e1', recipient: 'alice@example.com', sender: 'bob@other.com', subject: 'Hi' }

  it('builds a deep-linked notification carrying the exact unread count', () => {
    expect(PushRules.buildNewMailPushPayload(email, 4.8)).toEqual({
      title: 'New mail from bob@other.com',
      body: 'Hi',
      data: { url: '/email/e1', emailId: 'e1', soundId: 'chime' },
      tag: 'mail:e1',
      silent: false,
      actions: [
        { action: 'mark-read', title: 'Mark read' },
        { action: 'archive', title: 'Archive' },
      ],
      badgeCount: 4,
    })
    expect(PushRules.buildNewMailPushPayload(email, undefined)).not.toHaveProperty('badgeCount')
    expect(PushRules.buildUnsnoozedPushPayload({ ...email, threadId: 'thread-1' }, 3)).toEqual({
      title: 'Snoozed conversation returned',
      body: 'Hi',
      data: { url: '/email/e1', emailId: 'e1', reason: 'snooze-expired', soundId: 'chime' },
      tag: 'snooze:thread-1',
      silent: false,
      actions: [
        { action: 'mark-read', title: 'Mark read' },
        { action: 'archive', title: 'Archive' },
      ],
      badgeCount: 3,
    })
  })

  it('builds privacy-aware notifications and honors sound and action policy', () => {
    expect(PushRules.buildNewMailPushPayload(email, 1, {
      newMailNotificationPrivacy: 'private',
      newMailNotificationActions: false,
      soundId: 'none',
    })).toEqual({
      title: 'New mail',
      body: 'Open CADUCEUS to read it.',
      data: { url: '/email/e1', emailId: 'e1', soundId: 'none' },
      tag: 'mail:e1',
      silent: true,
      badgeCount: 1,
    })
    expect(PushRules.buildNewMailPushPayload(email, 1, {
      newMailNotificationPrivacy: 'sender',
    }).body).not.toContain('Hi')
  })

  it('counts unread inbox mail through the recipient normal domain scope', async () => {
    const calls = []
    const count = await PushRules.countUnreadInbox({
      list: async (domains, filters) => {
        calls.push({ domains, filters })
        return [{ id: 'one' }, { id: 'two' }]
      },
    }, { email: 'alice@example.com', domains: ['example.com', 'subsidiary.test'], role: 'admin' }, {
      ...email,
      domain: 'example.com',
    }, 'alice@example.com')
    expect(count).toBe(2)
    expect(calls).toEqual([{
      domains: ['example.com', 'subsidiary.test'],
      filters: { folder: 'inbox', read: false, ownerEmail: 'alice@example.com' },
    }])
  })

  it('counts only the persisted focused hosted account for background badges', async () => {
    const calls = []
    const count = await PushRules.countUnreadInbox({
      list: async (domains, filters) => {
        calls.push({ domains, filters })
        return [{ id: 'focused' }]
      },
    }, {
      email: 'alice@example.com',
      domains: ['example.com', 'subsidiary.test'],
      role: 'admin',
      unreadBadgeScope: 'domain',
      unreadBadgeDomain: 'subsidiary.test',
    }, { ...email, domain: 'example.com' }, 'alice@example.com')

    expect(count).toBe(1)
    expect(calls).toEqual([{
      domains: ['subsidiary.test'],
      filters: { folder: 'inbox', read: false, ownerEmail: 'alice@example.com' },
    }])
  })

  afterEach(() => {
    delete process.env.VAPID_PUBLIC_KEY
    delete process.env.VAPID_PRIVATE_KEY
    process.env.WEB_PUSH_DISABLED = 'true'
  })

  it('returns the configured VAPID public key', async () => {
    process.env.VAPID_PUBLIC_KEY = 'configured-key'
    expect(await service().getVapidPublicKey()).toBe('configured-key')
  })

  it('generates a key when none is configured', async () => {
    delete process.env.VAPID_PUBLIC_KEY
    expect((await service().getVapidPublicKey()).length).toBeGreaterThan(20)
  })

  it('does nothing while push is disabled', async () => {
    process.env.WEB_PUSH_DISABLED = 'true'
    let listed = false
    await service({ pushSubscriptions: { listForAddress: async () => { listed = true; return [] } } }).notifyNewEmail(email)
    expect(listed).toBe(false)
  })

  it('does nothing without VAPID keys', async () => {
    process.env.WEB_PUSH_DISABLED = 'false'
    let listed = false
    await service({ pushSubscriptions: { listForAddress: async () => { listed = true; return [] } } }).notifyNewEmail(email)
    expect(listed).toBe(false)
  })

  it('stops early when the recipient has no subscriptions', async () => {
    process.env.WEB_PUSH_DISABLED = 'false'
    process.env.VAPID_PUBLIC_KEY = 'pub'
    process.env.VAPID_PRIVATE_KEY = 'priv'
    let lookedUp
    await service({
      users: { findByEmail: async addr => { lookedUp = addr; return [null, null] } },
      pushSubscriptions: { listForAddress: async () => [] },
    }).notifyNewEmail(email)
    expect(lookedUp).toBe('alice@example.com')
  })

  it('does not enumerate subscriptions when the recipient disabled alerts for that hosted account', async () => {
    process.env.WEB_PUSH_DISABLED = 'false'
    process.env.VAPID_PUBLIC_KEY = 'pub'
    process.env.VAPID_PRIVATE_KEY = 'priv'
    let listed = false
    await service({
      users: {
        findByEmail: async () => [null, {
          email: 'alice@example.com',
          domains: ['example.com', 'subsidiary.test'],
          newMailNotificationDomains: ['subsidiary.test'],
        }],
      },
      pushSubscriptions: { listForAddress: async () => { listed = true; return [] } },
    }).notifyNewEmail({ ...email, domain: 'example.com' })
    expect(listed).toBe(false)
  })

  it('honors exact connected-account notification policy without exposing provider credentials', async () => {
    process.env.WEB_PUSH_DISABLED = 'false'
    process.env.VAPID_PUBLIC_KEY = 'pub'
    process.env.VAPID_PRIVATE_KEY = 'priv'
    let listed = false
    const result = await service({
      users: { findByEmail: async () => [null, {
        email: 'alice@example.com', domains: ['example.com'],
        newMailNotificationExternalAccounts: ['ext_BBBBBBBBBBBBBBBBBBBB'],
      }] },
      pushSubscriptions: { listForAddress: async () => { listed = true; return [] } },
    }).notifyProviderNewEmail({
      accountId: 'ext_AAAAAAAAAAAAAAAAAAAA', ownerAddress: 'alice@example.com', stableId: 'INBOX:77:10',
      sender: 'remote@example.net', subject: 'Provider alert', receivedAt: new Date().toISOString(),
    })
    expect(result).toBe(true)
    expect(listed).toBe(false)
  })

  it('swallows a web-push failure', async () => {
    process.env.WEB_PUSH_DISABLED = 'false'
    process.env.VAPID_PUBLIC_KEY = 'pub'
    process.env.VAPID_PRIVATE_KEY = 'priv'
    await service({
      users: { findByEmail: async () => [null, { email: 'alice@example.com' }] },
      pushSubscriptions: { listForAddress: async () => { throw new Error('fylo down') } },
    }).notifyNewEmail(email)
  })
})

describe('AcmeService', () => {
  let root
  const options = () => ({
    hostname: 'mail.example.com',
    email: 'admin@example.com',
    certPath: join(root, 'cert.pem'),
    keyPath: join(root, 'key.pem'),
    accountPath: join(root, 'account.pem'),
    httpPort: 0,
    staging: true,
  })

  beforeEach(() => { root = mkdtempSync(join(tmpdir(), 'caduceus-acme-')) })
  afterEach(() => rmSync(root, { recursive: true, force: true }))

  /** Stand-in for acme-client — issuance talks to Let's Encrypt. */
  const fakeAcme = (over = {}) => {
    const { key, cert } = tlsOptions({})
    return ({
    crypto: {
      createPrivateKey: async () => Buffer.from('ACCOUNT-KEY'),
      createCsr: async () => [Buffer.from(key), Buffer.from('CSR')],
    },
    directory: { letsencrypt: { staging: 'https://staging.test/dir', production: 'https://prod.test/dir' } },
    Client: class {
      constructor(opts) { this.opts = opts }
      async auto({ challengeCreateFn, challengeRemoveFn }) {
        await challengeCreateFn({}, { token: 'tok' }, 'key-auth')
        await challengeRemoveFn({}, { token: 'tok' })
        return cert
      }
    },
      ...over,
    })
  }

  it('requires a hostname and contact email', () => {
    expect(() => new AcmeService({ hostname: '', email: 'a@b.com' })).toThrow(/MAIL_HOSTNAME/)
  })

  it('reports -Infinity remaining for a missing certificate', () => {
    expect(AcmeService.certMsRemaining(join(root, 'nope.pem'))).toBe(-Infinity)
  })

  it('obtains a certificate and persists the account key', async () => {
    const service = new AcmeService(options())
    expect(await service.ensureCertificate(fakeAcme())).toEqual({ obtained: true })
    expect(Bun.file(options().certPath).size).toBeGreaterThan(0)
    expect(Bun.file(options().accountPath).size).toBeGreaterThan(0)
  })

  it('reuses an existing account key on the next run', async () => {
    const service = new AcmeService(options())
    await service.ensureCertificate(fakeAcme())
    let created = false
    await service.ensureCertificate(fakeAcme({
      crypto: {
        createPrivateKey: async () => { created = true; return Buffer.from('NEW') },
        createCsr: async () => [Buffer.from(tlsOptions({}).key), Buffer.from('CSR')],
      },
    }))
    expect(created).toBe(false)
  })

  it('closes the challenge listener when issuance fails', async () => {
    const service = new AcmeService(options())
    const failing = fakeAcme({ Client: class { async auto() { throw new Error('order failed') } } })
    await expect(service.ensureCertificate(failing)).rejects.toThrow('order failed')
    // A second attempt can bind the challenge port again.
    expect(await service.ensureCertificate(fakeAcme())).toEqual({ obtained: true })
  })

  it('skips renewal while the certificate is still fresh', async () => {
    const service = new AcmeService(options())
    const original = AcmeService.certMsRemaining
    AcmeService.certMsRemaining = () => Infinity
    try {
      expect(await service.ensureCertificate(fakeAcme())).toEqual({ obtained: false, reason: 'cert still valid' })
    } finally {
      AcmeService.certMsRemaining = original
    }
  })

  it('serves an SNI context from the cert files and errors when they are missing', async () => {
    const service = new AcmeService(options())
    await service.ensureCertificate(fakeAcme())
    const call = handler => new Promise(resolve => handler('mail.example.com', (err, ctx) => resolve({ err, ctx })))

    const served = await call(AcmeService.sniFromFiles(options().keyPath, options().certPath))
    expect(served.err).toBeNull()
    expect(served.ctx).toBeDefined()

    const missing = await call(AcmeService.sniFromFiles(join(root, 'no.pem'), join(root, 'no.pem')))
    expect(missing.err).toBeInstanceOf(Error)
  })

  it('starts and stops the renewal timer once', async () => {
    const service = new AcmeService(options())
    service.ensureCertificate = async () => ({ obtained: false })
    await service.startRenewal()
    const timer = service.renewalTimer
    await service.startRenewal()
    expect(service.renewalTimer).toBe(timer)
    service.stop()
    service.stop()
    expect(service.renewalTimer).toBeUndefined()
  })
})
