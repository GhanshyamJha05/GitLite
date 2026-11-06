# GitLite Development Status & Log

## Completed Milestones
- [x] **Milestone 1: Environment & Repository Initialization**
  - Go module initialized (`gitlite`, Go 1.27).
  - Cobra CLI integrated with root command and subcommand structure.
  - Safe, idempotent repository initialization in `internal/repository`.
  - Upward root discovery (`Discover`).
  - Unit tests for repository init, idempotency, and root discovery.
  - Integration tests for CLI `gitlite init <dir>`.
  - Data-race checks (`go test -race ./...`) and static analysis (`go vet ./...`) passing cleanly.
- [x] **Milestone 2: Object Database (Content-Addressable Storage Engine)**
  - Cryptographic object identification using SHA-256 (`object.Hash`).
  - Canonical object framing: `<type> <size>\0<content>`.
  - Object store (`Store`) with two-level hexadecimal directory sharding (`objects/ab/cdef...`).
  - Deflate/Inflate compression via `compress/zlib`.
  - Crash-safe, atomic disk persistence via `os.CreateTemp` and `os.Rename`.
  - Content integrity checks on read (detecting hash mismatches and bit rot).
  - De-duplication on identical object writes.
  - Comprehensive unit tests: hash validation, corruption detection, concurrent writes, and Go benchmarks.
- [x] **Milestone 3: Trees and Staging Area (Index)**
  - Tree objects (`object.Tree`, `object.TreeEntry`) with canonical serialization and deterministic POSIX byte-order sorting.
  - In-memory and on-disk Index (`internal/index`) tracking file metadata (mtime, size, mode, path, blob hash).
  - Atomic index serialization via temporary file write and atomic rename (`.gitlite/index`).
  - Bottom-up hierarchical tree generation from staged paths (`Index.WriteTree`).
  - Recursive working-tree traversal with path traversal guardrails and `.gitlite` exclusion.
  - `gitlite add <file>` and recursive `gitlite add .`.
  - `gitlite status` reporting untracked, modified, and staged changes.
  - Integration tests verifying full add, staging, status, and tree generation cycle.
- [x] **Milestone 4: Commits and History**
  - Canonical Commit object formatting with root tree hash, parent commit pointers, Author/Committer signatures, and commit message.
  - Commit parser (`ParseCommit`) and CAS loader (`ReadCommit`).
  - Atomic Reference Management (`internal/refs`): `ResolveHead`, `UpdateHead` with unborn branch resolution and crash-safe updates.
  - Snapshot creation via `gitlite commit -m "..."`.
  - Full DAG ancestry traversal via `gitlite log`.
  - Detailed commit inspection via `gitlite show [commit-hash]`.
  - Integration tests verifying commit chains, log outputs, and empty staging safeguards.

## Current Implementation Status
- `internal/object`:
  - `hash.go`: `Hash` type (32-byte SHA-256 hex string), validation, `ComputeHash`.
  - `object.go`: `Object` interface, `Type` enum (`blob`, `tree`, `commit`), `EncodeCanonical`, `DecodeCanonical`.
  - `blob.go`: `Blob` object implementation wrapping raw bytes.
  - `tree.go`: `Tree` and `TreeEntry` supporting binary SHA-256 packing, mode bits, deterministic sorting, and parsing.
  - `commit.go`: `Commit`, `Signature`, and `CommitWithHash` types, canonical framing, timestamp/timezone formatting, and parser.
  - `store.go`: `Store` engine providing `Write`, `Read`, `ReadBlob`, `ReadTree`, `ReadCommit`, `Has`, `Verify`, and 2-level directory path resolution.
  - `store_test.go`, `tree_test.go`, `commit_test.go`: Unit tests and write/read memory & throughput benchmarks.
- `internal/refs`:
  - `refs.go`: Reference manager (`ResolveHead`, `UpdateHead`, `atomicWriteRef`).
  - `refs_test.go`: Unit test validating unborn branch resolution and atomic pointer progression.
- `internal/index`:
  - `entry.go`: Staged file record (`Path`, `Hash`, `Mode`, `Size`, `ModTimeNano`).
  - `index.go`: Staging area management (`Add`, `Remove`, `Get`, `SortedEntries`, `Load`, `Save`, `WriteTree`).
  - `index_test.go`: Staging persistence and nested tree graph generation tests.
- `internal/repository`:
  - `repository.go`: Root repo structure & initialization.
  - `discovery.go`: Upward root discovery.
  - `staging.go`: Staging engine (`Add`, `stageFile`, path security guards).
  - `status.go`: Working tree scanning & change detection (`Status`).
  - `commit.go`: Commit execution pipeline (`Commit`, `Log`, `DefaultSignature`).
- `internal/cli`:
  - `root.go`: Cobra root setup.
  - `init.go`: `gitlite init [directory]`.
  - `add.go`: `gitlite add <path>...`.
  - `status.go`: `gitlite status`.
  - `commit.go`: `gitlite commit -m "<message>"`.
  - `log.go`: `gitlite log`.
  - `show.go`: `gitlite show [commit]`.
- `tests/integration`:
  - `init_test.go`: Repository initialization integration tests.
  - `staging_test.go`: Add, status, and staging integration tests.
  - `commit_test.go`: End-to-end commit creation, commit ancestry chain logging, and inspection tests.

## Architectural Decisions
1. **Commit Object Invariant**:
   - Commits contain a pointer to the root `Tree`, not individual file diffs. Every commit is a full, self-contained repository snapshot.
2. **Deterministic Signatures**:
   - `Signature` standardizes POSIX timestamp and timezone offset strings (`+0530`), guaranteeing reproducibility.
3. **Reference Safety**:
   - `refs.Manager` writes branch heads to temp files before replacing target refs atomically, eliminating dangling branch heads during crash events.
4. **Graph Traversal**:
   - `Log` performs first-parent commit chain traversal back to the root commit, with cycle detection guardrails.

## Outstanding Tasks / Roadmap
- [ ] **Milestone 5: Branch Management & Checkout**
  - Create branches: `gitlite branch <name>`.
  - List branches: `gitlite branch` (highlight active branch).
  - Delete branches: `gitlite branch -d <name>`.
  - Branch switching & Working-tree restoration: `gitlite checkout <branch>` / `gitlite switch <branch>`.
  - Dirty working-tree protection (never overwrite uncommitted local edits).
  - Detached HEAD support.
- [ ] **Milestone 6: Diff Engine**
- [ ] **Milestone 7: Three-Way Merge Engine**
- [ ] **Milestone 8: Integrity & Recovery (`fsck`)**
- [ ] **Milestone 9: Performance Benchmarking & Profiling**

## Next Recommended Task
- Proceed to **Milestone 5: Branch Management & Checkout**. Implement branch creation, branch listing, branch deletion, safe tree-to-working-directory checkout, and dirty working-tree safeguards.

