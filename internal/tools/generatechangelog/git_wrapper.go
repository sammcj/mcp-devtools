package generatechangelog

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// GitInterface is our own git interface that matches Chronicle's expectations
// but uses publicly accessible packages
type GitInterface interface {
	FirstCommit() (string, error)
	HeadTagOrCommit() (string, error)
	HeadTag() (string, error)
	RemoteURL() (string, error)
	SearchForTag(tagRef string) (*GitTag, error)
	TagsFromLocal() ([]GitTag, error)
	CommitsBetween(cfg GitRange) ([]string, error)
}

// GitTag represents a git tag with timestamp information
type GitTag struct {
	Name      string
	Timestamp time.Time
	Commit    string
	Annotated bool
}

// GitRange represents a range of commits
type GitRange struct {
	SinceRef     string
	UntilRef     string
	IncludeStart bool
	IncludeEnd   bool
}

// GitWrapper implements GitInterface using go-git
type GitWrapper struct {
	repoPath string
}

// NewGitWrapper creates a new git wrapper for the given repository path
func NewGitWrapper(repoPath string) (GitInterface, error) {
	// Verify it's a git repository
	_, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, fmt.Errorf("not a git repository: %q", repoPath)
	}

	return &GitWrapper{repoPath: repoPath}, nil
}

// FirstCommit returns the first commit in the repository
func (g *GitWrapper) FirstCommit() (string, error) {
	// Use git command as this is simpler than iterating through all commits
	cmd := exec.Command("git", "-C", g.repoPath, "rev-list", "--max-parents=0", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get first commit: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// HeadTagOrCommit returns the current HEAD tag if it exists, otherwise the commit hash
func (g *GitWrapper) HeadTagOrCommit() (string, error) {
	// Try to get tag first
	if tag, err := g.HeadTag(); err == nil && tag != "" {
		return tag, nil
	}

	// Fall back to commit hash
	cmd := exec.Command("git", "-C", g.repoPath, "rev-parse", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// HeadTag returns the tag at HEAD if it exists
func (g *GitWrapper) HeadTag() (string, error) {
	cmd := exec.Command("git", "-C", g.repoPath, "tag", "--points-at", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	tag := strings.TrimSpace(string(output))
	if tag == "" {
		return "", fmt.Errorf("no tag at HEAD")
	}
	// If multiple tags, return the first one
	lines := strings.Split(tag, "\n")
	return lines[0], nil
}

// RemoteURL returns the remote URL for origin
func (g *GitWrapper) RemoteURL() (string, error) {
	cmd := exec.Command("git", "-C", g.repoPath, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get remote URL: %w", err)
	}
	return strings.TrimSpace(string(output)), nil
}

// SearchForTag searches for a specific tag
func (g *GitWrapper) SearchForTag(tagRef string) (*GitTag, error) {
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return nil, err
	}

	ref, err := repo.Reference(plumbing.NewTagReferenceName(tagRef), false)
	if err != nil {
		return nil, fmt.Errorf("unable to find git tag %q: %w", tagRef, err)
	}

	return g.newTag(repo, ref)
}

// TagsFromLocal returns all local tags
func (g *GitWrapper) TagsFromLocal() ([]GitTag, error) {
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return nil, err
	}

	tagRefs, err := repo.Tags()
	if err != nil {
		return nil, err
	}

	var tags []GitTag
	err = tagRefs.ForEach(func(t *plumbing.Reference) error {
		tag, err := g.newTag(repo, t)
		if err != nil {
			return err
		}
		if tag != nil {
			tags = append(tags, *tag)
		}
		return nil
	})

	return tags, err
}

// CommitsBetween returns commits between two references
func (g *GitWrapper) CommitsBetween(cfg GitRange) ([]string, error) {
	repo, err := git.PlainOpen(g.repoPath)
	if err != nil {
		return nil, err
	}

	var sinceHash *plumbing.Hash
	if cfg.SinceRef != "" {
		sinceHash, err = repo.ResolveRevision(plumbing.Revision(cfg.SinceRef))
		if err != nil {
			return nil, fmt.Errorf("unable to find since git ref=%q: %w", cfg.SinceRef, err)
		}
	}

	untilHash, err := repo.ResolveRevision(plumbing.Revision(cfg.UntilRef))
	if err != nil {
		return nil, fmt.Errorf("unable to find until git ref=%q: %w", cfg.UntilRef, err)
	}

	iter, err := repo.Log(&git.LogOptions{From: *untilHash})
	if err != nil {
		return nil, fmt.Errorf("unable to get git log for ref=%q: %w", cfg.UntilRef, err)
	}

	var commits []string
	err = iter.ForEach(func(c *object.Commit) error {
		hash := c.Hash.String()

		switch {
		case untilHash != nil && c.Hash == *untilHash:
			if cfg.IncludeEnd {
				commits = append(commits, hash)
			}
		case sinceHash != nil && c.Hash == *sinceHash:
			if cfg.IncludeStart {
				commits = append(commits, hash)
			}
			return nil // Stop iteration
		default:
			commits = append(commits, hash)
		}

		return nil
	})

	return commits, err
}

// newTag creates a GitTag from a git reference
func (g *GitWrapper) newTag(repo *git.Repository, t *plumbing.Reference) (*GitTag, error) {
	if !t.Name().IsTag() {
		return nil, nil
	}

	// Try to get commit directly (lightweight tag)
	c, err := repo.CommitObject(t.Hash())
	if err == nil && c != nil {
		return &GitTag{
			Name:      t.Name().Short(),
			Timestamp: c.Committer.When,
			Commit:    c.Hash.String(),
			Annotated: false,
		}, nil
	}

	// Try to get annotated tag object
	tagObj, err := object.GetTag(repo.Storer, t.Hash())
	if err != nil {
		return nil, fmt.Errorf("unable to resolve tag for %q: %w", t.Name(), err)
	}

	if tagObj == nil {
		return nil, fmt.Errorf("unable to resolve tag for %q", t.Name())
	}

	return &GitTag{
		Name:      t.Name().Short(),
		Timestamp: tagObj.Tagger.When.In(time.Local),
		Commit:    tagObj.Target.String(),
		Annotated: true,
	}, nil
}
