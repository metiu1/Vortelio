package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vortelio/vortelio/internal/hub"
)

// ─── CLEANUP ──────────────────────────────────────────────────────────────────

type CleanupCommand struct{}

func NewCleanupCommand() *CleanupCommand { return &CleanupCommand{} }
func (c *CleanupCommand) Name() string   { return "cleanup" }
func (c *CleanupCommand) Run(args []string) error {
	doDelete := false
	for _, a := range args {
		if a == "--delete" || a == "-d" { doDelete = true }
	}
	return RunCleanup(!doDelete)
}

// RunCleanup analyzes ~/.vortelio/ and reports/removes wasted space.
func RunCleanup(dryRun bool) error {
	home, _ := os.UserHomeDir()
	vortDir := filepath.Join(home, ".vortelio")

	fmt.Println("🔍  Analyzing Vortelio disk usage...")
	fmt.Println()

	var totalFound int64
	var issues []cleanupIssue

	// ── 1. HuggingFace cache ────────────────────────────────────────────────
	for _, hfCache := range []string{
		filepath.Join(home, ".cache", "huggingface", "hub"),
		filepath.Join(home, ".cache", "huggingface"),
	} {
		if fi, err := os.Stat(hfCache); err == nil && fi.IsDir() {
			size := dirSize(hfCache)
			if size > 50*1024*1024 { // > 50MB
				issues = append(issues, cleanupIssue{
					Path: hfCache,
					Size: size,
					Desc: "HuggingFace cache (duplicate copy of models downloaded via snapshot_download)",
					Safe: true,
				})
				totalFound += size
				break // don't add both paths
			}
		}
	}

	// ── 2. Cartelle orfane (no manifest.json) ──────────────────────────────
	modelsDir := filepath.Join(vortDir, "models")
	for _, mtype := range []string{"llm", "image", "audio", "video", "3d"} {
		typeDir := filepath.Join(modelsDir, mtype)
		names, _ := os.ReadDir(typeDir)
		for _, name := range names {
			if !name.IsDir() { continue }
			tags, _ := os.ReadDir(filepath.Join(typeDir, name.Name()))
			for _, tag := range tags {
				if !tag.IsDir() { continue }
				tagDir := filepath.Join(typeDir, name.Name(), tag.Name())
				manifest := filepath.Join(tagDir, "manifest.json")
				if _, err := os.Stat(manifest); err != nil {
					size := dirSize(tagDir)
					if size > 1024*1024 { // > 1MB
						issues = append(issues, cleanupIssue{
							Path: tagDir,
							Size: size,
							Desc: fmt.Sprintf("Folder without manifest.json: %s/%s/%s", mtype, name.Name(), tag.Name()),
							Safe: true,
						})
						totalFound += size
					}
				}
			}
		}
	}

	// ── 3. Versioni duplicate ──────────────────────────────────────────────
	store := hub.NewModelStore()
	models, _ := store.List()
	byName := map[string][]*hub.Model{}
	for _, m := range models {
		key := fmt.Sprintf("%s/%s", m.Type, m.Name)
		byName[key] = append(byName[key], m)
	}
	for key, group := range byName {
		if len(group) <= 1 { continue }
		sort.Slice(group, func(i, j int) bool {
			return group[i].DownloadedAt.After(group[j].DownloadedAt)
		})
		for _, old := range group[1:] {
			dir := filepath.Dir(old.LocalPath)
			if !strings.HasPrefix(dir, modelsDir) { dir = old.LocalPath }
			size := dirSize(dir)
			issues = append(issues, cleanupIssue{
				Path: dir,
				Size: size,
				Desc: fmt.Sprintf("Duplicate: %s — keep tag '%s', remove '%s'", key, group[0].Tag, old.Tag),
				Safe: false,
			})
			totalFound += size
		}
	}

	// ── 4. File .bin/.pt quando esiste .safetensors equivalente ───────────
	_ = filepath.Walk(modelsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() { return nil }
		lower := strings.ToLower(path)
		if !strings.HasSuffix(lower, ".bin") && !strings.HasSuffix(lower, ".pt") { return nil }
		base := strings.TrimSuffix(strings.TrimSuffix(path, ".bin"), ".pt")
		base = strings.TrimSuffix(base, ".PT")
		if _, err := os.Stat(base + ".safetensors"); err == nil {
			issues = append(issues, cleanupIssue{
				Path: path,
				Size: info.Size(),
				Desc: fmt.Sprintf(".bin/.pt redundant (safetensors version exists): %s", filepath.Base(path)),
				Safe: true,
			})
			totalFound += info.Size()
		}
		return nil
	})

	// ── 5. File temporanei Python rimasti ─────────────────────────────────
	for _, pattern := range []string{
		filepath.Join(os.TempDir(), "vortelio-*.py"),
		filepath.Join(os.TempDir(), "vortelio-out-*"),
	} {
		matches, _ := filepath.Glob(pattern)
		for _, m := range matches {
			fi, err := os.Stat(m)
			if err != nil { continue }
			issues = append(issues, cleanupIssue{
				Path: m, Size: fi.Size(),
				Desc: "Python temp: " + filepath.Base(m),
				Safe: true,
			})
			totalFound += fi.Size()
		}
	}

	// ── Report ────────────────────────────────────────────────────────────
	fmt.Println("📊  Disk usage by type:")
	for _, mtype := range []string{"llm", "image", "audio", "video", "3d"} {
		size := dirSize(filepath.Join(modelsDir, mtype))
		if size > 1024*1024 {
			fmt.Printf("  %-8s  %s\n", strings.ToUpper(mtype), humanSize(size))
		}
	}
	fmt.Printf("  %-8s  %s\n", "TOTAL", humanSize(dirSize(modelsDir)))
	fmt.Println()

	if len(issues) == 0 {
		fmt.Println("✅  No issues found. All space is used by models.")
		return nil
	}

	safeSize := int64(0)
	for _, issue := range issues { if issue.Safe { safeSize += issue.Size } }

	fmt.Printf("⚠️   Found %d issues — %.2f  GB recoverable\n\n", len(issues), float64(totalFound)/1e9)
	for i, issue := range issues {
		mark := "🟡"
		if issue.Safe { mark = "🔴" }
		fmt.Printf("  %s [%d] %s\n       %s (%s)\n\n",
			mark, i+1, issue.Desc, issue.Path, humanSize(issue.Size))
	}
	fmt.Printf("🔴 Safe to delete:  %s\n", humanSize(safeSize))
	fmt.Printf("🟡 Requires confirmation: %s\n\n", humanSize(totalFound-safeSize))

	if dryRun {
		fmt.Println("💡  Run  vortelio cleanup --delete  to free safe space")
		return nil
	}

	deleted := int64(0)
	for _, issue := range issues {
		if !issue.Safe { continue }
		fi, err := os.Stat(issue.Path)
		if err != nil { continue }
		fmt.Printf("  🗑  %s (%s)\n", issue.Path, humanSize(issue.Size))
		if fi.IsDir() {
			os.RemoveAll(issue.Path)
		} else {
			os.Remove(issue.Path)
		}
		deleted += issue.Size
	}
	fmt.Printf("\n✅  Freed %s\n", humanSize(deleted))
	return nil
}

type cleanupIssue struct {
	Path string
	Size int64
	Desc string
	Safe bool
}

func dirSize(path string) int64 {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() { size += info.Size() }
		return nil
	})
	return size
}

func humanSize(b int64) string {
	if b >= 1<<30 { return fmt.Sprintf("%.2f GB", float64(b)/(1<<30)) }
	if b >= 1<<20 { return fmt.Sprintf("%.0f MB", float64(b)/(1<<20)) }
	return fmt.Sprintf("%.0f KB", float64(b)/(1<<10))
}
