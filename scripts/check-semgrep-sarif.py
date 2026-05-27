#!/usr/bin/env python3
"""Exit 1 when a Semgrep SARIF file contains unsuppressed findings."""

from __future__ import annotations

import json
import sys
from pathlib import Path


def main() -> None:
    sarif_path = Path(sys.argv[1] if len(sys.argv) > 1 else "semgrep.sarif")
    sarif = json.loads(sarif_path.read_text(encoding="utf-8"))
    results: list[dict] = []
    for run in sarif.get("runs", []):
        results.extend(run.get("results", []))

    active = [r for r in results if not (r.get("suppressions") or [])]
    if not active:
        print(f"No active Semgrep findings ({len(results)} suppressed).")
        return

    for result in active:
        locs = result.get("locations") or []
        physical = locs[0].get("physicalLocation", {}) if locs else {}
        uri = (physical.get("artifactLocation") or {}).get("uri", "?")
        line = (physical.get("region") or {}).get("startLine", "?")
        print(f"{result.get('ruleId', '?')} {uri}:{line}", file=sys.stderr)
    sys.exit(1)


if __name__ == "__main__":
    main()
