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
- [x] **Milestone 5: Branch Management & Safe Checkout**
  - Lightweight branch reference creation (`CreateBranch`) and safe deletion (`DeleteBranch`).
  - Branch listing (`ListBranches`) with active branch indicators (`* \033[32mbranch\033[0m`).
  - Tree flattening (`FlattenTree`) for directory restoration.
  - Safe working-tree checkout engine (`Checkout`): compares working tree, index, and target tree to protect uncommitted/untracked user edits (`ErrDirtyWorkingTree`).
  - Working tree restoration: deletes files not in target commit, writes new/updated files, cleans up empty directories, updates index, and shifts HEAD.
  - Dual commands: `gitlite checkout <branch-or-commit>` (including detached HEAD) and `gitlite switch <branch>`.
  - Integration tests verifying branch creation, file swapping between branches, switch back, and dirty tree clobber prevention.
- [x] **Milestone 6: Diff Engine**
  - Line-by-line file comparison engine (`internal/diff/algorithm.go`) based on the Longest Common Subsequence (LCS) algorithm.
  - Unified diff formatting (`--- a/... +++ b/... @@ -1,X +1,Y @@`).
  - Working tree vs index diffing (`DiffWorkingTreeAgainstIndex`).
  - Staged index vs HEAD commit diffing (`DiffIndexAgainstHead`).
  - Arbitrary commit-to-commit tree comparison (`DiffCommits`).
  - CLI command: `gitlite diff` (working tree vs index), `gitlite diff --staged` / `--cached` (index vs HEAD), and `gitlite diff <commit-a> <commit-b>`.
  - Full integration test suite validating working tree, staged, and commit-to-commit unified diff streams.

## Current Implementation Status
- `internal/object`:
  - `hash.go`: `Hash` type (32-byte SHA-256 hex string), validation, `ComputeHash`.
  - `object.go`: `Object` interface, `Type` enum (`blob`, `tree`, `commit`), `EncodeCanonical`, `DecodeCanonical`.
  - `blob.go`: `Blob` object implementation wrapping raw bytes.
  - `tree.go`: `Tree` and `TreeEntry` supporting binary SHA-256 packing, mode bits, deterministic sorting, and parsing.
  - `commit.go`: `Commit`, `Signature`, and `CommitWithHash` types, canonical framing, timestamp/timezone formatting, and parser.
  - `store.go`: `Store` engine providing `Write`, `Read`, `ReadBlob`, `ReadTree`, `ReadCommit`, `Has`, `Verify`, and 2-level directory path resolution.
  - `store_test.go`, `tree_test.go`, `commit_test.go`: Unit tests and write/read memory & throughput benchmarks.
- `internal/diff`:
  - `algorithm.go`: Line-based Longest Common Subsequence (LCS) edit script generation (`ComputeLCS`) and unified diff formatter (`UnifiedDiff`).
  - `algorithm_test.go`: LCS and unified diff unit tests.
- `internal/refs`:
  - `refs.go`: Reference manager (`ResolveHead`, `UpdateHead`, `atomicWriteRef`).
  - `branch.go`: Branch operations (`ListBranches`, `CreateBranch`, `DeleteBranch`, `SetHeadBranch`, `SetHeadDetached`, `ResolveBranch`).
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
  - `checkout.go`: `FlattenTree` and `Checkout` with working tree integrity guards.
  - `diff.go`: Unified diff coordination (`DiffWorkingTreeAgainstIndex`, `DiffIndexAgainstHead`, `DiffCommits`).
- `internal/cli`:
  - `root.go`: Cobra root setup.
  - `init.go`: `gitlite init [directory]`.
  - `add.go`: `gitlite add <path>...`.
  - `status.go`: `gitlite status`.
  - `commit.go`: `gitlite commit -m "<message>"`.
  - `log.go`: `gitlite log`.
  - `show.go`: `gitlite show [commit]`.
  - `branch.go`: `gitlite branch [-d <name>] [name]`.
  - `checkout.go`: `gitlite checkout <branch-or-commit>` & `gitlite switch <branch>`.
  - `diff.go`: `gitlite diff [--staged] [commit-a] [commit-b]`.
- `tests/integration`:
  - `init_test.go`: Repository initialization integration tests.
  - `staging_test.go`: Add, status, and staging integration tests.
  - `commit_test.go`: End-to-end commit creation, commit ancestry chain logging, and inspection tests.
  - `branch_test.go`: Branch lifecycle, safe checkout, and collision prevention integration tests.
  - `diff_test.go`: Working tree, staged, and commit-to-commit unified diff integration tests.

## Architectural Decisions
1. **LCS / Diff Algorithm**:
   - `ComputeLCS` breaks multiline file streams into discrete lines and performs dynamic-programming longest common subsequence analysis, emitting sequence edits (`OpEqual`, `OpInsert`, `OpDelete`).
2. **Unified Diff Format**:
   - Outputs standard unified diff notation (`--- a/path`, `+++ b/path`, `@@ -old,count +new,count @@`, with `-` and `+` line prefixes) compatible with developer expectations and terminal pagers.
3. **Multi-Source Diffing**:
   - Compares physically modified files against staged index blobs.
   - Compares staged index blobs against tree snapshots in `HEAD`.
   - Compares tree objects between two arbitrary historical commit snapshots.

## Outstanding Tasks / Roadmap
- [ ] **Milestone 7: Three-Way Merge Engine**
  - Common ancestor discovery via commit graph traversal (BFS / Lowest Common Ancestor).
  - Three-way file merging: ancestor vs current branch (`HEAD`) vs incoming branch.
  - Non-conflicting change resolution (independent additions, modifications, deletions).
  - Conflict detection & standard conflict marker generation:
    ```text
    <<<<<<< CURRENT
    ...
    =======
    ...
    >>>>>>> INCOMING
    ```
  - `gitlite merge <branch>` command.
  - Merge commit creation with two parent hashes (`parent <head> \n parent <incoming>`).
  - Merge abort (`gitlite merge --abort`) and continuation (`gitlite merge --continue`).
- [ ] **Milestone 8: Integrity & Recovery (`fsck`)**
- [ ] **Milestone 9: Performance Benchmarking & Profiling**

## Next Recommended Task
- Proceed to **Milestone 7: Three-Way Merge Engine**. Implement common ancestor search across commit DAG, three-way line merge resolution with conflict markers, `gitlite merge`, and merge commit creation.

