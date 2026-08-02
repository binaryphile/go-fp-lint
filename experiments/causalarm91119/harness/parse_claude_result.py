#!/usr/bin/env python3
"""parse_claude_result.py — the raw-result adapter (prereg §10 metric vector;
grade R1-F16/R2-9). Extracts ONLY metric fields from a `claude -p
--output-format json` result object; NEVER touches transcript free-text
(there is none in --output-format json's top-level result object beyond the
`result` field, which this adapter explicitly excludes).

Validated against fixtures/claude-result-schema.json, captured live from the
exact pinned claude CLI + model this session (see FROZEN.json). The
mock-delegate path enters DOWNSTREAM of this adapter (same extraction code),
so live and mock share everything except how the raw JSON was produced.

Usage: parse_claude_result.py RAW_JSON_PATH > metrics.json
Exits 1 with a diagnostic on missing/malformed input (caller treats as
infra-void — an unparseable result is a harness-attributable failure, not a
delegate-attributable one, since the delegate process DID produce output).
"""

import json
import sys

REQUIRED = ["subtype", "is_error", "duration_ms", "num_turns", "total_cost_usd", "usage"]


def extract(raw: dict) -> dict:
    missing = [k for k in REQUIRED if k not in raw]
    if missing:
        raise ValueError(f"missing required fields: {missing}")
    usage = raw["usage"]
    return {
        "subtype": raw["subtype"],
        "is_error": raw["is_error"],
        "duration_ms": raw["duration_ms"],
        "num_turns": raw["num_turns"],
        "total_cost_usd": raw["total_cost_usd"],
        "input_tokens": usage.get("input_tokens"),
        "output_tokens": usage.get("output_tokens"),
        "cache_read_input_tokens": usage.get("cache_read_input_tokens"),
        "cache_creation_input_tokens": usage.get("cache_creation_input_tokens"),
    }


def main():
    if len(sys.argv) != 2:
        print("usage: parse_claude_result.py RAW_JSON_PATH", file=sys.stderr)
        sys.exit(1)
    try:
        with open(sys.argv[1]) as f:
            raw = json.load(f)
        metrics = extract(raw)
    except (OSError, json.JSONDecodeError, ValueError) as e:
        print(f"parse_claude_result: {e}", file=sys.stderr)
        sys.exit(1)
    print(json.dumps(metrics))


if __name__ == "__main__":
    main()
