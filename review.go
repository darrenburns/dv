package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/gofrs/flock"
)

// Snapshots and anchors are immutable. Line coordinates always name a side of
// an exact snapshot, never a rendered row or the current working file.
type ReviewSnapshot struct {
	ID    string   `json:"id"`
	Diff  string   `json:"diff"`
	Base  string   `json:"base"`
	Head  string   `json:"head"`
	Files []string `json:"files,omitempty"`
}
type ReviewAnchor struct {
	ID       string `json:"id"`
	Snapshot string `json:"snapshot"`
	Path     string `json:"path"`
	Side     string `json:"side"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Quote    string `json:"quote"`
}
type ReviewMessage struct {
	Author string `json:"author"`
	Body   string `json:"body"`
}
type ReviewThread struct {
	ID        string          `json:"id"`
	Anchor    ReviewAnchor    `json:"anchor"`
	Kind      string          `json:"kind"`
	Status    string          `json:"status"`
	Submitted bool            `json:"submitted"`
	Messages  []ReviewMessage `json:"messages"`
	Revision  int             `json:"revision"`
}
type Review struct {
	Version   int                       `json:"version"`
	ID        string                    `json:"id"`
	Repo      string                    `json:"repo"`
	Title     string                    `json:"title"`
	Source    string                    `json:"source"`
	Current   string                    `json:"current"`
	Snapshots map[string]ReviewSnapshot `json:"snapshots"`
	Threads   []ReviewThread            `json:"threads"`
	Revision  int                       `json:"revision"`
}
type ReviewStore struct{ Dir string }

func defaultReviewStore() ReviewStore {
	if dir := os.Getenv("ASIDE_DATA_DIR"); dir != "" {
		return ReviewStore{Dir: dir}
	}
	return ReviewStore{Dir: filepath.Join(xdg.DataHome, "aside", "reviews")}
}
func digest(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:])[:24] }

var reviewIDPattern = regexp.MustCompile(`^[a-f0-9]{24}$`)

func (s ReviewStore) path(id string) (string, error) {
	if !reviewIDPattern.MatchString(id) {
		return "", errors.New("invalid review ID")
	}
	return filepath.Join(s.Dir, id+".json"), nil
}
func (s ReviewStore) Read(id string) (*Review, error) {
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Review
	if err = json.Unmarshal(data, &r); err != nil {
		return nil, fmt.Errorf("read review: %w", err)
	}
	if r.Version != 1 || r.ID != id || r.Snapshots == nil {
		return nil, errors.New("invalid or unsupported review data")
	}
	if _, ok := r.Snapshots[r.Current]; !ok {
		return nil, errors.New("review snapshot is missing")
	}
	for key, snap := range r.Snapshots {
		if !reviewIDPattern.MatchString(key) || snap.ID != key {
			return nil, errors.New("invalid snapshot data")
		}
	}
	seen := map[string]bool{}
	for _, thread := range r.Threads {
		a := thread.Anchor
		_, snapshotExists := r.Snapshots[a.Snapshot]
		if !reviewIDPattern.MatchString(thread.ID) || seen[thread.ID] || !snapshotExists || !reviewIDPattern.MatchString(a.ID) || a.Path == "" || (a.Side != "old" && a.Side != "new") || a.Start < 1 || a.End < a.Start || len(thread.Messages) == 0 {
			return nil, errors.New("invalid review thread data")
		}
		seen[thread.ID] = true
	}
	return &r, nil
}

// Update serializes writers across processes and atomically publishes a complete
// file. A failed validation leaves the previous review untouched.
func (s ReviewStore) Update(id string, create *Review, update func(*Review) error) (*Review, error) {
	path, err := s.path(id)
	if err != nil {
		return nil, err
	}
	if err = os.MkdirAll(s.Dir, 0700); err != nil {
		return nil, err
	}
	lock := flock.New(path + ".lock")
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ok, err := lock.TryLockContext(ctx, 10*time.Millisecond)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New("review is busy; retry")
	}
	defer lock.Unlock()
	r, err := s.Read(id)
	if errors.Is(err, os.ErrNotExist) && create != nil {
		r = create
		err = nil
	}
	if err != nil {
		return nil, err
	}
	if err = update(r); err != nil {
		return nil, err
	}
	r.Revision++
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	f, err := os.CreateTemp(s.Dir, ".aside-*")
	if err != nil {
		return nil, err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return nil, err
	}
	return r, nil
}
func (s ReviewStore) Open(repo, title, source string, snapshot ReviewSnapshot) (*Review, error) {
	if _, err := snapshotDocument(snapshot); err != nil {
		return nil, err
	}
	id := digest(repo + "\x00" + source)
	snapshot.ID = digest(snapshot.Base + "\x00" + snapshot.Head + "\x00" + snapshot.Diff)
	return s.Update(id, &Review{Version: 1, ID: id, Repo: repo, Title: title, Source: source, Snapshots: map[string]ReviewSnapshot{}, Threads: []ReviewThread{}}, func(r *Review) error {
		r.Title = title
		r.Current = snapshot.ID
		r.Snapshots[snapshot.ID] = snapshot
		return nil
	})
}
func (s ReviewStore) List(repo string) ([]*Review, error) {
	entries, err := os.ReadDir(s.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return []*Review{}, nil
	}
	if err != nil {
		return nil, err
	}
	var reviews []*Review
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		r, err := s.Read(strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			return nil, err
		}
		if repo == "" || r.Repo == repo {
			reviews = append(reviews, r)
		}
	}
	sort.Slice(reviews, func(i, j int) bool { return reviews[i].Title < reviews[j].Title })
	return reviews, nil
}
func snapshotDocument(snapshot ReviewSnapshot) (*DiffDocument, error) {
	return parseUnifiedDiff(snapshot.Diff)
}
func anchorFor(snapshot ReviewSnapshot, path, side string, start, end int) (ReviewAnchor, error) {
	if (side != "old" && side != "new") || start < 1 || end < start || end-start > 500 {
		return ReviewAnchor{}, errors.New("invalid side or line range (maximum 501 lines)")
	}
	doc, err := snapshotDocument(snapshot)
	if err != nil {
		return ReviewAnchor{}, err
	}
	lines := map[int]string{}
	for _, file := range doc.Files {
		if file.DisplayPath != path {
			continue
		}
		if file.IsBinary {
			return ReviewAnchor{}, errors.New("binary files cannot have line comments")
		}
		for _, hunk := range file.Hunks {
			for _, line := range hunk.Lines {
				number := line.NewLine
				if side == "old" {
					number = line.OldLine
				}
				if number > 0 {
					lines[number] = line.Content
				}
			}
		}
	}
	var quote []string
	for n := start; n <= end; n++ {
		value, ok := lines[n]
		if !ok {
			return ReviewAnchor{}, fmt.Errorf("%s line %d is outside this diff snapshot", side, n)
		}
		quote = append(quote, value)
	}
	a := ReviewAnchor{Snapshot: snapshot.ID, Path: path, Side: side, Start: start, End: end, Quote: strings.Join(quote, "\n")}
	a.ID = digest(fmt.Sprintf("%s\x00%s\x00%s\x00%d\x00%d", a.Snapshot, path, side, start, end))
	return a, nil
}
func validMessage(author, body string) error {
	if strings.TrimSpace(author) == "" || len(author) > 100 || strings.ContainsAny(author, "\r\n\x1b") {
		return errors.New("author must be 1–100 characters on one line")
	}
	if strings.TrimSpace(body) == "" || len(body) > 16000 {
		return errors.New("comment must contain 1–16000 bytes")
	}
	return nil
}

type AnnotationInput struct {
	// Key makes an agent's retry idempotent within this review.
	Key      string `json:"key"`
	Snapshot string `json:"snapshot"`
	Path     string `json:"path"`
	Side     string `json:"side"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
	Kind     string `json:"kind"`
	Body     string `json:"body"`
}

