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

## Current Implementation Status
- `internal/object`:
  - `hash.go`: `Hash` type (32-byte SHA-256 hex string), validation, `ComputeHash`.
  - `object.go`: `Object` interface, `Type` enum (`blob`, `tree`, `commit`), `EncodeCanonical`, `DecodeCanonical`.
  - `blob.go`: `Blob` object implementation wrapping raw bytes.
  - `tree.go`: `Tree` and `TreeEntry` supporting binary SHA-256 packing, mode bits, deterministic sorting, and parsing.
  - `store.go`: `Store` engine providing `Write`, `Read`, `ReadBlob`, `ReadTree`, `Has`, `Verify`, and 2-level directory path resolution.
  - `store_test.go` & `tree_test.go`: Unit tests and write/read memory & throughput benchmarks.
- `internal/index`:
  - `entry.go`: Staged file record (`Path`, `Hash`, `Mode`, `Size`, `ModTimeNano`).
  - `index.go`: Staging area management (`Add`, `Remove`, `Get`, `SortedEntries`, `Load`, `Save`, `WriteTree`).
  - `index_test.go`: Staging persistence and nested tree graph generation tests.
- `internal/repository`:
  - `repository.go`: Root repo structure & initialization.
  - `discovery.go`: Upward root discovery.
  - `staging.go`: Staging engine (`Add`, `stageFile`, path security guards).
  - `status.go`: Working tree scanning & change detection (`Status`).
- `internal/cli`:
  - `root.go`: Cobra root setup.
  - `init.go`: `gitlite init [directory]`.
  - `add.go`: `gitlite add <path>...`.
  - `status.go`: `gitlite status`.
- `tests/integration`:
  - `init_test.go`: Repository initialization integration tests.
  - `staging_test.go`: Add, status, and staging integration tests.

## Architectural Decisions
1. **Tree Entry Serialization**:
   - Canonical format: `<mode> <name>\0<32-byte-binary-hash>`.
   - Binary packing of SHA-256 (32 bytes per entry instead of 64-byte hex) mirrors Git's space efficiency.
2. **Atomic Index Persistence**:
   - `.gitlite/index` is serialized to a JSON payload through a temporary file and atomically renamed, ensuring uncorrupted staging state upon power failure or process kill.
3. **Bottom-Up Hierarchical Tree Generation**:
   - `idx.WriteTree(store)` converts flat path strings (`a/b/c.txt`) into an in-memory prefix tree, writing leaf directories first and linking parent hashes upwards until the root tree hash is returned.
4. **Security & Guardrails**:
   - Worktree path bounds checking prevents directory traversal attacks (`../`).
   - `.gitlite` and `.git` directories are strictly excluded from recursive staging.

## Outstanding Tasks / Roadmap
- [ ] **Milestone 4: Commits and History**
  - Commit object representation: root tree hash, parent commit hashes, author, committer, timestamp, message.
  - Commit object serialization & parsing.
  - `gitlite commit -m "message"`.
  - HEAD reference update to new commit hash.
  - `gitlite log` displaying commit ancestry chain.
  - `gitlite show <commit>`.
- [ ] **Milestone 5: Branching & Checkout**
- [ ] **Milestone 6: Diff Engine**
- [ ] **Milestone 7: Three-Way Merge Engine**
- [ ] **Milestone 8: Integrity & Recovery (`fsck`)**
- [ ] **Milestone 9: Performance Benchmarking & Profiling**

## Next Recommended Task
- Proceed to **Milestone 4: Commits and History**. Implement the `Commit` object, commit graph links, `gitlite commit -m "..."`, reference updates, and `gitlite log`.

