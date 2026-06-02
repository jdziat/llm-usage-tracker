package usage

import (
	"os"
	"path/filepath"
	"strings"
)

func defaultSourcePaths(source string) []string {
	home, _ := os.UserHomeDir()
	switch source {
	case SourceClaude:
		if v := os.Getenv("CLAUDE_CONFIG_DIR"); v != "" {
			return expandList(v, "projects")
		}
		return existing(filepath.Join(home, ".config", "claude", "projects"), filepath.Join(home, ".claude", "projects"))
	case SourceCodex:
		if v := os.Getenv("CODEX_HOME"); v != "" {
			return expandList(v, "sessions")
		}
		return existing(filepath.Join(home, ".codex", "sessions"))
	case SourceOpenCode:
		if v := os.Getenv("OPENCODE_DATA_DIR"); v != "" {
			return expandList(v, "")
		}
		return existing(filepath.Join(home, ".local", "share", "opencode"))
	case SourceAmp:
		if v := os.Getenv("AMP_DATA_DIR"); v != "" {
			return expandList(v, "")
		}
		return existing(filepath.Join(home, ".local", "share", "amp"))
	case SourcePI:
		if v := os.Getenv("PI_AGENT_DIR"); v != "" {
			return expandList(v, "")
		}
		return existing(filepath.Join(home, ".pi", "agent", "sessions"))
	default:
		return nil
	}
}

func existing(paths ...string) []string {
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

func expandList(v, appendDir string) []string {
	home, _ := os.UserHomeDir()
	var out []string
	for _, part := range strings.Split(v, ",") {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if p == "~" {
			p = home
		} else if strings.HasPrefix(p, "~/") {
			p = filepath.Join(home, strings.TrimPrefix(p, "~/"))
		}
		if appendDir != "" {
			if st, err := os.Stat(filepath.Join(p, appendDir)); err == nil && st.IsDir() {
				p = filepath.Join(p, appendDir)
			}
		}
		out = append(out, p)
	}
	return out
}

func discoverFiles(roots []string, want func(string) bool) ([]string, error) {
	var files []string
	for _, root := range roots {
		if root == "" {
			continue
		}
		if st, err := os.Stat(root); err != nil || !st.IsDir() {
			continue
		}
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if name == ".git" || name == "node_modules" || name == "memory" {
					return filepath.SkipDir
				}
				return nil
			}
			if want(path) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}
