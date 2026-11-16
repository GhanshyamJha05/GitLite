package webui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlite/internal/diff"
	"gitlite/internal/object"
	"gitlite/internal/refs"
	"gitlite/internal/remote"
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
	Commits       []CommitNode              `json:"commits"`
	Branches      []refs.BranchInfo         `json:"branches"`
	Remotes       map[string]RemoteResponse `json:"remotes"`
	CurrentBranch string                    `json:"current_branch"`
	HeadCommit    string                    `json:"head_commit"`
	WorkTree      string                    `json:"work_tree"`
}

// RemoteResponse is the safe remote info sent to the frontend.
type RemoteResponse struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	HasToken bool   `json:"has_token"`
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

	cfg, _ := s.Repo.LoadConfig()
	remotesMap := make(map[string]RemoteResponse)
	if cfg != nil {
		for name, r := range cfg.Remotes {
			remotesMap[name] = RemoteResponse{
				Name:     r.Name,
				URL:      r.URL,
				HasToken: r.Token != "",
			}
		}
	}

	return &GraphResponse{
		Commits:       commitNodes,
		Branches:      branchList,
		Remotes:       remotesMap,
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
				Patch:   diff.UnifiedDiff(path, path, string(oldContent), ""),
			})
		} else if !inParent && inCurr {
			// Added
			newContent, _ := s.readBlobContent(cEntry.Hash)
			diffs = append(diffs, FileDiff{
				Path:    path,
				Status:  "added",
				NewMode: string(cEntry.Mode),
				Patch:   diff.UnifiedDiff(path, path, "", string(newContent)),
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
				Patch:   diff.UnifiedDiff(path, path, string(oldContent), string(newContent)),
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

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// SetupHandler configures HTTP routes, API endpoints, and security middleware.
func (s *Server) SetupHandler() http.Handler {
	mux := http.NewServeMux()

	// API: Visualizer Graph
	mux.HandleFunc("/api/graph", func(w http.ResponseWriter, r *http.Request) {
		graph, err := s.BuildGraph()
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, graph)
	})

	// API: Commit details (GET) or New Commit (POST)
	mux.HandleFunc("/api/commit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var req struct {
				Message string `json:"message"`
				Amend   bool   `json:"amend"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSONError(w, "invalid json payload", http.StatusBadRequest)
				return
			}
			if strings.TrimSpace(req.Message) == "" {
				writeJSONError(w, "commit message cannot be empty", http.StatusBadRequest)
				return
			}

			sig := repository.DefaultSignature()
			if cfg, err := s.Repo.LoadConfig(); err == nil && cfg != nil {
				if cfg.User.Name != "" {
					sig.Name = cfg.User.Name
				}
				if cfg.User.Email != "" {
					sig.Email = cfg.User.Email
				}
			}

			var commitHash object.Hash
			var err error
			if req.Amend {
				commitHash, err = s.Repo.CommitAmend(req.Message, sig, sig)
			} else {
				commitHash, err = s.Repo.Commit(req.Message, sig, sig)
			}
			if err != nil {
				writeJSONError(w, err.Error(), http.StatusInternalServerError)
				return
			}

			writeJSON(w, http.StatusOK, map[string]interface{}{
				"success": true,
				"hash":    string(commitHash),
			})
			return
		}

		// GET: Commit details and diff
		hash := r.URL.Query().Get("hash")
		if hash == "" {
			writeJSONError(w, "missing hash parameter", http.StatusBadRequest)
			return
		}
		detail, err := s.GetCommitDetail(hash)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, detail)
	})

	// API: Status
	mux.HandleFunc("/api/status", func(w http.ResponseWriter, r *http.Request) {
		status, err := s.Repo.Status()
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, status)
	})

	// API: Stage files (POST)
	mux.HandleFunc("/api/stage", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Path string `json:"path"`
			All  bool   `json:"all"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeJSONError(w, "invalid json payload", http.StatusBadRequest)
			return
		}

		targetPath := req.Path
		if req.All || targetPath == "" || targetPath == "." {
			targetPath = "."
		}

		if err := s.Repo.Add(targetPath); err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	})

	// API: Checkout branch
	mux.HandleFunc("/api/checkout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Target string `json:"target"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "invalid json payload", http.StatusBadRequest)
			return
		}

		if err := s.Repo.Checkout(req.Target); err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	})

	// API: Branch list and management
	mux.HandleFunc("/api/branch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			branches, err := s.Repo.Refs().ListBranches()
			if err != nil {
				writeJSONError(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, http.StatusOK, branches)
			return
		}

		if r.Method == http.MethodPost {
			var req struct {
				Name       string `json:"name"`
				Delete     bool   `json:"delete"`
				StartPoint string `json:"start_point"`
			}
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeJSONError(w, "invalid json payload", http.StatusBadRequest)
				return
			}
			if req.Name == "" {
				writeJSONError(w, "branch name is required", http.StatusBadRequest)
				return
			}

			if req.Delete {
				if err := s.Repo.Refs().DeleteBranch(req.Name); err != nil {
					writeJSONError(w, err.Error(), http.StatusInternalServerError)
					return
				}
				writeJSON(w, http.StatusOK, map[string]bool{"success": true})
				return
			}

			// Create branch
			startPoint := req.StartPoint
			if startPoint == "" {
				startPoint = "HEAD"
			}
			targetHash, err := s.Repo.ResolveRevision(startPoint)
			if err != nil {
				writeJSONError(w, fmt.Sprintf("invalid start point: %v", err), http.StatusBadRequest)
				return
			}

			if err := s.Repo.Refs().CreateBranch(req.Name, targetHash); err != nil {
				writeJSONError(w, err.Error(), http.StatusInternalServerError)
				return
			}

			writeJSON(w, http.StatusOK, map[string]bool{"success": true})
			return
		}

		writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
	})

	// API: Arbitrary Diff (GET)
	mux.HandleFunc("/api/diff", func(w http.ResponseWriter, r *http.Request) {
		from := r.URL.Query().Get("from")
		to := r.URL.Query().Get("to")
		staged := r.URL.Query().Get("staged") == "true"

		var patch string
		var err error

		if from != "" && to != "" {
			hashA, errA := s.Repo.ResolveRevision(from)
			if errA != nil {
				writeJSONError(w, fmt.Sprintf("invalid revision 'from': %v", errA), http.StatusBadRequest)
				return
			}
			hashB, errB := s.Repo.ResolveRevision(to)
			if errB != nil {
				writeJSONError(w, fmt.Sprintf("invalid revision 'to': %v", errB), http.StatusBadRequest)
				return
			}
			patch, err = s.Repo.DiffCommits(hashA, hashB)
		} else if staged {
			patch, err = s.Repo.DiffIndexAgainstHead()
		} else {
			patch, err = s.Repo.DiffWorkingTreeAgainstIndex()
		}

		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ok",
			"diff":   patch,
		})
	})

	// API: Configure Remote
	mux.HandleFunc("/api/remote/add", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name  string `json:"name"`
			URL   string `json:"url"`
			Token string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "invalid json payload", http.StatusBadRequest)
			return
		}
		if req.Name == "" {
			req.Name = "origin"
		}
		if err := s.Repo.AddRemote(req.Name, req.URL, req.Token); err != nil {
			writeJSONError(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
	})

	// API: Push to GitHub
	mux.HandleFunc("/api/push", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSONError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Remote string `json:"remote"`
			Branch string `json:"branch"`
			Token  string `json:"token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
			writeJSONError(w, "invalid json payload", http.StatusBadRequest)
			return
		}
		if req.Remote == "" {
			req.Remote = "origin"
		}

		res, err := remote.Push(s.Repo, req.Remote, req.Branch, req.Token)
		if err != nil {
			writeJSONError(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, res)
	})

	// Serve Frontend
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(indexHTML)
	})

	// Wrap in security middleware: Basic Auth, Security Headers, CSRF protection
	return s.wrapSecurityMiddleware(mux)
}

// wrapSecurityMiddleware applies Basic Auth, security headers, and CSRF protection.
func (s *Server) wrapSecurityMiddleware(next http.Handler) http.Handler {
	authUser := os.Getenv("GITLITE_AUTH_USER")
	authPass := os.Getenv("GITLITE_AUTH_PASS")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Basic Auth check if configured
		if authUser != "" && authPass != "" {
			u, p, ok := r.BasicAuth()
			if !ok || u != authUser || p != authPass {
				w.Header().Set("WWW-Authenticate", `Basic realm="GitLite Visualizer"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
		}

		// 2. Security Headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "same-origin")

		// 3. CSRF Protection for state-changing requests
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			origin := r.Header.Get("Origin")
			if origin != "" {
				u, err := url.Parse(origin)
				if err != nil || (!strings.HasPrefix(u.Host, "127.0.0.1") && !strings.HasPrefix(u.Host, "localhost") && u.Host != r.Host) {
					http.Error(w, "Cross-site request blocked by CSRF protection", http.StatusForbidden)
					return
				}
			}
		}

		next.ServeHTTP(w, r)
	})
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
