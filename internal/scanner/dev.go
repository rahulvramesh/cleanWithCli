package scanner

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/rahulvramesh/cleanWithCli/internal/types"
	"github.com/rahulvramesh/cleanWithCli/internal/utils"
)

// ScanNodeModules scans for node_modules directories
func (s *Scanner) ScanNodeModules() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Node Modules",
		Items:    []types.FileItem{},
	}

	// Deep scan entire home directory
	filepath.WalkDir(s.HomeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if utils.ShouldSkipDir(path) {
				return filepath.SkipDir
			}

			if d.Name() == "node_modules" {
				size, _ := utils.GetDirSize(path)
				if size > 0 {
					// Get project path for better context
					projectPath := filepath.Dir(path)
					relPath, _ := filepath.Rel(s.HomeDir, projectPath)
					result.Items = append(result.Items, types.FileItem{
						Path:  path,
						Size:  size,
						Name:  fmt.Sprintf("📦 %s", relPath),
						IsDir: true,
					})
					result.Total += size
				}
				return filepath.SkipDir
			}
		}

		return nil
	})

	return result
}

// ScanPythonArtifacts scans Python virtual environments and caches
func (s *Scanner) ScanPythonArtifacts() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Python Artifacts",
		Items:    []types.FileItem{},
	}

	// Python cache directories
	pythonCaches := []string{
		filepath.Join(s.HomeDir, ".cache", "pip"),
		filepath.Join(s.HomeDir, "Library", "Caches", "pip"),
		filepath.Join(s.HomeDir, ".conda", "pkgs"),
	}

	// Add Python cache directories
	for _, dir := range pythonCaches {
		if _, err := os.Stat(dir); err == nil {
			size, _ := utils.GetDirSize(dir)
			if size > 0 {
				result.Items = append(result.Items, types.FileItem{
					Path: dir,
					Size: size,
					Name: "Python: " + filepath.Base(dir) + " cache",
				})
				result.Total += size
			}
		}
	}

	// Deep scan for Python virtual environments and caches
	filepath.WalkDir(s.HomeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if utils.ShouldSkipDir(path) {
				return filepath.SkipDir
			}

			name := d.Name()
			if name == "__pycache__" || name == "venv" || name == ".venv" ||
				name == "env" || name == ".env" || name == "virtualenv" ||
				name == ".pytest_cache" || name == ".tox" || name == ".mypy_cache" {
				size, _ := utils.GetDirSize(path)
				if size > 0 {
					projectPath := filepath.Dir(path)
					relPath, _ := filepath.Rel(s.HomeDir, projectPath)
					result.Items = append(result.Items, types.FileItem{
						Path:  path,
						Size:  size,
						Name:  fmt.Sprintf("🐍 %s (%s)", relPath, name),
						IsDir: true,
					})
					result.Total += size
				}
				return filepath.SkipDir
			}
		}

		return nil
	})

	return result
}

// ScanRustArtifacts scans Rust target directories and Cargo caches
func (s *Scanner) ScanRustArtifacts() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Rust Artifacts",
		Items:    []types.FileItem{},
	}

	// Cargo cache
	cargoHome := os.Getenv("CARGO_HOME")
	if cargoHome == "" {
		cargoHome = filepath.Join(s.HomeDir, ".cargo")
	}

	registryCache := filepath.Join(cargoHome, "registry", "cache")
	if _, err := os.Stat(registryCache); err == nil {
		size, _ := utils.GetDirSize(registryCache)
		if size > 0 {
			result.Items = append(result.Items, types.FileItem{
				Path:  registryCache,
				Size:  size,
				Name:  "🦀 Cargo registry cache",
				IsDir: true,
			})
			result.Total += size
		}
	}

	// Deep scan for Rust target directories
	filepath.WalkDir(s.HomeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if utils.ShouldSkipDir(path) {
				return filepath.SkipDir
			}

			if d.Name() == "target" {
				// Check if it's a Rust project (has Cargo.toml in parent)
				if _, err := os.Stat(filepath.Join(filepath.Dir(path), "Cargo.toml")); err == nil {
					size, _ := utils.GetDirSize(path)
					if size > 0 {
						projectPath := filepath.Dir(path)
						relPath, _ := filepath.Rel(s.HomeDir, projectPath)
						result.Items = append(result.Items, types.FileItem{
							Path:  path,
							Size:  size,
							Name:  fmt.Sprintf("🦀 %s", relPath),
							IsDir: true,
						})
						result.Total += size
					}
					return filepath.SkipDir
				}
			}
		}

		return nil
	})

	return result
}

// ScanBuildArtifacts scans build directories and artifacts
func (s *Scanner) ScanBuildArtifacts() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Build Artifacts",
		Items:    []types.FileItem{},
	}

	// Deep scan for various build directories
	filepath.WalkDir(s.HomeDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			if utils.ShouldSkipDir(path) {
				return filepath.SkipDir
			}

			name := d.Name()
			// Check for common build directories
			if name == "dist" || name == "build" || name == "out" ||
				name == ".next" || name == ".nuxt" || name == ".output" ||
				name == "coverage" || name == ".nyc_output" || name == ".parcel-cache" ||
				name == "tmp" || name == "temp" {
				// Check if it's likely a project build dir (has package.json, Cargo.toml, etc. in parent)
				parentDir := filepath.Dir(path)
				if utils.IsProjectDir(parentDir) {
					size, _ := utils.GetDirSize(path)
					if size > 0 {
						relPath, _ := filepath.Rel(s.HomeDir, parentDir)
						result.Items = append(result.Items, types.FileItem{
							Path:  path,
							Size:  size,
							Name:  fmt.Sprintf("🔨 %s (%s)", relPath, name),
							IsDir: true,
						})
						result.Total += size
					}
					return filepath.SkipDir
				}
			}
		}

		return nil
	})

	return result
}
