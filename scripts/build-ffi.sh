#!/usr/bin/env bash
# Build the asic-rs-ffi Rust bridge and install artifacts into asicrs/{include,lib}.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FFI="$ROOT/asic-rs-ffi"
OUT_INC="$ROOT/asicrs/include"
OUT_LIB="$ROOT/asicrs/lib"

PROFILE="${PROFILE:-release}"
TARGET_DIR="$FFI/target/$PROFILE"

echo "==> building asic-rs-ffi ($PROFILE)"
(
  cd "$FFI"
  if [[ "$PROFILE" == "release" ]]; then
    cargo build --release
  else
    cargo build
  fi
)

mkdir -p "$OUT_INC" "$OUT_LIB"

if [[ -f "$FFI/include/asic_rs_ffi.h" ]]; then
  cp "$FFI/include/asic_rs_ffi.h" "$OUT_INC/"
elif [[ -f "$TARGET_DIR/../asic_rs_ffi.h" ]]; then
  cp "$TARGET_DIR/../asic_rs_ffi.h" "$OUT_INC/"
else
  echo "error: asic_rs_ffi.h not found after build" >&2
  exit 1
fi

shopt -s nullglob
copied=0
for ext in a dylib so dll lib; do
  for f in "$TARGET_DIR"/libasic_rs_ffi."$ext" "$TARGET_DIR"/asic_rs_ffi."$ext"; do
    if [[ -f "$f" ]]; then
      cp "$f" "$OUT_LIB/"
      echo "    installed $(basename "$f")"
      copied=1
    fi
  done
done
shopt -u nullglob

if [[ "$copied" -eq 0 ]]; then
  echo "error: no libasic_rs_ffi library found in $TARGET_DIR" >&2
  exit 1
fi

echo "==> ffi artifacts ready in asicrs/{include,lib}"
