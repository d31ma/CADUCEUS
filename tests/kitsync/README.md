# kit-sync and vendored kit copies

`go test ./...` here checks that `scripts/kit-sync.sh` keeps a Go module's
vendored copy of the kit in step with `kit/` (SLICES K14). A module vendors
the kit when it has a `vendor/mesail.dev/packages` folder; today only
`edge/calls` does.

Each test builds a small repository in a temporary folder: a copy of
`scripts/kit-sync.sh`, `scripts/go-cache.sh` and `scripts/time-limit.sh` (which
`go-cache.sh` sources), a one-package kit, and one
module that vendors it. It needs no network. The tests check that:

- `kit-sync.sh --check` passes while the vendored copy is current;
- after a kit file changes, `kit-sync.sh --check` fails and names the
  module;
- `kit-sync.sh` runs `go mod vendor` in the module, prints the module, and
  `--check` passes again.

`scripts/gates.sh --shared-only` runs it with the other Go modules outside
`apps/`.
