#!/usr/bin/env python3
"""Run one pipeline for a long time and report whether it leaks.

P1 of docs/PLAN_VALIDATION_CONFIANCE.md. Every lab scenario lives about a second,
so nothing is known about what happens over hours: memory growth, file-descriptor
or connection leaks, CRON drift. This runs a pipeline on its schedule, samples the
process periodically, and fails when a resource grows instead of plateauing.

    test-lab/scripts/soak.py test-lab/pipelines/volume-1000.yaml --duration 2h
    test-lab/scripts/soak.py test-lab/pipelines/db-input-basic.yaml --duration 24h

Samples land in test-lab/soak/<run>/samples.csv next to the pipeline log, so a run
can be re-analysed later:

    test-lab/scripts/soak.py --analyse test-lab/soak/<run>

The binary exposes no pprof endpoint, so resident memory, descriptors and threads
are read from /proc. That is enough to catch a leak trend; wiring net/http/pprof
into the CLI would give a sharper per-allocation picture and is a runtime change
left out of this harness on purpose.
"""
from __future__ import annotations

import argparse
import csv
import json
import os
import re
import signal
import statistics
import subprocess
import sys
import time
from dataclasses import dataclass, asdict
from datetime import datetime
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
SOAK_DIR = REPO_ROOT / "test-lab" / "soak"
COMPOSE_FILE = REPO_ROOT / "test-lab" / "docker-compose.yml"
BIN = REPO_ROOT / "bin" / "cannectors"

# A resource may settle higher than it started (pools fill, caches warm), so the
# gate compares the last quarter of the run with the second quarter rather than
# with the very first samples, and allows a margin before calling it a leak.
GROWTH_TOLERANCE = {"rss_kb": 1.25, "open_fds": 1.20, "threads": 1.20}


@dataclass
class Sample:
    elapsed_s: int
    rss_kb: int
    open_fds: int
    threads: int
    db_connections: int
    executions: int


def parse_duration(text: str) -> int:
    """Accept 90, 30m, 2h, 24h."""
    match = re.fullmatch(r"(\d+)([smh]?)", text.strip())
    if not match:
        raise argparse.ArgumentTypeError(f"invalid duration: {text!r} (use 90, 30m, 2h)")
    value, unit = int(match.group(1)), match.group(2)
    return value * {"": 1, "s": 1, "m": 60, "h": 3600}[unit]


def proc_status(pid: int) -> dict[str, int]:
    values = {"rss_kb": 0, "threads": 0}
    try:
        for line in Path(f"/proc/{pid}/status").read_text().splitlines():
            if line.startswith("VmRSS:"):
                values["rss_kb"] = int(line.split()[1])
            elif line.startswith("Threads:"):
                values["threads"] = int(line.split()[1])
    except OSError:
        pass
    return values


def open_fds(pid: int) -> int:
    try:
        return len(os.listdir(f"/proc/{pid}/fd"))
    except OSError:
        return 0


def db_connections() -> int:
    """Server-side connection count, to catch a pool that never releases."""
    try:
        out = subprocess.run(
            [
                "docker", "compose", "-f", str(COMPOSE_FILE), "exec", "-T", "postgres",
                "psql", "-U", "cannectors_test", "-d", "cannectors_test", "-At", "-c",
                "SELECT count(*) FROM pg_stat_activity WHERE application_name <> 'psql'",
            ],
            capture_output=True, text=True, check=False, timeout=15,
        )
        return int(out.stdout.strip() or 0)
    except (subprocess.SubprocessError, ValueError):
        return -1


def count_executions(log_file: Path) -> int:
    try:
        return sum(
            1 for line in log_file.read_text(errors="ignore").splitlines()
            if '"msg":"execution completed"' in line
        )
    except OSError:
        return 0


def cron_drift_ms(log_file: Path) -> list[float]:
    """Delay between the slot a run was scheduled for and when it actually started."""
    drifts: list[float] = []
    try:
        lines = log_file.read_text(errors="ignore").splitlines()
    except OSError:
        return drifts
    for line in lines:
        if '"scheduled pipeline execution starting"' not in line:
            continue
        try:
            evt = json.loads(line)
            actual = datetime.fromisoformat(evt["time"])
            planned = datetime.fromisoformat(evt["scheduled_time"])
        except (json.JSONDecodeError, KeyError, ValueError):
            continue
        drifts.append((actual - planned).total_seconds() * 1000)
    return drifts


def build_binary() -> None:
    subprocess.run(
        ["go", "build", "-o", str(BIN), "./cmd/cannectors"],
        cwd=REPO_ROOT, check=True,
    )


