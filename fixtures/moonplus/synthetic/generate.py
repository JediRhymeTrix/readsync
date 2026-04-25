#!/usr/bin/env python3
"""
Generate synthetic Moon+ Reader Pro .po fixtures in the real plain-text format
confirmed from live device captures 2026-04-25.

Format: {file_id}*{position}@{chapter}#{scroll}:{percentage}%

Run from this directory: python3 generate.py
"""

# file_id = millisecond mtime of a synthetic test EPUB (fixed for reproducibility)
FILE_ID = "1703471974608"

levels = [
    ("010pct", 5,  0, 8192,  "10.0"),
    ("025pct", 12, 0, 33241, "25.0"),
    ("050pct", 28, 1, 16384, "50.0"),
    ("075pct", 42, 2, 4096,  "75.0"),
    ("100pct", 52, 0, 9486,  "100"),
]

for label, position, chapter, scroll, pct in levels:
    content = f"{FILE_ID}*{position}@{chapter}#{scroll}:{pct}%"
    path = f"{label}.po"
    with open(path, "w", encoding="utf-8") as f:
        f.write(content)
    print(f"Generated {path}  {content!r}")

print("Done.")
