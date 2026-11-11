package asistente

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/iatest"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Dobles de las dependencias del service: catálogo, perfil y descartes en memoria. La IA es
// el doble de iatest, sola o detrás de una ia.Guardia con almacenes en memoria.

type fuenteMem struct{ cands []negocios.Candidate }

func (f fuenteMem) Candidates(_ context.Context, q negocios.CandidateQuery) ([]negocios.Candidate, error) {
	out := []negocios.Candidate{}
	for _, c := range f.cands {
		if !excluido(q.Conditions, c.ID) {
			out = append(out, c)
		}
	}
	return out, nil
}

// excluido imita el `$nin` de los excluidos que el filtro manda a MongoDB.
func excluido(conds bson.A, id bson.ObjectID) bool {
	return strings.Contains(fmt.Sprint(conds), id.Hex())
}

type usuariosMem struct{ u usuarios.User }

func (m usuariosMem) FindByID(_ context.Context, id bson.ObjectID) (usuarios.User, error) {
	if id != m.u.ID {
		return usuarios.User{}, usuarios.ErrNotFound
	}
	return m.u, nil
}

type descartesMem struct{ n int }

func (d *descartesMem) Registrar(_ context.Context, _ string, ds []Descarte) error {
	d.n += len(ds)
	return nil
}

type contadorMem struct{ micro int64 }

func (c *contadorMem) Leer(context.Context, string) (int64, bool, error) { return c.micro, true, nil }
func (c *contadorMem) Fijar(context.Context, string, int64, time.Duration) error {
	return nil
}
func (c *contadorMem) Sumar(_ context.Context, _ string, micro int64, _ time.Duration) error {
	c.micro += micro
	return nil
}

type registroMem struct{ usos []ia.Uso }

func (r *registroMem) Guardar(_ context.Context, u ia.Uso) error {
	r.usos = append(r.usos, u)
	return nil
}
func (r *registroMem) GastoDelMes(context.Context, string) (int64, error) { return 0, nil }

type cacheVacia struct{}

func (cacheVacia) Leer(context.Context, string) ([]byte, bool, error)           { return nil, false, nil }
func (cacheVacia) Guardar(context.Context, string, []byte, time.Duration) error { return nil }

// limitadorMem devuelve `usuario` para la ventana del usuario y `global` para las demás.
type limitadorMem struct{ usuario, global int64 }

func (l limitadorMem) Tomar(_ context.Context, vs ...ia.Ventana) ([]ia.Cuenta, error) {
	out := make([]ia.Cuenta, 0, len(vs))
	for _, v := range vs {
		total := l.global
		if strings.HasPrefix(v.Llave, "rate:ia_usuario:") {
			total = l.usuario
		}
		out = append(out, ia.Cuenta{Total: total, Resta: time.Minute})
	}
	return out, nil
}

// cacahuates lleva el alérgeno del perfil: nunca puede salir, tampoco en el respaldo.
func catalogo() []negocios.Candidate {
	out := []negocios.Candidate{}
	for i, r := range append(candidatos(), rec("cacahuates", "restaurante", 100, 0.0005, nil), rec("cafe2", "cafeteria", 90, 0.004, nil)) {
		b := r.Negocio
		b.State = negocios.StatusApproved
		b.Allergens = []string{negocios.AllergenNone}
		if b.Name == "cacahuates" {
			b.Allergens = []string{"cacahuate"}
		}
		out = append(out, negocios.Candidate{Business: b, DistanceM: float64(100 * (i + 1))})
	}
	return out
}

type banco struct {
	svc      *Service
	modelo   *iatest.Modelo
	contador *contadorMem
	registro *registroMem
	user     usuarios.User
	logs     *testutil.LogBuffer
}

