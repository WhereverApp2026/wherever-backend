package config

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// envExamplePath apunta a la plantilla relativa a este paquete.
const envExamplePath = "../../../configs/.env.example"

// futurePrefixes son claves de integraciones fuera de esta etapa. FCM entró con nv-41.
var futurePrefixes = []string{"STRIPE_", "AWS_", "PERF_"}

// sensitivePatterns detectan valores reales que no deben vivir en Git.
var sensitivePatterns = map[string]*regexp.Regexp{
	"uri con credenciales":    regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`),
	"llave privada":           regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
	"cadena de alta entropía": regexp.MustCompile(`[A-Za-z0-9+/=_-]{40,}`),
	"llave de proveedor":      regexp.MustCompile(`(sk|pk|rk)_(live|test)_|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}`),
}

type envEntry struct {
	key   string
	value string
	line  int
}

func readEnvExample(t *testing.T) []envEntry {
	t.Helper()
	file, err := os.Open(envExamplePath)
	require.NoError(t, err, "configs/.env.example debe existir")
	t.Cleanup(func() { _ = file.Close() })

	var entries []envEntry
	scanner := bufio.NewScanner(file)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		require.Truef(t, ok, "línea %d sin formato CLAVE=valor: %q", n, line)
		entries = append(entries, envEntry{key: strings.TrimSpace(key), value: strings.TrimSpace(value), line: n})
	}
	require.NoError(t, scanner.Err())
	return entries
}

func TestEnvExample_IsValid(t *testing.T) {
	entries := readEnvExample(t)

	values := make(map[string]string, len(entries))
	for _, e := range entries {
		assert.Regexpf(t, `^[A-Z][A-Z0-9_]*$`, e.key, "línea %d: clave fuera de MAYÚSCULAS_SNAKE", e.line)
		_, dup := values[e.key]
		assert.Falsef(t, dup, "línea %d: clave duplicada %s", e.line, e.key)
		values[e.key] = e.value
	}

	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	assert.ElementsMatch(t, Keys(), keys, "solo las claves que lee config.Load, ni más ni menos")
	assert.Contains(t, []string{"dev", "staging", "prod"}, values["APP_ENV"], "APP_ENV admite solo dev, staging o prod")
	assert.Equal(t, "dev", values["APP_ENV"], "la plantilla arranca en dev")
}

func TestEnvExample_HasNoSensitivePatterns(t *testing.T) {
	for _, e := range readEnvExample(t) {
		for name, pattern := range sensitivePatterns {
			assert.Falsef(t, pattern.MatchString(e.value), "línea %d (%s): parece %s", e.line, e.key, name)
		}
	}
}

func TestEnvExample_HasNoFutureKeys(t *testing.T) {
	for _, e := range readEnvExample(t) {
		for _, prefix := range futurePrefixes {
			assert.Falsef(t, strings.HasPrefix(e.key, prefix), "línea %d: %s pertenece a una etapa futura", e.line, e.key)
		}
	}
}
