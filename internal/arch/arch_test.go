// Package arch verifica las reglas del molde DTO → validator → service (nv-02).
package arch

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

const (
	internalDir     = ".."
	validatorPkg    = "github.com/go-playground/validator/v10"
	registryFile    = "shared/validation/registry.go"
	httpPackageDir  = "http"
	httpMiddlewares = "http/middleware"
)

var forbiddenInHandlers = []string{
	"go.mongodb.org/mongo-driver",
	"github.com/neo4j/neo4j-go-driver",
}

type sourceFile struct {
	rel  string // relativo a internal/, con "/"
	file *ast.File
}

// sources lee todos los .go de internal/ que no son tests.
func sources(t *testing.T) []sourceFile {
	t.Helper()
	var out []sourceFile
	fset := token.NewFileSet()
	err := filepath.WalkDir(internalDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(internalDir, path)
		if err != nil {
			return err
		}
		out = append(out, sourceFile{rel: filepath.ToSlash(rel), file: f})
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, out)
	return out
}

// isHandler: archivos de rutas de internal/http (no middlewares) y handler.go de cada módulo.
func isHandler(rel string) bool {
	dir, name := filepath.ToSlash(filepath.Dir(rel)), filepath.Base(rel)
	if dir == httpPackageDir {
		return true
	}
	return dir != httpMiddlewares && (name == "handler.go" || strings.HasSuffix(name, "_handler.go"))
}

func handlers(t *testing.T) []sourceFile {
	t.Helper()
	var out []sourceFile
	for _, s := range sources(t) {
		if isHandler(s.rel) {
			out = append(out, s)
		}
	}
	require.NotEmpty(t, out, "no se encontró ningún handler: la regla no estaría probando nada")
	return out
}

func imports(f *ast.File) []string {
	out := make([]string, 0, len(f.Imports))
	for _, imp := range f.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err == nil {
			out = append(out, path)
		}
	}
	return out
}

func TestHandlers_DoNotImportDatabaseDrivers(t *testing.T) {
	for _, h := range handlers(t) {
		for _, path := range imports(h.file) {
			for _, forbidden := range forbiddenInHandlers {
				assert.False(t, strings.HasPrefix(path, forbidden),
					"%s importa %s: el acceso a datos va en el repository", h.rel, path)
			}
		}
	}
}

func TestValidators_RegisteredFromSinglePoint(t *testing.T) {
	for _, s := range sources(t) {
		for _, path := range imports(s.file) {
			if path == validatorPkg {
				assert.Equal(t, registryFile, s.rel, "solo %s usa %s", registryFile, validatorPkg)
			}
		}
		ast.Inspect(s.file, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && strings.HasPrefix(sel.Sel.Name, "RegisterValidation") {
				assert.Equal(t, registryFile, s.rel, "regla registrada fuera de %s", registryFile)
			}
			return true
		})
	}
}

func TestHandlers_HaveNoRecover(t *testing.T) {
	for _, h := range handlers(t) {
		ast.Inspect(h.file, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "recover" {
					assert.Fail(t, "recover en handler", "%s: los pánicos los atrapa la etapa errors", h.rel)
				}
			}
			return true
		})
	}
}

func TestPageResult_SerializesContractFields(t *testing.T) {
	raw, err := json.Marshal(pagination.NewPageResult[string](nil, "", pagination.DefaultLimit))
	require.NoError(t, err)
	assert.JSONEq(t, `{"items":[],"siguiente_cursor":"","limite":20}`, string(raw))

	var keys map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw, &keys))
	assert.Len(t, keys, 3)
}

func validationFields(t *testing.T, err error) []apperr.FieldError {
	t.Helper()
	var appErr *apperr.Error
	require.True(t, errors.As(err, &appErr), "se esperaba *apperr.Error, llegó %v", err)
	assert.Equal(t, 422, appErr.Kind.Status())
	return appErr.Fields
}

func TestPaginationQuery_InvalidCursorIsValidationError(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	q := validation.NewPaginationQuery()
	q.Cursor = "no-es-un-cursor"

	fields := validationFields(t, v.Struct(q))

	assert.Equal(t, []apperr.FieldError{{Field: "cursor", Message: "cursor inválido"}}, fields)
}

func TestPaginationQuery_LimitRange(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	for _, limit := range []int{1, pagination.DefaultLimit, pagination.MaxLimit} {
		q := validation.NewPaginationQuery()
		q.Limit = limit
		assert.NoError(t, v.Struct(q), "limite=%d debe ser válido", limit)
	}
	for _, limit := range []int{0, -1, pagination.MaxLimit + 1, 500} {
		q := validation.NewPaginationQuery()
		q.Limit = limit
		fields := validationFields(t, v.Struct(q))
		require.Len(t, fields, 1, "limite=%d", limit)
		assert.Equal(t, "limite", fields[0].Field)
	}
}
