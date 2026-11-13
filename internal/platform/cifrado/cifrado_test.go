package cifrado

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func llave(t testing.TB) string {
	t.Helper()
	k := make([]byte, 32)
	_, err := rand.Read(k)
	require.NoError(t, err)
	return base64.StdEncoding.EncodeToString(k)
}

var aqui = Lugar{Coleccion: "usuarios", Campo: "gustos.alergias", ID: "65f0c0ffee00000000000001"}

func TestCifrado_IdaYVuelta(t *testing.T) {
	l, err := CargarLlavero("v1:" + llave(t))
	require.NoError(t, err)
	valor, err := l.Cifrar(aqui, "cacahuate y mariscos")
	require.NoError(t, err)
	assert.True(t, EsCifrado(valor))
	assert.True(t, strings.HasPrefix(valor, "enc:v1:v1:"))
	assert.NotContains(t, valor, "cacahuate")
	texto, err := l.Descifrar(aqui, valor)
	require.NoError(t, err)
	assert.Equal(t, "cacahuate y mariscos", texto)
}

func TestCifrado_NonceDistintoCadaVez(t *testing.T) {
	l, err := CargarLlavero("v1:" + llave(t))
	require.NoError(t, err)
	a, err := l.Cifrar(aqui, "lo mismo")
	require.NoError(t, err)
	b, err := l.Cifrar(aqui, "lo mismo")
	require.NoError(t, err)
	assert.NotEqual(t, a, b)
}

func TestCifrado_AlteradoCopiadoYLlaveEquivocada(t *testing.T) {
	l, err := CargarLlavero("v1:" + llave(t))
	require.NoError(t, err)
	valor, err := l.Cifrar(aqui, "dato")
	require.NoError(t, err)

	// Un byte cambiado en el texto cifrado (se cambia el byte y se vuelve a codificar: el
	// último carácter base64 puede llevar solo bits de relleno que se ignoran).
	cut := strings.LastIndex(valor, ":")
	sealed, err := base64.RawURLEncoding.DecodeString(valor[cut+1:])
	require.NoError(t, err)
	sealed[0] ^= 0xFF
	_, err = l.Descifrar(aqui, valor[:cut+1]+base64.RawURLEncoding.EncodeToString(sealed))
	assert.ErrorIs(t, err, ErrNoAbre)

	// Copiado a otro documento o a otro campo.
	otro := aqui
	otro.ID = "65f0c0ffee00000000000002"
	_, err = l.Descifrar(otro, valor)
	assert.ErrorIs(t, err, ErrNoAbre)
	campo := aqui
	campo.Campo = "fecha_nacimiento"
	_, err = l.Descifrar(campo, valor)
	assert.ErrorIs(t, err, ErrNoAbre)

	// Otra llave con el mismo id.
	ajeno, err := CargarLlavero("v1:" + llave(t))
	require.NoError(t, err)
	_, err = ajeno.Descifrar(aqui, valor)
	assert.ErrorIs(t, err, ErrNoAbre)

	// Una llave que no está en el llavero.
	sinV1, err := CargarLlavero("v2:" + llave(t))
	require.NoError(t, err)
	_, err = sinV1.Descifrar(aqui, valor)
	assert.ErrorIs(t, err, ErrSinLlave)
}

func TestCifrado_Rotacion(t *testing.T) {
	vieja := "v1:" + llave(t)
	antes, err := CargarLlavero(vieja)
	require.NoError(t, err)
	valorViejo, err := antes.Cifrar(aqui, "fecha")
	require.NoError(t, err)

	// La nueva va primero (cifra); la vieja sigue descifrando.
	despues, err := CargarLlavero("v2:" + llave(t) + "," + vieja)
	require.NoError(t, err)
	assert.Equal(t, "v2", despues.Activa())
	texto, err := despues.Descifrar(aqui, valorViejo)
	require.NoError(t, err)
	assert.Equal(t, "fecha", texto)
	nuevo, err := despues.Cifrar(aqui, texto)
	require.NoError(t, err)
	id, err := LlaveDe(nuevo)
	require.NoError(t, err)
	assert.Equal(t, "v2", id)
	id, err = LlaveDe(valorViejo)
	require.NoError(t, err)
	assert.Equal(t, "v1", id)
}

func TestCargarLlavero_Invalido(t *testing.T) {
	corta := base64.StdEncoding.EncodeToString([]byte("corta"))
	for name, raw := range map[string]string{
		"vacío":             "",
		"sin id":            llave(t),
		"id con mayúsculas": "V1:" + llave(t),
		"llave corta":       "v1:" + corta,
		"no base64":         "v1:%%%",
		"id repetido":       "v1:" + llave(t) + ",v1:" + llave(t),
	} {
		t.Run(name, func(t *testing.T) {
			_, err := CargarLlavero(raw)
			assert.ErrorIs(t, err, ErrLlavero)
		})
	}
}

func TestDescifrar_FormatoInvalido(t *testing.T) {
	l, err := CargarLlavero("v1:" + llave(t))
	require.NoError(t, err)
	for _, raw := range []string{"texto en claro", "enc:v1:", "enc:v1:v1:abc", "enc:v1:v1:abc:def:ghi", "enc:v1:V1:AAAAAAAAAAAAAAAA:AAAAAAAAAAAAAAAAAAAAAA"} {
		_, err := l.Descifrar(aqui, raw)
		assert.ErrorIs(t, err, ErrFormato, raw)
	}
}

// FuzzDescifrar: ningún valor hace entrar en pánico al lector del formato (se suma a nv-10).
func FuzzDescifrar(f *testing.F) {
	l, err := CargarLlavero("v1:" + llave(f))
	require.NoError(f, err)
	ok, err := l.Cifrar(aqui, "semilla")
	require.NoError(f, err)
	for _, seed := range []string{ok, "", "enc:v1:", "enc:v1:v1::", "enc:v1:v1:AAAA:BBBB", "enc:v2:x:y:z"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		texto, err := l.Descifrar(aqui, raw)
		if err == nil && raw != ok {
			// Solo un valor auténtico abre; si abre, debe dar el texto original.
			assert.Equal(t, "semilla", texto)
		}
	})
}

func BenchmarkCifrarDescifrar(b *testing.B) {
	l, err := CargarLlavero("v1:" + llave(b))
	require.NoError(b, err)
	for range b.N {
		v, _ := l.Cifrar(aqui, "cacahuate")
		_, _ = l.Descifrar(aqui, v)
	}
}
