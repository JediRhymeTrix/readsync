# Moon+ Reader Pro WebDAV Fixture Recorder

Runs an embedded WebDAV server. Every `.po` file PUT by Moon+ Pro is captured
with a timestamp suffix to `fixtures/moonplus/captures/`.

## Quick Start

```bash
# Start the recorder
go run . --port 8765 --verbose

# Configure Moon+ Pro (Android):
# Settings → Sync → Sync reading positions → WebDAV
#   URL: http://<YOUR_PC_LAN_IP>:8765/dav/
#   Sync folder: /moonreader/
```

## Capture Session

Follow the script in `docs/research/moonplus.md` §5:

1. Open test book at 10% → close → file saved as `Test_Book_<timestamp>.po`
2. Open at 25% → close → another capture
3. ...repeat at 50%, 75%, 100%

Then diff the captures:
```bash
xxd captures/Test_Book_*_010pct*.po > /tmp/s1.hex
xxd captures/Test_Book_*_025pct*.po > /tmp/s2.hex
diff /tmp/s1.hex /tmp/s2.hex
```

## Generating Synthetic Fixtures (CI use)

```bash
go run ./cmd/generate-synthetic --out ../../fixtures/moonplus/synthetic
```

Produces: `010pct.po`, `025pct.po`, `050pct.po`, `075pct.po`, `100pct.po`

## Flags

| Flag            | Default                                | Description                    |
|-----------------|----------------------------------------|--------------------------------|
| `--port`        | `8765`                                 | HTTP port                      |
| `--capture-dir` | `../../fixtures/moonplus/captures`     | Where to save .po files        |
| `--verbose`     | `false`                                | Log all WebDAV operations      |

## .po File Format

See `docs/research/moonplus.md` §3 for binary format documentation.
