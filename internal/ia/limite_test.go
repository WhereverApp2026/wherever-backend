package ia

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var limitesPrueba = Limites{PorHora: 5, PorMinuto: 12, PorDia: 400}

// limitadorMem cuenta ventanas fijas en memoria con el reloj del banco; Redis real va en
// tests/integration.
type limitadorMem struct {
	totales map[string]int64
	nacidas map[string]time.Time
	vidas   map[string]time.Duration
	ahora   func() time.Time
	err     error
}

func (l *limitadorMem) Tomar(_ context.Context, ventanas ...Ventana) ([]Cuenta, error) {
	if l.err != nil {
		return nil, l.err
	}
	if l.vidas == nil {
		l.vidas = map[string]time.Duration{}
	}
	ahora := l.ahora()
	out := make([]Cuenta, len(ventanas))
	for i, v := range ventanas {
		if nacida, ok := l.nacidas[v.Llave]; !ok || !ahora.Before(nacida.Add(l.vidas[v.Llave])) {
			l.nacidas[v.Llave], l.vidas[v.Llave], l.totales[v.Llave] = ahora, v.Vida, 0
		}
		l.totales[v.Llave]++
		out[i] = Cuenta{Total: l.totales[v.Llave], Resta: l.nacidas[v.Llave].Add(l.vidas[v.Llave]).Sub(ahora)}
	}
	return out, nil
}

func generar(ctx context.Context, t *testing.T, b *banco) error {
	t.Helper()
	// Cada generación lleva otra frase para que la caché no la conteste.
	b.frases++
	var out salidaIDs
	_, err := b.guardia.GenerarValidado(ctx, Peticion{TextoUsuario: "tacos " + strconv.Itoa(b.frases)}, &out)
	return err
}

func TestLimiteUsuario_SextaGeneracionDiceCuandoRegresa(t *testing.T) {
	b := nuevoBanco(t, 5, Precios{})
	ctx := ConUsuario(context.Background(), bson.NewObjectID())
	inicio := b.ahora
	for i := range 5 {
		b.ahora = inicio.Add(time.Duration(i) * 10 * time.Minute)
		require.NoError(t, generar(ctx, t, b), "la generación %d cabe", i+1)
	}
	b.ahora = inicio.Add(45 * time.Minute)

	err := generar(ctx, t, b)

	var lim *LimiteUsuarioError
	require.ErrorAs(t, err, &lim)
	require.ErrorIs(t, err, ErrLimiteUsuario)
	assert.Equal(t, inicio.Add(time.Hour).UTC(), lim.Regreso, "la hora empieza con la primera generación")
	assert.Equal(t, 900, lim.RetryAfter())
	assert.Equal(t, 5, b.modelo.llamadas, "la sexta no llama al proveedor")
	assert.Equal(t, ResultadoLimiteUsuario, b.registro.usos[len(b.registro.usos)-1].Resultado, "el rechazo queda registrado")

	b.ahora = inicio.Add(time.Hour)
	require.NoError(t, generar(ctx, t, b), "a la hora de regreso vuelve a generar")
}

func TestLimiteUsuario_OtroUsuarioNoSeAfecta(t *testing.T) {
	b := nuevoBanco(t, 5, Precios{})
	uno := ConUsuario(context.Background(), bson.NewObjectID())
	for range 5 {
		require.NoError(t, generar(uno, t, b))
	}
	require.ErrorIs(t, generar(uno, t, b), ErrLimiteUsuario)

	require.NoError(t, generar(ConUsuario(context.Background(), bson.NewObjectID()), t, b))
}

func TestLimiteUsuario_SinUsuarioCompartenUnaCubeta(t *testing.T) {
	b := nuevoBanco(t, 5, Precios{})
	for range 5 {
		require.NoError(t, generar(context.Background(), t, b))
	}

	require.ErrorIs(t, generar(context.Background(), t, b), ErrLimiteUsuario, "olvidar ConUsuario no quita el límite")
	assert.Contains(t, b.limites.totales, "rate:ia_usuario:sin_usuario")
}

