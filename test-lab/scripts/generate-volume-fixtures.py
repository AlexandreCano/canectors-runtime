#!/usr/bin/env python3
"""Generate the volume fixtures used by the reconciliation scenarios.

The committed WireMock bodies are large, so they are generated rather than
hand-written. Running this script is only needed when changing the volumes; the
outputs are committed so the lab works from a fresh clone.

    test-lab/scripts/generate-volume-fixtures.py

Produces, under test-lab/wiremock:
  __files/source/volume-<n>.json            single response holding n records
  __files/source/volume-<strategy>-<k>.json one page of PAGE_SIZE records
  mappings/source/source-volume-*.json      the matching stubs

Record ids are globally unique across pages (VOL-<strategy>-<index>), so a
scenario that loses a page fails the declared-total reconciliation instead of
silently passing.
"""
from __future__ import annotations

import json
from pathlib import Path

LAB = Path(__file__).resolve().parent.parent
FILES = LAB / "wiremock" / "__files" / "source"
MAPPINGS = LAB / "wiremock" / "mappings" / "source"

SINGLE_VOLUMES = (1000, 10000)
PAGE_SIZE = 1000
PAGES = 3


def records(prefix: str, start: int, count: int) -> list[dict]:
    return [
        {"id": f"{prefix}-{i:05d}", "seq": i, "payload": f"value-{i}"}
        for i in range(start, start + count)
    ]


def write_json(path: Path, doc: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(doc, separators=(",", ":")))


def stub(name: str, url_path: str, body_file: str, query: dict | None = None) -> None:
    request: dict = {"method": "GET", "urlPath": url_path}
    if query:
        request["queryParameters"] = {k: {"equalTo": v} for k, v in query.items()}
    write_json(
        MAPPINGS / f"{name}.json",
        {
            "name": name,
            # Pages match on a query param; the first page of the cursor strategy
            # matches on the cursor being absent, so it needs a lower priority
            # than the follow-up pages.
            "priority": 1 if query else 5,
            "request": request,
            "response": {
                "status": 200,
                "headers": {"Content-Type": "application/json"},
                "bodyFileName": f"source/{body_file}",
            },
        },
    )


def main() -> int:
    written = 0

    for n in SINGLE_VOLUMES:
        write_json(FILES / f"volume-{n}.json", {"data": records("VOL", 1, n)})
        stub(f"source-volume-{n}", f"/source/volume/{n}", f"volume-{n}.json")
        written += 2

    total = PAGE_SIZE * PAGES

    # page strategy: total_pages tells the runtime when to stop.
    for page in range(1, PAGES + 1):
        body = f"volume-page-{page}.json"
        write_json(
            FILES / body,
            {
                "total_pages": PAGES,
                "data": records("VOL-PAGE", (page - 1) * PAGE_SIZE + 1, PAGE_SIZE),
            },
        )
        stub(
            f"source-volume-page-{page}",
            "/source/volume/paginated-page",
            body,
            {"page": str(page)},
        )
        written += 2

    # offset strategy: total drives termination.
    for page in range(PAGES):
        offset = page * PAGE_SIZE
        body = f"volume-offset-{offset}.json"
        write_json(
            FILES / body,
            {"total": total, "data": records("VOL-OFFSET", offset + 1, PAGE_SIZE)},
        )
        stub(
            f"source-volume-offset-{offset}",
            "/source/volume/paginated-offset",
            body,
            {"offset": str(offset), "limit": str(PAGE_SIZE)},
        )
        written += 2

    # cursor strategy: each page names the next cursor, the last one is empty.
    for page in range(1, PAGES + 1):
        body = f"volume-cursor-{page}.json"
        next_cursor = f"vol-cursor-{page + 1}" if page < PAGES else ""
        write_json(
            FILES / body,
            {
                "next_cursor": next_cursor,
                "data": records("VOL-CURSOR", (page - 1) * PAGE_SIZE + 1, PAGE_SIZE),
            },
        )
        if page == 1:
            stub("source-volume-cursor-1", "/source/volume/paginated-cursor", body)
        else:
            stub(
                f"source-volume-cursor-{page}",
                "/source/volume/paginated-cursor",
                body,
                {"cursor": f"vol-cursor-{page}"},
            )
        written += 2

    print(f"wrote {written} file(s); paginated total = {total} records per strategy")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
