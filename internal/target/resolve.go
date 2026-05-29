package target

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type BuildTarget struct {
	ModuleRoot string
	ModulePath string
	BuildArg   string
}

func Resolve(moduleRoot, buildTarget string) (BuildTarget, error) {
	if strings.TrimSpace(moduleRoot) == "" {
		return BuildTarget{}, errors.New("module root is required")
	}

	if strings.TrimSpace(buildTarget) == "" {
		buildTarget = "."
	}

	absModuleRoot, err := filepath.Abs(moduleRoot)
	if err != nil {
		return BuildTarget{}, fmt.Errorf("resolve module root: %w", err)
	}

	modulePath, err := readModulePath(filepath.Join(absModuleRoot, "go.mod"))
	if err != nil {
		return BuildTarget{}, err
	}

	buildArg, err := normalizeBuildArg(absModuleRoot, buildTarget)
	if err != nil {
		return BuildTarget{}, err
	}

	return BuildTarget{
		ModuleRoot: absModuleRoot,
		ModulePath: modulePath,
		BuildArg:   buildArg,
	}, nil
}

func resolveEntryPath(moduleRoot, entry string) (string, error) {
	if filepath.IsAbs(entry) {
		return "", fmt.Errorf("entry path must be relative to module root: %s", entry)
	}

	cleanEntry := filepath.Clean(entry)
	joined := filepath.Join(moduleRoot, cleanEntry)
	absEntry, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolve entry path: %w", err)
	}

	relToRoot, err := filepath.Rel(moduleRoot, absEntry)
	if err != nil {
		return "", fmt.Errorf("validate entry path: %w", err)
	}

	if relToRoot == ".." || strings.HasPrefix(relToRoot, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("entry path escapes module root: %s", entry)
	}

	return absEntry, nil
}

func normalizeBuildArg(moduleRoot, buildTarget string) (string, error) {
	normalized := strings.ReplaceAll(strings.TrimSpace(buildTarget), "\\", "/")
	if normalized == "" {
		normalized = "."
	}

	patternSuffix := ""
	if strings.HasSuffix(normalized, "/...") {
		patternSuffix = "/..."
		normalized = strings.TrimSuffix(normalized, "/...")
	} else if normalized == "..." {
		patternSuffix = "/..."
		normalized = "."
	}

	if normalized == "" {
		normalized = "."
	}

	dirPart := normalized
	if dirPart == "." || dirPart == "./" {
		dirPart = "."
	}

	absDir, err := resolveEntryPath(moduleRoot, dirPart)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(absDir)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("package path does not exist: %s", dirPart)
		}
		return "", fmt.Errorf("stat package path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("package path must be a directory or pattern: %s", dirPart)
	}

	relDir, err := filepath.Rel(moduleRoot, absDir)
	if err != nil {
		return "", fmt.Errorf("resolve package path: %w", err)
	}

	buildArg := "."
	if relDir != "." {
		buildArg = "./" + filepath.ToSlash(relDir)
	}

	if patternSuffix != "" {
		if buildArg == "." {
			return "./...", nil
		}
		return buildArg + patternSuffix, nil
	}

	return buildArg, nil
}

func readModulePath(goModPath string) (string, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("go.mod does not exist in module root: %s", goModPath)
		}
		return "", fmt.Errorf("open go.mod: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if !strings.HasPrefix(line, "module ") {
			continue
		}

		modulePath := strings.TrimSpace(strings.TrimPrefix(line, "module "))
		modulePath = strings.Trim(modulePath, "\"")
		if modulePath == "" {
			return "", fmt.Errorf("module path is empty in %s", goModPath)
		}
		return modulePath, nil
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read go.mod: %w", err)
	}

	return "", fmt.Errorf("module path not found in %s", goModPath)
}
