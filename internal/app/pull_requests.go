package app

import (
	"context"
	"errors"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/ourongxing/vl/internal/config"
	"github.com/ourongxing/vl/internal/diff"
	gitservice "github.com/ourongxing/vl/internal/git"
)

type pullRequestsMsg struct {
	seq   uint64
	after string
	list  gitservice.PullRequestList
	err   error
}

type cachedPullRequestsMsg struct {
	seq     uint64
	list    gitservice.PullRequestList
	refresh tea.Cmd
}

const pullRequestRefreshInterval = time.Minute
const pullRequestCacheSize = 32

func (m *Model) canRefreshPullRequestList() bool {
	return !m.editorOnly && !m.snapshot.NoRepository && (m.section == pullRequests || m.snapshot.GitHubURL != "")
}

func (m *Model) pullRequestTickDelay(delay time.Duration) time.Duration {
	if !m.canRefreshPullRequestList() {
		return delay
	}
	if m.pullRequestsCancel == nil {
		delay = min(delay, time.Until(m.pullRequestsFetchedAt.Add(pullRequestRefreshInterval)))
	}
	return max(0, delay)
}

func (m *Model) refreshPullRequests(force bool) tea.Cmd {
	if !m.canRefreshPullRequestList() || !force && (m.pullRequestsCancel != nil || time.Since(m.pullRequestsFetchedAt) < pullRequestRefreshInterval) {
		return nil
	}
	if m.pullRequestsCancel != nil {
		m.pullRequestsCancel()
	}
	m.pullRequestsFetchedAt = time.Now()
	m.pullRequestsError = nil
	return m.requestPullRequests("")
}

func (m *Model) loadMorePullRequests() tea.Cmd {
	if !m.canRefreshPullRequestList() || m.pullRequestsCancel != nil || m.pullRequestsNextCursor == "" {
		return nil
	}
	m.pullRequestsPageError = nil
	return m.requestPullRequests(m.pullRequestsNextCursor)
}

func (m *Model) loadVisiblePullRequests() tea.Cmd {
	if m.section != pullRequests || m.modal != "" || m.contextMenu != nil || !m.sidebarVisible ||
		!m.sideShown() && !m.sidebar || m.pullRequestsPageError != nil {
		return nil
	}
	if len(m.items) > 0 && m.items[len(m.items)-1].key == pullRequestMoreKey &&
		m.sideTop+m.treeList().height >= len(m.items) {
		return m.loadMorePullRequests()
	}
	return nil
}

func (m *Model) requestPullRequests(after string) tea.Cmd {
	ctx, cancel := context.WithCancel(m.ctx)
	m.pullRequestsCancel = cancel
	m.pullRequestsSeq++
	if m.section == pullRequests && (after != "" || len(m.pullRequests) == 0) {
		m.rebuildItems()
	}
	repo, filter, seq := m.repo, m.pullRequestFilter, m.pullRequestsSeq
	current := m.pullRequests
	return func() tea.Msg {
		var cache config.PullRequestListCache
		// Key the cached list by local remote settings without waiting for gh.
		remotes, err := repo.Run(ctx, "config", "--null", "--get-regexp", `^remote\..*\.(url|gh-resolved)$`)
		if err == nil {
			cache = config.NewPullRequestListCache(repo.Root, remotes, filter)
		}
		previous := current
		fetch := func() tea.Msg {
			defer cancel()
			list, err := repo.PullRequests(ctx, filter, after)
			switch {
			case err == nil && after != "":
				list.Requests = appendPullRequestPage(slices.Clone(previous), list.Requests)
			case err == nil:
				list, err = refreshPullRequestPages(ctx, repo, filter, list, len(previous))
			}
			if err == nil && ctx.Err() == nil {
				cache.Save(list)
			}
			return pullRequestsMsg{seq: seq, after: after, list: list, err: err}
		}
		if previous == nil {
			if list, ok := cache.Load(); ok {
				previous = list.Requests
				return cachedPullRequestsMsg{seq: seq, list: list, refresh: fetch}
			}
		}
		return fetch()
	}
}