func (s ReviewStore) Publish(id, author string, annotations []AnnotationInput, submitted bool) (*Review, error) {
	if len(annotations) == 0 || len(annotations) > 100 {
		return nil, errors.New("provide 1–100 annotations")
	}
	return s.Update(id, nil, func(r *Review) error {
		for _, input := range annotations {
			if err := validMessage(author, input.Body); err != nil {
				return err
			}
			if input.Key == "" || len(input.Key) > 200 {
				return errors.New("each annotation needs a stable key of at most 200 bytes")
			}
			if input.Kind != "feedback" && input.Kind != "finding" && input.Kind != "explanation" {
				return errors.New("kind must be feedback, finding, or explanation")
			}
			snap, ok := r.Snapshots[input.Snapshot]
			if !ok {
				return errors.New("unknown snapshot; read the review before commenting")
			}
			anchor, err := anchorFor(snap, input.Path, input.Side, input.Start, input.End)
			if err != nil {
				return err
			}
			thread := ReviewThread{ID: digest(author + "\x00" + input.Key), Anchor: anchor, Kind: input.Kind, Status: "open", Submitted: submitted, Messages: []ReviewMessage{{Author: author, Body: strings.TrimSpace(input.Body)}}, Revision: r.Revision + 1}
			exists := false
			for _, old := range r.Threads {
				if old.ID != thread.ID {
					continue
				}
				exists = true
				if old.Anchor != anchor || old.Kind != thread.Kind || old.Messages[0] != thread.Messages[0] {
					return errors.New("annotation key already used for different content")
				}
				break
			}
			if !exists {
				r.Threads = append(r.Threads, thread)
			}
		}
		return nil
	})
}
func (s ReviewStore) Reply(id, threadID, author, body string) (*Review, error) {
	if err := validMessage(author, body); err != nil {
		return nil, err
	}
	return s.Update(id, nil, func(r *Review) error {
		for i := range r.Threads {
			if r.Threads[i].ID == threadID {
				r.Threads[i].Messages = append(r.Threads[i].Messages, ReviewMessage{Author: author, Body: strings.TrimSpace(body)})
				r.Threads[i].Revision = r.Revision + 1
				return nil
			}
		}
		return errors.New("thread not found")
	})
}
func (s ReviewStore) SetStatus(id, threadID, status string) (*Review, error) {
	if status != "open" && status != "addressed" && status != "resolved" {
		return nil, errors.New("invalid thread status")
	}
	return s.Update(id, nil, func(r *Review) error {
		for i := range r.Threads {
			if r.Threads[i].ID == threadID {
				r.Threads[i].Status = status
				r.Threads[i].Revision = r.Revision + 1
				return nil
			}
		}
		return errors.New("thread not found")
	})
}
func (s ReviewStore) Submit(id string) (*Review, error) {
	return s.Update(id, nil, func(r *Review) error {
		for i := range r.Threads {
			if !r.Threads[i].Submitted {
				r.Threads[i].Submitted = true
				r.Threads[i].Revision = r.Revision + 1
			}
		}
		return nil
	})
}
func feedbackMarkdown(r *Review, after int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Aside feedback: %s\n\nReview: %s\nRepository: %s\nCurrent snapshot: %s\nRevision: %d\n", r.Title, r.ID, r.Repo, r.Current, r.Revision)
	for _, thread := range r.Threads {
		if !thread.Submitted || thread.Revision <= after || thread.Status == "resolved" {
			continue
		}
		a := thread.Anchor
		outdated := ""
		if a.Snapshot != r.Current {
			outdated = " — OUTDATED; inspect original snapshot"
		}
		fmt.Fprintf(&b, "\n## %s — %s lines %d–%d [%s]%s\n\nThread: %s\nSnapshot: %s\n\n", a.Path, a.Side, a.Start, a.End, thread.Status, outdated, thread.ID, a.Snapshot)
		for _, line := range strings.Split(a.Quote, "\n") {
			fmt.Fprintf(&b, "    %s\n", line)
		}
		for _, message := range thread.Messages {
			fmt.Fprintf(&b, "\n%s: %s\n", message.Author, message.Body)
		}
	}
	return b.String()
}
