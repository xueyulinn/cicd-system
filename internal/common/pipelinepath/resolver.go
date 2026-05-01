package pipelinepath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xueyulinn/cicd-system/internal/config"
)

// ResolveInputPath normalizes a validate/dryrun input path so file inputs
// resolve under the repository's .pipelines directory while directory inputs
// only allow the .pipelines root itself.
func ResolveInputPath(rootDir, inputPath string) (string, os.FileInfo, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", nil, fmt.Errorf("failed to get current working directory: %w", err)
	}

	return resolveInputPathFromWorkingDir(rootDir, cwd, inputPath)
}

func resolveInputPathFromWorkingDir(rootDir, workingDir, inputPath string) (string, os.FileInfo, error) {
	pipelineRoot := filepath.Join(rootDir, config.DefaultPipelineDir)

	cleanInput := filepath.Clean(strings.TrimSpace(inputPath))
	if strings.TrimSpace(inputPath) == "" {
		return "", nil, fmt.Errorf("pipeline path must not be empty")
	}

	absoluteInputPath := cleanInput
	if !filepath.IsAbs(cleanInput) {
		if isPipelineRootRelativePath(cleanInput) {
			absoluteInputPath = filepath.Join(rootDir, cleanInput)
		} else {
			absoluteInputPath = filepath.Join(workingDir, cleanInput)
		}
	}
	absoluteInputPath = filepath.Clean(absoluteInputPath)

	repoRelativeInput, err := toRepoRelativePipelinePath(rootDir, absoluteInputPath)
	if err != nil {
		return "", nil, err
	}

	explicitRepoPath := filepath.Clean(filepath.Join(rootDir, repoRelativeInput))
	explicitInfo, explicitErr := os.Stat(explicitRepoPath)
	if explicitErr == nil {
		if repoRelativeInput == config.DefaultPipelineDir {
			if !explicitInfo.IsDir() {
				return "", nil, fmt.Errorf("pipeline directory %q is not a directory", pipelineRoot)
			}
			return pipelineRoot, explicitInfo, nil
		}

		if isPathWithinDir(pipelineRoot, explicitRepoPath) {
			if explicitInfo.IsDir() {
				return "", nil, fmt.Errorf("pipeline directory must be %q", config.DefaultPipelineDir)
			}
			return explicitRepoPath, explicitInfo, nil
		}

		if explicitInfo.IsDir() {
			return "", nil, fmt.Errorf("pipeline directory must be %q", config.DefaultPipelineDir)
		}

		return "", nil, fmt.Errorf("pipeline file must be inside %q: %s", config.DefaultPipelineDir, inputPath)
	}
	if !os.IsNotExist(explicitErr) {
		return "", nil, fmt.Errorf("failed to get the info of path %q: %w", explicitRepoPath, explicitErr)
	}

	if repoRelativeInput == config.DefaultPipelineDir || isPathWithinDir(pipelineRoot, explicitRepoPath) {
		return "", nil, fmt.Errorf("failed to get the info of path %q: %w", explicitRepoPath, explicitErr)
	}

	if filepath.IsAbs(cleanInput) {
		return "", nil, fmt.Errorf("pipeline file must be inside %q: %s", config.DefaultPipelineDir, inputPath)
	}
	if pathEscapesParent(cleanInput) {
		return "", nil, fmt.Errorf("pipeline file must be inside %q: %s", config.DefaultPipelineDir, inputPath)
	}

	finalPath := filepath.Clean(filepath.Join(pipelineRoot, cleanInput))
	if !isPathWithinDir(pipelineRoot, finalPath) {
		return "", nil, fmt.Errorf("pipeline path escapes %q: %s", config.DefaultPipelineDir, inputPath)
	}

	info, err := statPipelineTarget(finalPath)
	if err != nil {
		return "", nil, err
	}
	if info.IsDir() {
		return "", nil, fmt.Errorf("pipeline directory must be %q", config.DefaultPipelineDir)
	}

	return finalPath, info, nil
}

func toRepoRelativePipelinePath(rootDir, inputPath string) (string, error) {
	rel, err := filepath.Rel(rootDir, inputPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve pipeline path %q relative to repository root: %w", inputPath, err)
	}
	rel = filepath.Clean(rel)
	if pathEscapesParent(rel) {
		return "", fmt.Errorf("pipeline path must stay within the repository: %s", inputPath)
	}

	return rel, nil
}

func statPipelineTarget(path string) (os.FileInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to get the info of path %q: %w", path, err)
	}
	return info, nil
}

func isPathWithinDir(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	rel = filepath.Clean(rel)
	return rel == "." || !pathEscapesParent(rel)
}

func pathEscapesParent(path string) bool {
	return path == ".." || strings.HasPrefix(path, ".."+string(os.PathSeparator))
}

func isPipelineRootRelativePath(path string) bool {
	return path == config.DefaultPipelineDir || strings.HasPrefix(path, config.DefaultPipelineDir+string(os.PathSeparator))
}
