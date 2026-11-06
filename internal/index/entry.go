package index

import (
	"gitlite/internal/object"
)

// Entry represents a single tracked file in the index staging area.
type Entry struct {
	// Path is the clean slash-separated relative path from the repository worktree root.
	Path string `json:"path"`

	// Hash is the SHA-256 identifier of the staged blob.
	Hash object.Hash `json:"hash"`

	// Mode is the file mode ("100644" for regular, "100755" for executable).
	Mode object.FileMode `json:"mode"`

	// Size is the file size in bytes at time of staging.
	Size int64 `json:"size"`

	// ModTimeUnix is the file modification timestamp in nanoseconds.
	ModTimeNano int64 `json:"mod_time_nano"`
}

