import { appendFile, readFile } from 'node:fs/promises'
import { resolve } from 'node:path'
import { execFile } from 'node:child_process'
import { promisify } from 'node:util'
import { fileURLToPath } from 'node:url'

const execFileAsync = promisify(execFile)
const projectRoot = fileURLToPath(new URL('.', import.meta.url))

/**
 * Extend Tachyon's generated root service worker without editing generated
 * artifacts. The hook runs inside Tachyon's atomic staging directory, so a
 * failed append cannot publish a half-built PWA.
 */
export async function postBundle({ targetRoots }) {
  const source = await readFile(new URL('./client/shared/service-worker/unread.js', import.meta.url), 'utf8')
  await appendFile(`${targetRoots.web}/tachyon-sw.js`, `\n${source.trim()}\n`)

  // Tachyon intentionally publishes shared JavaScript as standards-native ESM
  // and does not rewrite bare npm specifiers. Produce two self-contained ESM
  // adapters inside its atomic staging tree: detection remains eager and
  // small, while the much larger inference engine stays behind dynamic import.
  const entries = [
    ['translation-detection-vendor.js', 'translation-detection-vendor.js'],
    ['translation-engine-vendor.js', 'translation-engine-vendor.js'],
  ]
  for (const [sourceName, outputName] of entries) {
    await execFileAsync(process.env.BUN_BINARY || 'bun', [
      'build',
      resolve(projectRoot, 'client/shared/scripts', sourceName),
      '--outfile', resolve(targetRoots.web, 'shared/scripts', outputName),
      '--target', 'browser',
      '--format', 'esm',
      '--minify',
    ], { cwd: projectRoot })
  }
}
