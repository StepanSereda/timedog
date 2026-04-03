# timedog

### Why this exists

The classic **timedog** script answers “what changed between two Time Machine snapshots” as **plain terminal output**. That works for a quick check, but it does not help much when the list is huge: you cannot comfortably **see the directory structure**, **move around** the tree, or **open a file** to see what actually changed inside.

**`timedog-server` exists to add that layer:** a **browser UI** where you **navigate** changed paths as a tree, drill into folders, and open files with **diff-style views** (text and hex) against the **old** and **new** snapshot on disk. **Interactive navigation plus file diffs** are the headline feature of the Go + React stack; the JSONL report format and HTTP API wrap the same comparison logic for scans, saved reports, and automation. The original Perl script remains the simple CLI option.

> **WARNING — 100% vibecoding**  
> The Go server, web UI, and related tooling in this repository were produced with heavy AI assistance. The original Perl `timedog` script predates that work. Treat automation around backups as **experimental**: review permissions, paths, and code before relying on it with real Time Machine data.  
>  
> **Risk:** **You alone** bear **all** risks of using this software—wrong or incomplete diffs, misinterpreted paths, data loss, security issues, downtime, or decisions made from its output. It is provided **as-is** without warranty of any kind (see [LICENSE](LICENSE)). Nothing here is a substitute for your own judgment, backups, or verification. [Full disclaimer →](.github/DISCLAIMER.md)

**[Русская версия → README.ru.md](README.ru.md)**

---

timedog is a Perl script that lists files that changed between Time Machine backups (sizes before/after, totals, optional depth summary, sorting, filters). **This repository also ships `timedog-server`**: a Go program with an embedded React UI that implements the **same comparison semantics** (inode / hard-link aware walks on two snapshot directories) and exports a **JSONL report** instead of terminal-only output.

## What we added or rewrote (vs. the classic Perl tool)