// nuevoBanco arma el service; con limitador != nil el doble va detrás de una ia.Guardia con
// precio positivo, para ver que el respaldo no suma gasto.
func nuevoBanco(t *testing.T, lim ia.Limitador, gastado int64) *banco {
	t.Helper()
	log, logs := testutil.Logger()
	b := &banco{modelo: iatest.Nuevo(), contador: &contadorMem{micro: gastado}, registro: &registroMem{}, logs: logs}
	b.user = usuarios.User{ID: bson.NewObjectID(), Tastes: perfilSinDieta()}
	var modelo ia.Modelo = b.modelo
	if lim != nil {
		g, err := ia.NewGuardia(b.modelo, b.contador, b.registro, cacheVacia{}, ia.GuardiaConfig{
			TopeUSDMes: 1, Precios: ia.Precios{EntradaUSDMTok: 1, SalidaUSDMTok: 1},
			Limites: ia.Limites{PorHora: 10, PorMinuto: 12, PorDia: 400}, Limitador: lim,
		}, log)
		require.NoError(t, err)
		modelo = g
	}
	pesos, err := recomendador.PesosPorDefecto()
	require.NoError(t, err)
	b.svc, err = NewService(modelo, fuenteMem{catalogo()}, usuariosMem{b.user}, &descartesMem{},
		Config{Pesos: pesos, Ahora: func() time.Time { return sabadoTarde }, PlazoIA: 200 * time.Millisecond}, log)
	require.NoError(t, err)
	return b
}

func perfilSinDieta() *usuarios.Tastes {
	p := perfil()
	p.Restrictions, p.Allergies = nil, "cacahuate"
	return p
}

func pedidoPrueba() GenerarRequest {
	r := valido()
	r.Movilidad = "a_pie"
	return r
}

func (b *banco) generar(t *testing.T) GenerarResponse {
	t.Helper()
	out, err := b.svc.Generar(context.Background(), b.user.ID.Hex(), pedidoPrueba())
	require.NoError(t, err)
	return out
}

func assertReglas(t *testing.T, out GenerarResponse) {
	t.Helper()
	assert.Equal(t, FuenteReglas, out.Fuente)
	assert.Equal(t, avisoReglas, out.Aviso)
	require.Len(t, out.Plan.Paradas, ParadasObjetivo, "con la IA caída siempre hay 3 opciones")
	for _, p := range out.Plan.Paradas {
		assert.NotEqual(t, "cacahuates", p.Nombre, "la alergia del perfil sigue siendo filtro duro")
	}
}

const interpretacionPrueba = `{"compania":"amigos","con_ninos":false,"con_mascota":false,"presupuesto_max":0,"hora":"","animo":"sin_dato","palabras_clave":["tacos"]}`

func TestGenerar_ConIARespondeLaIA(t *testing.T) {
	b := nuevoBanco(t, nil, 0)
	f, ok := b.svc.fuente.(fuenteMem)
	require.True(t, ok)
	cands := f.cands
	b.modelo.Responder(interpretacionPrueba).Responder(fmt.Sprintf(
		`{"paradas":[{"id":%q,"llega":"14:30","minutos":60},{"id":%q,"llega":"15:40","minutos":60}]}`,
		cands[0].ID.Hex(), cands[2].ID.Hex()))
	out := b.generar(t)
	assert.Equal(t, FuenteIA, out.Fuente)
	assert.Equal(t, avisoIA, out.Aviso)
	assert.Len(t, out.Plan.Paradas, 2)
}

// Si el itinerario falla después de interpretar, el respaldo conserva lo que la IA entendió.
func TestGenerar_ItinerarioInvalidoTerminaEnReglasConLaInterpretacion(t *testing.T) {
	b := nuevoBanco(t, nil, 0)
	b.modelo.Responder(interpretacionPrueba)
	out := b.generar(t)
	assertReglas(t, out)
	assert.Equal(t, []string{"tacos"}, out.Parametros.PalabrasClave)
	assert.Len(t, b.modelo.Peticiones(), 3, "interpretar + itinerario con un reintento")
}