func refreshPullRequestPages(ctx context.Context, repo *gitservice.Repository, filter gitservice.PullRequestFilter, list gitservice.PullRequestList, target int) (gitservice.PullRequestList, error) {
	seen := map[string]bool{}
	for len(list.Requests) < target && list.NextCursor != "" {
		if seen[list.NextCursor] {
			return gitservice.PullRequestList{}, errors.New("GitHub returned a repeated pull request cursor")
		}
		seen[list.NextCursor] = true
		page, err := repo.PullRequests(ctx, filter, list.NextCursor)
		if err != nil {
			return gitservice.PullRequestList{}, err
		}
		list.Requests = appendPullRequestPage(list.Requests, page.Requests)
		list.NextCursor, list.OpenCount = page.NextCursor, page.OpenCount
	}
	return list, nil
}

func (m *Model) applyCachedPullRequests(msg cachedPullRequestsMsg) tea.Cmd {
	if msg.seq != m.pullRequestsSeq {
		return nil
	}
	m.openPullRequestCount = msg.list.OpenCount
	m.pullRequests = msg.list.Requests
	// Rebuild the loaded range with fresh cursors before requesting any additional pages.
	if m.section == pullRequests {
		m.rebuildItems()
	}
	return msg.refresh
}

func (m *Model) applyPullRequests(msg pullRequestsMsg) {
	if msg.seq != m.pullRequestsSeq {
		return
	}
	m.pullRequestsCancel = nil
	inPullRequests := m.section == pullRequests
	topKey := ""
	if inPullRequests && m.sideTop < len(m.items) {
		topKey = m.items[m.sideTop].key
	}
	if msg.err != nil {
		if msg.after == "" {
			m.pullRequestsError = msg.err
			if inPullRequests {
				m.notice = m.tr("Could not load pull requests") + " · " + pullRequestErrorText(msg.err)
			}
		} else {
			m.pullRequestsPageError = msg.err
		}
	} else {
		m.openPullRequestCount = msg.list.OpenCount
		m.pullRequestsPageError = nil
		oldCount := len(m.pullRequests)
		m.pullRequests = msg.list.Requests
		m.pullRequestsNextCursor = msg.list.NextCursor
		if msg.after != "" && inPullRequests && len(m.pullRequests) > oldCount {
			if entry := m.current(); entry != nil && entry.key == pullRequestMoreKey {
				entry.key = pullRequestKey(m.pullRequests[oldCount].Number)
			}
		}
	}
	if inPullRequests {
		m.rebuildItems()
		if topKey != "" {
			if row := m.sidebarRowWithKey(topKey); row >= 0 {
				m.sideTop = row
				m.clampSidebarScroll()
			}
		}
	}
}

func appendPullRequestPage(existing, page []gitservice.PullRequest) []gitservice.PullRequest {
	seen := make(map[int]bool, len(existing)+len(page))
	for _, pr := range existing {
		seen[pr.Number] = true
	}
	for _, pr := range page {
		if !seen[pr.Number] {
			existing = append(existing, pr)
			seen[pr.Number] = true
		}
	}
	return existing
}

func (m *Model) pullRequestFilterLabel(filter gitservice.PullRequestFilter) string {
	switch filter {
	case gitservice.PullRequestMerged:
		return m.tr("Merged pull requests")
	case gitservice.PullRequestClosed:
		return m.tr("Closed pull requests")
	case gitservice.PullRequestAll:
		return m.tr("All pull requests")
	default:
		return m.tr("Open pull requests")
	}
}

func (m *Model) openPullRequestFilter() tea.Cmd {
	m.palette = nil
	for _, filter := range []gitservice.PullRequestFilter{
		gitservice.PullRequestOpen, gitservice.PullRequestMerged, gitservice.PullRequestClosed, gitservice.PullRequestAll,
	} {
		choice := action{id: string(filter), label: m.pullRequestFilterLabel(filter), run: func() tea.Cmd {
			return m.setPullRequestFilter(filter)
		}}
		if filter == m.pullRequestFilter {
			choice.key = m.tr("Current")
		}
		m.palette = append(m.palette, choice)
	}
	return m.prompt("pullRequestFilter", m.tr("Filter Pull Requests"), "")
}

