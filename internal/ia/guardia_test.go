package ia

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// Dobles en memoria de los puertos de la guardia. Redis y MongoDB reales van en
// tests/integration (caché) y la suite Database (índices).
type modeloFijo struct {
	llamadas int
	salida   string
	entrada  int
	salidaT  int
	err      error
}

func (m *modeloFijo) Generar(context.Context, Peticion) (Respuesta, error) {
	m.llamadas++
	return Respuesta{JSON: []byte(m.salida), TokensEntrada: m.entrada, TokensSalida: m.salidaT, Duracion: 5 * time.Millisecond}, m.err
}

func (m *modeloFijo) Nombre() string { return "fijo/prueba" }

type contadorMem struct {
	valores map[string]int64
	err     error
}

func (c *contadorMem) Leer(_ context.Context, mes string) (int64, bool, error) {
	v, ok := c.valores[mes]
	return v, ok, c.err
}

func (c *contadorMem) Fijar(_ context.Context, mes string, micro int64, _ time.Duration) error {
	if _, ok := c.valores[mes]; !ok && c.err == nil {
		c.valores[mes] = micro
	}
	return c.err
}

func (c *contadorMem) Sumar(_ context.Context, mes string, micro int64, _ time.Duration) error {
	if c.err == nil {
		c.valores[mes] += micro
	}
	return c.err
}

type registroMem struct {
	usos []Uso
	err  error
}

func (r *registroMem) Guardar(_ context.Context, u Uso) error {
	r.usos = append(r.usos, u)
	return nil
}

func (r *registroMem) GastoDelMes(_ context.Context, mes string) (int64, error) {
	var total int64
	for _, u := range r.usos {
		if u.Mes == mes {
			total += u.CostoMicroUSD
		}
	}
	return total, r.err
}

type cacheMem struct{ valores map[string][]byte }

func (c *cacheMem) Leer(_ context.Context, h string) ([]byte, bool, error) {
	v, ok := c.valores[h]
	return v, ok, nil
}

func (c *cacheMem) Guardar(_ context.Context, h string, v []byte, _ time.Duration) error {
	c.valores[h] = v
	return nil
}

var noviembre = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)

type banco struct {
	modelo   *modeloFijo
	contador *contadorMem
	registro *registroMem
	cache    *cacheMem
	limites  *limitadorMem
	guardia  *Guardia
	frases   int
	ahora    time.Time
}

