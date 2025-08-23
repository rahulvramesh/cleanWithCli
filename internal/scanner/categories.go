package scanner

import (
	"os"
	"path/filepath"

	"github.com/rahulvramesh/cleanWithCli/internal/types"
	"github.com/rahulvramesh/cleanWithCli/internal/utils"
)

// ScanXcodeFiles scans Xcode build artifacts
func (s *Scanner) ScanXcodeFiles() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Xcode Files",
		Items:    []types.FileItem{},
	}

	xcodeDirs := []string{
		filepath.Join(s.HomeDir, "Library", "Developer", "Xcode", "DerivedData"),
		filepath.Join(s.HomeDir, "Library", "Developer", "Xcode", "Archives"),
		filepath.Join(s.HomeDir, "Library", "Developer", "CoreSimulator", "Devices"),
	}

	for _, dir := range xcodeDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			path := filepath.Join(dir, entry.Name())
			size, _ := utils.GetDirSize(path)
			if size > 0 {
				result.Items = append(result.Items, types.FileItem{
					Path: path,
					Size: size,
					Name: "Xcode: " + entry.Name(),
				})
				result.Total += size
			}
		}
	}

	return result
}

// ScanBrewCache scans Homebrew cache
func (s *Scanner) ScanBrewCache() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Homebrew Cache",
		Items:    []types.FileItem{},
	}

	brewCache := filepath.Join(s.HomeDir, "Library", "Caches", "Homebrew")
	if _, err := os.Stat(brewCache); err != nil {
		return result
	}

	entries, err := os.ReadDir(brewCache)
	if err != nil {
		return result
	}

	for _, entry := range entries {
		path := filepath.Join(brewCache, entry.Name())
		size, _ := utils.GetDirSize(path)
		result.Items = append(result.Items, types.FileItem{
			Path: path,
			Size: size,
			Name: "Brew: " + entry.Name(),
		})
		result.Total += size
	}

	return result
}

// ScanGoArtifacts scans Go build artifacts and module cache
func (s *Scanner) ScanGoArtifacts() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Go Artifacts",
		Items:    []types.FileItem{},
	}

	// Go module cache
	goPath := os.Getenv("GOPATH")
	if goPath == "" {
		goPath = filepath.Join(s.HomeDir, "go")
	}

	goCaches := []string{
		filepath.Join(goPath, "pkg", "mod"),
		filepath.Join(s.HomeDir, ".cache", "go-build"),
		filepath.Join(s.HomeDir, "Library", "Caches", "go-build"),
	}

	for _, dir := range goCaches {
		if _, err := os.Stat(dir); err == nil {
			size, _ := utils.GetDirSize(dir)
			if size > 0 {
				result.Items = append(result.Items, types.FileItem{
					Path: dir,
					Size: size,
					Name: "Go: " + filepath.Base(dir),
				})
				result.Total += size
			}
		}
	}

	return result
}

// ScanDockerArtifacts scans Docker artifacts
func (s *Scanner) ScanDockerArtifacts() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Docker Artifacts",
		Items:    []types.FileItem{},
	}

	// Docker Desktop data
	dockerData := filepath.Join(s.HomeDir, "Library", "Containers", "com.docker.docker", "Data")
	if _, err := os.Stat(dockerData); err == nil {
		size, _ := utils.GetDirSize(dockerData)
		if size > 100*1024*1024 { // Only if > 100MB
			result.Items = append(result.Items, types.FileItem{
				Path: dockerData,
				Size: size,
				Name: "Docker: Desktop Data",
			})
			result.Total += size
		}
	}

	return result
}

// ScanIDECaches scans IDE cache directories
func (s *Scanner) ScanIDECaches() *types.ScanResult {
	result := &types.ScanResult{
		Category: "IDE Caches",
		Items:    []types.FileItem{},
	}

	// VS Code
	vscodeDirs := []string{
		filepath.Join(s.HomeDir, "Library", "Application Support", "Code", "Cache"),
		filepath.Join(s.HomeDir, "Library", "Application Support", "Code", "CachedData"),
		filepath.Join(s.HomeDir, ".vscode", "extensions"),
	}

	for _, dir := range vscodeDirs {
		if _, err := os.Stat(dir); err == nil {
			size, _ := utils.GetDirSize(dir)
			if size > 0 {
				result.Items = append(result.Items, types.FileItem{
					Path: dir,
					Size: size,
					Name: "VS Code: " + filepath.Base(dir),
				})
				result.Total += size
			}
		}
	}

	// JetBrains IDEs
	jetbrainsDirs := []string{
		filepath.Join(s.HomeDir, "Library", "Caches", "JetBrains"),
		filepath.Join(s.HomeDir, "Library", "Application Support", "JetBrains"),
	}

	for _, dir := range jetbrainsDirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				path := filepath.Join(dir, entry.Name())
				size, _ := utils.GetDirSize(path)
				if size > 0 {
					result.Items = append(result.Items, types.FileItem{
						Path: path,
						Size: size,
						Name: "JetBrains: " + entry.Name(),
					})
					result.Total += size
				}
			}
		}
	}

	return result
}

