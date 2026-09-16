package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

var externalTestSuites = map[string]string{
	"services":    "services",
	"integration": "integration",
	"container":   "container",
}

const internalImportPrefix = "github.com/StephenQiu30/then-server/backend/internal/"

var allowedInternalImports = map[string]map[string]bool{
	"domain":      {},
	"application": {"domain": true},
	"adapter":     {"domain": true},
	"bootstrap":   {"adapter": true, "application": true, "platform": true},
	"platform":    {"platform": true},
}

var forbiddenFrameworkImports = map[string][]string{
	"domain":               {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "gorm.io/", "github.com/jackc/pgx", "github.com/rabbitmq/", "github.com/redis/", "github.com/minio/"},
	"application":          {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "gorm.io/", "github.com/jackc/pgx", "github.com/rabbitmq/", "github.com/redis/", "github.com/minio/"},
	"adapter/httpapi":      {"gorm.io/", "github.com/jackc/pgx", "github.com/rabbitmq/", "github.com/redis/", "github.com/minio/"},
	"adapter/postgres":     {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "github.com/rabbitmq/", "github.com/redis/", "github.com/minio/"},
	"adapter/objectstore":  {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "gorm.io/", "github.com/jackc/pgx", "github.com/rabbitmq/", "github.com/redis/"},
	"adapter/messagequeue": {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "gorm.io/", "github.com/jackc/pgx", "github.com/redis/", "github.com/minio/"},
}

func TestInternalPackageDependencyDirection(t *testing.T) {
	t.Helper()
	err := filepath.WalkDir("internal", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		layer := internalLayer(path)
		if _, exists := allowedInternalImports[layer]; !exists {
			t.Errorf("%s belongs to unregistered internal layer %q", path, layer)
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			t.Errorf("parse %s: %v", path, err)
			return nil
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				t.Errorf("parse import in %s: %v", path, err)
				continue
			}
			checkInternalImport(t, path, layer, importPath)
			checkFrameworkImport(t, path, internalComponent(path), importPath)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCommandEntrypointIsThin(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join("cmd", "main.go"), nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(importPath, internalImportPrefix) && importPath != internalImportPrefix+"bootstrap" {
			t.Errorf("cmd/main.go must delegate only to internal/bootstrap, imported %s", importPath)
		}
	}
}

func TestProductionSourcesDoNotImportTestPackages(t *testing.T) {
	t.Helper()
	forbidden := []string{"testing", "net/http/httptest", "github.com/testcontainers/"}
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path == "tests" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, spec := range file.Imports {
			importPath, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return err
			}
			for _, prefix := range forbidden {
				if importPath == prefix || strings.HasPrefix(importPath, prefix) {
					t.Errorf("%s: production source must not import test package %s", path, importPath)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestExternalTestSuiteLayout(t *testing.T) {
	entries, err := os.ReadDir("tests")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".go" {
			t.Errorf("tests/%s: place external tests in a named suite directory", entry.Name())
		}
	}
	for directory, tag := range externalTestSuites {
		root := filepath.Join("tests", directory)
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || filepath.Ext(path) != ".go" {
				return nil
			}
			if !strings.HasSuffix(path, "_test.go") {
				t.Errorf("%s: suite source must be a _test.go file", path)
			}
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if !strings.Contains(string(contents), "//go:build "+tag) {
				t.Errorf("%s: suite must use %q build tag", path, tag)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func internalLayer(path string) string {
	relative, err := filepath.Rel("internal", path)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func internalComponent(path string) string {
	relative, err := filepath.Rel("internal", path)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	if len(parts) > 1 && parts[0] == "adapter" {
		return parts[0] + "/" + parts[1]
	}
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func checkInternalImport(t *testing.T, source, sourceLayer, importPath string) {
	t.Helper()
	if !strings.HasPrefix(importPath, internalImportPrefix) {
		return
	}
	destination := strings.Split(strings.TrimPrefix(importPath, internalImportPrefix), "/")[0]
	if sourceLayer == destination || allowedInternalImports[sourceLayer][destination] {
		return
	}
	t.Errorf("%s: %s layer must not import internal/%s", source, sourceLayer, destination)
}

func checkFrameworkImport(t *testing.T, source, component, importPath string) {
	t.Helper()
	for _, prefix := range forbiddenFrameworkImports[component] {
		if strings.HasPrefix(importPath, prefix) {
			t.Errorf("%s: %s must not import %s", source, component, importPath)
		}
	}
}
