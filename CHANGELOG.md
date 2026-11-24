# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and this project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Docs

- README rewritten as a quick start: badges, table of contents, architecture
  sketch, accurate mirror-selection wording, and an explicit warning that `-f`
  indexes always follow the default view.
- MANUAL.md and MANUAL.en.md restructured with tables of contents, English
  anchor prefixes removed, and corrections for the 4 MiB doctor sample, the 15 s
  idle timeout, the disk precheck, proxy setup and antivirus guidance.
- SPEC.md updated to 1.2.1 and aligned with the implementation (disk precheck
  `total - have + 64 MiB`, 416 and ignored-Range semantics, JSON event names,
  log path precedence); a "known implementation gaps" section records the three
  remaining code/documentation differences.
- docs/verify.md corrected (journal version check, size-mismatch behaviour,
  15 s idle timeout, segmentation condition); docs/bench.md gained a measurement
  date, unit notes and the 4 MiB sample; docs/design-principles.md now lists all
  five languages; docs/collaboration.md marks the git and release tasks as done;
  docs/blog.md carries a date/version note.
- Added docs/README.md as the documentation index and maintenance rules.

## [1.2.1] - 2026-11-24

A settings save could fail silently: on a machine where the user config
directory cannot be created (an unsigned binary blocked by a security product,
a locked roaming profile), measuring mirrors printed
`mkdir C:\Users\...\AppData\Roaming\ggufget: Access is denied.` and the speeds
were thrown away — and the new log stayed empty, because only the terminal was
told.

### Fixed

- A failed config save is now written to the log with the full error chain and
  the numeric errno, instead of disappearing with the terminal line.
- When the default config location is not writable, settings fall back to a
  portable `ggufget.config.json` next to the binary, with a visible hint. That
  file is picked up automatically on the next start.
- `--config <path>` keeps its meaning: an explicit location that cannot be
  written reports an error instead of silently relocating.
- `ggufget doctor` prints the config file actually in use plus a write test on
  its directory.
- `ggufget doctor` no longer calls a disabled log "not writable" when `--no-log`
  is given.
- The manual now documents the one case that looks like a permissions bug but is
  not: an executable stored inside a sandboxed directory is confined to writing
  inside that directory, so `%APPDATA%` and the models folder answer
  `Access is denied` while Explorer succeeds. Copy the binary to an ordinary
  folder.

### Root cause of the reported `mkdir …: Access is denied.`

Diagnosed with a controlled experiment on the reporting machine: the very same
binary, run from inside a sandboxed workspace, could not create directories
anywhere outside it; copied to a normal folder, it wrote `%APPDATA%` and the LM
Studio models directory without complaint. Neither Go nor the code was at fault,
and no antivirus was involved.

## [1.2.0] - 2026-11-23

Diagnostics release. A download that failed with `mkdir …: Access is denied.`
could not be explained from the terminal, so the tool now writes down what it
tried, where, with which errno — and refuses to start a download it knows will
fail.

### Added

- A log file, on by default, written next to the binary (`--log`, `--no-log`,
  `--log-level`, `GGUFGET_LOG`, `GGUFGET_LOG_LEVEL`, config `log` / `log_level`;
  falls back to the user cache and then the temp directory; rotates at 4 MiB).
  It records the startup environment and argv (tokens masked), the effective
  config and where the models directory came from, every destination path, every
  probe (URL, status, size, ETag, range support), the full error chain with the
  numeric errno, verification, and the exit code.
- On failure the terminal adds one line: `log: <actual path>`.
- A write test on the models directory before the plan is printed, and one in
  `doctor` next to the models dir and the log path.

### Changed

- An unwritable destination now reports `permission denied writing <dir> (use
  --out or fix the directory permissions)` instead of the raw
  `mkdir …: Access is denied.`, and exits 7 without printing a download plan.

### Fixed

- A single failed file no longer prints its error twice, and the single-file case
  no longer adds a meaningless `0 succeeded, 1 failed.` line.
- `cfg.hint` pointed at the removed `ggufget manual` command; it now points at
  `ggufget help config`.

## [1.1.0] - 2026-11-22

First release. The tool exists because LM Studio's own download proxy measured
roughly 5 KB/s from mainland China, which is 2.7 hours for a 7 GB model.

### Added

- An interactive shell (no arguments, or double-click the exe) that opens with
  the models directory, the mirrors and the last measured speeds, then offers
  download / search / speed test / settings / help / quit. Downloading and
  listing are one flow now: list the repo, type an index, see the plan, confirm.
- `h`/`help` prints a command overview — every command with a one-line
  description, the global options, three tips, the exit codes and where the
  manual file lives. `help <command>` keeps the per-command detail.
- Line editing in every prompt: arrows, `Home`/`End`, `Backspace`/`Delete`,
  `Ctrl+A/E/K/U/W`, `↑`/`↓` history, and `TAB` completion for recent repository
  ids.
