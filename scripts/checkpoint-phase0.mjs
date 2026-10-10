#!/usr/bin/env bun

import { createHash } from 'node:crypto'
import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs'
import { resolve } from 'node:path'

const root = resolve(import.meta.dir, '..')
const contentRoot = resolve(process.env.PHASE0_CHECKPOINT_CONTENT_ROOT || root)
const stamp = new Date().toISOString().replace(/[:.]/g, '-')
const output = resolve(root, '.phase0-checkpoints', stamp)
mkdirSync(output, { recursive: true, mode: 0o700 })

if (!existsSync(contentRoot)) {
  throw new Error(`checkpoint content root does not exist: ${contentRoot}`)
}

function git(...args) {
  const result = Bun.spawnSync(['git', ...args], { cwd: root })
  if (result.exitCode !== 0) {
    throw new Error(`git ${args.join(' ')} failed: ${result.stderr.toString().trim()}`)
  }
  return result.stdout
}

function digest(path) {
  return createHash('sha256').update(readFileSync(path)).digest('hex')
}

const baseCommit = git('rev-parse', 'HEAD').toString().trim()
const branch = git('branch', '--show-current').toString().trim()
const tracked = git('ls-files', '--cached', '-z')
  .toString()
  .split('\0')
  .filter(Boolean)
const untracked = git('ls-files', '--others', '--exclude-standard', '-z')
  .toString()
  .split('\0')
  .filter(Boolean)
const deleted = git('ls-files', '--deleted', '-z')
  .toString()
  .split('\0')
  .filter(Boolean)
  .sort()
const snapshot = [...new Set([...tracked, ...untracked])]
  .filter(path => existsSync(resolve(root, path)))
  .sort()
const missingFromContentRoot = snapshot.filter(path => !existsSync(resolve(contentRoot, path)))
if (missingFromContentRoot.length > 0) {
  throw new Error(
    `checkpoint content root is missing ${missingFromContentRoot.length} workspace files, including ${missingFromContentRoot.slice(0, 5).join(', ')}`,
  )
}

const filesPath = resolve(output, 'workspace-files.txt')
const deletedPath = resolve(output, 'deleted-files.txt')
const archivePath = resolve(output, 'workspace.tar.gz')
writeFileSync(filesPath, snapshot.join('\0') + (snapshot.length ? '\0' : ''), { mode: 0o600 })
writeFileSync(deletedPath, deleted.join('\n') + (deleted.length ? '\n' : ''), { mode: 0o600 })

if (snapshot.length > 0) {
  const tar = Bun.spawnSync(
    ['/usr/bin/tar', '-czf', archivePath, '--null', '-T', filesPath],
    { cwd: contentRoot },
  )
  if (tar.exitCode !== 0) {
    throw new Error(`tar failed: ${tar.stderr.toString().trim()}`)
  }
} else {
  writeFileSync(archivePath, new Uint8Array(), { mode: 0o600 })
}

const manifest = {
  schemaVersion: 1,
  capturedAt: new Date().toISOString(),
  branch,
  baseCommit,
  contentSource: {
    mode: contentRoot === root ? 'workspace' : 'staged-copy',
    path: contentRoot,
  },
  workspaceArchive: {
    path: 'workspace.tar.gz',
    files: snapshot.length,
    bytes: readFileSync(archivePath).byteLength,
    sha256: digest(archivePath),
  },
  deletedFiles: {
    path: 'deleted-files.txt',
    files: deleted.length,
    sha256: digest(deletedPath),
  },
  inventory: {
    trackedFiles: tracked.length,
    untrackedFiles: untracked.length,
    archivedFiles: snapshot.length,
  },
  ignoredFilesExcluded: true,
  restoreOrder: [
    'Check out baseCommit in a clean recovery clone.',
    'Remove paths listed in deleted-files.txt.',
    'Extract workspace.tar.gz at the repository root.',
    'Verify the archive and deleted-file digests in manifest.json.',
  ],
}
writeFileSync(
  resolve(output, 'manifest.json'),
  JSON.stringify(manifest, null, 2) + '\n',
  { mode: 0o600 },
)

console.log(output)
console.log(`workspace archive: ${manifest.workspaceArchive.files} files, ${manifest.workspaceArchive.bytes} bytes`)
console.log(`deleted paths: ${manifest.deletedFiles.files}`)