| Area | Notes |
|------|--------|
| **Comparison engine** | New implementation in Go (`internal/scan`): walks the *new* snapshot root, compares each path to the *old* root via `lstat` (inodes, sizes), prunes unchanged subtrees with `SkipDir` like the Perl tool. |
| **Parallel walk** | Optional [`fastwalk`](https://github.com/charlievieth/fastwalk) traversal (`fast_walk` in API / UI); sequential `filepath.WalkDir` still available. |
| **Report format** | JSONL: first line metadata (`timedog-report-meta`), one JSON object per changed path. Optional gzip (`.jsonl.gz`). |
| **Streaming while scanning** | Background scan **creates the output file immediately**, writes **meta**, then **one line per changed path as it is found**, then **re-sorts / applies `-d` rollup** and **rewrites the file** with the final report (totals, skipped paths, sorted order). |
| **HTTP API + UI** | `timedog-server`: list snapshots (`tmutil`), run scans with SSE progress, open reports as a tree, diff text/hex for files under sandboxed roots. Static UI is built with Vite/React and embedded in the binary. |
| **Helpers** | `timecopy.py`, `timediff`, docs like `UsingTimecopy.md` — unchanged purpose; see those files for details. |

The **Perl script** (`./timedog`) remains in the repo for terminal use and as a reference; behaviour of the server is intended to match its model, not line-for-line output formatting.

---

## Legacy Perl script (terminal)

1. Open Terminal (`/Applications/Utilities`).
2. `/path/to/timedog -h` — help in a pager (`q` to exit).
3. Example: `/path/to/timedog -d 5 -l` — summarize up to 5 path segments deep, omit symlinks.

Pass a backup stamp as an argument to compare specific snapshots; `-t` lists backups. Example output shape:

```shell
$ ~/Desktop/timedog -d 5 -l
==> Comparing TM backup 2009-01-15-080533 to 2009-01-15-070632
    1.6KB->    2.9KB        /.Backup.log
...
==> Total Backup: 111 changed files/directories, 8.08MB
```

Numbers in brackets (e.g. `[26]`) count nested changes under a directory when using `-d`.

### Time Machine over the network

Mount the backup sparsebundle/disk image (e.g. via Disk Utility) before pointing the tool at the backup.

### Copying Time Machine volumes

See [timecopy.py](./timecopy.py) and [UsingTimecopy.md](./UsingTimecopy.md).

### Permissions

Unreadable paths may require `sudo`. On modern macOS, grant **Full Disk Access** to your terminal app (System Settings → Privacy & Security → Full Disk Access).

---

## timedog-server (Go + web UI)

The `timedog-server` binary serves a small HTTP API and the built React UI. It compares two Time Machine snapshot directories the same way as the Perl script (hard-link / inode semantics), writes a **JSONL report** (metadata line + one object per changed path), and **does not embed file contents** in the report. File bodies are read **only** when you open the Text/Hex panel and the snapshot roots still exist on disk.

### Requirements

- **macOS** (uses `tmutil listbackups -m` and local filesystem walks).
- **[Go](https://go.dev/dl/)** to build the server; **[Node.js](https://nodejs.org/)** only if you rebuild the web UI for embedding.
- Mount the **Time Machine** backup volume (or disk image) before **New scan** or before loading file content from paths that live on that volume.

### macOS: Full Disk Access (important)

Without **Full Disk Access**, macOS often blocks reads under backup trees, system folders, and other locations—you may see empty lists, skipped paths, or “Operation not permitted” during scans.

1. Open **System Settings** (or **System Preferences** on older macOS) → **Privacy & Security** → **Full Disk Access**.
2. Turn **on** access for the app that **runs** `timedog-server`:
   - If you start the binary from **Terminal.app** or **iTerm**, add that terminal app and enable it.
   - If you run the server from an IDE (Cursor, VS Code, etc.), add that app—or use a normal terminal after granting access there.
3. **Quit and reopen** the terminal (or IDE) after changing the list so the permission is picked up.
4. Optionally add `timedog-server` itself if you launch the binary from Finder or a wrapper; the process that performs filesystem access must be allowed.

This is the same class of permission as for the Perl `timedog` script when reading protected paths.

### Build

From the repository root:

```shell
go build -o timedog-server ./cmd/timedog-server
```

To embed the production UI into the binary, build the frontend and copy files into `cmd/timedog-server/web/dist`:

```shell
cd web && npm install && npm run build
# then copy web/dist/* → cmd/timedog-server/web/dist (e.g. rsync or cp)
```

Alternatively pass `-static /path/to/dist` when starting the server to load assets from a folder without copying into the repo.

### Run

```shell
./timedog-server -addr 127.0.0.1:8080
```

Open in a browser: **http://127.0.0.1:8080** (or the host/port you passed to `-addr`).

**Development (live-reload UI):** in one terminal run `./timedog-server`; in another, `cd web && npm run dev`. The Vite dev server proxies `/api` to the Go backend—set the dev server URL shown by Vite (often `http://localhost:5173`).

### Behaviour notes

- **Scan policy:** Starting a scan runs in a background job on the server; closing the browser does not stop the job unless you click **Cancel**. Completed jobs leave a session id for tree/content viewing.
- **Report format:** `.jsonl` (optional `.jsonl.gz`). Upload/parser detects gzip by magic bytes.
- **Safety:** Content endpoints resolve paths only under the `old_root` / `new_root` recorded for the session.

## License and disclaimer

[GNU General Public License v2.0](LICENSE) (full text; software is provided **without warranty**). Attribution and mixed components: [.github/COPYRIGHT.md](.github/COPYRIGHT.md) (`timecopy.py` is GPL-3.0). **Liability and AI-assisted code:** [.github/DISCLAIMER.md](.github/DISCLAIMER.md).

## Community

Contributing, issue templates, security contact, disclaimer: [.github/](.github/) (`CONTRIBUTING.md`, `SECURITY.md`, `DISCLAIMER.md`).