func nuevoBanco(t *testing.T, tope float64, precios Precios) *banco {
	t.Helper()
	b := &banco{
		modelo:   &modeloFijo{salida: `{"ids":["a"]}`, entrada: 150, salidaT: 60},
		contador: &contadorMem{valores: map[string]int64{}},
		registro: &registroMem{},
		cache:    &cacheMem{valores: map[string][]byte{}},
		limites:  &limitadorMem{totales: map[string]int64{}, nacidas: map[string]time.Time{}},
		ahora:    noviembre,
	}
	b.limites.ahora = func() time.Time { return b.ahora }
	g, err := NewGuardia(b.modelo, b.contador, b.registro, b.cache,
		GuardiaConfig{TopeUSDMes: tope, Precios: precios, Limites: limitesPrueba, Limitador: b.limites,
			Ahora: func() time.Time { return b.ahora }},
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	require.NoError(t, err)
	b.guardia = g
	return b
}

var preciosPrueba = Precios{EntradaUSDMTok: 0.10, SalidaUSDMTok: 0.40}

func TestCosto_SumaTokensPorPrecio(t *testing.T) {
	// 150 × 0.10 + 60 × 0.40 = 39 micro-USD; una fracción se redondea hacia arriba.
	assert.Equal(t, int64(39), preciosPrueba.CostoMicroUSD(150, 60))
	assert.Equal(t, int64(1), preciosPrueba.CostoMicroUSD(1, 0))
	assert.Equal(t, int64(0), Precios{}.CostoMicroUSD(150, 60), "capa gratuita: costo cero")
	assert.Equal(t, int64(5_000_000), MicroUSD(5))
}

func TestGuardia_SumaElCostoDeCadaLlamada(t *testing.T) {
	b := nuevoBanco(t, 5, preciosPrueba)

	for range 2 {
		_, err := b.guardia.Generar(context.Background(), Peticion{TextoUsuario: "tacos"})
		require.NoError(t, err)
	}

	assert.Equal(t, int64(78), b.contador.valores["2026-11"])
	require.Len(t, b.registro.usos, 2)
	assert.Equal(t, int64(39), b.registro.usos[0].CostoMicroUSD)
	assert.Equal(t, ResultadoOK, b.registro.usos[0].Resultado)
	assert.Equal(t, 150, b.registro.usos[0].TokensEntrada)
}

func TestGuardia_TopeExactoNoLlamaAlModelo(t *testing.T) {
	b := nuevoBanco(t, 5, preciosPrueba)
	b.contador.valores["2026-11"] = MicroUSD(5)

	_, err := b.guardia.Generar(context.Background(), Peticion{})

	require.ErrorIs(t, err, ErrTope)
	assert.Zero(t, b.modelo.llamadas, "al llegar al tope no se llama al proveedor")
	assert.Equal(t, ResultadoTope, b.registro.usos[0].Resultado)

	b.contador.valores["2026-11"] = MicroUSD(5) - 1
	_, err = b.guardia.Generar(context.Background(), Peticion{})
	require.NoError(t, err, "un micro-dólar por debajo del tope todavía llama")
	assert.Equal(t, 1, b.modelo.llamadas)
}

func TestGuardia_MesNuevoEmpiezaEnCero(t *testing.T) {
	b := nuevoBanco(t, 5, preciosPrueba)
	b.contador.valores["2026-11"] = MicroUSD(5)
	b.ahora = time.Date(2026, 12, 1, 0, 0, 1, 0, time.UTC)

	_, err := b.guardia.Generar(context.Background(), Peticion{})

	require.NoError(t, err)
	assert.Equal(t, int64(39), b.contador.valores["2026-12"])
	assert.Equal(t, "2026-12", b.registro.usos[0].Mes)
	assert.Equal(t, 24*time.Hour+time.Hour, vidaMes(time.Date(2026, 11, 30, 23, 0, 0, 0, time.UTC)), "la llave vive hasta un día después del mes")
}

func TestGuardia_ReconstruyeElContadorDesdeUsos(t *testing.T) {
	b := nuevoBanco(t, 5, preciosPrueba)
	b.registro.usos = []Uso{
		{Mes: "2026-11", CostoMicroUSD: MicroUSD(3)},
		{Mes: "2026-11", CostoMicroUSD: MicroUSD(2)},
		{Mes: "2026-10", CostoMicroUSD: MicroUSD(4)},
	}

	_, err := b.guardia.Generar(context.Background(), Peticion{})

	require.ErrorIs(t, err, ErrTope, "noviembre ya suma 5 USD en usos_ia aunque Redis no tenga la llave")
	assert.Equal(t, MicroUSD(5), b.contador.valores["2026-11"], "el contador queda reconstruido")
	assert.Zero(t, b.modelo.llamadas)
}

func TestGuardia_RedisCaidoUsaUsos(t *testing.T) {
	b := nuevoBanco(t, 5, preciosPrueba)
	b.contador.err = errors.New("redis caído")
	b.registro.usos = []Uso{{Mes: "2026-11", CostoMicroUSD: MicroUSD(1)}}

	_, err := b.guardia.Generar(context.Background(), Peticion{})

	require.NoError(t, err)
	assert.Equal(t, 1, b.modelo.llamadas)
}

func TestGuardia_SinGastoConocidoCierra(t *testing.T) {
	b := nuevoBanco(t, 5, preciosPrueba)
	b.contador.err = errors.New("redis caído")
	b.registro.err = errors.New("mongo caído")

	_, err := b.guardia.Generar(context.Background(), Peticion{})

	require.ErrorIs(t, err, ErrGastoDesconocido)
	assert.Zero(t, b.modelo.llamadas)
}

func TestGuardia_CuentaLosTokensDeUnaSalidaInvalida(t *testing.T) {
	b := nuevoBanco(t, 5, preciosPrueba)
	b.modelo.err = ErrRespuestaInvalida

	_, err := b.guardia.Generar(context.Background(), Peticion{})

	require.ErrorIs(t, err, ErrRespuestaInvalida)
	assert.Equal(t, int64(39), b.contador.valores["2026-11"], "el proveedor cobra aunque la salida no sirva")
	assert.Equal(t, ResultadoInvalida, b.registro.usos[0].Resultado)
}

func TestNewGuardia_RechazaTopeYPreciosInvalidos(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	m, c, r, k, l := &modeloFijo{}, &contadorMem{}, &registroMem{}, &cacheMem{}, &limitadorMem{}
	for _, cfg := range []GuardiaConfig{
		{TopeUSDMes: 0, Limites: limitesPrueba, Limitador: l},
		{TopeUSDMes: 5, Precios: Precios{EntradaUSDMTok: -1}, Limites: limitesPrueba, Limitador: l},
		{TopeUSDMes: 5, Limites: Limites{PorHora: 0, PorMinuto: 12, PorDia: 400}, Limitador: l},
		{TopeUSDMes: 5, Limites: limitesPrueba},
	} {
		_, err := NewGuardia(m, c, r, k, cfg, log)
		assert.Error(t, err)
	}
	_, err := NewGuardia(nil, c, r, k, GuardiaConfig{TopeUSDMes: 5, Limites: limitesPrueba, Limitador: l}, log)
	assert.Error(t, err)
}

type salidaIDs struct {
	IDs []string `json:"ids"`
}

func TestGuardia_CacheAciertaSinLlamarDeNuevo(t *testing.T) {
	b := nuevoBanco(t, 5, preciosPrueba)
	usuario := bson.NewObjectID()
	ctx := ConUsuario(context.Background(), usuario)
	pet := Peticion{Instrucciones: "elige", Contexto: "candidatos: a,b", TextoUsuario: "Tacos  baratos"}

	var primera, segunda salidaIDs
	_, err := b.guardia.GenerarValidado(ctx, pet, &primera)
	require.NoError(t, err)
	pet.TextoUsuario = "tacos baratos "
	_, err = b.guardia.GenerarValidado(ctx, pet, &segunda)
	require.NoError(t, err)

	assert.Equal(t, 1, b.modelo.llamadas, "la misma entrada normalizada acierta en la caché")
	assert.Equal(t, primera, segunda)
	require.Len(t, b.registro.usos, 2)
	assert.Equal(t, ResultadoCache, b.registro.usos[1].Resultado)
	assert.Equal(t, usuario, b.registro.usos[1].UsuarioID)
	assert.Zero(t, b.registro.usos[1].CostoMicroUSD)
}

func TestGuardia_NoGuardaUnaSalidaQueRepiteLaFrase(t *testing.T) {
	b := nuevoBanco(t, 5, preciosPrueba)
	b.modelo.salida = `{"ids":["Tacos baratos"]}`

	var out salidaIDs
	_, err := b.guardia.GenerarValidado(context.Background(), Peticion{TextoUsuario: "tacos   baratos"}, &out)

	require.NoError(t, err)
	assert.Empty(t, b.cache.valores, "la caché nunca guarda el texto del usuario")
}

func TestHuella_CambiaConCandidatosYNoConEspacios(t *testing.T) {
	base := Peticion{Instrucciones: "elige", Contexto: "candidatos: a,b", TextoUsuario: "Café cerca", Esquema: map[string]any{"type": "object"}}
	otra := base
	otra.TextoUsuario = "  café   CERCA "
	assert.Equal(t, Huella("m", base), Huella("m", otra))

	otra.Contexto = "candidatos: a,c"
	assert.NotEqual(t, Huella("m", base), Huella("m", otra), "otra lista de candidatos es otra entrada")
	assert.NotEqual(t, Huella("m", base), Huella("n", base), "otro modelo es otra entrada")

	conTel := base
	conTel.TextoUsuario = "Café cerca 55 1234 5678"
	assert.NotContains(t, Normalizar(conTel.TextoUsuario), "5678", "la huella usa la frase anonimizada")
	assert.Len(t, Huella("m", base), 64)
}
