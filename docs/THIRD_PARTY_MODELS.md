# Third-party model notices

CADUCEUS release images include the files pinned in
`translation-models.lock.json`. They are served by the same CADUCEUS origin and
are used only for translation explicitly enabled by the mailbox owner. Runtime
code disables Hugging Face Hub and CDN access.

## SMaLL-100 ONNX

- Base model: `alirezamsh/small100`
- Browser ONNX conversion: `casawolice/small100-onnx`
- Pinned conversion revision: `5c2c73ac70bee9c58f5a7ac5e84a36bee25db8ee`
- License: MIT
- Sources: <https://huggingface.co/alirezamsh/small100> and
  <https://huggingface.co/casawolice/small100-onnx>

The conversion repository declares the model MIT-licensed, identifies the base
model, and contains int8-quantized encoder and cached-decoder graphs. CADUCEUS
does not modify or fine-tune the weights. The pinned model supports direct
translation among 100 languages without an English pivot. Its standard ONNX
filenames are selected with Transformers.js's `fp32` filename selector even
though the weights inside those graphs remain int8. CADUCEUS uses this model
for the 80 languages it shares with Mailspring's 93-language reader catalogue.

The same pinned checkpoint is also the reproducible source for the static
authenticated-interface catalogs. `bun run localization:generate` inventories
only static HTML text and allowlisted accessibility attributes, adds a model-
only sentence boundary to prevent open-ended label decoding, and writes raw
catalogs only to a temporary directory. `bun run localization:publish` then
enforces exact inventory/locale coverage, placeholder and script preservation,
bounded output, lexical-diversity and collapsed-translation checks, reviewed
French/Arabic core overrides, and explicit English fallback. Runtime clients
fetch only the resulting same-origin JSON asset; they never invoke a remote
translation service for interface text.

Taiwan Traditional Chinese output is deterministically converted from the
qualified Simplified Chinese candidate with `opencc-js` 1.4.1 (`cn` to `tw`).
That package is licensed MIT AND Apache-2.0 and carries no runtime dependency
or network access in CADUCEUS; its version and transformation are recorded in
the published catalog provenance.

## OPUS-MT Marian ONNX

- Base models: `Helsinki-NLP/opus-mt-en-mul` and
  `Helsinki-NLP/opus-mt-mul-en`
- Browser ONNX conversions: `Xenova/opus-mt-en-mul` and
  `Xenova/opus-mt-mul-en`
- Pinned conversion revisions: `e57f9759f8e1e9ed97067bfc11e53e1eab63e57e`
  and `72a05e47cee89c718a9db4dc70d02fef3bc39de8`
- License: Apache-2.0
- Sources: <https://huggingface.co/Helsinki-NLP/opus-mt-en-mul>,
  <https://huggingface.co/Helsinki-NLP/opus-mt-mul-en>,
  <https://huggingface.co/Xenova/opus-mt-en-mul>, and
  <https://huggingface.co/Xenova/opus-mt-mul-en>

The two int8 Marian checkpoints load lazily and supply the 13 Mailspring
catalogue entries not present in SMaLL-100. A pair involving one of those
languages pivots through English only when necessary. Transformers.js's Marian
fast-tokenizer compatibility path does not preserve the leading destination
selector, so CADUCEUS inserts the checkpoint's locked vocabulary id directly.
ONNX Runtime's optional QDQ graph rewrite is disabled for these tied-embedding
graphs; inference otherwise uses the same single-threaded WASM boundary.

The compact OPUS family exposes Meadow Mari (`mhr`) but not a distinct Hill
Mari (`mrj`) constituent. CADUCEUS retains both upstream UI identifiers and
maps Hill Mari to the local Mari checkpoint. This is an explicit translation-
quality boundary, not a claim of dialect-specific model training.

The complete pinned browser payload is 839.7 MiB. Files larger than Tachyon's
16 MiB asset ceiling are emitted as checksum-verified chunks outside the
frontend tree and served only through the authenticated, catalog-allowlisted
`/translation-assets` route. `bun run translation:vendor` installs it,
`bun run translation:verify` rechecks every published file, and
`bun run translation:qualify` performs real inference in Chrome, Firefox, and
WebKit.

## Transformers.js and ONNX Runtime Web

- `@huggingface/transformers` 4.2.0: Apache-2.0
- `onnxruntime-web` 1.26.0-dev.20260416-b7804b056c: MIT

Their package licenses remain available in `node_modules` in source builds.
The release image vendors only the checksum-pinned
`ort-wasm-simd-threaded.asyncify.wasm` binary required by the bundled browser
factory. The factory and WASM variant must remain paired; the standard binary
does not initialize correctly with this Asyncify factory.

## Local language detection

- `franc` 6.2.0: MIT

The full detector is bundled with the browser client and restricted to the
locked translation catalogue. Generic Mari detection resolves to the shared
Mari constituent; the two upstream Mari labels cannot be distinguished by the
local detector. Detection and translation both run locally; no message sample
is sent to CADUCEUS or another service.