// ScanJavaArtifacts scans Java/JVM artifacts
func (s *Scanner) ScanJavaArtifacts() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Java/JVM Artifacts",
		Items:    []types.FileItem{},
	}

	// Maven cache
	m2Repo := filepath.Join(s.HomeDir, ".m2", "repository")
	if _, err := os.Stat(m2Repo); err == nil {
		size, _ := utils.GetDirSize(m2Repo)
		if size > 0 {
			result.Items = append(result.Items, types.FileItem{
				Path: m2Repo,
				Size: size,
				Name: "Maven: .m2 repository",
			})
			result.Total += size
		}
	}

	// Gradle cache
	gradleCache := filepath.Join(s.HomeDir, ".gradle", "caches")
	if _, err := os.Stat(gradleCache); err == nil {
		size, _ := utils.GetDirSize(gradleCache)
		if size > 0 {
			result.Items = append(result.Items, types.FileItem{
				Path: gradleCache,
				Size: size,
				Name: "Gradle: caches",
			})
			result.Total += size
		}
	}

	return result
}

// ScanNpmYarnCaches scans NPM, Yarn, and PNPM caches
func (s *Scanner) ScanNpmYarnCaches() *types.ScanResult {
	result := &types.ScanResult{
		Category: "NPM/Yarn/PNPM Caches",
		Items:    []types.FileItem{},
	}

	nodeCaches := []struct {
		path string
		name string
	}{
		{filepath.Join(s.HomeDir, ".npm"), "NPM cache"},
		{filepath.Join(s.HomeDir, "Library", "Caches", "npm"), "NPM cache (Library)"},
		{filepath.Join(s.HomeDir, ".yarn", "cache"), "Yarn cache"},
		{filepath.Join(s.HomeDir, "Library", "Caches", "Yarn"), "Yarn cache (Library)"},
		{filepath.Join(s.HomeDir, ".pnpm-store"), "PNPM store"},
	}

	for _, cache := range nodeCaches {
		if _, err := os.Stat(cache.path); err == nil {
			size, _ := utils.GetDirSize(cache.path)
			if size > 0 {
				result.Items = append(result.Items, types.FileItem{
					Path: cache.path,
					Size: size,
					Name: cache.name,
				})
				result.Total += size
			}
		}
	}

	return result
}

// ScanRubyArtifacts scans Ruby gems and caches
func (s *Scanner) ScanRubyArtifacts() *types.ScanResult {
	result := &types.ScanResult{
		Category: "Ruby Artifacts",
		Items:    []types.FileItem{},
	}

	// Ruby gems
	gemHome := os.Getenv("GEM_HOME")
	if gemHome == "" {
		gemHome = filepath.Join(s.HomeDir, ".gem")
	}

	if _, err := os.Stat(gemHome); err == nil {
		size, _ := utils.GetDirSize(gemHome)
		if size > 0 {
			result.Items = append(result.Items, types.FileItem{
				Path: gemHome,
				Size: size,
				Name: "Ruby: Gem cache",
			})
			result.Total += size
		}
	}

	// Bundler
	bundleCache := filepath.Join(s.HomeDir, ".bundle", "cache")
	if _, err := os.Stat(bundleCache); err == nil {
		size, _ := utils.GetDirSize(bundleCache)
		if size > 0 {
			result.Items = append(result.Items, types.FileItem{
				Path: bundleCache,
				Size: size,
				Name: "Ruby: Bundler cache",
			})
			result.Total += size
		}
	}

	return result
}

// ScanCocoaPods scans CocoaPods cache
func (s *Scanner) ScanCocoaPods() *types.ScanResult {
	result := &types.ScanResult{
		Category: "CocoaPods",
		Items:    []types.FileItem{},
	}

	cocoapodsCache := filepath.Join(s.HomeDir, "Library", "Caches", "CocoaPods")
	if _, err := os.Stat(cocoapodsCache); err == nil {
		size, _ := utils.GetDirSize(cocoapodsCache)
		if size > 0 {
			result.Items = append(result.Items, types.FileItem{
				Path: cocoapodsCache,
				Size: size,
				Name: "CocoaPods cache",
			})
			result.Total += size
		}
	}

	return result
}