func TestLimiteUsuario_AciertoDeCacheNoCuenta(t *testing.T) {
	b := nuevoBanco(t, 5, Precios{})
	ctx := ConUsuario(context.Background(), bson.NewObjectID())
	pet := Peticion{Instrucciones: "elige", TextoUsuario: "café"}
	for range 8 {
		var out salidaIDs
		_, err := b.guardia.GenerarValidado(ctx, pet, &out)
		require.NoError(t, err)
	}
	assert.Equal(t, 1, b.modelo.llamadas)
}

func TestLimiteGlobal_PorMinutoYPorDia(t *testing.T) {
	b := nuevoBanco(t, 5, Precios{})
	inicio := b.ahora
	for range 12 {
		_, err := b.guardia.Generar(context.Background(), Peticion{})
		require.NoError(t, err)
	}
	_, err := b.guardia.Generar(context.Background(), Peticion{})
	require.ErrorIs(t, err, ErrLimiteGlobal, "la llamada 13 del minuto no sale")
	assert.Equal(t, 12, b.modelo.llamadas)
	assert.Equal(t, ResultadoLimiteGlobal, b.registro.usos[len(b.registro.usos)-1].Resultado)

	b.ahora = inicio.Add(time.Minute)
	_, err = b.guardia.Generar(context.Background(), Peticion{})
	require.NoError(t, err, "el minuto siguiente es otra ventana")

	b.limites.totales[llaveDia(b.ahora, b.guardia.zona)] = 400
	b.ahora = inicio.Add(2 * time.Minute)
	_, err = b.guardia.Generar(context.Background(), Peticion{})
	require.ErrorIs(t, err, ErrLimiteGlobal, "la llamada 401 del día no sale")
}

func TestLimiteGlobal_CuentaElReintento(t *testing.T) {
	b := nuevoBanco(t, 5, Precios{})
	b.modelo.salida = `no es json`
	var out salidaIDs
	_, err := b.guardia.GenerarValidado(ConUsuario(context.Background(), bson.NewObjectID()), Peticion{}, &out)

	require.ErrorIs(t, err, ErrRespuestaInvalida)
	assert.Equal(t, int64(2), b.limites.totales[llaveMinuto(b.ahora)], "cada llamada cuenta, también el reintento")
}

func TestLimites_RedisCaidoCierra(t *testing.T) {
	b := nuevoBanco(t, 5, Precios{})
	b.limites.err = errors.New("redis caído")

	_, err := b.guardia.Generar(context.Background(), Peticion{})
	require.ErrorIs(t, err, ErrLimiteDesconocido)
	err = generar(ConUsuario(context.Background(), bson.NewObjectID()), t, b)
	require.ErrorIs(t, err, ErrLimiteDesconocido)

	assert.Zero(t, b.modelo.llamadas, "sin límites no se llama al proveedor")
	assert.Equal(t, ResultadoSinLimite, b.registro.usos[0].Resultado)
}

func TestLlaves_DiaDelPacificoYMinutoUTC(t *testing.T) {
	b := nuevoBanco(t, 5, Precios{})
	// 2026-11-12 06:30 UTC son las 22:30 del 11 en Los Ángeles (PST, UTC-8).
	t1 := time.Date(2026, 11, 12, 6, 30, 0, 0, time.UTC)
	assert.Equal(t, "rate:ia_dia:2026-11-11", llaveDia(t1, b.guardia.zona))
	assert.Equal(t, "rate:ia_dia:2026-11-12", llaveDia(t1.Add(90*time.Minute), b.guardia.zona))
	assert.Equal(t, "rate:ia_min:202611120630", llaveMinuto(t1))

	u := bson.NewObjectID()
	llave := llaveUsuario(ConUsuario(context.Background(), u))
	assert.True(t, strings.HasPrefix(llave, "rate:ia_usuario:"))
	assert.Equal(t, u.Hex(), strings.TrimPrefix(llave, "rate:ia_usuario:"))
}

func TestLimiteUsuarioError_RetryAfterRedondeaArriba(t *testing.T) {
	assert.Equal(t, 2, (&LimiteUsuarioError{Espera: 1500 * time.Millisecond}).RetryAfter())
	assert.Equal(t, 1, (&LimiteUsuarioError{}).RetryAfter())
	assert.Contains(t, (&LimiteUsuarioError{Regreso: noviembre}).Error(), "regresa")
}