func (m *Model) setPullRequestFilter(filter gitservice.PullRequestFilter) tea.Cmd {
	if filter == m.pullRequestFilter {
		return nil
	}
	if m.pullRequestsCancel != nil {
		m.pullRequestsCancel()
		m.pullRequestsCancel = nil
		m.pullRequestsSeq++
	}
	m.pullRequestFilter = filter
	m.pullRequests = nil
	m.pullRequestsNextCursor = ""
	m.pullRequestsPageError = nil
	m.pullRequestReview = nil
	m.selected, m.sideTop = -1, 0
	return m.refreshPullRequests(true)
}

const pullRequestRepositoryKey = "pr:repository"
const pullRequestMoreKey = "pr:more"

func pullRequestKey(number int) string { return "pr:" + strconv.Itoa(number) }

func (entry item) isPullRequest() bool {
	return entry.pullRequest != nil && entry.pullRequestFile == nil && entry.folder == ""
}

func (m *Model) pullRequestExpanded(entry item) bool {
	review := m.pullRequestReviews[entry.pullRequest.URL]
	return review != nil && review.expanded
}

func (m *Model) pullRequestItems() []item {
	items := []item{m.repositoryHeader("", pullRequestRepositoryKey)}
	if m.sidebarCollapsed[pullRequestRepositoryKey] {
		return items
	}
	if len(m.pullRequests) == 0 {
		for i, message := range m.pullRequestSidebarMessage() {
			items = append(items, item{key: "pr:message:" + strconv.Itoa(i), label: message})
		}
		return items
	}
	for i := range m.pullRequests {
		pr := &m.pullRequests[i]
		number := strconv.Itoa(pr.Number)
		entry := item{key: pullRequestKey(pr.Number), label: "#" + number + " " + pr.Title, pullRequest: pr, depth: 1}
		items = append(items, entry)
		if m.pullRequestExpanded(entry) {
			items = append(items, m.pullRequestTree(entry)...)
		}
	}
	if m.pullRequestsNextCursor != "" {
		label := m.tr("Load more pull requests…")
		if m.pullRequestsCancel != nil {
			label = m.tr("Loading pull requests…")
		} else if m.pullRequestsPageError != nil {
			label = m.tr("Failed to load; press Enter to retry: %s", pullRequestErrorText(m.pullRequestsPageError))
		}
		items = append(items, item{key: pullRequestMoreKey, label: label})
	}
	return items
}

func (m *Model) pullRequestTree(entry item) []item {
	review := m.pullRequestReviews[entry.pullRequest.URL]
	rows, _ := changeTree(review.changes, "", true, review.collapsed, entry.key+":")
	for i := range rows {
		row := &rows[i]
		row.pullRequest = entry.pullRequest
		if row.change != nil {
			row.pullRequestFile = review.files[row.change.Path]
			row.change = nil
		}
	}
	return rows
}

type pullRequestReview struct {
	pr        gitservice.PullRequest
	files     map[string]*diff.FilePatch
	changes   []gitservice.Change
	collapsed map[string]bool
	cancel    context.CancelFunc
	seq       uint64
	expanded  bool
	version   string
	lastUsed  time.Time
}

type pullRequestDiffMsg struct {
	review  *pullRequestReview
	seq     uint64
	version string
	files   []diff.FilePatch
	err     error
}

func (m *Model) forceRefreshPullRequests() tea.Cmd {
	list := m.refreshPullRequests(true)
	if m.snapshot.NoRepository {
		return list
	}
	review := m.pullRequestReview
	if review == nil && m.diffPR.Number != 0 {
		review = m.pullRequestReviewFor(m.diffPR)
		m.pullRequestReview = review
	}
	if review == nil {
		return list
	}
	return tea.Batch(list, m.fetchPullRequestDiff(review, true))
}

