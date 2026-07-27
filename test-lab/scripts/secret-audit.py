#!/usr/bin/env python3
"""Prove that no credential ever reaches a log line.

P5 of docs/PLAN_VALIDATION_CONFIANCE.md. The documentation states that resolved
secrets are never logged. That is a claim about every code path that formats a
message, so it cannot be checked by reading a few of them.

This gives each credential slot a unique sentinel value, drives the pipeline
through every output surface the CLI has — validate, validate --verbose, dry-run,
run, run --verbose — and greps the captured output for those sentinels. A single
occurrence is a leak.

    test-lab/scripts/secret-audit.py
    test-lab/scripts/secret-audit.py --keep    # leave the generated pipelines

Sentinels are distinctive strings, so a hit tells you which slot leaked and which
command printed it.
"""
from __future__ import annotations

import argparse
import re
import subprocess
import sys
from dataclasses import dataclass, field
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
BIN = REPO_ROOT / "bin" / "cannectors"
TMP = REPO_ROOT / "test-lab" / "tmp" / "secret-audit"
WIREMOCK = "http://localhost:18080"


@dataclass
class Slot:
    """One credential position, with the value that must never be printed."""

    name: str
    sentinel: str
    pipeline: str
    # Slots whose leak is already known and accepted are marked so the report
    # separates "new leak" from "documented gap".
    known_gap: str = ""


def sentinel(tag: str) -> str:
    return f"SENTINEL-{tag}-9f3a71c4"


def slots() -> list[Slot]:
    src = f"{WIREMOCK}/source/matrix/record"
    sink = f"{WIREMOCK}/destination/matrix/sink"

    def pipeline(name: str, input_extra: str = "", output_extra: str = "",
                 endpoint: str = src) -> str:
        return f"""name: {name}
version: 1.0.0
description: Secret audit fixture.
input:
  type: httpPolling
  schedule: "* * * * * *"
  endpoint: {endpoint}
  dataField: data
{input_extra}filters: []
output:
  type: httpRequest
  endpoint: {sink}
  method: POST
  headers:
    Content-Type: application/json
{output_extra}"""

    bearer = sentinel("BEARER")
    basic = sentinel("BASICPW")
    apikey = sentinel("APIKEY")
    oauth = sentinel("OAUTHSECRET")
    outbearer = sentinel("OUTBEARER")
    userinfo = sentinel("URLCREDS")
    query = sentinel("QUERYTOKEN")
    dbpw = sentinel("DBPASSWORD")

    return [
        Slot("input bearer token", bearer, pipeline(
            "audit-bearer",
            input_extra=f"  authentication:\n    type: bearer\n    credentials:\n      token: {bearer}\n",
        )),
        Slot("input basic password", basic, pipeline(
            "audit-basic",
            input_extra=("  authentication:\n    type: basic\n    credentials:\n"
                         f"      username: lab-user\n      password: {basic}\n"),
        )),
        Slot("input api key", apikey, pipeline(
            "audit-apikey",
            input_extra=("  authentication:\n    type: api-key\n    credentials:\n"
                         f"      key: {apikey}\n      location: header\n      headerName: X-API-Key\n"),
        )),
        Slot("oauth2 client secret", oauth, pipeline(
            "audit-oauth2",
            input_extra=("  authentication:\n    type: oauth2\n    credentials:\n"
                         f"      tokenUrl: {WIREMOCK}/auth/oauth2/token\n"
                         f"      clientId: lab-client-id\n      clientSecret: {oauth}\n"),
        )),
        Slot("output bearer token", outbearer, pipeline(
            "audit-output-bearer",
            output_extra=("  authentication:\n    type: bearer\n    credentials:\n"
                          f"      token: {outbearer}\n"),
        )),
        Slot("credentials in the endpoint URL", userinfo, pipeline(
            "audit-url-userinfo",
            endpoint=f"http://lab-user:{userinfo}@localhost:18080/source/matrix/record",
        )),
        Slot("token in a query parameter", query, pipeline(
            "audit-query-token",
            endpoint=f"{src}?access_token={query}",
        )),
        Slot("database password in the DSN", dbpw, f"""name: audit-db-password
version: 1.0.0
description: Secret audit fixture.
input:
  type: database
  schedule: "* * * * * *"
  driver: postgres
  connectionString: postgres://cannectors_test:{dbpw}@localhost:15432/cannectors_test?sslmode=disable
  query: "SELECT 1 AS one"
filters: []
output:
  type: httpRequest
  endpoint: {sink}
  method: POST
  headers:
    Content-Type: application/json
"""),
    ]


COMMANDS = [
    ("validate", ["validate"]),
    ("validate --verbose", ["validate", "--verbose"]),
    ("run --dry-run", ["run", "--dry-run"]),
    ("run", ["run"]),
    ("run --verbose", ["run", "--verbose"]),
]


def capture(args: list[str], pipeline: Path, timeout: int = 8) -> str:
    """Run one CLI command and return everything it printed."""
    try:
        proc = subprocess.run(
            [str(BIN), *args, str(pipeline)],
            capture_output=True, text=True, timeout=timeout, cwd=REPO_ROOT,
        )
        return proc.stdout + proc.stderr
    except subprocess.TimeoutExpired as exc:
        # A scheduled pipeline never exits on its own; whatever it printed before
        # the timeout is exactly what we want to inspect.
        out = exc.stdout or b""
        err = exc.stderr or b""
        return (out.decode(errors="replace") if isinstance(out, bytes) else out) + \
               (err.decode(errors="replace") if isinstance(err, bytes) else err)


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--keep", action="store_true", help="keep the generated pipelines")
    args = parser.parse_args(argv[1:])

    subprocess.run(["go", "build", "-o", str(BIN), "./cmd/cannectors"],
                   cwd=REPO_ROOT, check=True)
    TMP.mkdir(parents=True, exist_ok=True)

    leaks: list[tuple[str, str, str]] = []
    print(f"{'credential slot':<36}{'command':<20}verdict")
    for slot in slots():
        path = TMP / f"{slot.name.replace(' ', '-')}.yaml"
        path.write_text(slot.pipeline)
        for label, cmd in COMMANDS:
            output = capture(cmd, path)
            if slot.sentinel in output:
                leaks.append((slot.name, label, output))
                print(f"{slot.name:<36}{label:<20}LEAKED")
            else:
                print(f"{slot.name:<36}{label:<20}clean")

    if not args.keep:
        for path in TMP.glob("*.yaml"):
            path.unlink()

    print()
    if not leaks:
        print("VERDICT: PASS — no credential reached any output stream")
        return 0

    print(f"VERDICT: FAIL — {len(leaks)} leak(s)\n")
    for name, command, output in leaks:
        print(f"--- {name} via `{command}`")
        for line in output.splitlines():
            if re.search(r"SENTINEL-[A-Z]+-9f3a71c4", line):
                print(f"    {line.strip()[:200]}")
                break
    return 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
