# No public or cloud defaults

`go test ./...` here checks ADR 0007 and SPEC AC-OP-5: no code or setting
file in the tree names a public resolver or a cloud host (for example
`cloudflare-dns.com`, `amazonaws.com`, `8.8.8.8`, `1.1.1.1`,
`dns.google`), and the settings that reach outside the install have no
default that does: the directory's `dnsresolver` is empty (the host's own
resolver) and notify's `push.enabled` is `false`.

Docs (`*.md`), tests (`*_test.go`) and vendored code are not scanned. A
use that is not an address Mesail connects to is listed, with its reason,
in `allowedUses` in `NoCloudDefaults_test.go`.

`scripts/gates.sh --shared-only` runs it with the other Go modules outside
`apps/`.
