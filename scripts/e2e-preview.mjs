/**
 * Starts the Tachyon preview server for Playwright E2E tests.
 * Explicitly sets CWD to the project root so `ty preview` finds the bundled
 * `dist/` directory created by `ty bundle`.
 */
import { spawn } from 'node:child_process'
import { fileURLToPath } from 'node:url'
import { resolve } from 'node:path'

const projectRoot = resolve(fileURLToPath(new URL('.', import.meta.url)), '..')
const proc = spawn('ty', ['preview', '--port', '19875'], {
  cwd: projectRoot,
  stdio: 'inherit',
  env: { ...process.env },
})

proc.on('exit', code => process.exit(code ?? 0))