def run_soak(pipeline: Path, duration_s: int, interval_s: int) -> Path:
    run_dir = SOAK_DIR / time.strftime("%Y%m%d-%H%M%S")
    run_dir.mkdir(parents=True, exist_ok=True)
    log_file = run_dir / "pipeline.log"
    samples_file = run_dir / "samples.csv"

    print(f"building {BIN.relative_to(REPO_ROOT)}")
    build_binary()

    print(f"soaking {pipeline.name} for {duration_s}s, sampling every {interval_s}s")
    print(f"output: {run_dir.relative_to(REPO_ROOT)}")

    with log_file.open("w") as log:
        proc = subprocess.Popen(
            [str(BIN), "run", str(pipeline)], stdout=log, stderr=subprocess.STDOUT,
            cwd=REPO_ROOT,
        )

    (run_dir / "meta.json").write_text(
        json.dumps(
            {"pipeline": str(pipeline.relative_to(REPO_ROOT)), "duration_s": duration_s,
             "interval_s": interval_s, "pid": proc.pid},
            indent=2,
        )
    )

    started = time.time()
    samples: list[Sample] = []
    try:
        while time.time() - started < duration_s:
            time.sleep(interval_s)
            if proc.poll() is not None:
                print(f"pipeline exited early with code {proc.returncode}", file=sys.stderr)
                break
            status = proc_status(proc.pid)
            samples.append(
                Sample(
                    elapsed_s=int(time.time() - started),
                    rss_kb=status["rss_kb"],
                    open_fds=open_fds(proc.pid),
                    threads=status["threads"],
                    db_connections=db_connections(),
                    executions=count_executions(log_file),
                )
            )
            last = samples[-1]
            print(
                f"  t+{last.elapsed_s:>6}s  rss={last.rss_kb:>8} KiB  fds={last.open_fds:>4}"
                f"  threads={last.threads:>3}  db={last.db_connections:>3}"
                f"  runs={last.executions}"
            )
    except KeyboardInterrupt:
        print("interrupted, stopping the pipeline")
    finally:
        if proc.poll() is None:
            proc.send_signal(signal.SIGTERM)
            try:
                proc.wait(timeout=20)
            except subprocess.TimeoutExpired:
                proc.kill()

    with samples_file.open("w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=list(asdict(samples[0]).keys()) if samples else
                                [f.name for f in Sample.__dataclass_fields__.values()])
        writer.writeheader()
        for s in samples:
            writer.writerow(asdict(s))
    return run_dir


def load_samples(run_dir: Path) -> list[Sample]:
    path = run_dir / "samples.csv"
    with path.open() as f:
        return [Sample(**{k: int(v) for k, v in row.items()}) for row in csv.DictReader(f)]


def analyse(run_dir: Path) -> int:
    samples = load_samples(run_dir)
    log_file = run_dir / "pipeline.log"
    if len(samples) < 8:
        print(f"only {len(samples)} sample(s); run longer for a trend to mean anything")
        return 1

    quarter = len(samples) // 4
    baseline_window = samples[quarter : 2 * quarter]
    final_window = samples[3 * quarter :]

    print(f"\nsamples: {len(samples)}  duration: {samples[-1].elapsed_s}s  "
          f"executions: {samples[-1].executions}")
    print(f"{'metric':<16}{'baseline':>12}{'final':>12}{'ratio':>9}   verdict")

    failures = 0
    for metric, tolerance in GROWTH_TOLERANCE.items():
        baseline = statistics.median(getattr(s, metric) for s in baseline_window)
        final = statistics.median(getattr(s, metric) for s in final_window)
        if baseline <= 0:
            continue
        ratio = final / baseline
        ok = ratio <= tolerance
        failures += 0 if ok else 1
        print(f"{metric:<16}{baseline:>12.0f}{final:>12.0f}{ratio:>8.2f}x   "
              f"{'ok' if ok else f'LEAK? > {tolerance}x'}")

    db = [s.db_connections for s in samples if s.db_connections >= 0]
    if db:
        print(f"{'db_connections':<16}{min(db):>12}{max(db):>12}{'':>9}   "
              f"{'ok' if max(db) - min(db) <= 2 else 'growing — check the pool'}")

    drifts = cron_drift_ms(log_file)
    if drifts:
        drifts.sort()
        p50 = drifts[len(drifts) // 2]
        p99 = drifts[int(len(drifts) * 0.99)] if len(drifts) > 1 else drifts[0]
        print(f"\ncron drift over {len(drifts)} run(s): p50={p50:.0f}ms  p99={p99:.0f}ms  "
              f"max={drifts[-1]:.0f}ms")

    errors = sum(
        1 for line in log_file.read_text(errors="ignore").splitlines()
        if '"level":"ERROR"' in line
    )
    print(f"error lines in log: {errors}")
    if errors:
        failures += 1

    print("\nVERDICT:", "PASS" if failures == 0 else f"FAIL ({failures} issue(s))")
    return 0 if failures == 0 else 1


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("target", help="pipeline yaml to soak, or a run directory with --analyse")
    parser.add_argument("--duration", type=parse_duration, default="2h",
                        help="how long to run (90, 30m, 2h, 24h). Default 2h")
    parser.add_argument("--interval", type=parse_duration, default="30s",
                        help="sampling interval. Default 30s")
    parser.add_argument("--analyse", action="store_true",
                        help="re-analyse an existing run directory instead of soaking")
    args = parser.parse_args(argv[1:])

    target = Path(args.target)
    if args.analyse:
        return analyse(target if target.is_absolute() else REPO_ROOT / target)

    pipeline = target if target.is_absolute() else REPO_ROOT / target
    if not pipeline.exists():
        print(f"pipeline not found: {pipeline}", file=sys.stderr)
        return 2
    run_dir = run_soak(pipeline, args.duration, args.interval)
    return analyse(run_dir)


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
