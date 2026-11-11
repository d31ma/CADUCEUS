#!/bin/sh
# Rebuilds the mockups' vendored CSS and JS. Usage: build.sh <path to a DuVay v26.35.01-1 checkout>
# Needs bun. Writes ../assets/vendor/{duvay-mesail.js,duvay-mesail.css,tailwind.css}.
set -eu
DUVAY="${1:?give the path to a DuVay v26.35.01-1 checkout}"
HERE="$(cd "$(dirname "$0")" && pwd)"
OUT="$HERE/../assets/vendor"
WORK="$(mktemp -d)"
mkdir -p "$OUT"
sed "s#DUVAY/#$DUVAY/src/#g" "$HERE/duvay-mesail.entry.js" > "$WORK/entry.js"
sed "s#DUVAY/#$DUVAY/src/#g" "$HERE/mesail-icons.js" > "$WORK/mesail-icons.js"
sed "s#DUVAY/#$DUVAY/src/#g" "$HERE/duvay-mesail.entry.css" > "$WORK/entry.css"
echo "=== duvay js $(date +%T)"
bun build "$WORK/entry.js" --minify --outfile "$OUT/duvay-mesail.js"
echo "=== duvay css $(date +%T)"
bun build "$WORK/entry.css" --minify --outfile "$OUT/duvay-mesail.css"
echo "=== tailwind $(date +%T)"
(cd "$WORK" && echo '{}' > package.json && bun add --exact tailwindcss@4.3.5 @tailwindcss/cli@4.3.5 >/dev/null)
sed "s#@source \"../\*.html\";#@source \"$HERE/../*.html\";#" "$HERE/tailwind.input.css" > "$WORK/tailwind.input.css"
(cd "$WORK" && ./node_modules/.bin/tailwindcss -i tailwind.input.css -o "$OUT/tailwind.css" --minify)
rm -rf "$WORK"
for f in "$OUT"/*; do printf '%-22s %8s bytes, %7s gzip\n' "$(basename "$f")" "$(wc -c < "$f" | tr -d ' ')" "$(gzip -9c "$f" | wc -c | tr -d ' ')"; done
