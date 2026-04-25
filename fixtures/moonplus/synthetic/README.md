# Synthetic Moon+ .po Fixtures

> **Format updated from real device captures 2026-04-25.**  
> Moon+ Pro v9+ uses **plain UTF-8 text**, not binary.

## Format

```
{file_id}*{position}@{chapter}#{scroll}:{percentage}%
```

Example: `1703471974608*35@2#20432:73.2%`

## Files

| File      | Content                        | Progress |
|-----------|--------------------------------|----------|
| 010pct.po | `1703471974608*5@0#8192:10.0%` | 10%      |
| 025pct.po | `1703471974608*12@0#33241:25.0%`| 25%     |
| 050pct.po | `1703471974608*28@1#16384:50.0%`| 50%     |
| 075pct.po | `1703471974608*42@2#4096:75.0%`| 75%      |
| 100pct.po | `1703471974608*52@0#9486:100%` | 100%     |

## Generate

```bash
cd tools/moon-fixture-recorder
go run ./cmd/generate-synthetic --out ../../fixtures/moonplus/synthetic
```
