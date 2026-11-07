package object

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
)

// Type represents the kind of GitLite object stored in the object database.
type Type string

const (
	// TypeBlob represents arbitrary raw file content.
	TypeBlob Type = "blob"

	// TypeTree represents a directory snapshot containing entries with names, modes, and hashes.
	TypeTree Type = "tree"

	// TypeCommit represents an immutable snapshot with metadata, parents, and a root tree reference.
	TypeCommit Type = "commit"
)

var (
	// ErrCorruptObject indicates header or payload malformation.
	ErrCorruptObject = errors.New("corrupt object format")

	// ErrUnknownType indicates an unrecognized object type.
	ErrUnknownType = errors.New("unknown object type")
)

// Object defines the standard interface that all stored objects must satisfy.
type Object interface {
	// Type returns the object kind (blob, tree, commit).
	Type() Type

	// SerializeContent returns the raw uncompressed content payload (excluding header).
	SerializeContent() []byte
}

// EncodeCanonical wraps the object's content with the canonical header:
// `<type> <size>\0<content>`
func EncodeCanonical(obj Object) []byte {
	content := obj.SerializeContent()
	header := fmt.Sprintf("%s %d\x00", obj.Type(), len(content))
	buf := make([]byte, 0, len(header)+len(content))
	buf = append(buf, header...)
	buf = append(buf, content...)
	return buf
}

// DecodeCanonical parses a raw uncompressed canonical buffer into its Object Type and content slice.
// Expected format: `<type> <size>\0<content>`
func DecodeCanonical(data []byte) (Type, []byte, error) {
	nullIdx := bytes.IndexByte(data, 0)
	if nullIdx == -1 {
		return "", nil, fmt.Errorf("%w: missing null delimiter", ErrCorruptObject)
	}

	header := string(data[:nullIdx])
	spaceIdx := bytes.IndexByte([]byte(header), ' ')
	if spaceIdx == -1 {
		return "", nil, fmt.Errorf("%w: malformed header", ErrCorruptObject)
	}

	objTypeStr := header[:spaceIdx]
	sizeStr := header[spaceIdx+1:]

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil || size < 0 {
		return "", nil, fmt.Errorf("%w: invalid content size %q", ErrCorruptObject, sizeStr)
	}

	content := data[nullIdx+1:]
	if int64(len(content)) != size {
		return "", nil, fmt.Errorf("%w: declared size %d does not match payload size %d", ErrCorruptObject, size, len(content))
	}

	objType := Type(objTypeStr)
	switch objType {
	case TypeBlob, TypeTree, TypeCommit:
		return objType, content, nil
	default:
		return "", nil, fmt.Errorf("%w: %q", ErrUnknownType, objTypeStr)
	}
}
