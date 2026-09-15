package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

const internalImportPrefix = "github.com/StephenQiu30/then-server/backend/internal/"

var allowedInternalImports = map[string]map[string]bool{
	"model":      {},
	"service":    {"model": true},
	"repository": {"model": true},
	"transport":  {"model": true},
	"platform":   {"platform": true},
}

var forbiddenFrameworkImports = map[string][]string{
	"model":      {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "gorm.io/", "github.com/jackc/pgx", "github.com/rabbitmq/", "github.com/redis/", "github.com/minio/"},
	"service":    {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "gorm.io/", "github.com/jackc/pgx", "github.com/rabbitmq/", "github.com/redis/", "github.com/minio/"},
	"repository": {"github.com/gin-gonic/gin", "github.com/danielgtaylor/huma", "github.com/rabbitmq/", "github.com/redis/", "github.com/minio/"},
	"transport":  {"gorm.io/", "github.com/jackc/pgx", "github.com/rabbitmq/", "github.com/redis/", "github.com/minio/"},
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
			checkFrameworkImport(t, path, layer, importPath)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
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

func checkFrameworkImport(t *testing.T, source, layer, importPath string) {
	t.Helper()
	for _, prefix := range forbiddenFrameworkImports[layer] {
		if strings.HasPrefix(importPath, prefix) {
			t.Errorf("%s: %s layer must not import %s", source, layer, importPath)
		}
	}
}