- Interface languages: 中文 / English / Français / Español / Русский, switchable
  in settings, via `--lang`, or via `GGUFGET_LANG`; the test suite fails if any
  language is missing a key or drops a format verb.
- Settings screen for language, layout, default mirror and models directory;
  changes apply immediately and are written to the config file.
- The config file now remembers `history` (TAB completion), `favorites`,
  `speeds` and `speeds_at` so the shell can show the last measurements.
- `list` — list a repo's files with index, size and quantisation; `--any`,
  `--filter`, `--sort`, `--limit`.
- `get` — download into the LM Studio models directory using the layout LM
  Studio indexes — `<models>/<file-stem>/lmstudio-ai/<file>.gguf`, i.e. the
  first directory is the file name without `.gguf` and the second is always
  `lmstudio-ai`; `-f`/`--file` accepts an index (`2`), a list (`1,3`), a range
  (`1-3`), a keyword (`q4_k_m`) or an exact filename, and may be repeated.
- `--layout lmstudio|repo|flat` (config `layout`, env `GGUFGET_LAYOUT`) picks
  the on-disk shape: the LM Studio hierarchy (default), the older
  `<publisher>/<repo>/<file>.gguf` form, or a flat copy. `--no-lmstudio` is now
  shorthand for `--layout flat`. Both directories of the default layout are
  created automatically.
- `info` — branch, commit, licence, gated/private flags, total size and which
  mirrors carry the repo.
- `search` — repo search through the Hugging Face API.
- `doctor` — environment report plus a serial speed test of every mirror,
  because parallel probes fight over the same bandwidth.
- `config` — show the effective configuration, its file path, and `--init` to
  write a default file without overwriting an existing one.
- Automatic mirror selection: ModelScope, then hf-mirror, then huggingface.co,
  probed concurrently and chosen by priority.
- Segmented downloads: files above 64 MiB split into 4 range requests, with a
  `.part.seg` journal so a killed download resumes where it stopped.
- Adaptive rate limiting: 429/503 responses back off and halve the number of
  connections instead of hammering the mirror.
- Tiered verification (`auto`, `size`, `sha256`, `off`); `auto` hashes small
  files and any download that was resumed or retried.
- `--json` NDJSON event stream, `--proxy`, `--log`, `--no-progress`, and an
  exit-code contract (0/2/3/4/5/6/7/130).
- Single static binaries for windows/linux/darwin on amd64 and arm64, built
  with `CGO_ENABLED=0` and no third-party modules.
- `install/install.ps1` (Windows PowerShell 5.1 compatible) and
  `install/install.sh` (POSIX sh) that download a release, verify its SHA256
  and put it on PATH.

### Changed

- `--lang` accepts `zh|en|fr|es|ru` instead of `zh|en`.
- The long manual is an external document (`MANUAL.md` / `MANUAL.en.md`) that
  ships next to the binary; the program prints its location instead of embedding
  a few hundred lines of prose. Help inside the shell uses a built-in pager so
  Windows consoles no longer depend on `less`.
- The old numbered menu is gone; the shell is the same code path as the flags
  (`ggufget get …` is what the shell runs), so behaviour cannot drift.

### Removed

- `manual` command, `--no-pager`, and the `internal/manual` embedded text.

### Fixed during development

- The idle watchdog originally returned on its first tick, so a stalled mirror
  could hang a download forever; it now tracks activity across all connections.
- A `416 Range Not Satisfiable` response was treated as success, which could
  silently mark a truncated file complete.
- `hf-mirror.com` answers `/api/models` with a 308 redirect to huggingface.co;
  the client follows it and reports both endpoints when neither is reachable.
- Dug holes in polluted DNS: IPv4 addresses are dialled before IPv6, because a
  resolvable AAAA record that does not route costs one dial timeout per
  request.
- A mistyped command used to be reported twice (once as an unknown option, once
  as an unknown command); it now suggests the closest real command and says it
  once.
- The interactive menu printed its title twice (`ggufget 1.0.0 — ggufget — …`).
- Windows consoles rendered the help text as mojibake when it went through
  `less`/`more`; the pager is now internal and writes UTF-8 directly.
- The shell listed every file in a repo (`.gitattributes`, `README.md`, …) with
  an empty quantisation column; it now shows the same GGUF-only view `list`
  does, so a typed index means the same thing in both places.
- `Delete`/`Home`/`End` (`ESC [ 3~`, `ESC [ 1~`, …) decoded as an unknown key
  because the first digit of the sequence was dropped.
- `speeds_at` was written as `0001-01-01T00:00:00Z` when speeds had never been
  measured, because `time.Time` ignores `omitempty`; it is a pointer now.

## Pre-history

Before this binary there were two Python scripts (`hf_download.py`,
`modelscope_download.py`) and a `download-models.bat` wrapper. They live in
[docs/legacy](docs/legacy) for comparison; they are not maintained.
