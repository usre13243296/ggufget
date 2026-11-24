# ggufget manual

ggufget downloads GGUF models from mirrors straight into the LM Studio models
directory. One static binary, no Python, no pip, no hosts file.

## intro what it solves

LM Studio fetches models through its own cloud proxy
(`search.lmstudio.ai/v1/hf-proxy`). In many networks that path runs at a few
KB/s and large files keep breaking, so a 7 GB model never finishes.

ggufget takes a different route:

1. it asks a mirror for the real file list, so you see each quant and its size;
2. it downloads with parallel ranged requests and resumes after interruptions;
3. it writes `<models dir>/<file-stem>/lmstudio-ai/<file>.gguf`, the layout LM
   Studio indexes.

It never touches `hosts`, installs drivers or changes system settings, so
security software has nothing to object to.

## install install

Windows (PowerShell 5.1 or 7):

```powershell
irm https://raw.githubusercontent.com/usre13243296/ggufget/main/install/install.ps1 | iex
```

Linux / macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/usre13243296/ggufget/main/install/install.sh | sh
```

Or from source (Go 1.24+):

```sh
git clone https://github.com/usre13243296/ggufget
cd ggufget
make build
```

Re-run the installer to upgrade (same version is skipped, `-Force` reinstalls).

## shell the interactive shell

Run `ggufget` with no arguments (or double-click the exe) to get the shell. It
opens with a status block:

```
ggufget 1.2.1
models dir  C:\Users\you\.lmstudio\models
mirrors     modelscope 8.5 MiB/s  ·  hf-mirror 0.7 MiB/s
[1] download   [2] search   [3] speed test   [4] settings   [h] help   [q] quit
```

The speeds are the last measurements; if there are none it says "press [3]".

```
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
| `TAB` | complete: repo history, command names, language/layout/mirror values, menu keys |
| `↑` `↓` | input history (last 200 entries, kept in the config file) |
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

- Press `h` in the menu, or run `ggufget help`: every command with a one-line
  description, the global options, and the exit codes.
- `ggufget help get`: full options and examples for one command.
- The long-form manual is this file. The program looks for `MANUAL.md` in the
  current directory, next to the exe, and in `exe/docs`, then prints the path it
  found — the text itself is **not** compiled into the binary.
- With a pipe or redirected stdin the shell degrades to plain line-by-line
  questions instead of raw mode.

## providers mirrors

| name | endpoint | notes |
| --- | --- | --- |
| `modelscope` | modelscope.cn | fastest from mainland China, ~8.5 MB/s measured |
| `hf-mirror` | hf-mirror.com | full Hugging Face mirror, best coverage |
| `hf` | huggingface.co | upstream, usually needs a proxy |

`--provider auto` (the default) probes all three at once and takes the first
one that answers, in the order modelscope → hf-mirror → hf. Pin one with:

```sh
ggufget get owner/repo -f 1 -p modelscope
```

ModelScope carries only a subset of Hugging Face; when a repo is missing there,
auto falls through to hf-mirror.

## select choosing files

```sh
ggufget list lmstudio-community/Qwen2.5-7B-Instruct-GGUF
```

```
[modelscope] 4 files, 21.5 GiB total (use --any for the rest)
idx  file                                    size        quant
---  --------------------------------------  ----------  --------
  1  Qwen2.5-7B-Instruct-Q3_K_L.gguf         3.8 GiB     Q3_K_L
  2  Qwen2.5-7B-Instruct-Q4_K_M.gguf         4.4 GiB     Q4_K_M
```

`-f` accepts four forms and may be repeated:

- indexes: `-f 2`, `-f 1,3`, `-f 1-3`
- a fragment: `-f q4_k`
- a full filename
- everything: `-a`

Matching is case-insensitive. An ambiguous fragment never guesses: it prints the
candidates and asks you to be more specific. Narrow the list first with
`ggufget list --filter q4 --sort size`.

## download downloading

```sh
ggufget get owner/repo -f 2
```

ggufget prints a plan (mirror, commit, destination, sizes) and asks once.
Useful flags:

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

How it works: a `Range: bytes=0-0` probe learns the total size and whether
ranges are supported; big files are split across connections with progress
journalled to `<file>.part.seg` every two seconds; network errors, timeouts and
HTTP 429 back off and retry; the `.part` file is renamed only after the whole
file is written and verified, so a half `.gguf` never appears.

Ctrl+C is safe. The `.part` file stays and re-running the same command resumes.

## verify verification

`--verify` has four modes:

- `auto` (default): SHA256 for files under 4 GiB or for any download that
  resumed or retried; otherwise compare size only.
- `size`: size only, fastest.
- `sha256`: always hash; fail with exit code 4 when the mirror has no hash.
- `off`: no checks.

A hash mismatch deletes the bad file, so you can simply retry.
`--sha256 <hex>` supplies an expected value by hand.

## log the log file

The log is on by default and lives **next to the binary**:

```sh
ggufget get owner/repo -f 1
# on failure the terminal adds one line: log: D:\tools\ggufget\ggufget.log
```

When something fails without explaining itself (`Access is denied`, a strange
mirror status, a size that does not add up after a resume), read that file. It
carries what the terminal never shows:

- startup: version, OS, pid, full command line (tokens masked), working directory
- config: which file was read, every effective value, where the models dir came from
- the absolute destination of each file, its size, the expected SHA256
- every probe: URL, HTTP status, real size, ETag, whether ranges are supported
- on failure the whole error chain plus the numeric `errno` (`Access is denied` is `errno=5`)
- verification result and the exit code

