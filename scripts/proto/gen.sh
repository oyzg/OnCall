#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROTO_DIR="$ROOT_DIR/proto"
GO_OUT="$ROOT_DIR/backend/go-api/gen/proto"
PY_OUT="$ROOT_DIR/backend/python-ai/app/gen/proto"
TOOLS_DIR="$ROOT_DIR/tmp/proto-tools"
BUF_BIN="$TOOLS_DIR/bin/buf"
PY_TMP_OUT="$TOOLS_DIR/python"
PYTHON_BIN="${PYTHON_BIN:-$ROOT_DIR/backend/python-ai/.venv/bin/python}"

mkdir -p "$GO_OUT" "$PY_OUT" "$TOOLS_DIR/bin" "$PY_TMP_OUT"

if [[ ! -x "$BUF_BIN" ]]; then
  GOBIN="$TOOLS_DIR/bin" go install github.com/bufbuild/buf/cmd/buf@v1.67.0
fi

if ! "$PYTHON_BIN" -c "import grpc_tools.protoc" >/dev/null 2>&1; then
  echo "Missing grpc_tools.protoc. Install backend/python-ai codegen dependencies first." >&2
  exit 1
fi

cd "$PROTO_DIR"

PATH="$TOOLS_DIR/bin:$PATH" "$BUF_BIN" generate

PY_PROTO_FILES=()
while IFS= read -r proto_file; do
  PY_PROTO_FILES+=("${proto_file#"$PROTO_DIR/"}")
done < <(find "$PROTO_DIR/common" "$PROTO_DIR/ai" -type f -name '*.proto' | LC_ALL=C sort)

if [[ ${#PY_PROTO_FILES[@]} -eq 0 ]]; then
  echo "No Python proto sources found under proto/common or proto/ai." >&2
  exit 1
fi

rm -rf "$PY_TMP_OUT" "$PY_OUT/ai" "$PY_OUT/common" "$ROOT_DIR/backend/python-ai/ai" "$ROOT_DIR/backend/python-ai/common"
mkdir -p "$PY_TMP_OUT"

if ! "$PYTHON_BIN" -m grpc_tools.protoc \
  -I . \
  --python_out="$PY_TMP_OUT" \
  --grpc_python_out="$PY_TMP_OUT" \
  "${PY_PROTO_FILES[@]}"
then
  exit 1
fi

ROOT_DIR="$ROOT_DIR" PY_TMP_OUT="$PY_TMP_OUT" PY_OUT="$PY_OUT" "$PYTHON_BIN" - <<'PY'
from __future__ import annotations

import os
import shutil
from pathlib import Path

py_tmp = Path(os.environ["PY_TMP_OUT"])
py_out = Path(os.environ["PY_OUT"])
python_root = Path(os.environ["ROOT_DIR"]) / "backend/python-ai"
compat_root = python_root

def ensure_init(directory: Path, doc: str) -> None:
    directory.mkdir(parents=True, exist_ok=True)
    init_file = directory / "__init__.py"
    if not init_file.exists():
        init_file.write_text(doc, encoding="utf-8")

def module_name(relative_path: Path) -> str:
    return ".".join(relative_path.with_suffix("").parts)

def write_wrapper(source: Path, wrapper_root: Path) -> None:
    relative_path = source.relative_to(py_tmp)
    wrapper = wrapper_root / relative_path
    wrapper.parent.mkdir(parents=True, exist_ok=True)
    wrapper.write_text(
        '"""Compatibility wrapper for generated %s."""\n\n'
        'from app.gen.proto.%s import *  # noqa: F401,F403\n'
        % (module_name(relative_path), module_name(relative_path)),
        encoding="utf-8",
    )

for source in py_tmp.rglob("*"):
    if source.is_dir():
        continue
    target = py_out / source.relative_to(py_tmp)
    target.parent.mkdir(parents=True, exist_ok=True)
    shutil.copy2(source, target)

for directory in [py_out] + [path for path in py_out.rglob("*") if path.is_dir()]:
    ensure_init(directory, '"""Generated protobuf package."""\n')

for source in py_tmp.rglob("*_pb2*.py"):
    write_wrapper(source, compat_root)

for directory in [path for path in py_tmp.rglob("*") if path.is_dir()]:
    ensure_init(
        compat_root / directory.relative_to(py_tmp),
        '"""Compatibility package for generated protobuf imports."""\n',
    )

for directory in [compat_root / "ai", compat_root / "common"]:
    ensure_init(directory, '"""Compatibility package for generated protobuf imports."""\n')
PY
