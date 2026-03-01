package testutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// SetupXDGDirs creates an isolated XDG directory structure for testing.
// It sets XDG_CONFIG_HOME and XDG_DATA_HOME to temp directories and creates
// the axe subdirectory tree (agents/, skills/ under config; axe/ under data).
// Returns the axe-level config path and data path.
func SetupXDGDirs(t *testing.T) (configDir, dataDir string) {
	t.Helper()

	root := t.TempDir()

	configHome := filepath.Join(root, "config")
	dataHome := filepath.Join(root, "data")

	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_DATA_HOME", dataHome)

	configDir = filepath.Join(configHome, "axe")
	dataDir = filepath.Join(dataHome, "axe")

	dirs := []string{
		filepath.Join(configDir, "agents"),
		filepath.Join(configDir, "skills"),
		dataDir,
	}

	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create directory %s: %v", d, err)
		}
	}

	return configDir, dataDir
}

// SeedFixtureAgents copies all .toml files from srcDir to dstAgentsDir.
// Files are copied byte-for-byte with no parsing or validation.
func SeedFixtureAgents(t *testing.T, srcDir, dstAgentsDir string) {
	t.Helper()

	entries, err := os.ReadDir(srcDir)
	if err != nil {
		t.Fatalf("source directory does not exist: %s: %v", srcDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".toml" {
			continue
		}

		srcPath := filepath.Join(srcDir, entry.Name())
		data, err := os.ReadFile(srcPath)
		if err != nil {
			t.Fatalf("failed to read %s: %v", srcPath, err)
		}

		dstPath := filepath.Join(dstAgentsDir, entry.Name())
		if err := os.WriteFile(dstPath, data, 0644); err != nil {
			t.Fatalf("failed to write %s: %v", dstPath, err)
		}
	}
}

// SeedFixtureSkills recursively copies the entire directory tree from srcDir
// to dstSkillsDir, preserving directory structure. Only regular files and
// directories are processed; symlinks are not followed.
func SeedFixtureSkills(t *testing.T, srcDir, dstSkillsDir string) {
	t.Helper()

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dstSkillsDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, 0755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, 0644)
	})
	if err != nil {
		t.Fatalf("failed to copy skills from %s to %s: %v", srcDir, dstSkillsDir, err)
	}
}

// SeedGlobalConfig writes content to configDir/config.toml.
// The content parameter is a raw TOML string. No validation is performed.
func SeedGlobalConfig(t *testing.T, configDir, content string) {
	t.Helper()

	path := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config.toml: %v", err)
	}
}

var (
	buildOnce     sync.Once
	buildBinPath  string
	buildErr      error
	buildCacheDir string
)

// findModuleRoot walks up from the given directory to find go.mod.
func findModuleRoot(start string) (string, error) {
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find module root (go.mod)")
		}
		dir = parent
	}
}

// BuildBinary compiles the axe binary once per test process and returns the
// cached path. Subsequent calls return the same path without rebuilding.
// If the build fails, t.Fatal is called with the error.
func BuildBinary(t *testing.T) string {
	t.Helper()

	buildOnce.Do(func() {
		// Find module root by walking up from this file's directory
		_, thisFile, _, _ := runtime.Caller(0)
		thisDir := filepath.Dir(thisFile)

		moduleRoot, err := findModuleRoot(thisDir)
		if err != nil {
			buildErr = err
			return
		}

		// Create temp dir for the binary (NOT t.TempDir - must persist across tests)
		cacheDir, err := os.MkdirTemp("", "axe-test-bin-*")
		if err != nil {
			buildErr = fmt.Errorf("failed to create cache dir: %w", err)
			return
		}
		buildCacheDir = cacheDir

		binName := "axe"
		if runtime.GOOS == "windows" {
			binName = "axe.exe"
		}
		binPath := filepath.Join(cacheDir, binName)

		cmd := exec.Command("go", "build", "-o", binPath, ".")
		cmd.Dir = moduleRoot
		var stderr strings.Builder
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			buildErr = fmt.Errorf("go build failed: %w\nstderr: %s", err, stderr.String())
			return
		}

		buildBinPath = binPath
	})

	if buildErr != nil {
		t.Fatalf("BuildBinary: %v", buildErr)
	}

	return buildBinPath
}

// CleanupBinary removes the cached binary directory. Intended to be called
// from TestMain after all tests have run.
func CleanupBinary() {
	if buildCacheDir != "" {
		os.RemoveAll(buildCacheDir)
	}
}

// resetBuildState resets the build state for testing purposes.
// This is only used in tests for CleanupBinary itself.
func resetBuildState() {
	buildOnce = sync.Once{}
	buildBinPath = ""
	buildErr = nil
	buildCacheDir = ""
}
