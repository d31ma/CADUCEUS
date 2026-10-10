import { afterAll, beforeAll, describe, expect, it } from 'bun:test'
import { req, setupRoutes, teardownRoutes, token } from './helpers.js'
import { ExtensionsController as Extensions } from '@/controllers/ExtensionsController.js'
import { ExtensionsInspectController as Inspect } from '@/controllers/ExtensionsInspectController.js'
import { ExtensionsIdController as Extension } from '@/controllers/ExtensionsIdController.js'
import { call } from '../support/controller-call.mjs'

const alice = () => token()
const bob = () => token({ email: 'bob@example.com' })
const plugin = {
  manifest: {
    schemaVersion: 1,
    id: 'example.follow-up',
    displayName: 'Follow-up helper',
    description: 'Adds a toolbar command.',
    version: '1.0.0',
    kind: 'plugin',
    engine: { caduceus: '26' },
    permissions: ['ui:notify', 'mail:metadata'],
    contributions: [{ id: 'remind', label: 'Remind me', slot: 'message-toolbar' }],
  },
  source: 'caduceus.activate({ onCommand(command) { return { notify: command.id } } })',
}
const theme = (id, primary) => ({
  manifest: {
    schemaVersion: 1, id, displayName: id, description: '', version: '1.0.0', kind: 'theme',
    engine: { caduceus: '26' }, permissions: [],
    theme: { mode: 'dark', tokens: { primary, surface: '#101218', text: '#f5f7ff' } },
  },
  source: '',
})

beforeAll(async () => {
  const app = await setupRoutes()
  await app.users.put({ email: 'alice@example.com', domains: ['example.com'], role: 'admin' })
  await app.users.put({ email: 'bob@example.com', domains: ['example.com'], role: 'admin' })
}, 300_000)
afterAll(teardownRoutes)

describe('trusted extension lifecycle', () => {
  it('requires inspection and the exact explicit-trust digest before installation', async () => {
    const inspected = await call(Inspect, 'POST', req({ token: alice(), body: { bundle: plugin } }))
    expect(inspected.digest).toMatch(/^[a-f0-9]{64}$/)
    expect(inspected.manifest.id).toBe('example.follow-up')
    expect(inspected).not.toHaveProperty('source')

    const denied = await call(Extensions, 'POST', req({ token: alice(), body: { bundle: plugin, trustedDigest: '0'.repeat(64) } }))
    expect(denied.error).toMatch(/digest/i)

    const installed = await call(Extensions, 'POST', req({ token: alice(), body: { bundle: plugin, trustedDigest: inspected.digest } }))
    expect(installed).toMatchObject({ id: 'example.follow-up', digest: inspected.digest, enabled: false })
    expect(installed).not.toHaveProperty('source')
  })

  it('isolates packages by exact mailbox owner and exposes source only to the runtime endpoint', async () => {
    expect(await call(Extensions, 'GET', req({ token: bob() }))).toEqual([])
    const aliceList = await call(Extensions, 'GET', req({ token: alice() }))
    expect(aliceList).toHaveLength(1)
    expect(aliceList[0]).not.toHaveProperty('source')

    const foreign = await call(Extension, 'GET', req({ token: bob(), paths: { id: 'example.follow-up' } }))
    expect(foreign.notFound).toBe(true)
    const runtime = await call(Extension, 'GET', req({ token: alice(), paths: { id: 'example.follow-up' } }))
    expect(runtime.source).toContain('caduceus.activate')
  })

  it('enables plugins and keeps only one owner theme active', async () => {
    const enabled = await call(Extension, 'PUT', req({ token: alice(), paths: { id: 'example.follow-up' }, body: { enabled: true } }))
    expect(enabled.enabled).toBe(true)

    for (const bundle of [theme('example.midnight', '#88aaff'), theme('example.ocean', '#00aacc')]) {
      const inspected = await call(Inspect, 'POST', req({ token: alice(), body: { bundle } }))
      await call(Extensions, 'POST', req({ token: alice(), body: { bundle, trustedDigest: inspected.digest } }))
      await call(Extension, 'PUT', req({ token: alice(), paths: { id: bundle.manifest.id }, body: { enabled: true } }))
    }
    const list = await call(Extensions, 'GET', req({ token: alice() }))
    expect(list.find(item => item.id === 'example.midnight').enabled).toBe(false)
    expect(list.find(item => item.id === 'example.ocean').enabled).toBe(true)
  })

  it('removes only an exact-owner package', async () => {
    expect((await call(Extension, 'DELETE', req({ token: bob(), paths: { id: 'example.follow-up' } }))).notFound).toBe(true)
    expect(await call(Extension, 'DELETE', req({ token: alice(), paths: { id: 'example.follow-up' } }))).toEqual({ deleted: true })
    expect((await call(Extensions, 'GET', req({ token: alice() }))).some(item => item.id === 'example.follow-up')).toBe(false)
  })
})