// Cada falla del proveedor termina en el motor de reglas con `fuente: reglas` (US-039).
func TestGenerar_CadaFallaDeLaIATerminaEnReglas(t *testing.T) {
	casos := map[string]func(m *iatest.Modelo){
		"tiempo":            func(m *iatest.Modelo) { m.Fallar(ia.ErrTiempo) },
		"429 del proveedor": func(m *iatest.Modelo) { m.Fallar(&ia.LimiteError{Espera: 30 * time.Second}) },
		"límite proveedor":  func(m *iatest.Modelo) { m.Fallar(ia.ErrLimiteProveedor) },
		"falla":             func(m *iatest.Modelo) { m.Fallar(ia.ErrProveedor) },
		"bloqueada":         func(m *iatest.Modelo) { m.Fallar(ia.ErrBloqueada) },
		"inválida":          func(m *iatest.Modelo) { m.Responder(`{"plan":"inventado"}`) },
	}
	for nombre, programar := range casos {
		t.Run(nombre, func(t *testing.T) {
			b := nuevoBanco(t, nil, 0)
			programar(b.modelo)
			out := b.generar(t)
			assertReglas(t, out)
			assert.Equal(t, SinDato, out.Parametros.Compania)
			assert.NotContains(t, b.logs.String(), "tacos con la banda", "la frase no va a los logs")
		})
	}
}

func TestGenerar_SinProveedorRespondeElMotor(t *testing.T) {
	b := nuevoBanco(t, nil, 0)
	b.svc.modelo = nil
	assertReglas(t, b.generar(t))
	assert.Contains(t, b.logs.String(), "apagada")
}

// Tope y límites (nv-27/nv-28) llevan al respaldo sin llamar al proveedor ni sumar gasto.
func TestGenerar_TopeYLimitesTerminanEnReglasSinGastar(t *testing.T) {
	casos := map[string]struct {
		lim     limitadorMem
		gastado int64
		motivo  string
	}{
		"tope agotado":      {limitadorMem{usuario: 1, global: 1}, ia.MicroUSD(1), "tope"},
		"límite de usuario": {limitadorMem{usuario: 11, global: 1}, 0, "limite_usuario"},
		"límite global":     {limitadorMem{usuario: 1, global: 401}, 0, "limite_global"},
	}
	for nombre, c := range casos {
		t.Run(nombre, func(t *testing.T) {
			b := nuevoBanco(t, c.lim, c.gastado)
			assertReglas(t, b.generar(t))
			assert.Empty(t, b.modelo.Peticiones(), "no llega al proveedor")
			assert.Equal(t, c.gastado, b.contador.micro, "el respaldo no cuenta contra el tope")
			assert.Contains(t, b.logs.String(), c.motivo)
		})
	}
}

// Con la IA que sí responde, cada llamada suma gasto; el respaldo posterior no suma más.
func TestGenerar_RespaldoNoSumaGastoTrasUnaLlamadaFallida(t *testing.T) {
	b := nuevoBanco(t, limitadorMem{usuario: 1, global: 1}, 0)
	b.modelo.Responder(interpretacionPrueba).Fallar(ia.ErrProveedor)
	assertReglas(t, b.generar(t))
	assert.Len(t, b.modelo.Peticiones(), 2)
	assert.Equal(t, ia.Precios{EntradaUSDMTok: 1, SalidaUSDMTok: 1}.CostoMicroUSD(100, 40), b.contador.micro,
		"solo la llamada que respondió suma gasto; el respaldo no")
}

func TestGenerar_IALentaTerminaEnReglasDentroDelPlazo(t *testing.T) {
	b := nuevoBanco(t, nil, 0)
	b.modelo.Tardar(5 * time.Second)
	inicio := time.Now()
	out := b.generar(t)
	assertReglas(t, out)
	assert.Less(t, time.Since(inicio), time.Second, "plazo de la IA + respaldo, sin esperar al proveedor")
	assert.Contains(t, b.logs.String(), "tiempo")
}

