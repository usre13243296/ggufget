# ggufget manual

ggufget downloads GGUF models from mirrors straight into the LM Studio models directory. One static binary, no Python, no pip, no hosts file.

## Contents

- [What it solves](#what-it-solves)
- [Install](#install)
- [Interactive shell](#interactive-shell)
- [Mirrors](#mirrors)
- [Selecting files](#selecting-files)
- [Downloading](#downloading)
- [Verification](#verification)
- [Log](#log)
- [Configuration](#configuration)
- [Working with LM Studio](#working-with-lm-studio)
- [Exit codes](#exit-codes)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)

## What it solves

LM Studio fetches models through its own cloud proxy (`search.lmstudio.ai/v1/hf-proxy`). In many networks that path runs at a few KB/s and large files keep breaking, so a 7 GB model never finishes.

ggufget takes a different route:

1. it asks a mirror for the real file list, so you see each quant and its size;
2. it downloads with parallel ranged requests and resumes after interruptions;
3. it writes `<models dir>/<file-stem>/lmstudio-ai/<file>.gguf`, the layout LM Studio indexes.

It never touches `hosts`, installs drivers, changes system settings or needs administrator rights.

## Install

Windows (PowerShell 5.1 or 7):

```powershell
irm https://raw.githubusercontent.com/usre13243296/ggufget/main/install/install.ps1 | iex
```

Linux / macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/usre13243296/ggufget/main/install/install.sh | sh
```

The installer identifies the architecture, downloads the matching asset, verifies its SHA256 and installs it (adding it to PATH when it can). On Windows it appends the user PATH; on Linux/macOS it prints the export line if the install directory is not on PATH. Everything lands in the user profile; no admin rights are needed.

Or build from source (Go 1.24+, no third-party dependencies):

```sh
git clone https://github.com/usre13243296/ggufget
cd ggufget
make build
```

On Windows the output is `dist/ggufget` without an extension; rename it if you need `ggufget.exe`, or use `make dist` for the six-platform release set.

Re-run the installer to upgrade (the same version is skipped, `-Force` reinstalls).

Supported: Windows, Linux and macOS on amd64 and arm64, all single files with no runtime dependencies.

## Interactive shell

Run `ggufget` with no arguments (or double-click the exe) to get the shell. It opens with a status block:

```text
ggufget 1.2.1
models dir  C:\Users\you\.lmstudio\models
mirrors     modelscope 8.1 MiB/s  ·  hf-mirror 0.7 MiB/s
[1] download   [2] search   [3] speed test   [4] settings   [h] help   [q] quit
```

The speeds are the last measurements; if there are none it says "not measured yet (press 3 in the menu)".

```text
[1] download    repo → file list → index/keyword → plan → confirm
[2] search      keyword → numbered list → pick one → download it
[3] speed test  measures all three mirrors serially and saves the numbers
[4] settings    language / layout / mirror / models dir
[h] help        every command, one line each, plus global options and exit codes
[q] quit
```

**Keys**

| Key | Action |
|---|---|
| `1`..`4`, `h`, `q` | pick an item; you can also type the label (`download`) |
| `TAB` | complete a repository id from `history` / `favorites` |
| `↑` `↓` | input history (last 200 in this session; the config keeps the most recent 50) |
| `←` `→` `Home` `End` | move the cursor |
| `Backspace` `Delete` | delete characters |
| `Ctrl+A` / `Ctrl+E` | jump to start / end of line |
| `Ctrl+K` / `Ctrl+U` / `Ctrl+W` | kill to end / to start / one word back |
| `Ctrl+C` | cancel the running download, or go back one level |
| `q` | leave the current level; on the main menu it quits |

**Settings** (menu `[4]`) apply immediately and are saved to the config file:

1. Language: 中文 / English / Français / Español / Русский (also `--lang fr` or `GGUFGET_LANG=ru`).
2. Layout: `lmstudio` / `repo` / `flat` — same as `--layout`.
3. Default mirror: `auto` / `modelscope` / `hf-mirror` / `hf` — same as `--provider`.
4. Models dir: leave empty to keep whatever LM Studio says.

**Help vs manual**

- Press `h` in the menu, or run `ggufget help`: every command with a one-line description, the global options, and the exit codes.
- `ggufget help get`: full options and examples for one command.
- The long-form manual is `MANUAL.md`; this file is its English edition. The program looks for `MANUAL.md` next to the exe, in `exe/docs`, and in the current directory, then prints the path it found — the text itself is **not** compiled into the binary.
- With a pipe or redirected stdin the shell degrades to plain line-by-line questions instead of raw mode.

## Mirrors

| name | endpoint | notes |
| --- | --- | --- |
| `modelscope` | modelscope.cn | usually fastest from mainland China, up to ~8.5 MB/s measured |
| `hf-mirror` | hf-mirror.com | full Hugging Face mirror, best coverage |
| `hf` | huggingface.co | upstream, usually needs a proxy |

`--provider auto` (the default) probes all three at once and takes the first one that can **actually list the repository**, in the order modelscope → hf-mirror → hf. It chooses availability, not speed. To see which one is fastest on your network, run `ggufget doctor`, then pin one:

```sh
ggufget get owner/repo -f 1 -p modelscope
```

ModelScope carries only a subset of Hugging Face; when a repo is missing there, auto falls through to hf-mirror. hf-mirror mirrors files only: its `/api/models` answers with a 308 redirect to huggingface.co, so listing through hf-mirror may also need hf.co to be reachable. Use `--proxy`, or `-p hf`.

## Selecting files

```sh
ggufget list lmstudio-community/Qwen2.5-7B-Instruct-GGUF
```

```text
[modelscope] 4 files, 21.5 GiB total (use --any for the rest)
idx  file                                    size        quant
---  --------------------------------------  ----------  --------
  1  Qwen2.5-7B-Instruct-Q3_K_L.gguf         3.8 GiB     Q3_K_L
  2  Qwen2.5-7B-Instruct-Q4_K_M.gguf         4.4 GiB     Q4_K_M
  3  Qwen2.5-7B-Instruct-Q6_K.gguf           5.8 GiB     Q6_K
  4  Qwen2.5-7B-Instruct-Q8_0.gguf           7.5 GiB     Q8_0
```

`-f` accepts four forms and may be repeated:

- indexes: `-f 2`, `-f 1,3`, `-f 1-3` (a reversed `3-1` is corrected)
- a fragment: `-f q4_k`
- a full filename
- everything: `-a`

**Index rule (important):** `-f` indexes refer to the **default view** — `.gguf` only, sorted by filename. `list`'s `--any` / `--filter` / `--sort` / `--limit` change the display only; they do not change how `-f` interprets an index. If you are looking at a filtered or sorted list, use a keyword or the full filename instead of an index. Full filenames still match any file.

Matching a fragment is case-insensitive. An ambiguous fragment never guesses: it prints the candidates and asks you to be more specific. You can narrow the list first with `ggufget list --filter q4 --sort size`, but select with a keyword or filename.

Input is cleaned automatically: surrounding spaces and quotes (including Chinese quotes), prefixes such as `--file=` / `-f ` / `filename:`, and `\` become `/`.

## Downloading

```sh
ggufget get owner/repo -f 2
```

ggufget prints a plan (mirror, commit, destination, sizes) and asks once. Useful flags:

| flag | effect |
| --- | --- |
| `-y, --yes` | skip confirmation, good for scripts |
| `--dry-run` | print the plan only |
| `-o, --out` | different output directory |
| `-j, --jobs` | files at once (default 1) |
| `-c, --connections` | connections per file (0 = auto, default 4) |
| `--layout` | `lmstudio` (default), `repo`, or `flat` |
| `--no-lmstudio` | same as `--layout flat`; no subdirectories |
| `--overwrite` | allow replacing an existing file |
| `--keep-part` | compatibility no-op; `.part` is kept by default |
| `-b, --rev` | pin a branch or commit (defaults to the commit from the listing) |
| `--retries` | retry attempts (default 3) |
| `--token` | access token (or `HF_TOKEN`) |
| `--json` | NDJSON event stream for scripts |
| `--sha256` | expected SHA256 by hand (single file only) |

How it works:

- a `Range: bytes=0-0` probe learns the total size and whether ranges are supported;
- a file ≥ 64 MiB with range support and at least 2 connections is split across segments (4 by default), with progress journalled to `<file>.part.seg` every two seconds;
- network errors, timeouts and HTTP 429/503 back off and retry; in segmented mode 429/503 also halve the connection count;
- a connection that produces no bytes for 15 seconds is reconnected instead of looking merely slow;
- a server that ignores Range and answers 200 makes single-connection mode restart from zero;
- the `.part` file is renamed only after the whole file is written and verified, so a half `.gguf` never appears.

Ctrl+C is safe: the `.part` file stays and re-running the same command resumes. Before starting, free space is checked against `total - have + 64 MiB`; if it is insufficient the command exits with code 5.

## Verification

`--verify` has four modes:

| value | behaviour |
| --- | --- |
| `auto` (default) | hash when the mirror provides SHA256 and the file is ≤ 4 GiB or the download resumed/retried; otherwise size only |
| `size` | size only, fastest |
| `sha256` | always hash; fail with exit code 4 when the mirror has no hash |
| `off` | no hash check (size is still compared) |

The reason for `auto`: a large first-time download already has the total size, the ETag and segment completeness; hashing 7 GB again costs minutes. But if this download was interrupted or retried, the risk changes and the hash is worth it.

A hash mismatch deletes the bad `.part`, so you can simply retry. `--sha256 <hex>` supplies an expected value by hand.

Hashes come from Hugging Face's `siblings[].lfs.sha256` or ModelScope's `Files[].Sha256`.

## Log

The log is on by default and lives **next to the binary**:

```sh
ggufget get owner/repo -f 1
# on failure the terminal adds one line: log: D:\tools\ggufget\ggufget.log
```

When something fails without explaining itself (`Access is denied`, a strange mirror status, a size that does not add up after a resume), read that file. It carries what the terminal never shows:

- startup: version, OS, pid, full command line (tokens masked), working directory
- config: which file was read, every effective value, the effective models dir
- the absolute destination of each file, its size, the expected SHA256
- every probe: URL, HTTP status, real size, ETag, whether ranges are supported
- on failure the whole error chain plus the numeric `errno` (`Access is denied` is `errno=5`)
- verification result and the exit code

```sh
ggufget get owner/repo -f 1 --log ./debug.log   # pick the file
ggufget get owner/repo -f 1 --log-level debug   # error|warn|info|debug, default info
ggufget get owner/repo -f 1 --no-log            # skip it once
```

`GGUFGET_LOG` / `GGUFGET_LOG_LEVEL` and the `log_level` config key do the same (`log` as a path key is not wired up in 1.2.1, see SPEC §15). `--verbose` raises the level to `debug` for that run.

If the program directory is not writable (read-only media, an install under `Program Files`), it falls back to the user cache (`%LOCALAPPDATA%\ggufget\logs\` on Windows, `~/.cache/ggufget/logs/` elsewhere) and then to the temp directory. If none of them works it keeps going and tells you which attempts failed.

The file rotates once at 4 MiB; the previous one becomes `ggufget.log.1`.

### When the config file cannot be written

Settings live in the user config directory (`%APPDATA%\ggufget\config.json` on Windows). If that directory cannot be created — a security product blocking an unsigned binary from writing there, a roaming profile locked by policy — nothing is lost: the tool steps back to a **portable `ggufget.config.json` next to the binary**, prints "saved to …", and uses that file from then on. Pass `--config <path>` to pin a location; if that one is not writable either, it reports the error instead of quietly relocating.

The "config file" row in `ggufget doctor` shows the path actually in use and whether its directory is writable.

## Configuration

See what is actually in effect before changing anything:

```sh
ggufget config          # file path, whether it exists, every effective value
ggufget config --json   # the same, machine readable
ggufget config --init   # write a default file (never overwrites an existing one)
```

- Windows: `%APPDATA%\ggufget\config.json`
- Linux / macOS: `~/.config/ggufget/config.json`

`GGUFGET_CONFIG` relocates the file, `--config <path>` overrides it for one run. A missing file simply means defaults, so you can ignore it entirely.

Every field is optional:

```json
{
  "provider": "auto",
  "hf_endpoint": "https://hf-mirror.com",
  "ms_endpoint": "https://modelscope.cn",
  "models_dir": "",
  "layout": "lmstudio",
  "connections": 4,
  "jobs": 1,
  "retries": 3,
  "timeout": 30,
  "verify": "auto",
  "proxy": "",
  "lang": "en",
  "color": true
}
```

`connections` and `jobs` accept `0` for automatic (4 segments per file, 1 file at a time).

Environment overrides: `GGUFGET_PROVIDER`, `GGUFGET_HF_ENDPOINT`, `GGUFGET_MS_ENDPOINT`, `GGUFGET_MODELS_DIR`, `GGUFGET_LAYOUT`, `GGUFGET_CONNECTIONS`, `GGUFGET_JOBS`, `GGUFGET_RETRIES`, `GGUFGET_TIMEOUT`, `GGUFGET_VERIFY`, `GGUFGET_LANG`, `GGUFGET_LOG`, `GGUFGET_LOG_LEVEL`, `GGUFGET_TOKEN`, plus `HF_TOKEN` and `HTTPS_PROXY` / `HTTP_PROXY`. `NO_COLOR=1` turns colour off. Precedence: **command line > environment > config file > defaults**.

## Working with LM Studio

LM Studio indexes this layout:

```text
<models dir>/<file-stem>/
└── lmstudio-ai/
    └── <file>.gguf
```

so `DeepSeek-Coder-V2-Lite-Instruct-IQ2_S.gguf` ends up in `<models dir>/DeepSeek-Coder-V2-Lite-Instruct-IQ2_S/lmstudio-ai/`. ggufget creates both directories for you. To write a different shape, use `--layout`:

- `lmstudio` (default): `<file-stem>/lmstudio-ai/<file>.gguf`
- `repo`: `<publisher>/<repo>/<file>.gguf`
- `flat`: straight into the target directory, no subdirectories (`--no-lmstudio` is the same thing)

The default can also be set with the `layout` config field or the `GGUFGET_LAYOUT` environment variable.

The models directory is resolved in this order:

1. `--out`
2. `GGUFGET_MODELS_DIR`
3. `models_dir` in the config file
4. `downloadsFolder` in `~/.lmstudio/settings.json`
5. `~/.lmstudio/models` (created on download if missing)

After a download, when `lms` is available, ggufget checks the index and tells you whether the model is already visible. If it is not, refresh in LM Studio or run `ggufget doctor` to confirm the models directory.

## Exit codes

| code | meaning |
| --- | --- |
| 0 | success |
| 1 | other error |
| 2 | bad arguments |
| 3 | network error (unreachable, timeout, 429) |
| 4 | verification failed |
| 5 | not enough disk space |
| 6 | repo or file not found |
| 7 | no write permission |
| 130 | interrupted |

## Troubleshooting

**Stopped halfway** — run the same command again; it resumes from `.part`.

**Only a few hundred KB/s** — run `ggufget doctor`, compare the mirrors, then pin the faster one with `-p`. Behind a proxy, pass `--proxy`.

**HTTP 404** — the repo id is wrong, or the repo only exists on Hugging Face. Find the exact id with `ggufget search <keyword>`.

**Private repo** — pass `--token` or set `HF_TOKEN`.

**Nothing can be written — not the config, not the models directory — yet File Explorer can create folders there** — the program is running from inside a sandboxed directory. Sandboxes of that kind confine *executables stored in the directory* to writing inside it only, so `%APPDATA%` and `~/.lmstudio/models` answer `Access is denied` while Explorer (a different, signed program) succeeds. Copy the binary to an ordinary folder (`D:\tools\ggufget\`, the Desktop) and run it there. `ggufget doctor` prints `writable` / `not writable` for the models directory and the config file, so it is the quickest way to confirm.

**Downloaded but LM Studio cannot see it** — `ggufget doctor` shows the models directory in use; files written with `--no-lmstudio` are flat and LM Studio will not index them.

**Antivirus complains** — Go binaries are sometimes flagged heuristically. Build from source, or add the binary to your security product's allow list; `--json` changes the output format only and does not help with a false positive.

**Setting a proxy** — `--proxy http://127.0.0.1:7890`, or set `HTTPS_PROXY` / `HTTP_PROXY`. The tool does not read the operating system's proxy settings automatically.

## FAQ

**Does it modify hosts or system settings?** No. Everything stays inside the models directory and the user config directory.

**Which platforms?** Windows, Linux and macOS on x64 and arm64, all single static files.

**Non-gguf files?** `list --any` shows everything and `-f` accepts a full filename; `-a` only takes gguf.

**Logging?** `--log ggufget.log`.

**Unattended use?** Yes: `-y` skips prompts, exit codes identify the failure and `--json` emits one JSON event per line.

**Why doesn't auto pick the fastest mirror?** Because "fastest" depends on the network and the time of day. Auto only guarantees a mirror that can actually serve the repo; measure speeds with `doctor`, then pin one with `--provider`.
