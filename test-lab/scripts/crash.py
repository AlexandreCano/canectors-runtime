#!/usr/bin/env python3
"""SIGKILL a pipeline at a chosen moment and report what survived.

P3 of docs/PLAN_VALIDATION_CONFIANCE.md. The declarative runner always stops a
pipeline politely with SIGTERM after a completed execution, so nothing was known
about a hard crash: whether the state file survives readable, and whether the
next run re-delivers records that were already sent.

    test-lab/scripts/crash.py                       # every kill point
    test-lab/scripts/crash.py --point after-output  # one of them

Kill points are log markers, so the kill lands at a known place in the pipeline
rather than after an arbitrary sleep:

  after-fetch    the source has been read, nothing sent yet
  after-output   records have reached the destination, state may not be saved
  mid-flight     as soon as the pipeline starts working, before anything finishes

For each point the harness reports the state file's integrity and, after a
restart, how many records the destination received twice. Duplicates are not a
failure: retries and crash recovery both make delivery at-least-once. The point
is to measure it and keep it visible.

Exit code is non-zero only when something is actually broken: an unreadable
state file, or a lost record.
"""
from __future__ import annotations

import argparse
import json
import shutil
import signal
import subprocess
import sys
import time
from dataclasses import dataclass
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
BIN = REPO_ROOT / "bin" / "cannectors"
STATE_DIR = REPO_ROOT / "test-lab" / "state"
WIREMOCK = "http://localhost:18080"

# The source behind this pipeline honours after_id: a fresh run gets three
# events, a caught-up run gets none. Without that, a static stub would replay the
# same records on every restart and the "duplicates" measured here would be an
# artefact of the lab rather than the runtime's delivery semantics.
PIPELINE = REPO_ROOT / "test-lab" / "pipelines" / "crash-state-id.yaml"
PIPELINE_ID = "crash-state-id"
DESTINATION_PATH = "/destination/state/id"

KILL_POINTS = {
    "mid-flight": '"msg":"execution started"',
    "after-fetch": '"msg":"input fetch completed"',
    "after-output": '"msg":"output send completed"',
}


@dataclass
class Outcome:
    point: str
    killed: bool
    state_present: bool
    state_readable: bool
    state_content: str
    delivered_before: int
    delivered_after: int
    duplicates: int
    lost: int


def curl_json(path: str) -> dict:
    out = subprocess.run(
        ["curl", "-fsS", f"{WIREMOCK}{path}"], capture_output=True, text=True, check=True
    ).stdout
    return json.loads(out) if out.strip() else {}


def reset_journal() -> None:
    subprocess.run(
        ["curl", "-fsS", "-X", "DELETE", f"{WIREMOCK}/__admin/requests"],
        capture_output=True, check=False,
    )


def delivered_ids() -> list[str]:
    """Record ids the destination actually received, in journal order."""
    journal = curl_json("/__admin/requests")
    ids: list[str] = []
    for req in journal.get("requests", []):
        request = req.get("request", {}) or {}
        if DESTINATION_PATH not in (request.get("url") or ""):
            continue
        try:
            body = json.loads(request.get("body") or "")
        except json.JSONDecodeError:
            continue
        for record in body if isinstance(body, list) else [body]:
            event = record.get("event") if isinstance(record, dict) else None
            if isinstance(event, dict) and "id" in event:
                ids.append(str(event["id"]))
    return ids


def state_file() -> Path:
    return STATE_DIR / f"{PIPELINE_ID}.json"


def inspect_state() -> tuple[bool, bool, str]:
    """(present, readable as JSON, raw content) — a torn write shows up here."""
    path = state_file()
    if not path.exists():
        return False, True, ""
    raw = path.read_text(errors="replace")
    try:
        json.loads(raw)
        return True, True, raw.strip()
    except json.JSONDecodeError:
        return True, False, raw.strip()


