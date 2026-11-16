package index

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"

	"gitlite/internal/object"
)

const (
	// BinaryIndexMagic is the 4-byte header signature "DIRC" (Directory Cache).
	BinaryIndexMagic = "DIRC"

	// BinaryIndexVersion is the current index binary format version.
	BinaryIndexVersion uint32 = 2
)

var (
	// ErrCorruptIndexChecksum indicates the index file's trailing SHA-256 checksum did not match.
	ErrCorruptIndexChecksum = errors.New("corrupt index: checksum mismatch")

	// ErrUnsupportedIndexVersion indicates an unknown binary index format version.
	ErrUnsupportedIndexVersion = errors.New("unsupported index version")
)

// EncodeBinary serializes the index into a compact, checksummed binary representation.
func (idx *Index) EncodeBinary() ([]byte, error) {
	entries := idx.SortedEntries()

	// Estimate capacity: 12 bytes header + ~70 bytes per entry + 32 bytes checksum
	buf := make([]byte, 0, 12+len(entries)*70+32)

	// 1. Header: Magic (4B), Version (4B), Entry Count (4B)
	buf = append(buf, BinaryIndexMagic...)

	var hdr [8]byte
	binary.BigEndian.PutUint32(hdr[0:4], BinaryIndexVersion)
	binary.BigEndian.PutUint32(hdr[4:8], uint32(len(entries)))
	buf = append(buf, hdr[:]...)

	// 2. Entries
	for _, entry := range entries {
		var entryHdr [20]byte
		binary.BigEndian.PutUint64(entryHdr[0:8], uint64(entry.ModTimeNano))
		binary.BigEndian.PutUint64(entryHdr[8:16], uint64(entry.Size))

		// Mode: parse octal string into uint32
		modeVal, _ := strconv.ParseUint(string(entry.Mode), 8, 32)
		binary.BigEndian.PutUint32(entryHdr[16:20], uint32(modeVal))
		buf = append(buf, entryHdr[:]...)

		// 32-byte binary SHA-256
		hashBytes, err := hex.DecodeString(string(entry.Hash))
		if err != nil || len(hashBytes) != 32 {
			// Pad or copy up to 32 bytes
			hashBytes = make([]byte, 32)
			copy(hashBytes, []byte(entry.Hash))
		}
		buf = append(buf, hashBytes...)

		// Path: 2 bytes length prefix + raw UTF-8 bytes
		pathBytes := []byte(entry.Path)
		var lenBuf [2]byte
		binary.BigEndian.PutUint16(lenBuf[:], uint16(len(pathBytes)))
		buf = append(buf, lenBuf[:]...)
		buf = append(buf, pathBytes...)
	}

	// 3. Trailing Checksum: SHA-256 of everything up to this point
	checksum := sha256.Sum256(buf)
	buf = append(buf, checksum[:]...)

	return buf, nil
}

// DecodeBinary parses a binary index buffer with SHA-256 integrity verification.
func DecodeBinary(data []byte) (*Index, error) {
	if len(data) < 12+32 {
		return nil, fmt.Errorf("index data too small (%d bytes)", len(data))
	}

	// 1. Verify Magic
	magic := string(data[0:4])
	if magic != BinaryIndexMagic {
		return nil, fmt.Errorf("invalid index header magic: %q", magic)
	}

	// 2. Verify SHA-256 Checksum
	payload := data[:len(data)-32]
	expectedChecksum := data[len(data)-32:]
	actualChecksum := sha256.Sum256(payload)

	for i := 0; i < 32; i++ {
		if actualChecksum[i] != expectedChecksum[i] {
			return nil, ErrCorruptIndexChecksum
		}
	}

	// 3. Verify Version
	version := binary.BigEndian.Uint32(data[4:8])
	if version != BinaryIndexVersion {
		return nil, fmt.Errorf("%w: %d (expected %d)", ErrUnsupportedIndexVersion, version, BinaryIndexVersion)
	}

	entryCount := int(binary.BigEndian.Uint32(data[8:12]))
	idx := New()

	offset := 12
	payloadLen := len(payload)

	for i := 0; i < entryCount; i++ {
		// Minimum entry size: 20 bytes (mtime, size, mode) + 32 bytes (hash) + 2 bytes (pathLen) = 54 bytes
		if offset+54 > payloadLen {
			return nil, fmt.Errorf("unexpected EOF reading entry %d", i)
		}

		modTimeNano := int64(binary.BigEndian.Uint64(data[offset : offset+8]))
		size := int64(binary.BigEndian.Uint64(data[offset+8 : offset+16]))
		modeVal := binary.BigEndian.Uint32(data[offset+16 : offset+20])
		offset += 20

		hashBytes := data[offset : offset+32]
		offset += 32
		hashHex := hex.EncodeToString(hashBytes)

		pathLen := int(binary.BigEndian.Uint16(data[offset : offset+2]))
		offset += 2

		if offset+pathLen > payloadLen {
			return nil, fmt.Errorf("unexpected EOF reading path for entry %d", i)
		}

		pathStr := string(data[offset : offset+pathLen])
		offset += pathLen

		modeStr := object.FileMode(fmt.Sprintf("%06o", modeVal))
		if modeStr == "000000" {
			modeStr = object.ModeRegular
		}

		idx.Add(Entry{
			Path:        pathStr,
			Hash:        object.Hash(hashHex),
			Mode:        modeStr,
			Size:        size,
			ModTimeNano: modTimeNano,
		})
	}

	return idx, nil
}