func (m *Model) fetchPullRequestDiff(review *pullRequestReview, force bool) tea.Cmd {
	if review.cancel != nil {
		if !force {
			return nil
		}
		review.cancel()
	}
	ctx, cancel := context.WithCancel(m.ctx)
	review.cancel = cancel
	review.seq++
	repo, target, seq := m.repo, review.pr.URL, review.seq
	expectedVersion := ""
	if !force {
		expectedVersion = review.pr.DiffVersion()
	}
	return func() tea.Msg {
		defer cancel()
		files, version, err := readPullRequestFiles(ctx, repo, target, expectedVersion)
		return pullRequestDiffMsg{review: review, seq: seq, version: version, files: files, err: err}
	}
}

func readPullRequestFiles(ctx context.Context, repo *gitservice.Repository, target, expectedVersion string) ([]diff.FilePatch, string, error) {
	cache := config.NewPullRequestDiffCache(repo.Root, pullRequestCacheSize)
	if expectedVersion != "" {
		if raw, ok := cache.Load(target, expectedVersion); ok {
			if files, err := diff.SplitFiles(raw); err == nil {
				return files, expectedVersion, nil
			}
		}
	}
	// gh renders the current PR diff. Check the version on both sides of the
	// fetch so a concurrent push cannot label the patch with a different revision.
	version, err := repo.PullRequestDiffVersion(ctx, target)
	if err != nil {
		return nil, "", err
	}
	for range 2 {
		raw, err := repo.PullRequestDiff(ctx, target)
		if err != nil {
			return nil, "", err
		}
		current, err := repo.PullRequestDiffVersion(ctx, target)
		if err != nil {
			return nil, "", err
		}
		if current == version {
			files, err := diff.SplitFiles(raw)
			if err == nil && ctx.Err() == nil {
				cache.Save(target, version, raw)
			}
			return files, version, err
		}
		version = current
	}
	return nil, "", errors.New("pull request changed while fetching its diff")
}

func (m *Model) pullRequestReviewFor(pr gitservice.PullRequest) *pullRequestReview {
	if m.pullRequestReviews == nil {
		m.pullRequestReviews = make(map[string]*pullRequestReview)
	}
	review := m.pullRequestReviews[pr.URL]
	if review == nil {
		if len(m.pullRequestReviews) == pullRequestCacheSize {
			var oldest *pullRequestReview
			for _, cached := range m.pullRequestReviews {
				if oldest == nil || cached.lastUsed.Before(oldest.lastUsed) {
					oldest = cached
				}
			}
			if oldest.cancel != nil {
				oldest.cancel()
			}
			delete(m.pullRequestReviews, oldest.pr.URL)
		}
		review = &pullRequestReview{collapsed: make(map[string]bool)}
		m.pullRequestReviews[pr.URL] = review
	}
	review.pr, review.lastUsed = pr, time.Now()
	return review
}

func (m *Model) loadPullRequest(entry item) tea.Cmd {
	pr := *entry.pullRequest
	review := m.pullRequestReviewFor(pr)
	review.expanded = true
	m.pullRequestReview = review
	m.rebuildItems()
	if review.version != "" && review.version == pr.DiffVersion() {
		return m.touchPullRequestDiffCache(pr.URL)
	}
	return m.fetchPullRequestDiff(review, false)
}

func (m *Model) touchPullRequestDiffCache(target string) tea.Cmd {
	workspace := m.repo.Root
	return func() tea.Msg {
		config.NewPullRequestDiffCache(workspace, pullRequestCacheSize).Touch(target)
		return nil
	}
}

func (m *Model) togglePullRequest(entry item) tea.Cmd {
	if review := m.pullRequestReviews[entry.pullRequest.URL]; review != nil && review.expanded {
		review.expanded = false
		m.rebuildItems()
		return nil
	}
	return m.loadPullRequest(entry)
}

