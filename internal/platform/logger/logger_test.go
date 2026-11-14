package logger_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/logger"
)

func line(t *testing.T, write func(*slog.Logger)) (string, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	write(logger.New(&buf, "staging"))
	var fields map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &fields))
	return buf.String(), fields
}

func TestLogger_LineaJSONConCamposParaCloudWatch(t *testing.T) {
	_, fields := line(t, func(l *slog.Logger) {
		l.Info("petición", "request_id", "abc", "metodo", "GET", "ruta", "/health", "estado", 200, "duracion_ms", 3)
	})

	assert.Equal(t, "INFO", fields["level"], "los filtros de métricas dependen de este valor")
	assert.NotEmpty(t, fields["time"])
	for _, key := range []string{"request_id", "metodo", "ruta", "estado", "duracion_ms"} {
		assert.Contains(t, fields, key)
	}
}

func TestLogger_LlavesSensiblesSeOcultan(t *testing.T) {
	for _, key := range []string{"password", "token", "refresh_token", "authorization", "jwt", "clabe", "rfc", "razon_social", "archivo"} {
		raw, _ := line(t, func(l *slog.Logger) { l.Info("x", key, "valor-secreto") })
		assert.NotContains(t, raw, "valor-secreto", key)
	}
}

func TestLogger_CorreosYCredencialesDentroDeUnTextoSeOcultan(t *testing.T) {
	duplicate := `E11000 duplicate key error: dup key: { correo: "ana.lopez+x@example.com" }`
	raw, fields := line(t, func(l *slog.Logger) {
		l.Error("error interno", "error", duplicate, "destino", "mongodb+srv://"+"usuario:"+"clave-secreta"+"@cluster.example.net/db")
	})

	assert.NotContains(t, raw, "ana.lopez")
	assert.NotContains(t, raw, "clave-secreta")
	assert.Contains(t, fields["error"], "E11000", "el resto del mensaje se conserva para diagnosticar")
	assert.Contains(t, fields["destino"], "cluster.example.net")
}

func TestLogger_ElCorreoYaEnmascaradoSeConserva(t *testing.T) {
	_, fields := line(t, func(l *slog.Logger) { l.Info("correo", "destinatario", "a***@example.com") })

	assert.Equal(t, "a***@example.com", fields["destinatario"])
}

func TestLogger_LlaveDeGoogleDentroDeUnTextoSeOculta(t *testing.T) {
	fake := "AIza" + "Sy0000000000000000000000000000000AB"
	raw, _ := line(t, func(l *slog.Logger) { l.Error("error de proveedor", "error", "solicitud rechazada para "+fake) })

	assert.NotContains(t, raw, fake)
	assert.Contains(t, raw, "solicitud rechazada")
}
