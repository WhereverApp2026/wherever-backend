#!/usr/bin/env python3
"""Cobertura del delta (nv-03, advisory).

Mide qué parte de las líneas .go cambiadas bajo internal/ (desde `git merge-base`)
cubren las pruebas, según el perfil unido que deja scripts/check_coverage.sh.
Una línea cuenta si cae en algún bloque del perfil; está cubierta si alguno de
esos bloques se ejecutó (unión por línea). La higiene queda fuera del delta.

Uso: python3 scripts/check_delta_coverage.py --min 80 --advisory
     [--base origin/main] [--profile bin/coverage.out]
Con --advisory siempre sale 0 e informa; sin él, bajo el mínimo sale 1.
"""
from __future__ import annotations

import argparse
import fnmatch
import os
import re
import subprocess
import sys
from dataclasses import dataclass, field

# Higiene: código sin lógica propia o de apoyo a pruebas (spec nv-03 §6).
HYGIENE = (
    "cmd/*/main.go",
    "internal/testutil/*",
    "*/mocks/*",
    "*mock*.go",
    "*/dto.go",
    "scripts/indexes/*",
)

HUNK = re.compile(r"^@@ -\d+(?:,\d+)? \+(\d+)(?:,(\d+))? @@")
BLOCK = re.compile(r"^(.+\.go):(\d+)\.\d+,(\d+)\.\d+ \d+ (\d+)$")


@dataclass
class FileDelta:
    coverable: set[int] = field(default_factory=set)
    covered: set[int] = field(default_factory=set)


def is_hygiene(path: str) -> bool:
    return any(fnmatch.fnmatch(path, pattern) for pattern in HYGIENE)


def in_scope(path: str) -> bool:
    return (
        path.startswith("internal/")
        and path.endswith(".go")
        and not path.endswith("_test.go")
        and not is_hygiene(path)
    )


def parse_diff(diff: str) -> dict[str, set[int]]:
    """Líneas agregadas o cambiadas por archivo (lado nuevo de `git diff -U0`)."""
    changed: dict[str, set[int]] = {}
    current = None
    for line in diff.splitlines():
        if line.startswith("+++ "):
            target = line[4:]
            current = target[2:] if target.startswith("b/") else None
            continue
        match = HUNK.match(line)
        if match and current and in_scope(current):
            start, count = int(match.group(1)), int(match.group(2) or "1")
            changed.setdefault(current, set()).update(range(start, start + count))
    return {path: lines for path, lines in changed.items() if lines}


def parse_profile(text: str, module: str) -> dict[str, list[tuple[int, int, int]]]:
    """Bloques (línea inicial, línea final, conteo) por archivo relativo al repo."""
    blocks: dict[str, list[tuple[int, int, int]]] = {}
    prefix = module.rstrip("/") + "/"
    for line in text.splitlines():
        match = BLOCK.match(line.strip())
        if not match:
            continue
        path = match.group(1)
        path = path[len(prefix):] if path.startswith(prefix) else path
        blocks.setdefault(path, []).append(
            (int(match.group(2)), int(match.group(3)), int(match.group(4)))
        )
    return blocks


def measure(changed: dict[str, set[int]], blocks: dict[str, list[tuple[int, int, int]]]) -> dict[str, FileDelta]:
    result: dict[str, FileDelta] = {}
    for path, lines in sorted(changed.items()):
        delta = FileDelta()
        for start, end, count in blocks.get(path, []):
            hit = lines.intersection(range(start, end + 1))
            delta.coverable |= hit
            if count > 0:
                delta.covered |= hit
        if delta.coverable:
            result[path] = delta
    return result


def evaluate(deltas: dict[str, FileDelta], minimum: float, advisory: bool) -> tuple[int, list[str]]:
    """Devuelve el código de salida y el reporte."""
    coverable = sum(len(d.coverable) for d in deltas.values())
    if coverable == 0:
        return 0, ["check_delta_coverage: OK (delta vacío: ninguna línea medible cambiada)"]
    covered = sum(len(d.covered) for d in deltas.values())
    pct = 100.0 * covered / coverable
    report = [
        f"  {path}: {len(d.covered)}/{len(d.coverable)} ({100.0 * len(d.covered) / len(d.coverable):.1f}%)"
        for path, d in deltas.items()
    ]
    summary = f"{pct:.1f}% del delta ({covered}/{coverable} líneas), mínimo {minimum:g}%"
    if pct >= minimum:
        return 0, report + [f"check_delta_coverage: OK ({summary})"]
    if advisory:
        return 0, report + [f"check_delta_coverage: ADVISORY bajo el mínimo ({summary}); no bloquea"]
    return 1, report + [f"check_delta_coverage: FAIL ({summary})"]


def git(*args: str) -> str:
    # UTF-8 explícito: en Windows el predeterminado es cp1252 y el diff trae acentos.
    return subprocess.run(["git", *args], check=True, capture_output=True, encoding="utf-8", errors="replace").stdout


def default_base() -> str:
    has_remote = subprocess.run(
        ["git", "rev-parse", "--verify", "--quiet", "origin/main"], capture_output=True
    ).returncode == 0
    return "origin/main" if has_remote else "main"


def module_path() -> str:
    with open("go.mod", encoding="utf-8") as fh:
        for line in fh:
            if line.startswith("module "):
                return line.split()[1]
    raise SystemExit("check_delta_coverage: go.mod sin línea module")


def main(argv: list[str]) -> int:
    sys.stdout.reconfigure(encoding="utf-8")
    sys.stderr.reconfigure(encoding="utf-8")
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("--min", type=float, required=True)
    parser.add_argument("--advisory", action="store_true")
    parser.add_argument("--base", default=None)
    parser.add_argument("--profile", default="bin/coverage.out")
    args = parser.parse_args(argv)

    os.chdir(os.path.join(os.path.dirname(os.path.abspath(__file__)), ".."))
    if not os.path.isfile(args.profile):
        message = f"check_delta_coverage: falta {args.profile}; corre antes scripts/check_coverage.sh"
        print(message, file=sys.stderr)
        return 0 if args.advisory else 2

    base = git("merge-base", args.base or default_base(), "HEAD").strip()
    diff = git("diff", "--unified=0", "--no-color", "--no-ext-diff", base, "--", "internal")
    with open(args.profile, encoding="utf-8") as fh:
        blocks = parse_profile(fh.read(), module_path())

    code, report = evaluate(measure(parse_diff(diff), blocks), args.min, args.advisory)
    print(f"check_delta_coverage: base {base[:12]}")
    print("\n".join(report))
    return code


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
