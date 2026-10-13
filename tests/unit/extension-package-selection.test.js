import { describe, expect, it } from 'bun:test'
import { readFileSync } from 'node:fs'
import { runInNewContext } from 'node:vm'

class PackageSelectionHarness {
  constructor() {
    this.events = []
    const file = process.env.CADUCEUS_PACKAGE_SOURCE || new URL('../../client/services/ExtensionPackageService.js', import.meta.url)
    const source = readFileSync(file, 'utf8').replace(/^\s*@\w+.*$/gm, '')
      .replace('export default class ExtensionPackageService', 'class ExtensionPackageService')
    const context = {}
    runInNewContext(`${source}\nglobalThis.service = new ExtensionPackageService();`, context)
    this.service = context.service
    this.service.applicationEventPublisher = { publishEvent: value => this.events.push(value) }
  }

  select(size = 10) {
    let resolve
    let reject
    const file = { size, text: () => new Promise((value, failure) => { resolve = value; reject = failure }) }
    const pending = this.service.read({ target: { files: [file] } })
    return { pending, finish: value => resolve(value), fail: () => reject(new Error('Read failed')) }
  }
}

describe('extension package selection order', () => {
  it('ignores an older file that finishes after the latest selection', async () => {
    const host = new PackageSelectionHarness()
    const older = host.select()
    const latest = host.select()
    latest.finish('latest package')
    await latest.pending
    older.finish('previous package')
    await older.pending
    expect(host.events).toEqual([{ text: 'latest package', problem: '' }])
  })

  it('rejecting a newer oversized package invalidates a pending older read', async () => {
    const host = new PackageSelectionHarness()
    const older = host.select()
    const newer = host.select(100000)
    await newer.pending
    older.finish('previous package')
    await older.pending
    expect(host.events).toEqual([{ text: '', problem: 'Extension package exceeds 60 KiB.' }])
  })

  it('a failed latest read clears review and ignores an older pending read', async () => {
    const host = new PackageSelectionHarness()
    const older = host.select()
    const latest = host.select()
    latest.fail()
    await latest.pending
    older.finish('previous package')
    await older.pending
    expect(host.events).toEqual([{ text: '', problem: 'Extension package could not be read.' }])
  })
})
