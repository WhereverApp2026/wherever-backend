package imagenes_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
)

// La política la aplica S3 (en vivo); aquí se comprueba que la API la firma con los límites
// de la spec: clave exacta, tipo exacto, de 1 byte a 5 MB y 5 minutos. Firmar no usa la red.
func TestS3_FirmarSubidaLlevaLaPolitica(t *testing.T) {
	a, err := imagenes.NewS3(context.Background(), imagenes.S3Config{
		Bucket: "wherever-dev-imagenes", Region: "us-east-2",
		Credenciales: credentials.NewStaticCredentialsProvider("AKIDPRUEBAS", "secreto-de-prueba", ""),
	})
	require.NoError(t, err)
	antes := time.Now().UTC()
	sub, err := a.FirmarSubida(context.Background(), "cuarentena/abc", "image/png", imagenes.DefaultMaxBytes, imagenes.DefaultVence)
	require.NoError(t, err)

	assert.Contains(t, sub.URL, "wherever-dev-imagenes")
	assert.Equal(t, "cuarentena/abc", sub.Campos["key"])
	assert.Equal(t, "image/png", sub.Campos["Content-Type"])
	require.NotEmpty(t, sub.Campos["X-Amz-Signature"])

	raw, err := base64.StdEncoding.DecodeString(sub.Campos["policy"])
	require.NoError(t, err)
	var politica struct {
		Expiration time.Time `json:"expiration"`
		Conditions []any     `json:"conditions"`
	}
	require.NoError(t, json.Unmarshal(raw, &politica))
	assert.WithinDuration(t, antes.Add(5*time.Minute), politica.Expiration, 5*time.Second)

	rango, tipo, clave := condiciones(politica.Conditions)
	assert.True(t, rango, "un archivo de 6 MB no pasa: content-length-range 1..5 MB")
	assert.True(t, tipo, "el tipo es exacto")
	assert.True(t, clave, "la clave es exacta, dentro de cuarentena/")
}

// condiciones dice si la política fija el rango 1..5 MB, el tipo image/png y la clave.
func condiciones(conds []any) (rango, tipo, clave bool) {
	for _, c := range conds {
		switch v := c.(type) {
		case []any:
			r, k := condicionLista(v)
			rango, clave = rango || r, clave || k
		case map[string]any:
			tipo = tipo || v["Content-Type"] == "image/png"
			clave = clave || v["key"] == "cuarentena/abc"
		}
	}
	return rango, tipo, clave
}

// condicionLista lee ["content-length-range", 1, 5 MB] o ["eq", "$key", clave].
func condicionLista(v []any) (rango, clave bool) {
	if len(v) != 3 {
		return false, false
	}
	rango = v[0] == "content-length-range" && v[1] == float64(1) && v[2] == float64(5<<20)
	clave = v[0] == "eq" && v[1] == "$key" && v[2] == "cuarentena/abc"
	return rango, clave
}

func TestNewS3_SinBucketFalla(t *testing.T) {
	_, err := imagenes.NewS3(context.Background(), imagenes.S3Config{Region: "us-east-2"})
	assert.Error(t, err)
}
