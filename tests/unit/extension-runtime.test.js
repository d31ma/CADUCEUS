import { describe, expect, it } from 'bun:test'
import { ExtensionRuntimeUtils } from '../../client/shared/utils/ExtensionRuntimeUtils.js'

const extensionSandboxDocument = (...args) => ExtensionRuntimeUtils.extensionSandboxDocument(...args)
const extensionThemeDeclarations = (...args) => ExtensionRuntimeUtils.extensionThemeDeclarations(...args)
const filterExtensionContext = (...args) => ExtensionRuntimeUtils.filterExtensionContext(...args)
const filterExtensionResult = (...args) => ExtensionRuntimeUtils.filterExtensionResult(...args)

describe('browser extension runtime boundary', () => {
  it('maps declarative theme colors to a fixed CSS-variable allowlist', () => {
    expect(extensionThemeDeclarations({
      kind: 'theme',
      theme: {
        mode: 'dark',
        tokens: { primary: '#88aaff', surface: '#101218', text: '#f5f7ff', unknown: '#ffffff' },
      },
    })).toEqual({
      '--ms-primary': '#88aaff',
      '--ms-surface': '#101218',
      '--ms-on-surface': '#f5f7ff',
    })
    expect(extensionThemeDeclarations({ theme: { tokens: { primary: 'url(https://tracker.test)' } } })).toEqual({})
  })

  it('reveals only context covered by the granted permissions', () => {
    const input = {
      slot: 'message-toolbar',
      mail: { id: 'm-1', subject: 'Quarterly plan', sender: 'ceo@example.com', receivedAt: '2026-08-22T00:00:00Z', body: 'secret' },
      compose: { to: 'a@example.com', cc: '', bcc: '', subject: 'Draft', text: 'private draft', html: '<img src=x>' },
    }
    expect(filterExtensionContext(['mail:metadata'], input)).toEqual({
      slot: 'message-toolbar',
      mail: { id: 'm-1', subject: 'Quarterly plan', sender: 'ceo@example.com', receivedAt: '2026-08-22T00:00:00Z' },
    })
    expect(filterExtensionContext(['compose:read'], input)).toEqual({
      slot: 'message-toolbar',
      compose: { to: 'a@example.com', cc: '', bcc: '', subject: 'Draft', text: 'private draft' },
    })
  })

  it('accepts only bounded result fields covered by granted permissions', () => {
    expect(filterExtensionResult(['ui:notify'], {
      notify: 'Done', composePatch: { subject: 'stolen' }, navigate: 'https://evil.test',
    })).toEqual({ notify: 'Done' })
    expect(filterExtensionResult(['compose:write'], {
      notify: 'not granted', composePatch: { subject: 'Updated', text: 'Hello', html: '<script>x</script>', to: 'a@example.com' },
    })).toEqual({ composePatch: { to: 'a@example.com', subject: 'Updated', text: 'Hello' } })
  })

  it('builds an opaque worker sandbox with no network and no raw source in the iframe', () => {
    const source = 'caduceus.activate({ onCommand() { return { notify: "ready" } } })'
    const document = extensionSandboxDocument({ id: 'example.follow-up', source }, 'nonce-123')
    expect(document).toContain("connect-src 'none'")
    expect(document).toContain('worker-src blob:')
    expect(document).not.toContain(source)
    expect(document).toContain('nonce-123')
  })
})