```sh
ggufget get owner/repo -f 1 --log ./debug.log   # pick the file
ggufget get owner/repo -f 1 --log-level debug   # error|warn|info|debug, default info
ggufget get owner/repo -f 1 --no-log            # skip it once
```

`GGUFGET_LOG` / `GGUFGET_LOG_LEVEL` and the `log` / `log_level` config keys do the
same. `--verbose` raises the level to `debug` for that run.

If the program directory is not writable (read-only media, an install under
`Program Files`), it falls back to the user cache (`%LOCALAPPDATA%\ggufget\logs\`
on Windows, `~/.cache/ggufget/logs/` elsewhere) and then to the temp directory.
If none of them works it keeps going and tells you which attempts failed.

The file rotates once at 4 MiB; the previous one becomes `ggufget.log.1`.

### When the config file cannot be written

Settings live in the user config directory (`%APPDATA%\ggufget\config.json` on
Windows). If that directory cannot be created — a security product blocking an
unsigned binary from writing there, a roaming profile locked by policy — nothing
is lost: the tool steps back to a **portable `ggufget.config.json` next to the
binary**, prints "saved to …", and uses that file from then on. Pass
`--config <path>` to pin a location; if that one is not writable either, it
reports the error instead of quietly relocating.

The "config file" row in `ggufget doctor` shows the path actually in use and
whether its directory is writable.

`ggufget doctor` prints the active log path with "writable / not writable", and
the result of a write test on the models directory.

## lmstudio working with LM Studio

LM Studio indexes this layout:

```
<models dir>/<file-stem>/
└── lmstudio-ai/
    └── <file>.gguf
```

so `DeepSeek-Coder-V2-Lite-Instruct-IQ2_S.gguf` ends up in
`<models dir>/DeepSeek-Coder-V2-Lite-Instruct-IQ2_S/lmstudio-ai/`. ggufget
creates both directories for you. To write a different shape, use `--layout`:

- `lmstudio` (default): `<file-stem>/lmstudio-ai/<file>.gguf`
- `repo`: `<publisher>/<repo>/<file>.gguf`
- `flat`: straight into the target directory, no subdirectories
  (`--no-lmstudio` is the same thing)

The default can also be set with the `layout` config field or the
`GGUFGET_LAYOUT` environment variable.

The models directory is resolved in this order:

1. `--out`
2. `GGUFGET_MODELS_DIR`
3. `models_dir` in the config file
4. `downloadsFolder` in `~/.lmstudio/settings.json`
5. `~/.lmstudio/models`

After a download, when `lms` is available, ggufget checks the index and tells
you whether the model is already visible. If it is not, refresh in LM Studio or
run `ggufget doctor` to confirm the models directory.

## config configuration

See what is actually in effect before changing anything:

```sh
ggufget config          # file path, whether it exists, every effective value
ggufget config --json   # the same, machine readable
ggufget config --init   # write a default file (never overwrites an existing one)
```

- Windows: `%APPDATA%\ggufget\config.json`
- Linux / macOS: `~/.config/ggufget/config.json`

`GGUFGET_CONFIG` relocates the file, `--config <path>` overrides it for one run.
A missing file simply means defaults, so you can ignore it entirely.

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
  "lang": "en",
  "color": true
}
```

`connections` and `jobs` accept `0` for automatic (4 segments per file, 1 file
at a time).

Environment overrides: `GGUFGET_PROVIDER`, `GGUFGET_MODELS_DIR`,
`GGUFGET_LAYOUT`, `GGUFGET_HF_ENDPOINT`, `GGUFGET_TIMEOUT`, `GGUFGET_PROXY`,
`GGUFGET_LANG`, `HF_TOKEN`. `NO_COLOR=1` turns colour off.

## exit-codes exit codes

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

## troubleshooting troubleshooting

**Stopped halfway** — run the same command again; it resumes from `.part`.

**Only a few hundred KB/s** — run `ggufget doctor`, compare the mirrors, then
pin the faster one with `-p`. Behind a proxy, pass `--proxy`.

**HTTP 404** — the repo id is wrong, or the repo only exists on Hugging Face.
Find the exact id with `ggufget search <keyword>`.

**Private repo** — pass `--token` or set `HF_TOKEN`.

**Nothing can be written — not the config, not the models directory — yet File
Explorer can create folders there** — the program is running from inside a
sandboxed directory. Sandboxes of that kind confine *executables stored in the
directory* to writing inside it only, so `%APPDATA%` and `~/.lmstudio/models`
answer `Access is denied` while Explorer (a different, signed program) succeeds.
Copy the binary to an ordinary folder (`D:\tools\ggufget\`, the Desktop) and run
it there. `ggufget doctor` prints `writable` / `not writable` for the models
directory and the config file, so it is the quickest way to confirm.

**Downloaded but LM Studio cannot see it** — `ggufget doctor` shows the models
directory in use; files written with `--no-lmstudio` are flat and LM Studio will
not index them.

**Antivirus complains** — Go binaries are sometimes flagged heuristically; build
from source or use the `--json` output instead of shelling out.

## faq faq

**Does it modify hosts or system settings?** No. Everything stays inside the
models directory.

**Which platforms?** Windows, Linux and macOS on x64 and arm64, all single static
files.

**Non-gguf files?** `list --any` shows everything and `-f` accepts a full
filename; `-a` only takes gguf.

**Logging?** `--log ggufget.log`.

**Unattended use?** Yes: `-y` skips prompts, exit codes identify the failure and
`--json` emits one JSON event per line.
