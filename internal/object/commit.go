package object

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Signature represents an author or committer identity.
type Signature struct {
	Name  string
	Email string
	When  time.Time
}

// String formats a Signature in canonical format:
// "Name <email> 1700000000 +0000"
func (s Signature) String() string {
	timestamp := s.When.Unix()
	_, offset := s.When.Zone()
	sign := "+"
	if offset < 0 {
		sign = "-"
		offset = -offset
	}
	hours := offset / 3600
	mins := (offset % 3600) / 60
	zoneStr := fmt.Sprintf("%s%02d%02d", sign, hours, mins)
	return fmt.Sprintf("%s <%s> %d %s", s.Name, s.Email, timestamp, zoneStr)
}

// ParseSignature parses a signature line like "John Doe <john@example.com> 1700000000 +0530".
func ParseSignature(line string) (Signature, error) {
	lt := strings.IndexByte(line, '<')
	gt := strings.IndexByte(line, '>')
	if lt == -1 || gt == -1 || lt >= gt {
		return Signature{}, fmt.Errorf("%w: malformed signature %q", ErrCorruptObject, line)
	}

	name := strings.TrimSpace(line[:lt])
	email := strings.TrimSpace(line[lt+1 : gt])
	rest := strings.TrimSpace(line[gt+1:])

	parts := strings.Fields(rest)
	if len(parts) < 2 {
		return Signature{}, fmt.Errorf("%w: missing timestamp/zone in signature %q", ErrCorruptObject, line)
	}

	unixSec, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return Signature{}, fmt.Errorf("%w: invalid timestamp in signature: %v", ErrCorruptObject, err)
	}

	zoneStr := parts[1]
	if len(zoneStr) != 5 {
		return Signature{}, fmt.Errorf("%w: invalid timezone offset in signature: %q", ErrCorruptObject, zoneStr)
	}

	tzHours, _ := strconv.Atoi(zoneStr[1:3])
	tzMins, _ := strconv.Atoi(zoneStr[3:5])
	totalOffsetSec := (tzHours*60 + tzMins) * 60
	if zoneStr[0] == '-' {
		totalOffsetSec = -totalOffsetSec
	}

	loc := time.FixedZone("", totalOffsetSec)
	when := time.Unix(unixSec, 0).In(loc)

	return Signature{
		Name:  name,
		Email: email,
		When:  when,
	}, nil
}

// Commit represents an immutable snapshot object in GitLite.
type Commit struct {
	TreeHash  Hash
	Parents   []Hash
	Author    Signature
	Committer Signature
	Message   string
}

// CommitWithHash associates a parsed Commit with its cryptographic identifier.
type CommitWithHash struct {
	Hash   Hash
	Commit *Commit
}

// NewCommit creates a new Commit.
func NewCommit(treeHash Hash, parents []Hash, author, committer Signature, message string) *Commit {
	return &Commit{
		TreeHash:  treeHash,
		Parents:   parents,
		Author:    author,
		Committer: committer,
		Message:   message,
	}
}

// Type returns TypeCommit.
func (c *Commit) Type() Type {
	return TypeCommit
}

// SerializeContent formats the commit payload canonically:
// tree <tree_hash>\n
// [parent <parent_hash>\n...]
// author <author_signature>\n
// committer <committer_signature>\n
// \n
// <commit message>\n
func (c *Commit) SerializeContent() []byte {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("tree %s\n", c.TreeHash))
	for _, p := range c.Parents {
		buf.WriteString(fmt.Sprintf("parent %s\n", p))
	}
	buf.WriteString(fmt.Sprintf("author %s\n", c.Author.String()))
	buf.WriteString(fmt.Sprintf("committer %s\n", c.Committer.String()))
	buf.WriteString("\n")
	buf.WriteString(strings.TrimSpace(c.Message))
	buf.WriteString("\n")

	return buf.Bytes()
}

// ParseCommit parses the uncompressed content of a Commit object.
func ParseCommit(data []byte) (*Commit, error) {
	lines := strings.Split(string(data), "\n")
	c := &Commit{}
	var msgLines []string
	readingMessage := false

	for _, line := range lines {
		if readingMessage {
			msgLines = append(msgLines, line)
			continue
		}

		if line == "" {
			readingMessage = true
			continue
		}

		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		field := parts[0]
		value := parts[1]

		switch field {
		case "tree":
			c.TreeHash = Hash(value)
			if err := c.TreeHash.Validate(); err != nil {
				return nil, fmt.Errorf("%w: invalid tree hash in commit: %v", ErrCorruptObject, err)
			}
		case "parent":
			pHash := Hash(value)
			if err := pHash.Validate(); err != nil {
				return nil, fmt.Errorf("%w: invalid parent hash in commit: %v", ErrCorruptObject, err)
			}
			c.Parents = append(c.Parents, pHash)
		case "author":
			author, err := ParseSignature(value)
			if err != nil {
				return nil, err
			}
			c.Author = author
		case "committer":
			committer, err := ParseSignature(value)
			if err != nil {
				return nil, err
			}
			c.Committer = committer
		}
	}

	c.Message = strings.TrimSpace(strings.Join(msgLines, "\n"))
	return c, nil
}

// ReadCommit retrieves, verifies, and decodes a Commit object from the store.
func (s *Store) ReadCommit(h Hash) (*Commit, error) {
	objType, content, err := s.Read(h)
	if err != nil {
		return nil, err
	}
	if objType != TypeCommit {
		return nil, fmt.Errorf("expected object %s to be commit, got %s", h, objType)
	}
	return ParseCommit(content)
}
