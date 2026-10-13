#!/usr/bin/env bun

import { createHash } from 'node:crypto'
import {
  cpSync,
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs'
import { dirname, join, relative, resolve } from 'node:path'

const root = resolve(import.meta.dir, '..')
const source = resolve(root, 'dist', 'web')
const target = resolve(root, 'build', 'static')
const manifestName = '.caduceus-bundle.json'

if (!existsSync(resolve(source, 'index.html'))) {
  console.error("Tachyon bundle is missing dist/web/index.html. Run 'ty bundle' first.")
  process.exit(1)
}

rmSync(target, { recursive: true, force: true })
mkdirSync(dirname(target), { recursive: true })
cpSync(source, target, { recursive: true, force: true })

/** @type {Array<{path: string, bytes: number, sha256: string}>} */
const files = []
walk(target)
files.sort((left, right) => left.path.localeCompare(right.path))

const aggregate = createHash('sha256')
for (const file of files) {
  aggregate.update(file.path)
  aggregate.update('\0')
  aggregate.update(String(file.bytes))
  aggregate.update('\0')
  aggregate.update(file.sha256)
  aggregate.update('\n')
}

const manifest = {
  schemaVersion: 1,
  source: 'tachyon',
  files,
  digest: aggregate.digest('hex'),
}
writeFileSync(resolve(target, manifestName), JSON.stringify(manifest, null, 2) + '\n', {
  mode: 0o600,
})

console.log(`Synchronized ${files.length} Tachyon assets to build/static`)
console.log(`Frontend digest: ${manifest.digest}`)

/** @param {string} directory */
function walk(directory) {
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const absolute = join(directory, entry.name)
    if (entry.isDirectory()) {
      walk(absolute)
      continue
    }
    if (!entry.isFile() || entry.name === manifestName) continue
    const path = relative(target, absolute).split('\\').join('/')
    const bytes = statSync(absolute).size
    const sha256 = createHash('sha256').update(readFileSync(absolute)).digest('hex')
    files.push({ path, bytes, sha256 })
  }
}
