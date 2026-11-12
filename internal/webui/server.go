package webui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"html"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"gitlite/internal/object"
	"gitlite/internal/refs"
	"gitlite/internal/repository"
)

//go:embed assets/index.html
var indexHTML []byte

// Server represents an embedded web server for GitLite visualizer.
type Server struct {
	Repo *repository.Repository
	Port int
}

// NewServer initializes a new Server instance.
func NewServer(repo *repository.Repository, port int) *Server {
	return &Server{
		Repo: repo,
		Port: port,
	}
}

// CommitNode represents a single commit in the visualizer graph.
type CommitNode struct {
	Hash        string   `json:"hash"`
	ShortHash   string   `json:"short_hash"`
	Parents     []string `json:"parents"`
	AuthorName  string   `json:"author_name"`
	AuthorEmail string   `json:"author_email"`
	AuthorDate  string   `json:"author_date"`
	Message     string   `json:"message"`
	Subject     string   `json:"subject"`
	Branches    []string `json:"branches"`
	IsHead      bool     `json:"is_head"`
}

// GraphResponse is the JSON payload returned by /api/graph.
type GraphResponse struct {
	Commits       []CommitNode      `json:"commits"`
	Branches      []refs.BranchInfo `json:"branches"`
	CurrentBranch string            `json:"current_branch"`
	HeadCommit    string            `json:"head_commit"`
	WorkTree      string            `json:"work_tree"`
}

// CommitDetailResponse represents commit metadata and changed file diffs.
type CommitDetailResponse struct {
	CommitNode
	Diffs []FileDiff `json:"diffs"`
}

// FileDiff represents a file patch in a commit.
type FileDiff struct {
	Path    string `json:"path"`
	OldPath string `json:"old_path,omitempty"`
	Status  string `json:"status"` // "added", "modified", "deleted"
	OldMode string `json:"old_mode,omitempty"`
	NewMode string `json:"new_mode,omitempty"`
	Patch   string `json:"patch"`
}

// BuildGraph collects all reachable commits across all branch refs and HEAD.
func (s *Server) BuildGraph() (*GraphResponse, error) {
	refMgr := s.Repo.Refs()
	branchList, err := refMgr.ListBranches()
	if err != nil {
		return nil, fmt.Errorf("listing branches: %w", err)
	}

	headCommit, currentBranch, _ := refMgr.ResolveHead()

	commitToBranches := make(map[string][]string)
	var startCommits []object.Hash

	for _, b := range branchList {
		if b.Commit != "" {
			bHash := string(b.Commit)
			commitToBranches[bHash] = append(commitToBranches[bHash], b.Name)
			startCommits = append(startCommits, b.Commit)
		}
	}

	if headCommit != "" {
		startCommits = append(startCommits, headCommit)
	}

	// BFS/queue traversal to collect all commits in DAG
	visited := make(map[object.Hash]bool)
	var queue []object.Hash
	for _, h := range startCommits {
		if !visited[h] {
			visited[h] = true
			queue = append(queue, h)
		}
	}

	var commitNodes []CommitNode
	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		cObj, err := s.Repo.Store.ReadCommit(curr)
		if err != nil {
			continue
		}

		hashStr := string(curr)
		shortHash := hashStr
		if len(shortHash) > 8 {
			shortHash = shortHash[:8]
		}

		var parents []string
		for _, p := range cObj.Parents {
			parents = append(parents, string(p))
			if !visited[p] {
				visited[p] = true
				queue = append(queue, p)
			}
		}

		msgLines := strings.Split(strings.TrimSpace(cObj.Message), "\n")
		subject := ""
		if len(msgLines) > 0 {
			subject = msgLines[0]
		}

		commitNodes = append(commitNodes, CommitNode{
			Hash:        hashStr,
			ShortHash:   shortHash,
			Parents:     parents,
			AuthorName:  cObj.Author.Name,
			AuthorEmail: cObj.Author.Email,
			AuthorDate:  cObj.Author.When.Format(time.RFC3339),
			Message:     cObj.Message,
			Subject:     subject,
			Branches:    commitToBranches[hashStr],
			IsHead:      hashStr == string(headCommit),
		})
	}

	return &GraphResponse{
		Commits:       commitNodes,
		Branches:      branchList,
		CurrentBranch: currentBranch,
		HeadCommit:    string(headCommit),
		WorkTree:      s.Repo.WorkTree,
	}, nil
}

