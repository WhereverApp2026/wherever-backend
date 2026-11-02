package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func decode(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&body))
	return body
}

func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(raw)
}

var snakeCase = regexp.MustCompile(`^[a-z][a-z0-9]*(_[a-z0-9]+)*$`)

// assertSnakeCase revisa todas las llaves del JSON, también las anidadas.
func assertSnakeCase(t *testing.T, value any) {
	t.Helper()
	switch v := value.(type) {
	case map[string]any:
		for key, nested := range v {
			require.Regexp(t, snakeCase, key, "llave fuera de snake_case")
			assertSnakeCase(t, nested)
		}
	case []any:
		for _, nested := range v {
			assertSnakeCase(t, nested)
		}
	}
}
