package tester

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/auto-code-os/auto-code-os/server/internal/orchestrator/workspace"
)

type ProjectKind string

const (
	ProjectUnknown ProjectKind = ""
	ProjectGo      ProjectKind = "go"
	ProjectJS      ProjectKind = "javascript"
	ProjectPython  ProjectKind = "python"
	ProjectJava    ProjectKind = "java"
)

type projectProfile struct {
	kind    ProjectKind
	markers []string
	exts    []string
}

var projectProfiles = []projectProfile{
	{kind: ProjectGo, markers: []string{"go.mod"}, exts: []string{".go"}},
	{kind: ProjectJS, markers: []string{"package.json"}, exts: []string{".ts", ".tsx", ".js", ".jsx"}},
	{kind: ProjectPython, markers: []string{"requirements.txt", "pyproject.toml", "pytest.ini"}, exts: []string{".py"}},
	{kind: ProjectJava, markers: []string{"pom.xml", "build.gradle"}, exts: []string{".java"}},
}

func kindForExtension(ext string) (ProjectKind, []string) {
	for _, profile := range projectProfiles {
		for _, candidate := range profile.exts {
			if ext == candidate {
				return profile.kind, profile.markers
			}
		}
	}
	return ProjectUnknown, nil
}

func DetectProjectKind(dir string) ProjectKind {
	for _, profile := range projectProfiles {
		for _, marker := range profile.markers {
			if stat, err := os.Stat(filepath.Join(dir, marker)); err == nil && !stat.IsDir() {
				return profile.kind
			}
		}
	}
	return ProjectUnknown
}

func DetectProjectKindNear(root string, relFile string) (ProjectKind, []string) {
	ext := filepath.Ext(relFile)
	if kind, markers := kindForExtension(ext); kind != ProjectUnknown {
		return kind, markers
	}
	kind := DetectProjectKind(filepath.Join(root, filepath.Dir(relFile)))
	if kind == ProjectUnknown {
		return ProjectUnknown, nil
	}
	for _, profile := range projectProfiles {
		if profile.kind == kind {
			return kind, profile.markers
		}
	}
	return ProjectUnknown, nil
}

func FullVerificationScript() string {
	script := `
run_verification() {
	local dir="$1"
	echo "Verifying repository in $dir..."
	cd "$dir" || return 1

	if [ -f go.mod ]; then
		go test ./... || return 1
	elif [ -f package.json ]; then
		npm test || return 1
	elif [[ -f requirements.txt || -f pyproject.toml || -f pytest.ini ]]; then
		pytest || return 1
	elif [[ -f pom.xml || -f build.gradle ]]; then
		if [ -f pom.xml ]; then
			mvn test || return 1
		else
			./gradlew test || gradle test || return 1
		fi
	fi

	local lint_ran=0
	if [ -f .golangci.yml ] && command -v golangci-lint >/dev/null 2>&1; then
		golangci-lint run || return 1
		lint_ran=1
	fi
	if [ -f package.json ] && grep -q '"lint"' package.json; then
		npm run lint || return 1
		lint_ran=1
	fi
	if [ $lint_ran -eq 1 ]; then
		echo "LINT_STATUS: PASSED"
	else
		echo "LINT_STATUS: NOT_CONFIGURED"
	fi

	local build_ran=0
	if [ -f go.mod ]; then
		go build ./... || return 1
		build_ran=1
	elif [ -f package.json ] && grep -q '"build"' package.json; then
		npm run build || return 1
		build_ran=1
	elif [ -f pom.xml ]; then
		mvn compile || return 1
		build_ran=1
	elif [ -f build.gradle ]; then
		./gradlew classes || gradle classes || return 1
		build_ran=1
	fi
	if [ $build_ran -eq 1 ]; then
		echo "BUILD_STATUS: PASSED"
	else
		echo "BUILD_STATUS: NOT_CONFIGURED"
	fi
}

is_test_project_dir() {
	local dir="$1"
	[ -d "$dir/.git" ] || [ -f "$dir/go.mod" ] || [ -f "$dir/package.json" ] || [ -f "$dir/requirements.txt" ] || [ -f "$dir/pyproject.toml" ] || [ -f "$dir/pytest.ini" ] || [ -f "$dir/pom.xml" ] || [ -f "$dir/build.gradle" ]
}

found_repos=0
for r in REPOS_DIR/* ; do
	if [ -d "$r" ]; then
		for d in "$r"/* ; do
			if [ -d "$d" ] && [ "$(basename "$d")" != "worktrees" ]; then
				(run_verification "$d") || exit 1
				found_repos=1
			fi
		done
	fi
done

if [ $found_repos -eq 0 ]; then
	if is_test_project_dir "."; then
		run_verification "." || exit 1
	else
		for d in */ ; do
			d_clean="${d%/}"
			if is_test_project_dir "$d_clean"; then
				(run_verification "$d_clean") || exit 1
			fi
		done
	fi
fi
`
	return strings.ReplaceAll(script, "REPOS_DIR", workspace.ReposDirName)
}

func TargetedTestCommand(kind ProjectKind, containerModPath string, files []string, goPackages map[string]bool) (string, bool) {
	quotedModPath := workspace.QuoteShellArg(containerModPath)
	switch kind {
	case ProjectGo:
		pkgs := []string{}
		for pkg := range goPackages {
			pkgs = append(pkgs, pkg+"/...")
		}
		if len(pkgs) == 0 {
			pkgs = append(pkgs, "./...")
		}
		return fmt.Sprintf("cd %s && go test -v %s", quotedModPath, strings.Join(pkgs, " ")), true
	case ProjectJS:
		var quotedFiles []string
		for _, f := range files {
			quotedFiles = append(quotedFiles, workspace.QuoteShellArg(f))
		}
		return fmt.Sprintf("cd %s && (npm test -- --findRelatedTests %s || npm test -- %s || npm test)", quotedModPath, strings.Join(quotedFiles, " "), strings.Join(quotedFiles, " ")), true
	case ProjectPython:
		return fmt.Sprintf("cd %s && pytest", quotedModPath), true
	case ProjectJava:
		return fmt.Sprintf("cd %s && if [ -f pom.xml ]; then mvn test; else ./gradlew test || gradle test; fi", quotedModPath), true
	default:
		return "", false
	}
}
