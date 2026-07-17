package orchestrator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/auto-code-os/auto-code-os/server/internal/context/provider"
)

func (o *Orchestrator) StartGlobalCachePrewarmer(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	// Initial run
	o.prewarmAllCaches(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.prewarmAllCaches(ctx)
		}
	}
}

func (o *Orchestrator) StartCacheGarbageCollector(ctx context.Context) {
	// Daily ticker
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	// Initial run on start
	o.runGarbageCollection(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			o.runGarbageCollection(ctx)
		}
	}
}

func (o *Orchestrator) prewarmAllCaches(ctx context.Context) {
	if o.repositories == nil || o.gitOps == nil || o.ctxEngine == nil {
		return
	}

	repos, err := o.repositories.ListAll(ctx)
	if err != nil {
		return
	}

	globalCacheDir := filepath.Join(o.dataRoot, "database", "global_cache")
	if err := os.MkdirAll(globalCacheDir, 0755); err != nil {
		return
	}

	for _, repo := range repos {
		parts := strings.Split(repo.URL, "/")
		if len(parts) == 0 {
			continue
		}
		repoName := parts[len(parts)-1]
		repoName = strings.TrimSuffix(repoName, ".git")

		// Create a temp folder to clone the repo into
		tempPath := filepath.Join(o.workspaceRoot, fmt.Sprintf("temp_prewarm_%s_%s", repoName, repo.ID))
		_ = os.RemoveAll(tempPath)

		_, err := o.gitOps.CloneForTask(ctx, repo.URL, repo.Branch, tempPath)
		if err != nil {
			_ = os.RemoveAll(tempPath)
			continue
		}

		// Run git rev-parse HEAD to get commit hash
		commitHash, errCommit := runGitCmd(tempPath, "rev-parse", "HEAD")
		if errCommit != nil {
			_ = os.RemoveAll(tempPath)
			continue
		}

		globalCachePath := filepath.Join(globalCacheDir, fmt.Sprintf("global_cache_%s_%s.db", repoName, commitHash))
		if _, errStat := os.Stat(globalCachePath); errStat == nil {
			// Already indexed
			_ = os.RemoveAll(tempPath)
			continue
		}

		// Run indexing on tempPath
		prewarmCtx := context.WithValue(ctx, provider.WorkspaceRootKey, tempPath)
		if errIndex := o.ctxEngine.IndexWorkspace(prewarmCtx); errIndex != nil {
			_ = os.RemoveAll(tempPath)
			continue
		}

		localDbPath := filepath.Join(tempPath, "context", "workspace_cache.db")
		if _, errStat := os.Stat(localDbPath); errStat == nil {
			tmpGlobalPath := globalCachePath + ".tmp"
			if errCopy := copyFile(localDbPath, tmpGlobalPath); errCopy == nil {
				if errRename := os.Rename(tmpGlobalPath, globalCachePath); errRename == nil {
					// GC: Keep only this latest version, delete older versions for this repository
					entries, errRead := os.ReadDir(globalCacheDir)
					if errRead == nil {
						prefix := fmt.Sprintf("global_cache_%s_", repoName)
						expectedName := filepath.Base(globalCachePath)
						for _, entry := range entries {
							if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) && entry.Name() != expectedName {
								_ = os.Remove(filepath.Join(globalCacheDir, entry.Name()))
							}
						}
					}
				} else {
					_ = os.Remove(tmpGlobalPath)
				}
			}
		}

		_ = os.RemoveAll(tempPath)
	}
}

func (o *Orchestrator) runGarbageCollection(ctx context.Context) {
	if o.repositories == nil || o.dataRoot == "" {
		return
	}

	repos, err := o.repositories.ListAll(ctx)
	if err != nil {
		return
	}

	globalCacheDir := filepath.Join(o.dataRoot, "database", "global_cache")
	entries, err := os.ReadDir(globalCacheDir)
	if err != nil {
		return
	}

	// 1. Build a set of referenced repo-commits from active task workspaces
	referenced := make(map[string]bool) // key: "repoName_commitHash"

	// Scan task workspaces under o.workspaceRoot
	if workspaceEntries, err := os.ReadDir(o.workspaceRoot); err == nil {
		for _, wEntry := range workspaceEntries {
			if !wEntry.IsDir() {
				continue
			}
			metaJSONPath := filepath.Join(o.workspaceRoot, wEntry.Name(), "metadata.json")
			metaData, errRead := os.ReadFile(metaJSONPath)
			if errRead != nil {
				continue
			}

			// We unmarshal metadata.json to find what repositories and commits are checked out
			type metadataRepo struct {
				Name  string `json:"name"`
				Paths struct {
					Main string `json:"main"`
				} `json:"paths"`
			}
			type taskMeta struct {
				Repos []metadataRepo `json:"repos"`
			}

			var metadata taskMeta
			if errUnmarshal := json.Unmarshal(metaData, &metadata); errUnmarshal == nil {
				for _, r := range metadata.Repos {
					if r.Paths.Main == "" {
						continue
					}
					repoAbsPath := filepath.Join(o.workspaceRoot, wEntry.Name(), r.Paths.Main)
					commitHash, errCommit := runGitCmd(repoAbsPath, "rev-parse", "HEAD")
					if errCommit == nil && commitHash != "" {
						key := fmt.Sprintf("%s_%s", r.Name, commitHash)
						referenced[key] = true
					}
				}
			}
		}
	}

	now := time.Now()
	sevenDaysAgo := now.AddDate(0, 0, -7)

	for _, repo := range repos {
		parts := strings.Split(repo.URL, "/")
		if len(parts) == 0 {
			continue
		}
		repoName := parts[len(parts)-1]
		repoName = strings.TrimSuffix(repoName, ".git")

		prefix := fmt.Sprintf("global_cache_%s_", repoName)
		var matches []os.DirEntry
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), prefix) {
				matches = append(matches, entry)
			}
		}

		if len(matches) <= 1 {
			continue
		}

		// Keep the latest one by default (sort modTime)
		type fileInfo struct {
			entry   os.DirEntry
			modTime time.Time
		}
		var infos []fileInfo
		for _, m := range matches {
			if info, errInfo := m.Info(); errInfo == nil {
				infos = append(infos, fileInfo{entry: m, modTime: info.ModTime()})
			}
		}

		if len(infos) <= 1 {
			continue
		}

		// Sort descending by modTime
		for i := 0; i < len(infos); i++ {
			for j := i + 1; j < len(infos); j++ {
				if infos[i].modTime.Before(infos[j].modTime) {
					infos[i], infos[j] = infos[j], infos[i]
				}
			}
		}

		// Delete older files if:
		// 1. They are older than 7 days (modTime is before sevenDaysAgo)
		// 2. They are NOT referenced by any active task workspace
		for i := 1; i < len(infos); i++ {
			commitHash := strings.TrimPrefix(infos[i].entry.Name(), prefix)
			commitHash = strings.TrimSuffix(commitHash, ".db")

			key := fmt.Sprintf("%s_%s", repoName, commitHash)
			if referenced[key] {
				// Don't delete if referenced
				continue
			}

			if infos[i].modTime.Before(sevenDaysAgo) {
				_ = os.Remove(filepath.Join(globalCacheDir, infos[i].entry.Name()))
			}
		}
	}
}

func runGitCmd(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Sync()
}
