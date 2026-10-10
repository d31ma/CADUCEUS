import { describe, expect, it } from 'bun:test'
import {
  inspectExtensionBundle,
  presentExtension,
  trustedExtensionRecord,
} from '@/shared/extensions/extensions.js'

const pluginBundle = (over = {}) => ({
  manifest: {
    schemaVersion: 1,
    id: 'example.follow-up',
    displayName: 'Follow-up helper',
    description: 'Adds a bounded toolbar command.',
    version: '1.2.3',
    kind: 'plugin',
    engine: { caduceus: '26' },
    permissions: ['ui:notify', 'mail:metadata'],
    contributions: [{ id: 'remind', label: 'Remind me', slot: 'message-toolbar' }],
  },
  source: 'caduceus.activate({ onCommand(command) { return { notify: command.id } } })',
  ...over,
})

describe('trusted extension packages', () => {
  it('normalizes a plugin into a stable digest and redacted public projection', () => {
    const first = inspectExtensionBundle(pluginBundle())
    const reordered = inspectExtensionBundle({
      source: pluginBundle().source,
      manifest: { ...pluginBundle().manifest },
    })
    expect(first).not.toBeInstanceOf(Error)
    expect(reordered).not.toBeInstanceOf(Error)
    expect(first.digest).toMatch(/^[a-f0-9]{64}$/)
    expect(reordered.digest).toBe(first.digest)
    expect(first.manifest.permissions).toEqual(['mail:metadata', 'ui:notify'])

    const stored = trustedExtensionRecord('Alice@Example.com', first, new Date('2026-08-22T10:00:00.000Z'))
    expect(stored).toMatchObject({ owner: 'alice@example.com', domain: 'example.com', enabled: false })

    const projection = presentExtension({
      ...first,
      owner: 'alice@example.com', enabled: true,
      trustedAt: '2026-08-22T10:00:00.000Z', installedAt: '2026-08-22T10:00:00.000Z', updatedAt: '2026-08-22T10:00:00.000Z',
    })
    expect(projection).not.toHaveProperty('owner')
    expect(projection).not.toHaveProperty('source')
    expect(projection.manifest).toEqual(first.manifest)
  })

  it('rejects unknown authority, executable themes, unsafe tokens, and unbounded source', () => {
    const unknownPermission = pluginBundle({ manifest: { ...pluginBundle().manifest, permissions: ['mail:delete'] } })
    expect(inspectExtensionBundle(unknownPermission)).toBeInstanceOf(Error)

    const unknownManifestField = pluginBundle({ manifest: { ...pluginBundle().manifest, homepage: 'https://tracker.test' } })
    expect(inspectExtensionBundle(unknownManifestField)).toBeInstanceOf(Error)

    const executableTheme = {
      manifest: {
        schemaVersion: 1, id: 'example.midnight', displayName: 'Midnight', version: '1.0.0', kind: 'theme',
        engine: { caduceus: '26' }, permissions: [],
        theme: { mode: 'dark', tokens: { primary: '#88aaff', surface: '#101218', text: '#f5f7ff' } },
      },
      source: 'alert(1)',
    }
    expect(inspectExtensionBundle(executableTheme)).toBeInstanceOf(Error)
    executableTheme.source = ''
    executableTheme.manifest.theme.tokens.surface = 'url(https://tracker.test/pixel)'
    expect(inspectExtensionBundle(executableTheme)).toBeInstanceOf(Error)

    expect(inspectExtensionBundle(pluginBundle({ source: 'x'.repeat(131_073) }))).toBeInstanceOf(Error)
  })
})
