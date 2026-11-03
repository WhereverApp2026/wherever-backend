package contract

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var snakeCase = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// contract es lo que se captura de una respuesta: el código y la forma del cuerpo.
type contract struct {
	Status int `json:"status"`
	Body   any `json:"body"`
}

// shape cambia cada valor por su tipo; las listas se describen por su primer elemento.
func shape(t *testing.T, v any) any {
	t.Helper()
	switch val := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, nested := range val {
			assert.Regexp(t, snakeCase, k, "llave fuera de snake_case")
			out[k] = shape(t, nested)
		}
		return out
	case []any:
		if len(val) == 0 {
			return []any{}
		}
		return []any{shape(t, val[0])}
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "bool"
	case nil:
		return "null"
	default:
		return "desconocido"
	}
}

func call(t *testing.T, app *fiber.App, method, path string, body any, token string) (int, any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	if token != "" {
		req.Header.Set(fiber.HeaderAuthorization, "Bearer "+token)
	}
	resp, err := app.Test(req, -1)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var parsed any
	if len(raw) > 0 {
		require.NoError(t, json.Unmarshal(raw, &parsed), string(raw))
	}
	return resp.StatusCode, parsed
}

// check compara (o captura con CONTRACT_CAPTURE=1) el contrato en testdata/<nombre>.json.
func check(t *testing.T, name string, status int, body any) {
	t.Helper()
	got := contract{Status: status, Body: shape(t, body)}
	path := filepath.Join("testdata", name+".json")
	if os.Getenv("CONTRACT_CAPTURE") == "1" {
		raw, err := json.MarshalIndent(got, "", "  ")
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o750))
		require.NoError(t, os.WriteFile(path, append(raw, '\n'), 0o600))
		return
	}
	raw, err := os.ReadFile(path) // #nosec G304 -- ruta armada con nombres fijos de las pruebas
	require.NoError(t, err, "falta la captura %s; correr con CONTRACT_CAPTURE=1", path)
	var want contract
	require.NoError(t, json.Unmarshal(raw, &want))
	gotRaw, err := json.Marshal(got)
	require.NoError(t, err)
	wantRaw, err := json.Marshal(want)
	require.NoError(t, err)
	assert.JSONEq(t, string(wantRaw), string(gotRaw), name)
}

// keys devuelve las llaves ordenadas, para afirmar el conjunto exacto de campos.
func keys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Afirmaciones de tipo comprobadas (errcheck con check-type-assertions).

func str(t *testing.T, v any) string {
	t.Helper()
	s, ok := v.(string)
	require.True(t, ok, "se esperaba string: %v", v)
	return s
}

func obj(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	require.True(t, ok, "se esperaba objeto: %v", v)
	return m
}
