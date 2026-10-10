#!/usr/bin/env bun

const root = new URL('..', import.meta.url).pathname

run(['gofmt', '-l', 'cmd', 'internal', 'test', 'tools'], {
  assertEmpty: 'Go formatting drift detected; run gofmt.',
})
run(['go', 'test', '-count=1', './...'])
run(['go', 'vet', './...'])
run(['go', 'test', '-race', '-count=1', './...'])
const foundationPackages = [
  './internal/platform/buildinfo',
  './internal/platform/config',
  './internal/platform/httpserver',
  './internal/platform/preflight',
  './internal/system',
]
run([
  'go',
  'test',
  '-count=1',
  '-covermode=atomic',
  '-coverprofile=coverage.out',
  ...foundationPackages,
])
run(['go', 'run', './tools/coverage', '-profile', 'coverage.out', '-min', '80'])
run(['go', 'run', './tools/crap', '-profile', 'coverage.out', '-max', '15'])
run(['bun', 'scripts/build-go.mjs', '--output', 'dist-bin/caduceus-phase1-check'], {
  env: {
    BUILD_VERSION: 'phase1-check',
    BUILD_COMMIT: '0123456789abcdef',
    SOURCE_DATE_EPOCH: '1785067200',
  },
})
const version = run(['dist-bin/caduceus-phase1-check', 'version', '--output', 'json'], {
  capture: true,
})
const metadata = JSON.parse(version)
if (
  metadata.version !== 'phase1-check'
  || metadata.commit !== '0123456789abcdef'
  || metadata.built_at !== '2026-07-26T12:00:00.000Z'
) {
  console.error('Injected Go build metadata did not survive compilation.')
  process.exit(1)
}

console.log('Phase 1 Go quality gate passed.')

/**
 * @param {string[]} command
 * @param {{assertEmpty?: string, capture?: boolean, env?: Record<string, string>}} [options]
 * @returns {string}
 */
function run(command, options = {}) {
  const result = Bun.spawnSync(command, {
    cwd: root,
    stdout: 'pipe',
    stderr: 'pipe',
    env: { ...process.env, GOFLAGS: '-buildvcs=false', ...options.env },
  })
  const stdout = result.stdout.toString()
  const stderr = result.stderr.toString()
  if (stdout && !options.capture) process.stdout.write(stdout)
  if (stderr) process.stderr.write(stderr)
  if (result.exitCode !== 0) process.exit(result.exitCode || 1)
  if (options.assertEmpty && stdout.trim()) {
    console.error(options.assertEmpty)
    process.exit(1)
  }
  return stdout
}
