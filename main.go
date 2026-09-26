package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/adrg/xdg"
	t "github.com/darrenburns/terma"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "aside:", err)
		os.Exit(1)
	}
}
func run(args []string, in io.Reader, out, errOut io.Writer) error {
	store := defaultReviewStore()
	if len(args) > 0 && args[0] == "review" {
		return runReviewCLI(store, args[1:], in, out, errOut)
	}
	if len(args) > 0 && args[0] == "mcp" {
		flags := flag.NewFlagSet("aside mcp", flag.ContinueOnError)
		flags.SetOutput(errOut)
		repo := flags.String("repo", ".", "repository this server may review")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		root, err := repoRoot(*repo)
		if err != nil {
			return err
		}
		return serveMCP(context.Background(), store, root)
	}
	flags := flag.NewFlagSet("aside", flag.ContinueOnError)
	flags.SetOutput(errOut)
	repo := flags.String("repo", ".", "repository to browse (read only)")
	reviewID := flags.String("review", "", "open a saved review ID")
	base := flags.String("base", "", "compare the merge-base of this ref with --head")
	head := flags.String("head", "HEAD", "head ref for --base")
	pr := flags.String("pr", "", "load a PR number or URL using gh (no checkout)")
	view := flags.String("view", "unified", "unified or split")
	intraline := flags.String("intraline-style", "background", "background, underline, or off")
	theme := flags.String("theme", t.ThemeNameObsidianTide, "theme name")
	config := flags.String("config", "", "configuration file")
	noConfig := flags.Bool("no-config", false, "ignore configuration")
	all := flags.Bool("all-files", false, "show all tracked and unignored files")
	showVersion := flags.Bool("version", false, "print version")
	flags.Usage = func() {
		fmt.Fprintln(errOut, "Aside — a read-only diff browser with inline review feedback.\n\nUsage: aside [options]\n       aside review <open|list|read|publish|feedback|reply|status> [options]\n       aside mcp [--repo path]\n\nKeys: c comment on selection, f submit/copy feedback, [/] comments, e reply,\n      x resolve, v layout, / filter, a all/changed files, r refresh, q quit.")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %v", flags.Args())
	}
	if *showVersion {
		fmt.Fprintf(out, "Aside %s\n", version)
		return nil
	}
	cfg, err := loadStartupConfig(xdg.ConfigHome, *config, *noConfig)
	if err != nil {
		return err
	}
	explicit := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	values := applyStartupConfig(startupFlagValues{ViewMode: *view, ThemeName: *theme, IntralineStyle: *intraline}, cfg, explicit)
	initial, err := startupInitialStateFromFlags(values.ViewMode, values.ThemeName, values.IntralineStyle)
	if err != nil {
		return err
	}
	var review *Review
	if *reviewID != "" {
		review, err = store.Read(*reviewID)
	} else {
		review, err = openReview(store, *repo, *base, *head, *pr)
	}
	if err != nil {
		return err
	}
	t.SetTheme(initial.ThemeName)
	app := NewAside(store, review, initial)
	if *all {
		app.allFiles.Set(true)
		app.rebuildFiles()
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	app.Watch(ctx)
	go func() { <-ctx.Done(); t.Quit() }()
	return t.Run(app)
}

type ReviewRequest struct {
	ID          string            `json:"id,omitempty"`
	Base        string            `json:"base,omitempty"`
	Head        string            `json:"head,omitempty"`
	PR          string            `json:"pr,omitempty"`
	Author      string            `json:"author,omitempty"`
	Annotations []AnnotationInput `json:"annotations,omitempty"`
	Thread      string            `json:"thread,omitempty"`
	Body        string            `json:"body,omitempty"`
	Status      string            `json:"status,omitempty"`
	Since       int               `json:"since,omitempty"`
}

func reviewOperation(store ReviewStore, repo, operation string, input ReviewRequest) (result any, err error) {
	defer func() {
		switch value := result.(type) {
		case *Review:
			result = submittedReview(value)
		case []*Review:
			for i, r := range value {
				value[i] = submittedReview(r)
			}
		}
	}()
	if operation == "open" {
		return openReview(store, repo, input.Base, input.Head, input.PR)
	}
	if operation == "list" {
		return store.List(repo)
	}
	r, err := store.Read(input.ID)
	if err != nil {
		return nil, err
	}
	if r.Repo != repo {
		return nil, fmt.Errorf("review belongs to a different repository")
	}
	if operation == "reply" || operation == "status" {
		found := false
		for _, thread := range r.Threads {
			if thread.ID == input.Thread && thread.Submitted {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("submitted thread not found")
		}
	}
	switch operation {
	case "read":
		return r, nil
	case "publish":
		if input.Author == "" {
			input.Author = "agent"
		}
		return store.Publish(input.ID, input.Author, input.Annotations, true)
	case "reply":
		if input.Author == "" {
			input.Author = "agent"
		}
		return store.Reply(input.ID, input.Thread, input.Author, input.Body)
	case "status":
		return store.SetStatus(input.ID, input.Thread, input.Status)
	case "feedback":
		if input.Since < 0 {
			return nil, fmt.Errorf("since must be nonnegative")
		}
		threads := []ReviewThread{}
		for _, thread := range r.Threads {
			if thread.Submitted && thread.Revision > input.Since && thread.Status != "resolved" {
				threads = append(threads, thread)
			}
		}
		return struct {
			ID       string         `json:"id"`
			Revision int            `json:"revision"`
			Threads  []ReviewThread `json:"threads"`
			Markdown string         `json:"markdown"`
		}{r.ID, r.Revision, threads, feedbackMarkdown(r, input.Since)}, nil
	default:
		return nil, fmt.Errorf("unknown review operation %q", operation)
	}
}
func runReviewCLI(store ReviewStore, args []string, in io.Reader, out, errOut io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: aside review <open|list|read|publish|feedback|reply|status> [options]")
	}
	op := args[0]
	flags := flag.NewFlagSet("aside review "+op, flag.ContinueOnError)
	flags.SetOutput(errOut)
	var req ReviewRequest
	flags.StringVar(&req.ID, "id", "", "review ID")
	flags.StringVar(&req.Base, "base", "", "base ref")
	flags.StringVar(&req.Head, "head", "HEAD", "head ref")
	flags.StringVar(&req.PR, "pr", "", "PR number or URL")
	flags.StringVar(&req.Author, "author", "agent", "comment author")
	flags.StringVar(&req.Thread, "thread", "", "thread ID")
	flags.StringVar(&req.Body, "body", "", "reply body")
	flags.StringVar(&req.Status, "status", "addressed", "open, addressed, or resolved")
	flags.IntVar(&req.Since, "since", 0, "feedback revision cursor")
	repo := flags.String("repo", ".", "repository")
	input := flags.String("input", "-", "JSON request file for publish (- reads stdin)")
	format := flags.String("format", "json", "json or markdown (feedback)")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments")
	}
	if op == "publish" {
		reader := in
		if *input != "-" {
			f, err := os.Open(*input)
			if err != nil {
				return err
			}
			defer f.Close()
			reader = f
		}
		decoder := json.NewDecoder(io.LimitReader(reader, 2*1024*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&req); err != nil {
			return err
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			return fmt.Errorf("expected one JSON request")
		}
	}
	root, err := repoRoot(*repo)
	if err != nil {
		return err
	}
	if req.Since < 0 {
		return fmt.Errorf("since must be nonnegative")
	}
	if *format == "markdown" {
		if op != "feedback" {
			return fmt.Errorf("markdown is only available for feedback")
		}
		r, err := store.Read(req.ID)
		if err != nil {
			return err
		}
		if r.Repo != root {
			return fmt.Errorf("review belongs to a different repository")
		}
		_, err = fmt.Fprint(out, feedbackMarkdown(r, req.Since))
		return err
	}
	if *format != "json" {
		return fmt.Errorf("format must be json or markdown")
	}
	result, err := reviewOperation(store, root, op, req)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// Agent-facing results expose only feedback the user has submitted.
func submittedReview(r *Review) *Review {
	if r == nil {
		return nil
	}
	copy := *r
	copy.Threads = []ReviewThread{}
	for _, thread := range r.Threads {
		if thread.Submitted {
			copy.Threads = append(copy.Threads, thread)
		}
	}
	return &copy
}
