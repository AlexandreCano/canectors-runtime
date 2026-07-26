# Pending scenarios

Scenarios in this directory describe behaviour the runtime **does not implement yet**.
They are valid scenario files but live outside `scenarios/` so `run.py` (and therefore CI)
does not pick them up. Each one is documented in `../KNOWN-FAILURES.md`.

Run one explicitly to confirm the gap still exists:

```bash
cp scenarios-pending/<name>.yaml scenarios/ && python3 test-lab/run.py <name>
# then remove it from scenarios/ again, or keep it once the runtime supports it
```