// US-040: si la app corta, el contexto de la petición cancela la llamada al modelo y no se
// arma el respaldo.
func TestGenerar_CancelacionCortaLaIAYNoArmaRespaldo(t *testing.T) {
	b := nuevoBanco(t, nil, 0)
	b.svc.cfg.PlazoIA = 5 * time.Second
	b.modelo.Tardar(5 * time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(50*time.Millisecond, cancel)
	inicio := time.Now()
	_, err := b.svc.Generar(ctx, b.user.ID.Hex(), pedidoPrueba())
	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, http.StatusServiceUnavailable, ae.Kind.Status())
	assert.Less(t, time.Since(inicio), time.Second)
	assert.Contains(t, b.logs.String(), motivoCancelada)
}

func TestOtraIdea_NoRepiteLugares(t *testing.T) {
	b := nuevoBanco(t, nil, 0)
	b.modelo.Fallar(ia.ErrTiempo)
	primero := b.generar(t)
	excluir := []string{}
	for _, p := range primero.Plan.Paradas {
		excluir = append(excluir, p.NegocioID.Hex())
	}
	b.modelo.Fallar(ia.ErrTiempo)
	out, err := b.svc.OtraIdea(context.Background(), b.user.ID.Hex(), OtraIdeaRequest{GenerarRequest: pedidoPrueba(), Excluir: excluir})
	require.NoError(t, err)
	require.NotEmpty(t, out.Plan.Paradas)
	for _, p := range out.Plan.Paradas {
		assert.NotContains(t, excluir, p.NegocioID.Hex())
		assert.NotEqual(t, "cacahuates", p.Nombre)
	}

	// Sin más lugares: 422.
	for _, p := range out.Plan.Paradas {
		excluir = append(excluir, p.NegocioID.Hex())
	}
	_, err = b.svc.OtraIdea(context.Background(), b.user.ID.Hex(), OtraIdeaRequest{GenerarRequest: pedidoPrueba(), Excluir: excluir})
	var ae *apperr.Error
	require.ErrorAs(t, err, &ae)
	assert.Equal(t, http.StatusUnprocessableEntity, ae.Kind.Status())
}

func TestOtraIdeaRequest_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	ok := OtraIdeaRequest{GenerarRequest: valido(), Excluir: []string{bson.NewObjectID().Hex()}}
	require.NoError(t, v.Struct(ok))
	muchos := make([]string, MaxExcluir+1)
	for i := range muchos {
		muchos[i] = bson.NewObjectID().Hex()
	}
	for nombre, excluir := range map[string][]string{"vacío": {}, "no es id": {"abc"}, "31 ids": muchos} {
		t.Run(nombre, func(t *testing.T) {
			r := ok
			r.Excluir = excluir
			var ae *apperr.Error
			require.ErrorAs(t, v.Struct(r), &ae)
			assert.Equal(t, http.StatusUnprocessableEntity, ae.Kind.Status())
		})
	}
	sinFrase := ok
	sinFrase.Frase = ""
	assert.Error(t, v.Struct(sinFrase), "la frase sigue siendo obligatoria")
}

func TestMotivoIA_PorError(t *testing.T) {
	casos := map[error]string{
		ia.ErrTiempo: "tiempo", context.DeadlineExceeded: "tiempo", &ia.LimiteError{}: "limite_proveedor",
		ia.ErrTope: "tope", ia.ErrLimiteUsuario: "limite_usuario", ia.ErrLimiteGlobal: "limite_global",
		ia.ErrGastoDesconocido: "gasto_desconocido", ia.ErrLimiteDesconocido: "limite_desconocido",
		fmt.Errorf("x: %w", ia.ErrRespuestaInvalida): "respuesta_invalida", errApagada: "apagada",
		context.Canceled: motivoCancelada, errors.New("otro"): "proveedor",
	}
	for err, motivo := range casos {
		assert.Equal(t, motivo, motivoIA(err), err.Error())
	}
}