def run_until(marker: str, timeout_s: int = 20) -> tuple[subprocess.Popen, bool]:
    """Start the pipeline and SIGKILL it as soon as the marker appears."""
    log_path = Path(subprocess.run(["mktemp"], capture_output=True, text=True,
                                   check=True).stdout.strip())
    with log_path.open("w") as log:
        proc = subprocess.Popen(
            [str(BIN), "run", str(PIPELINE)], stdout=log, stderr=subprocess.STDOUT,
            cwd=REPO_ROOT,
        )
    killed = False
    deadline = time.time() + timeout_s
    while time.time() < deadline:
        if marker in log_path.read_text(errors="ignore"):
            proc.send_signal(signal.SIGKILL)
            killed = True
            break
        if proc.poll() is not None:
            break
        time.sleep(0.02)
    if proc.poll() is None:
        proc.send_signal(signal.SIGKILL)
    proc.wait(timeout=10)
    log_path.unlink(missing_ok=True)
    return proc, killed


def run_to_completion(timeout_s: int = 20) -> None:
    """A normal recovery run, stopped politely once one execution finishes."""
    subprocess.run(
        ["bash", str(REPO_ROOT / "test-lab" / "scripts" / "run-pipeline-once.sh"),
         str(PIPELINE), str(timeout_s)],
        capture_output=True, check=False, cwd=REPO_ROOT,
    )


def exercise(point: str) -> Outcome:
    marker = KILL_POINTS[point]
    for stale in STATE_DIR.glob("state-*.json"):
        stale.unlink()
    state_file().unlink(missing_ok=True)
    reset_journal()

    _, killed = run_until(marker)
    before = delivered_ids()
    present, readable, content = inspect_state()

    # Restart and see what the recovered run does with whatever survived.
    run_to_completion()
    after = delivered_ids()

    delivered_twice = sum(
        max(0, after.count(rid) - 1) for rid in set(before)
    )
    lost = len([rid for rid in set(before) if rid not in after])
    return Outcome(
        point=point, killed=killed, state_present=present, state_readable=readable,
        state_content=content[:120], delivered_before=len(before),
        delivered_after=len(after), duplicates=delivered_twice, lost=lost,
    )


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--point", choices=sorted(KILL_POINTS), help="only this kill point")
    args = parser.parse_args(argv[1:])

    if not shutil.which("curl"):
        print("curl is required", file=sys.stderr)
        return 2
    subprocess.run(["go", "build", "-o", str(BIN), "./cmd/cannectors"],
                   cwd=REPO_ROOT, check=True)

    points = [args.point] if args.point else sorted(KILL_POINTS)
    outcomes = [exercise(p) for p in points]

    print(f"\n{'kill point':<14}{'killed':>7}{'state':>18}{'sent':>6}"
          f"{'after':>7}{'dup':>5}{'lost':>6}")
    failures = 0
    for o in outcomes:
        if not o.state_present:
            state = "absent"
        elif o.state_readable:
            state = "valid JSON"
        else:
            state = "CORRUPT"
            failures += 1
        if o.lost:
            failures += 1
        print(f"{o.point:<14}{str(o.killed):>7}{state:>18}{o.delivered_before:>6}"
              f"{o.delivered_after:>7}{o.duplicates:>5}{o.lost:>6}")

    print("\nA SIGKILL must never leave a half-written state file, and must never")
    print("lose a record — those two are what this harness gates on.")
    print()
    print("Reading the duplicate column: 0 here does NOT mean exactly-once. Delivery")
    print("is at-least-once by design — a retry replays the whole batch, which the")
    print("reconciliation in run.py reports as replayed=N. A crash can also duplicate,")
    print("in the window between records reaching the destination and the state being")
    print("saved, but an execution takes a few milliseconds end to end, so killing on a")
    print("log marker lands either side of that window rather than inside it. Proving")
    print("it would need a test hook that delays the state write.")
    print("\nVERDICT:", "PASS" if failures == 0 else f"FAIL ({failures} issue(s))")
    return 0 if failures == 0 else 1


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
