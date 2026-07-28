#!/usr/bin/env python3
"""Run pipelines for a long time and report whether they leak.

P1 of docs/PLAN_VALIDATION_CONFIANCE.md. Every lab scenario lives about a second,
so nothing is known about what happens over hours: memory growth, file-descriptor
or connection leaks, CRON drift. This runs pipelines on their schedule, samples
each process periodically, and fails when a resource grows instead of plateauing.

    test-lab/scripts/soak.py test-lab/pipelines/volume-1000.yaml --duration 2h
    test-lab/scripts/soak.py test-lab/pipelines/*.yaml --duration 12h --schedule '*/5 * * * * *'

Several pipelines can soak at once, each in its own process: what a leak hunt needs
is exposure — ticks executed across distinct subsystems — and wall-clock is the one
budget that cannot be stretched. Running six pipelines side by side for twelve hours
buys six times the surface of running one, and `--schedule` raises the tick rate so
the same window carries more executions.

Two things still need real elapsed time and cannot be compressed by ticking faster:
anything keyed on a lifetime (`connMaxLifetimeSeconds`, OAuth2 token expiry, cache
TTL) and slow monotonic drift, whose detectability scales with how far apart the
compared windows sit. A 12 h run resolves half the drift a 24 h run would.

Samples land in test-lab/soak/<run>/<pipeline>/samples.csv next to each log, so a
run can be re-analysed later:

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

import yaml

REPO_ROOT = Path(__file__).resolve().parent.parent.parent
SOAK_DIR = REPO_ROOT / "test-lab" / "soak"
COMPOSE_FILE = REPO_ROOT / "test-lab" / "docker-compose.yml"
BIN = REPO_ROOT / "bin" / "cannectors"

# A resource may settle higher than it started (pools fill, caches warm), so the
# gate compares the last quarter of the run with the second quarter rather than
# with the very first samples, and allows a margin before calling it a leak.
#
# The ratio alone is not enough on metrics that count in units rather than
# kilobytes. The Go runtime spawns OS threads on demand and never hands them back,
# so a process settles onto a plateau in steps: one observed run stepped 10 -> 13
# threads at t+90s and stayed flat, which is a 1.30x ratio for three threads. A
# breach therefore has to clear an absolute floor as well, sized so that a plateau
# step stays quiet while a genuine per-tick leak — which compounds over hours — does
# not.
GROWTH_TOLERANCE = {"rss_kb": 1.25, "open_fds": 1.20, "threads": 1.20}
GROWTH_FLOOR = {"rss_kb": 32 * 1024, "open_fds": 16, "threads": 8}

# Below this, the quarter-vs-quarter windows hold too few points to mean anything.
MIN_SAMPLES = 8


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


def reset_wiremock_journal() -> None:
    """Drop the recorded requests so a long run does not exhaust WireMock's heap.

    Twelve hours of fast ticks push tens of thousands of requests through the lab,
    and a volume pipeline carries a thousand records in each body. The journal keeps
    all of it in memory, so without this the container dies of an unrelated cause and
    the soak reports a leak that belongs to the lab rather than to cannectors. No
    assertion here reads the journal, so dropping it costs nothing.
    """
    try:
        subprocess.run(
            ["curl", "-s", "-X", "POST", "http://localhost:18080/__admin/requests/reset"],
            capture_output=True, check=False, timeout=15,
        )
    except subprocess.SubprocessError:
        pass


def prepare_pipeline(pipeline: Path, work_dir: Path, schedule: str | None) -> Path:
    """Copy the pipeline into the run directory, overriding its schedule if asked.

    The copy is what makes a run reproducible: the tick rate a soak was measured at
    is recorded next to its samples instead of living in a flag nobody wrote down.
    Relative paths inside the pipeline (query files, script files) still resolve
    because the binary keeps running from the repository root.
    """
    if schedule is None:
        return pipeline

    doc = yaml.safe_load(pipeline.read_text())
    doc.setdefault("input", {})["schedule"] = schedule
    target = work_dir / "pipeline.yaml"
    target.write_text(yaml.safe_dump(doc, sort_keys=False))
    return target


@dataclass
class SoakProcess:
    """One soaking pipeline: its process, where it writes, and what it has shown."""
    name: str
    proc: subprocess.Popen
    log_file: Path
    work_dir: Path
    samples: list[Sample]


def run_soak(pipelines: list[Path], duration_s: int, interval_s: int,
             schedule: str | None) -> Path:
    run_dir = SOAK_DIR / time.strftime("%Y%m%d-%H%M%S")
    run_dir.mkdir(parents=True, exist_ok=True)

    print(f"building {BIN.relative_to(REPO_ROOT)}")
    build_binary()

    print(f"soaking {len(pipelines)} pipeline(s) for {duration_s}s, "
          f"sampling every {interval_s}s"
          + (f", schedule overridden to {schedule!r}" if schedule else ""))
    print(f"output: {run_dir.relative_to(REPO_ROOT)}")

    running: list[SoakProcess] = []
    for pipeline in pipelines:
        work_dir = run_dir / pipeline.stem
        work_dir.mkdir(parents=True, exist_ok=True)
        log_file = work_dir / "pipeline.log"
        target = prepare_pipeline(pipeline, work_dir, schedule)
        with log_file.open("w") as log:
            proc = subprocess.Popen(
                [str(BIN), "run", str(target)], stdout=log, stderr=subprocess.STDOUT,
                cwd=REPO_ROOT,
            )
        running.append(SoakProcess(pipeline.stem, proc, log_file, work_dir, []))
        print(f"  started {pipeline.stem} (pid {proc.pid})")

    (run_dir / "meta.json").write_text(
        json.dumps(
            {"pipelines": [str(p.relative_to(REPO_ROOT)) for p in pipelines],
             "duration_s": duration_s, "interval_s": interval_s, "schedule": schedule,
             "pids": {r.name: r.proc.pid for r in running}},
            indent=2,
        )
    )

    started = time.time()
    last_journal_reset = started
    try:
        while time.time() - started < duration_s:
            time.sleep(interval_s)
            elapsed = int(time.time() - started)

            # One query per tick, shared by every pipeline: the count is server-side
            # and global, so it cannot be attributed to a single process anyway.
            db_count = db_connections()

            for r in running:
                if r.proc.poll() is not None:
                    continue
                status = proc_status(r.proc.pid)
                r.samples.append(
                    Sample(
                        elapsed_s=elapsed,
                        rss_kb=status["rss_kb"],
                        open_fds=open_fds(r.proc.pid),
                        threads=status["threads"],
                        db_connections=db_count,
                        executions=count_executions(r.log_file),
                    )
                )

            alive = [r for r in running if r.proc.poll() is None]
            for r in running:
                if r.proc.poll() is not None and r.samples:
                    print(f"  {r.name} exited early with code {r.proc.returncode}",
                          file=sys.stderr)
                    r.samples.clear()  # reported once, then left alone
            summary = "  ".join(
                f"{r.name}: rss={r.samples[-1].rss_kb}KiB runs={r.samples[-1].executions}"
                for r in alive if r.samples
            )
            print(f"  t+{elapsed:>6}s  db={db_count:>3}  {summary}")

            if time.time() - last_journal_reset >= 600:
                reset_wiremock_journal()
                last_journal_reset = time.time()

            if not alive:
                print("every pipeline exited; stopping", file=sys.stderr)
                break
    except KeyboardInterrupt:
        print("interrupted, stopping the pipelines")
    finally:
        for r in running:
            if r.proc.poll() is None:
                r.proc.send_signal(signal.SIGTERM)
        for r in running:
            try:
                r.proc.wait(timeout=20)
            except subprocess.TimeoutExpired:
                r.proc.kill()

    for r in running:
        write_samples(r.work_dir / "samples.csv", r.samples)
    return run_dir


def write_samples(path: Path, samples: list[Sample]) -> None:
    fields = (list(asdict(samples[0]).keys()) if samples
              else [f.name for f in Sample.__dataclass_fields__.values()])
    with path.open("w", newline="") as f:
        writer = csv.DictWriter(f, fieldnames=fields)
        writer.writeheader()
        for s in samples:
            writer.writerow(asdict(s))


def load_samples(run_dir: Path) -> list[Sample]:
    path = run_dir / "samples.csv"
    with path.open() as f:
        return [Sample(**{k: int(v) for k, v in row.items()}) for row in csv.DictReader(f)]


def analyse(run_dir: Path) -> int:
    """Report on every pipeline in the run, failing if any one of them does."""
    work_dirs = sorted(d for d in run_dir.iterdir() if (d / "samples.csv").exists())
    if not work_dirs:
        print(f"no sampled pipeline under {run_dir}", file=sys.stderr)
        return 2

    failures = 0
    for work_dir in work_dirs:
        print(f"\n=== {work_dir.name} " + "=" * max(0, 60 - len(work_dir.name)))
        failures += analyse_one(work_dir)

    print("\nVERDICT:", "PASS" if failures == 0
          else f"FAIL ({failures} issue(s) across {len(work_dirs)} pipeline(s))")
    return 0 if failures == 0 else 1


def analyse_one(run_dir: Path) -> int:
    samples = load_samples(run_dir)
    log_file = run_dir / "pipeline.log"
    if len(samples) < MIN_SAMPLES:
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
        ok = ratio <= tolerance or (final - baseline) < GROWTH_FLOOR[metric]
        failures += 0 if ok else 1
        print(f"{metric:<16}{baseline:>12.0f}{final:>12.0f}{ratio:>8.2f}x   "
              f"{'ok' if ok else f'LEAK? > {tolerance}x and +{GROWTH_FLOOR[metric]}'}")

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

    return failures


def main(argv: list[str]) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("targets", nargs="+",
                        help="pipeline yaml files to soak, or one run directory with --analyse")
    parser.add_argument("--schedule",
                        help="override each pipeline's input schedule, e.g. '*/5 * * * * *'. "
                             "Raises the tick rate so a shorter window carries more executions")
    parser.add_argument("--duration", type=parse_duration, default="2h",
                        help="how long to run (90, 30m, 2h, 24h). Default 2h")
    parser.add_argument("--interval", type=parse_duration, default="30s",
                        help="sampling interval. Default 30s")
    parser.add_argument("--analyse", action="store_true",
                        help="re-analyse an existing run directory instead of soaking")
    args = parser.parse_args(argv[1:])

    def resolve(text: str) -> Path:
        path = Path(text)
        return path if path.is_absolute() else REPO_ROOT / path

    if args.analyse:
        if len(args.targets) != 1:
            print("--analyse takes exactly one run directory", file=sys.stderr)
            return 2
        return analyse(resolve(args.targets[0]))

    pipelines = [resolve(t) for t in args.targets]
    missing = [p for p in pipelines if not p.exists()]
    if missing:
        for p in missing:
            print(f"pipeline not found: {p}", file=sys.stderr)
        return 2

    # A bare number means seconds, so `--duration 12` intending twelve hours soaks
    # for twelve seconds and only fails at the end, after building and launching
    # everything. The trend needs MIN_SAMPLES to mean anything, so refuse up front
    # and name the unit suffix rather than waste the run.
    if args.duration < MIN_SAMPLES * args.interval:
        needed = MIN_SAMPLES * args.interval
        print(f"--duration {args.duration}s gives fewer than {MIN_SAMPLES} samples at a "
              f"{args.interval}s interval, which is too few for a trend.\n"
              f"Use at least {needed}s, and remember a bare number means seconds: "
              f"'12h' for twelve hours, '12' for twelve seconds.", file=sys.stderr)
        return 2

    run_dir = run_soak(pipelines, args.duration, args.interval, args.schedule)
    return analyse(run_dir)


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
