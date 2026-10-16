# Typecheck declarations

Run `bun scripts/build-translation-vendors.mjs`, then `ty types .`, before
`bun run typecheck` with the qualified Tachyon CLI on PATH. Configure TMPDIR to
an owned ANNEX directory in the local runner. Translation vendor generation uses
an isolated scratch entry tree and reads the existing dependency tree; it also
writes matching public package type declarations beside the browser adapters.
The package typecheck command remains `tsc --noEmit && tsc -p tsconfig.strict.json`.

Tachyon generates `.tachyon/types/client/composed.d.ts`; `tachyon-env.d.ts` loads
that declaration file. The generated aliases import the authored JS model and
service types rather than duplicating their contracts. `.tachyon` remains an
ignored build artifact. Regenerate after adding or renaming composed classes.

`tachyon-behaviors.d.ts` is an exact copy from the qualified Tachyon source
`types/tachyon-behaviors.d.ts` (SHA-256
`6c841585bfdb2b036718523309698ae600329b346fc79982e6fb946159adc508`). It supplies
typed decorators and injected runtime contracts. The two native model payloads
used directly by browser JS, `GrammarRange` and `AttachmentOffer`, match the
JSON fields of their Go model twins and are declared in `client/caduceus-env.d.ts`.

The legacy fixture declarations import actual helper/service and model types.
`tests/support/legacy-server/deps/fylo.d.mts` describes the public NDJSON adapter
API for the retained FYLO client; returned documents keep unknown fields until
first-party consumers validate their domain models. It does not modify the
vendor implementation or runtime behavior.

The strict stage now follows the surviving JS runtime modules and runtime tests.
Its former TypeScript includes named files removed by the Go migration and
failed with TS18003. `strict`, `checkJs`, and `noImplicitAny` remain enabled in
that stage; imported first-party dependencies are checked with those rules.
