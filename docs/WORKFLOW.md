**English** · [Русский](WORKFLOW.ru.md) · [Українська](WORKFLOW.uk.md)

# Regulation: branches, commits, versions, `clc.sh`

How work on Ζωή is done. The regulation is a custom versioning scheme `R.sB.C+T.hash` ("reverse increment fixation") and a git flow where local history is free and remote history is protected.

- [1. Version](#1-version)
- [2. Two zones: local and remote](#2-two-zones-local-and-remote)
- [3. Branches and GitHub rules](#3-branches-and-github-rules)
- [4. Commits](#4-commits)
- [5. Life of a feature](#5-life-of-a-feature)
- [6. Build](#6-build)
- [7. Hotfix and release (plan)](#7-hotfix-and-release-plan)
- [8. clc.sh](#8-clcsh)
- [9. CI](#9-ci)
- [10. Before "done"](#10-before-done)

## 1. Version

`R.sB.C+T.hash`

| Part | Meaning |
|---|---|
| `R` | Release: major, grows only by the team's decision (breaking changes), not automatically on a merge into `main` |
| `s` | state: `a` alpha, `b` beta, `s` stable, `m` milestone (a version that gives real benefit: input → processing → output) |
| `B` | Build: build number (in our web project a build = a macro theme, not a compilation) |
| `C` | Commit: sequence number of a named commit within the build |
| `T` | Total: total number of named commits in the major version |
| `hash` | 7 characters of the hash of the feature's last atomic commit (the one being frozen) |

**Reverse increment fixation:** the version is fixed *before* the digit is reset (in SemVer — after). That is why there are no zeros: the first entry is `1.1.1`. Build 1 was closed at `1.1.29`, the next build starts from `1.2.1`; likewise build 2 is closed at `1.2.113+143`, build 3 starts from `1.3.1+144` (the freeze tag of build 2 is `v1.s2.113+143`) (`C` resets, `T` goes on). Release of a named build: `B+1`, `C` is reset; release: `R+1`, `B` and `T` are reset.

State grows left to right: `a → b → s → m`; `a` is the most frequent, we do not hurry with `s`. The team assigns the letter to a build. Users usually see `R.sB` or `R.sB.C`; `+T.hash` is for developers.

**The state letter exists only in branches above `dev`** (`build`, `main`, `fix`). On `dev` the version has no letter: `v1.1.29+29`.

Examples: `v1.1.29+29.3487fe9` (a named commit on `dev`), `v1.b1.29+29.8ac5080` (build freeze).

## 2. Two zones: local and remote

**Locally (trunk-based, modified).** One branch `<module>/<date>-<word>` per feature (a module is a set of features of one context: `ui`, `candles`, `binance`, `sources`, `macro`, `docs`, `tooling`…; there is no list, any latin word except service branch names such as `dev` or `build` works, `./clc.sh modules` shows what already exists); atomic commits, noise is allowed (delete, bring back, rewrite — it is personal history). The atomic commits and the module branch stay only on your machine (the module branch itself is pushed to `origin` for the PR, but a single squashed commit lands in `dev`).

**Remotely.** Protected branches: no authors' pushes, no force-push, no local merge-push; everything goes through a PR, CI checks and review. A PR is merged into `dev` by **Squash**: all atomic commits of the feature turn into one **named (frozen) commit**.

Named commit: `feature summary | vR.B.C+T.<hash7>`, where `hash7` is the short hash of the branch's last atomic commit. One branch = one feature = one named commit.

> Consequence of Squash: the `dev` graph is a single line of named commits, and the module branches hang beside it (a squash commit has no parent at the branch tip). This is not an error. A clean line of versions: `git log --first-parent dev`.

## 3. Branches and GitHub rules

| Branch | What it is | Merge method |
|---|---|---|
| `<module>/<date>-<word>` | feature working branch of a module (old `wip/…` are the same kind) | local |
| `dev` (default) | stream of named commits, no letter | **Squash** only |
| `build` | build freezes | **Merge commit** only |
| `main` | release line (demonstration for now; optionally a separate `release` for tags) | **Merge commit** only |
| `fix` | a duplicate of `dev` for hotfixes (plan) | protected from force-push |

Ruleset on `dev`, `build`, `main`: deletion forbidden, force-push forbidden, a PR is required (approvals: 0), required checks `backend`, `frontend`, `hygiene`. Do not enable "Require branches to be up to date" and "Linear history" (the build's merge commits are needed). Auto-deletion of head branches is on for PRs; the default branch is `dev`.

## 4. Commits

- **Atomic:** Conventional Commits `type: what was done`, **in English**, imperative mood, lowercase, no longer than 72 characters after `type: `. Types: `feat`, `fix`, `refactor`, `perf`, `test`, `docs`, `style`, `chore`; a breaking change is `feat!: …`. The version is not written in them.
  Examples: `feat: add render pipeline`, `fix: lift bans on cards when IBAN is relinked`.
- **Named** (appear at a PR): `Summary | vR.B.C+T.hash7`. Each such commit increases `C` and `T`.
- One commit = one logical change that builds and passes the tests. Do not mix backend, frontend and refactoring in one commit.
- `*.db`, `.env`, keys, statements and archives never get into the repository (CI checks it, `clc.sh` does not let them into the index).

## 5. Life of a feature

```bash
git fetch -q && ./clc.sh next <module> <word>      # a new module branch from a fresh origin/dev
# ...edits...
./clc.sh ship all "feat: add thing" -z "Thing: what came out" -f
```

`ship` does it all itself: applies patches from `~/Downloads/patch*.zip`, runs the checks (`go test`, frontend build), commits; `-z "summary"` then opens a PR into `dev` with the named-commit title, waits for the checks and merges by Squash. After that `./clc.sh next <module> <word>` starts the next feature from a fresh `dev`, not from the tip of the previous branch.

The first feature of a new build is opened with `-v R.B`, for example `./clc.sh ship all "feat: …" -z "Summary" -v 1.3`: `C` resets, `T` goes on (`next_version`). Inside a build `-v` is not needed.

To merely add to what is already committed: `./clc.sh amend <scope>` (while the commit is not on the remote).

## 6. Build

A build = a snapshot of the last named commit of `dev`, **without source edits**, until a stable product is assembled. We have no compilation, so "assembly" = `go test` + `bun run build` + manual check in the UI.

```bash
./clc.sh build            # beta (b): PR dev → build, Merge commit, tag
./clc.sh build a          # another letter (a|b|s|m)
./clc.sh build s "Build 2: macro and market"   # custom name; the tag is computed from the dev tip
```

What `build` does:
1. Takes the tip of `origin/dev` (it must be a named commit) and computes the version: `C` and `T` do not move, a letter is added (`v1.1.29+29` → `v1.b1.29+29`).
2. Waits until CI on this commit finishes and cuts the build only if everything is green.
3. Opens (or finds) the PR `dev → build` and waits for the checks of exactly this PR.
4. Merges it by **Merge commit** with the title `Build 1 beta | v1.b1.29+29.<hash7 of the dev tip>`: a named freeze commit with the letter appears on `build`, and the `dev` tip is its second parent.
5. Puts an annotated tag `v1.b1.29+29` on this commit.

If `dev` has nothing new compared to `build`, there is nothing to freeze. The command creates no new branches.

## 7. Hotfix and release (plan)

The `hotfix` command is not implemented yet; the scheme is:

- **Build hotfix:** a bug found during testing is fixed in `build` (an extra increment) through the `fix` branch (a duplicate of `dev`) or directly in `build`. Version `R.sB-F.C+T.hash`, for example `1.a1-3.29`: the number after "−" (the build rebuild number) grows; `C` does **not** move; `T` grows. A hotfix does not open a new build. After hotfixes `fix` is merged into `dev` by a named commit through a PR: the fix returns to the main stream.
- **Release:** successful builds the team decided to publish are merged into a release named commit in `main`. `R` grows only by the team's decision.

## 8. clc.sh

A helper in the repository root (needs `7z`, and `gh` for PRs). Detailed help: `./clc.sh help`.

| Command | What it does |
|---|---|
| `ship <be\|fe\|docs\|all> "type: what" [-b module/word] [-p] [-f] [-n] [-z "summary"] [-v R.B]` | branch → `apply` → checks → commit → (`-z`) PR and merge |
| `commit <scope> "type: what" [-n] [-b module/word]` | checks → commit (without `apply`) |
| `apply [--force]` | all `patch*.zip` from `~/Downloads` one by one, each has a "before" snapshot in `.commits/applied/` |
| `rollback [list\|N]` | return `backend/` and `frontend/` to before the N-th patch was applied |
| `init` | set up `.commits/`, `.claude` and the lines in `.gitignore` |
| `land "summary" [-v R.B]` | pour the module branch into `dev` as one named commit; if there is a remote, does the same as `done` |
| `branch <module> <word>` / `next <module> <word>` | a module branch from a fresh dev (`next` also does `fetch`); a new module name is announced together with the existing ones, so typos are visible |
| `modules [module]` | modules that have branches (local and `origin`): how many and when the last one is; with a name — the branches of the module |
| `pr "summary" [-v R.B]` | push + PR into `dev` with the named-commit title |
| `merge` | wait for the PR checks → Squash and merge → local `dev` ← `origin/dev` |
| `done "summary"` | `pr` + `merge` |
| `build [a\|b\|s\|m] ["summary"]` | build freeze (see §6) |
| `publish` | `git push -u origin <module branch>` |
| `amend`, `undo` | add to the last commit; take it off (the edits stay in the index) |
| `park`, `unpark` | stash and bring back the uncommitted |
| `good`, `restore` | a mark of a working version after the checks, and return to it |
| `look`, `run`, `push` | a repository snapshot into `.claude`, running a command with the output recorded, a file package for the chat |
| `sw <branch>`, `drop <branch> [-f]` | switch to / delete a local branch |

Scope for `ship`/`commit`: `be` (only `backend/`), `fe` (`frontend/`), `docs` (only `*.md` and `.github`), `all` (everything except the unneeded). Flags: `-p` push the package, `-f` apply patches on a dirty tree, `-n` no checks.

The script protects the repository: databases, keys and `.env` are taken off the stage, committing directly to `dev`/`build`/`fix`/`release` is not possible.

## 9. CI

`.github/workflows/ci.yml` runs on every PR and push to `dev`, `build`, `main` (a new push cancels the stale run):

- **backend:** `gofmt -l` (the tree must be formatted), `go vet`, `golangci-lint` (version pinned, v2.15.0; config `backend/.golangci.yml`), `go test ./... -count=1`, `go build`;
- **frontend:** `bun install --frozen-lockfile`, `bun run test` (`node --test` for logic and vitest for components), `bun run build`;
- **hygiene:** no databases, keys, archives (`.db`, `.sqlite`, `.pem`, `.key`, `.p12`, `.zip`), `.env`, `.commits/`, `.claude`; no secrets (`AKIA…`, `ghp_…`, private keys); no 16-digit numbers that look like a card number in `backend` and `frontend/src` (test stubs are put into an allowlist inside `ci.yml`).

These three jobs are the required checks in the ruleset.

## 10. Before "done"

1. `cd backend && go test ./...`, `gofmt -l .` with no output and `golangci-lint run` with no remarks.
2. `cd frontend && bun run test && bun run build`.
3. New parsers and rules — tests on anonymous data, including a repeated import (no duplicates).
4. No personal data in fixtures and documentation: fictional names, numbers with asterisks.
5. If you edit a "Go ↔ JS" pair (`idKey`, `looseKey`, `isPanLike`, the cache index) — edit both sides.
