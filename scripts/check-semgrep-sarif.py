#!/usr/bin/env python3
"""Exit 1 when a Semgrep SARIF file contains unsuppressed findings.

Also rewrites the SARIF in place with suppressed (nosemgrep) results removed,
because GitHub code scanning ignores Semgrep's `suppressions` entries and
would otherwise open alerts for findings already suppressed in source.
"""

from __future__ import annotations

import json
import sys
from pathlib import Path


def main() -> None:
    sarif_path = Path(sys.argv[1] if len(sys.argv) > 1 else "semgrep.sarif")
    sarif = json.loads(sarif_path.read_text(encoding="utf-8"))
    active: list[dict] = []
    suppressed = 0
    for run in sarif.get("runs", []):
        kept = [r for r in run.get("results", []) if not (r.get("suppressions") or [])]
        suppressed += len(run.get("results", [])) - len(kept)
        run["results"] = kept
        active.extend(kept)

    sarif_path.write_text(json.dumps(sarif), encoding="utf-8")

    if not active:
        print(f"No active Semgrep findings ({suppressed} suppressed, stripped from SARIF).")
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
