package merge

import (
	"gitlite/internal/object"
)

// CommitReader is an interface for reading commits from an object store.
type CommitReader interface {
	ReadCommit(h object.Hash) (*object.Commit, error)
}

// FindCommonAncestor discovers the lowest common ancestor (LCA) between two commits in the DAG.
// Uses breadth-first search and ancestor set tracking.
func FindCommonAncestor(reader CommitReader, commitA, commitB object.Hash) (object.Hash, error) {
	if commitA == commitB {
		return commitA, nil
	}

	// 1. Gather all ancestors of commitA
	ancestorsA := make(map[object.Hash]bool)
	queueA := []object.Hash{commitA}

	for len(queueA) > 0 {
		curr := queueA[0]
		queueA = queueA[1:]

		if ancestorsA[curr] {
			continue
		}
		ancestorsA[curr] = true

		c, err := reader.ReadCommit(curr)
		if err != nil {
			return "", err
		}
		for _, p := range c.Parents {
			if !ancestorsA[p] {
				queueA = append(queueA, p)
			}
		}
	}

	// 2. BFS from commitB until we intersect ancestorsA
	queueB := []object.Hash{commitB}
	visitedB := make(map[object.Hash]bool)

	for len(queueB) > 0 {
		curr := queueB[0]
		queueB = queueB[1:]

		if ancestorsA[curr] {
			return curr, nil
		}

		if visitedB[curr] {
			continue
		}
		visitedB[curr] = true

		c, err := reader.ReadCommit(curr)
		if err != nil {
			return "", err
		}
		for _, p := range c.Parents {
			if !visitedB[p] {
				queueB = append(queueB, p)
			}
		}
	}

	return "", nil
}
