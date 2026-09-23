package main

import (
	"go/ast"
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
	"application": {},
	"adapter":     {"application": true},
	"bootstrap":   {"adapter": true, "application": true, "platform": true},
	"platform":    {"platform": true},
}

var allowedApplicationImports = map[string]map[string]bool{
	"account":        {},
	"privacy":        {"account": true},
	"wardrobe":       {"account": true},
	"outfitplan":     {"account": true, "wardrobe": true},
	"outfitfeedback": {"account": true},
	"wearevent":      {"account": true, "outfitplan": true},
	"diary":          {"account": true},
	"community":      {"account": true},
	"media":          {"account": true},
	"eventworker":    {"media": true},
}

var forbiddenFrameworkImports = map[string][]string{
	"application":          {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "gorm.io/", "github.com/jackc/pgx", "github.com/twmb/franz-go", "github.com/redis/", "github.com/minio/"},
	"adapter/httpapi":      {"gorm.io/", "github.com/jackc/pgx", "github.com/twmb/franz-go", "github.com/redis/", "github.com/minio/"},
	"adapter/postgres":     {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "github.com/twmb/franz-go", "github.com/redis/", "github.com/minio/"},
	"adapter/objectstore":  {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "gorm.io/", "github.com/jackc/pgx", "github.com/twmb/franz-go", "github.com/redis/"},
	"adapter/messagequeue": {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "gorm.io/", "github.com/jackc/pgx", "github.com/redis/", "github.com/minio/"},
}

func TestApplicationPackageDependencyDirection(t *testing.T) {
	root := filepath.Join("internal", "application")
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		source := strings.Split(filepath.ToSlash(relative), "/")[0]
		allowed, exists := allowedApplicationImports[source]
		if !exists {
			t.Errorf("%s belongs to unregistered application package %q", path, source)
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
			const prefix = internalImportPrefix + "application/"
			if !strings.HasPrefix(importPath, prefix) {
				continue
			}
			destination := strings.Split(strings.TrimPrefix(importPath, prefix), "/")[0]
			if destination != source && !allowed[destination] {
				t.Errorf("%s: application/%s must not import application/%s", path, source, destination)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
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
	command := "main.go"
	file, err := parser.ParseFile(token.NewFileSet(), command, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	allowed := map[string]bool{"log/slog": true, "os": true, internalImportPrefix + "bootstrap": true}
	for _, spec := range file.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			t.Fatal(err)
		}
		if !allowed[importPath] {
			t.Errorf("%s only creates the logger and delegates to bootstrap, imported %s", command, importPath)
		}
	}
}

func TestProductionSourcesDoNotImportTestPackages(t *testing.T) {
	t.Helper()
	forbidden := []string{"testing", "net/http/httptest", "github.com/testcontainers/", strings.TrimSuffix(internalImportPrefix, "internal/") + "tests/"}
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
		if entry.IsDir() && entry.Name() != "internal" {
			if _, ok := externalTestSuites[entry.Name()]; !ok {
				t.Errorf("tests/%s: unregistered test suite", entry.Name())
			}
		}
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
	if sourceLayer == "adapter" && destination == "adapter" {
		parts := strings.Split(strings.TrimPrefix(importPath, internalImportPrefix), "/")
		if len(parts) < 2 || strings.Join(parts[:2], "/") != internalComponent(source) {
			t.Errorf("%s: adapters must be assembled by bootstrap, imported %s", source, importPath)
		}
		return
	}
	if sourceLayer == destination || allowedInternalImports[sourceLayer][destination] {
		return
	}
	t.Errorf("%s: %s layer must not import internal/%s", source, sourceLayer, destination)
}

func TestProductionDirectoryLayout(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() == "go.mod" && path != "go.mod" {
			t.Errorf("%s: backend must have one Go module", path)
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if path == "main.go" {
			return nil
		}
		parts := strings.Split(filepath.ToSlash(path), "/")
		if len(parts) < 2 || (parts[0] != "internal" && parts[0] != "tests") {
			t.Errorf("%s: production code belongs in internal; main.go is the only root entry", path)
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.PackageClauseOnly)
		if err != nil {
			return err
		}
		if file.Name.Name == "main" {
			t.Errorf("%s: main.go is the only executable package", path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestHTTPFileResponsibilities(t *testing.T) {
	paths, err := filepath.Glob("internal/adapter/httpapi/*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			if strings.HasSuffix(path, "_routes.go") {
				switch decl := declaration.(type) {
				case *ast.GenDecl:
					if decl.Tok == token.TYPE {
						t.Errorf("%s: DTOs belong in contract files, handler types in handlers", path)
					}
				case *ast.FuncDecl:
					if decl.Recv != nil {
						t.Errorf("%s: receiver methods belong with their contract or handler", path)
					}
				}
			}
			if strings.HasSuffix(path, "_contract.go") {
				if decl, ok := declaration.(*ast.FuncDecl); ok && (decl.Recv == nil || decl.Name.Name != "Schema") {
					t.Errorf("%s: contract files only contain DTOs and schema methods", path)
				}
				if decl, ok := declaration.(*ast.GenDecl); ok && decl.Tok == token.TYPE {
					for _, spec := range decl.Specs {
						typ := spec.(*ast.TypeSpec)
						_, port := typ.Type.(*ast.InterfaceType)
						if port || strings.HasSuffix(typ.Name.Name, "Handler") {
							t.Errorf("%s: handler types and consumed ports belong in handlers", path)
						}
					}
				}
			}
		}
	}
}

func checkFrameworkImport(t *testing.T, source, component, importPath string) {
	t.Helper()
	for _, prefix := range forbiddenFrameworkImports[component] {
		if strings.HasPrefix(importPath, prefix) {
			t.Errorf("%s: %s must not import %s", source, component, importPath)
		}
	}
}
