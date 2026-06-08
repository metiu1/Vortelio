package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vortelio/vortelio/internal/version"
)

const (
	RepoInstallSpec = "git+https://github.com/metiu1/Vortelio#subdirectory=vortelio-pip"
	LatestAPIURL    = "https://api.github.com/repos/metiu1/Vortelio/releases/latest"
)

type Info struct {
	Current        string `json:"current"`
	Latest         string `json:"latest"`
	Available      bool   `json:"available"`
	InstallCommand string `json:"install_command"`
	Source         string `json:"source"`
}

type StartResult struct {
	Started bool   `json:"started"`
	Message string `json:"message"`
	LogPath string `json:"log_path,omitempty"`
}

func Check(ctx context.Context) (Info, error) {
	info := Info{
		Current:        version.Version,
		InstallCommand: "uv tool install --force \"" + RepoInstallSpec + "\"",
		Source:         LatestAPIURL,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, LatestAPIURL, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "vortelio-updater/"+version.Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return info, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return info, fmt.Errorf("GitHub returned HTTP %d", resp.StatusCode)
	}

	var payload struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return info, err
	}
	latest := cleanVersion(payload.TagName)
	if latest == "" {
		latest = cleanVersion(payload.Name)
	}
	if latest == "" {
		return info, errors.New("latest release does not contain a version")
	}

	info.Latest = latest
	info.Available = compareVersions(latest, version.Version) > 0
	return info, nil
}

func CheckWithTimeout(timeout time.Duration) (Info, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return Check(ctx)
}

func cleanVersion(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(strings.TrimPrefix(s, "V"), "v")
	re := regexp.MustCompile(`\d+(?:\.\d+){1,3}`)
	if m := re.FindString(s); m != "" {
		return m
	}
	return ""
}

func compareVersions(a, b string) int {
	ap := versionParts(a)
	bp := versionParts(b)
	max := len(ap)
	if len(bp) > max {
		max = len(bp)
	}
	for i := 0; i < max; i++ {
		var av, bv int
		if i < len(ap) {
			av = ap[i]
		}
		if i < len(bp) {
			bv = bp[i]
		}
		if av > bv {
			return 1
		}
		if av < bv {
			return -1
		}
	}
	return 0
}

func versionParts(v string) []int {
	v = cleanVersion(v)
	if v == "" {
		return nil
	}
	raw := strings.Split(v, ".")
	out := make([]int, 0, len(raw))
	for _, part := range raw {
		n, _ := strconv.Atoi(part)
		out = append(out, n)
	}
	return out
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
