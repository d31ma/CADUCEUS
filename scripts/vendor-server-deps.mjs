// Bundles the npm packages the legacy JavaScript server imports into
// tests/support/legacy-server/deps/npm, and copies the client data it reads
// into tests/support/legacy-server/deps/data.
//
// The product runs the Go server; the legacy server is kept only as the test
// suites' reference and seeding container (tests/support/TestContainer.mjs),
// so only the test scripts run this. Its code imports these self-contained
// ESM bundles by relative path; the output is generated and not committed.
//
//   bun scripts/vendor-server-deps.mjs          write the bundles
//   bun scripts/vendor-server-deps.mjs --check  fail when a bundle is missing
import { copyFileSync, existsSync, mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { dirname, join, resolve } from 'node:path'

class ServerDependencies {
  static ROOT = resolve(import.meta.dirname, '..')
  static OUTPUT = join(ServerDependencies.ROOT, 'tests', 'support', 'legacy-server', 'deps', 'npm')

  /** Package specifier and the file it is bundled to. */
  static PACKAGES = [
    ['@modelcontextprotocol/server', 'modelcontextprotocol-server.mjs'],
    ['htmlparser2', 'htmlparser2.mjs'],
    ['mailparser', 'mailparser.mjs'],
    ['nodemailer', 'nodemailer.mjs'],
    ['nodemailer/lib/mail-composer/index.js', 'nodemailer-mail-composer.mjs'],
    ['rrule', 'rrule.mjs'],
    ['web-push', 'web-push.mjs'],
    ['zod', 'zod.mjs'],
  ]

  /**
   * zod (also inside the MCP SDK) probes for `new Function` and, when it
   * works, compiles validators from generated source. Tachyon refuses code
   * that runs text (TY1325), so the probe answers false and zod keeps to its
   * interpreted validators.
   */
  static EVAL_PROBE = /try\{return new Function\(""\),!0\}catch\(\w+\)\{return!1\}/g

  /** Client data the server also reads, copied so the handler snapshot holds it. */
  static DATA = [
    ['client/shared/data/keymap.json', 'tests/support/legacy-server/deps/data/keymap.json'],
  ]

  static async main() {
    if (process.argv.includes('--check')) return ServerDependencies.check()
    for (const [source, target] of ServerDependencies.DATA) {
      mkdirSync(dirname(join(ServerDependencies.ROOT, target)), { recursive: true })
      copyFileSync(join(ServerDependencies.ROOT, source), join(ServerDependencies.ROOT, target))
      console.log(`${target} <- ${source}`)
    }
    rmSync(ServerDependencies.OUTPUT, { recursive: true, force: true })
    mkdirSync(ServerDependencies.OUTPUT, { recursive: true })
    const scratch = mkdtempSync(join(ServerDependencies.ROOT, '.server-deps-'))
    try {
      for (const [specifier, file] of ServerDependencies.PACKAGES) {
        await ServerDependencies.bundle(specifier, file, scratch)
      }
    } finally {
      rmSync(scratch, { recursive: true, force: true })
    }
  }

  static check() {
    const missing = [
      ...ServerDependencies.PACKAGES.map(([, file]) => join('tests/support/legacy-server/deps/npm', file)),
      ...ServerDependencies.DATA.map(([, target]) => target),
    ].filter(file => !existsSync(join(ServerDependencies.ROOT, file)))
    if (missing.length) {
      console.error(`Missing server dependency bundles: ${missing.join(', ')}. Run bun scripts/vendor-server-deps.mjs`)
      process.exit(1)
    }
  }

  /**
   * @param {string} specifier
   * @param {string} file
   * @param {string} scratch
   */
  static async bundle(specifier, file, scratch) {
    const module = await import(specifier)
    const names = Object.keys(module).filter(name => name !== 'default' && /^[A-Za-z_$][\w$]*$/.test(name))
    const entry = join(scratch, `${file}.entry.mjs`)
    const lines = [`import * as module from ${JSON.stringify(specifier)}`]
    lines.push('export default module.default ?? module')
    if (names.length) lines.push(`export const { ${names.join(', ')} } = module`)
    writeFileSync(entry, `${lines.join('\n')}\n`)
    const result = await Bun.build({
      entrypoints: [entry],
      target: 'node',
      format: 'esm',
      minify: true,
      root: ServerDependencies.ROOT,
    })
    if (!result.success) throw new AggregateError(result.logs, `Cannot bundle ${specifier}`)
    const [output] = result.outputs
    const text = (await output.text()).replace(ServerDependencies.EVAL_PROBE, 'return!1')
    if (/new Function\(|\beval\(/.test(text)) throw new Error(`${specifier} still runs text as code after bundling`)
    writeFileSync(join(ServerDependencies.OUTPUT, file), text)
    console.log(`server/deps/npm/${file} <- ${specifier}`)
  }
}

await ServerDependencies.main()
