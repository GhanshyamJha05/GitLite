# GitLite — Version Control System from Scratch in Go

GitLite is an independent, production-grade distributed version control system built from scratch in Go. It implements fundamental computer science and distributed systems concepts:
- Content-addressable storage (CAS)
- Immutable object database (blobs, trees, commits)
- Cryptographic object identification using SHA-256
- Snapshot-based versioning
- Index / staging area
- Branch references & HEAD management
- Differential algorithms (Myers Diff, LCS)
- 3-Way merge resolution with conflict markers
- Repository integrity checking (`fsck`) and crash recovery

## Architecture & Layout

Every GitLite repository contains a `.gitlite` directory:
```text
.gitlite/
├── objects/        # Content-addressed immutable objects (ab/cdef...)
├── refs/
│   └── heads/      # Branch pointers (e.g. main)
├── HEAD            # Current reference (e.g. ref: refs/heads/main)
├── config          # Repository configuration
└── index           # Binary/JSON staging area
```

## Quick Start

### Build
```bash
go build -o bin/gitlite ./cmd/gitlite
```

### Initialize a Repository
```bash
./bin/gitlite init
# Or initialize in a specific path:
./bin/gitlite init my-repo
```

### Run Tests & Verification
```bash
go test -v -race ./...
go vet ./...
```

