package update

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	selfupdate "github.com/creativeprojects/go-selfupdate"
)

const checkTimeout = 10 * time.Second

// Release holds information about an available update.
type Release struct {
	Version      string
	URL          string
	ReleaseNotes string
}

// CheckForUpdate queries GitHub Releases for a newer version.
// Returns nil if the current version is already the latest or if version is "dev".
func CheckForUpdate(currentVersion, repo string) (*Release, error) {
	if currentVersion == "dev" || currentVersion == "" {
		return nil, nil
	}

	current, err := parseSemver(currentVersion)
	if err != nil {
		return nil, nil // unparseable version (e.g. dirty build) — skip silently
	}

	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("create github source: %w", err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{Source: source})
	if err != nil {
		return nil, fmt.Errorf("create updater: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	latest, found, err := updater.DetectLatest(ctx, selfupdate.ParseSlug(repo))
	if err != nil {
		return nil, fmt.Errorf("detect latest release: %w", err)
	}
	if !found {
		return nil, nil
	}

	latestVer, err := semver.NewVersion(latest.Version())
	if err != nil {
		return nil, nil
	}

	if !latestVer.GreaterThan(current) {
		return nil, nil
	}

	return &Release{
		Version:      latest.Version(),
		URL:          latest.URL,
		ReleaseNotes: latest.ReleaseNotes,
	}, nil
}

// Apply downloads the latest release binary and replaces the current executable.
func Apply(currentVersion, repo string) (*Release, error) {
	if currentVersion == "dev" || currentVersion == "" {
		return nil, fmt.Errorf("cannot update a development build — install from a release first")
	}

	cleanVersion := strings.TrimPrefix(currentVersion, "v")

	source, err := selfupdate.NewGitHubSource(selfupdate.GitHubConfig{})
	if err != nil {
		return nil, fmt.Errorf("create github source: %w", err)
	}

	updater, err := selfupdate.NewUpdater(selfupdate.Config{Source: source})
	if err != nil {
		return nil, fmt.Errorf("create updater: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	rel, err := updater.UpdateSelf(ctx, cleanVersion, selfupdate.ParseSlug(repo))
	if err != nil {
		return nil, fmt.Errorf("update failed: %w", err)
	}

	return &Release{
		Version:      rel.Version(),
		URL:          rel.URL,
		ReleaseNotes: rel.ReleaseNotes,
	}, nil
}

// CompareVersions compares two semver strings.
// Returns -1 if current < latest, 0 if equal, 1 if current > latest.
// Unparseable versions are treated as less than any valid version.
func CompareVersions(current, latest string) int {
	cv, errC := parseSemver(current)
	lv, errL := parseSemver(latest)

	if errC != nil && errL != nil {
		return 0
	}
	if errC != nil {
		return -1
	}
	if errL != nil {
		return 1
	}

	return cv.Compare(lv)
}

// parseSemver strips a leading "v" and handles git-describe suffixes
// like "0.1.0-3-gabcdef" by parsing only the base version.
func parseSemver(s string) (*semver.Version, error) {
	s = strings.TrimPrefix(s, "v")
	return semver.NewVersion(s)
}