// GetCommitDetail retrieves detailed diffs for a given commit hash.
func (s *Server) GetCommitDetail(commitHash string) (*CommitDetailResponse, error) {
	h := object.Hash(commitHash)
	if err := h.Validate(); err != nil {
		return nil, fmt.Errorf("invalid hash: %w", err)
	}

	cObj, err := s.Repo.Store.ReadCommit(h)
	if err != nil {
		return nil, fmt.Errorf("reading commit %s: %w", commitHash, err)
	}

	hashStr := string(h)
	shortHash := hashStr
	if len(shortHash) > 8 {
		shortHash = shortHash[:8]
	}

	var parents []string
	for _, p := range cObj.Parents {
		parents = append(parents, string(p))
	}

	msgLines := strings.Split(strings.TrimSpace(cObj.Message), "\n")
	subject := ""
	if len(msgLines) > 0 {
		subject = msgLines[0]
	}

	node := CommitNode{
		Hash:        hashStr,
		ShortHash:   shortHash,
		Parents:     parents,
		AuthorName:  cObj.Author.Name,
		AuthorEmail: cObj.Author.Email,
		AuthorDate:  cObj.Author.When.Format(time.RFC3339),
		Message:     cObj.Message,
		Subject:     subject,
	}

	// Compare with first parent tree if exists
	currEntries, err := s.readTreeEntries(cObj.TreeHash)
	if err != nil {
		return nil, fmt.Errorf("reading tree entries: %w", err)
	}

	parentEntries := make(map[string]object.TreeEntry)
	if len(cObj.Parents) > 0 {
		pCommit, err := s.Repo.Store.ReadCommit(cObj.Parents[0])
		if err == nil {
			pe, err := s.readTreeEntries(pCommit.TreeHash)
			if err == nil {
				parentEntries = pe
			}
		}
	}

	// Calculate file diffs
	allPaths := make(map[string]bool)
	for p := range parentEntries {
		allPaths[p] = true
	}
	for p := range currEntries {
		allPaths[p] = true
	}

	var diffs []FileDiff
	for path := range allPaths {
		pEntry, inParent := parentEntries[path]
		cEntry, inCurr := currEntries[path]

		if inParent && !inCurr {
			// Deleted
			oldContent, _ := s.readBlobContent(pEntry.Hash)
			diffs = append(diffs, FileDiff{
				Path:    path,
				Status:  "deleted",
				OldMode: string(pEntry.Mode),
				Patch:   formatSimpleDiff(path, oldContent, nil),
			})
		} else if !inParent && inCurr {
			// Added
			newContent, _ := s.readBlobContent(cEntry.Hash)
			diffs = append(diffs, FileDiff{
				Path:    path,
				Status:  "added",
				NewMode: string(cEntry.Mode),
				Patch:   formatSimpleDiff(path, nil, newContent),
			})
		} else if inParent && inCurr && pEntry.Hash != cEntry.Hash {
			// Modified
			oldContent, _ := s.readBlobContent(pEntry.Hash)
			newContent, _ := s.readBlobContent(cEntry.Hash)
			diffs = append(diffs, FileDiff{
				Path:    path,
				Status:  "modified",
				OldMode: string(pEntry.Mode),
				NewMode: string(cEntry.Mode),
				Patch:   formatSimpleDiff(path, oldContent, newContent),
			})
		}
	}

	return &CommitDetailResponse{
		CommitNode: node,
		Diffs:      diffs,
	}, nil
}

func (s *Server) readBlobContent(h object.Hash) ([]byte, error) {
	objType, content, err := s.Repo.Store.Read(h)
	if err != nil {
		return nil, err
	}
	if objType != object.TypeBlob {
		return nil, fmt.Errorf("object %s is not a blob", h)
	}
	return content, nil
}

func (s *Server) readTreeEntries(treeHash object.Hash) (map[string]object.TreeEntry, error) {
	entries := make(map[string]object.TreeEntry)
	var walk func(prefix string, th object.Hash) error

	walk = func(prefix string, th object.Hash) error {
		t, err := s.Repo.Store.ReadTree(th)
		if err != nil {
			return err
		}
		for _, e := range t.Entries {
			fullPath := filepath.ToSlash(filepath.Join(prefix, e.Name))
			if e.Mode == object.ModeDirectory {
				if err := walk(fullPath, e.Hash); err != nil {
					return err
				}
			} else {
				entries[fullPath] = e
			}
		}
		return nil
	}

	err := walk("", treeHash)
	return entries, err
}

func formatSimpleDiff(path string, oldData, newData []byte) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path))

	oldLines := splitLines(string(oldData))
	newLines := splitLines(string(newData))

	if oldData == nil {
		for _, l := range newLines {
			sb.WriteString("+" + l + "\n")
		}
		return sb.String()
	}
	if newData == nil {
		for _, l := range oldLines {
			sb.WriteString("-" + l + "\n")
		}
		return sb.String()
	}

	for _, l := range oldLines {
		sb.WriteString("-" + l + "\n")
	}
	for _, l := range newLines {
		sb.WriteString("+" + l + "\n")
	}
	return sb.String()
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	s = strings.ReplaceAll(s, "\r\n", "\n")
	lines := strings.Split(s, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// SetupHandler returns the HTTP multiplexer with all API endpoints and UI routes.
func (s *Server) SetupHandler() http.Handler {
	mux := http.NewServeMux()

	// API: Graph data
	mux.HandleFunc("/api/graph", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		graph, err := s.BuildGraph()
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": %q}`, html.EscapeString(err.Error())), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(graph)
	})

	// API: Commit details and diff
	mux.HandleFunc("/api/commit", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		hash := r.URL.Query().Get("hash")
		if hash == "" {
			http.Error(w, `{"error": "missing hash parameter"}`, http.StatusBadRequest)
			return
		}
		detail, err := s.GetCommitDetail(hash)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": %q}`, html.EscapeString(err.Error())), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(detail)
	})

	// API: Status
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		status, err := s.Repo.Status()
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error": %q}`, html.EscapeString(err.Error())), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(status)
	})

	// API: Checkout branch
	mux.HandleFunc("/api/checkout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, `{"error": "method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Target string `json:"target"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error": "invalid json payload"}`, http.StatusBadRequest)
			return
		}

		if err := s.Repo.Checkout(req.Target); err != nil {
			http.Error(w, fmt.Sprintf(`{"error": %q}`, html.EscapeString(err.Error())), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success": true}`))
	})

	// Serve Frontend
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})

	return mux
}

// Start launches the web server and listens on the configured port.
func (s *Server) Start(readyChan chan<- string) error {
	addr := fmt.Sprintf("127.0.0.1:%d", s.Port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		// Fallback to random free port if requested port is busy
		listener, err = net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("listening on network address: %w", err)
		}
	}

	actualAddr := listener.Addr().String()
	if readyChan != nil {
		readyChan <- fmt.Sprintf("http://%s", actualAddr)
	}

	server := &http.Server{
		Handler:      s.SetupHandler(),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	return server.Serve(listener)
}