func (m *Model) applyPullRequestDiff(msg pullRequestDiffMsg) tea.Cmd {
	review := msg.review
	if m.pullRequestReviews[review.pr.URL] != review || msg.seq != review.seq {
		return nil
	}
	review.cancel = nil
	inPullRequests := m.section == pullRequests
	active := review == m.pullRequestReview && inPullRequests
	if msg.err != nil {
		if active {
			m.notice = m.tr("Could not load PR #%d diff: %s", review.pr.Number, pullRequestErrorText(msg.err))
		}
		return nil
	}
	review.files = make(map[string]*diff.FilePatch, len(msg.files))
	review.changes = make([]gitservice.Change, 0, len(msg.files))
	for i := range msg.files {
		file := &msg.files[i]
		review.files[file.Path] = file
		review.changes = append(review.changes, gitservice.Change{Path: file.Path, Index: file.Status, Worktree: ' '})
	}
	review.version = msg.version
	if inPullRequests && review.expanded {
		m.rebuildItems()
	}
	var commands []tea.Cmd
	for _, tab := range m.tabs {
		d := tab.diff
		if d == nil || d.diffPR.URL != review.pr.URL {
			continue
		}
		d.diffPRVersion = msg.version
		raw := ""
		if file := review.files[d.diffPath]; file != nil {
			raw = file.Raw
		}
		if raw != d.patch.Raw {
			d.read = readPullRequestPatch(raw)
			commands = append(commands, m.loadDiff(d, false))
		}
	}
	return tea.Batch(commands...)
}

func readPullRequestPatch(raw string) func(context.Context) (string, error) {
	// A kept tab owns its file patch independently of the expanded PR.
	raw = strings.Clone(raw)
	return func(context.Context) (string, error) { return raw, nil }
}

func (m *Model) requestPullRequestDiff(entry item) tea.Cmd {
	review := m.pullRequestReviews[entry.pullRequest.URL]
	review.lastUsed = time.Now()
	changed := m.pullRequestReview != review
	m.pullRequestReview = review
	target := diffTarget{path: entry.pullRequestFile.Path, pr: *entry.pullRequest, prVersion: review.version, preview: true}
	cmd := m.requestDiff(target, readPullRequestPatch(entry.pullRequestFile.Raw))
	if changed {
		return tea.Batch(cmd, m.touchPullRequestDiffCache(entry.pullRequest.URL))
	}
	return cmd
}

func (m *Model) pullRequestSidebarMessage() []string {
	switch {
	case m.snapshot.NoRepository:
		return []string{m.tr("Not a Git repository")}
	case m.pullRequestsCancel != nil && m.pullRequests == nil:
		return []string{m.tr("Loading pull requests…")}
	case errors.Is(m.pullRequestsError, exec.ErrNotFound):
		return []string{m.tr("Install GitHub CLI (gh)"), m.tr("F5: Retry")}
	case m.pullRequestsError != nil:
		return []string{m.tr("Could not load pull requests"), m.tr("F5: Retry"), "", pullRequestErrorText(m.pullRequestsError)}
	case m.pullRequestFilter == gitservice.PullRequestOpen:
		return []string{m.tr("No open pull requests"), m.tr("F5: Refresh")}
	default:
		return []string{m.tr("No pull requests"), m.tr("F5: Refresh")}
	}
}

func pullRequestErrorText(err error) string {
	return safe(strings.Join(strings.Fields(ansi.Strip(err.Error())), " "))
}

func (m *Model) pullRequestStatus(entry item) string {
	pr := entry.pullRequest
	if review := m.pullRequestReviews[pr.URL]; review != nil && review.cancel != nil && review.version == "" {
		return m.tr("Loading…")
	}
	switch {
	case pr.State == "OPEN" && pr.IsDraft:
		return m.tr("Draft")
	case pr.State == "OPEN":
		return m.tr("Open")
	case pr.State == "MERGED":
		return m.tr("Merged")
	case pr.State == "CLOSED":
		return m.tr("Closed")
	}
	return ""
}
