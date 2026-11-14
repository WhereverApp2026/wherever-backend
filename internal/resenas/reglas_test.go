package resenas

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestDistanciaM_Haversine(t *testing.T) {
	// Zócalo de la CDMX: 0.001° de latitud ≈ 111 m.
	assert.Equal(t, 0, DistanciaM(19.4326, -99.1332, 19.4326, -99.1332))
	assert.InDelta(t, 111, DistanciaM(19.4326, -99.1332, 19.4336, -99.1332), 1)
	assert.InDelta(t, 56, DistanciaM(19.4326, -99.1332, 19.4331, -99.1332), 1)
}

func TestMotivoUbicacion(t *testing.T) {
	assert.Empty(t, MotivoUbicacion(100, 100), "en el borde del radio y la precisión")
	assert.Equal(t, MotivoFueraDeRadio, MotivoUbicacion(101, 10))
	assert.Equal(t, MotivoFueraDeRadio, MotivoUbicacion(10, 101), "precisión no creíble")
}

func TestSaltoImposible(t *testing.T) {
	assert.False(t, SaltoImposible(0.8, time.Second), "vecinos: abajo de 1 km no se revisa")
	assert.False(t, SaltoImposible(60, time.Hour), "60 km/h")
	assert.False(t, SaltoImposible(300, time.Hour), "justo 300 km/h")
	assert.True(t, SaltoImposible(301, time.Hour))
	assert.True(t, SaltoImposible(50, 5*time.Minute), "600 km/h")
	assert.True(t, SaltoImposible(5, 0), "sin tiempo transcurrido")
}

func TestDia_EnZonaDelNegocio(t *testing.T) {
	loc := negocios.MexicoCity()
	// 05:30 UTC es 23:30 del día anterior en la CDMX (UTC-6).
	assert.Equal(t, "2026-11-13", Dia(time.Date(2026, 11, 14, 5, 30, 0, 0, time.UTC), loc))
	assert.Equal(t, "2026-11-14", Dia(time.Date(2026, 11, 14, 6, 0, 0, 0, time.UTC), loc))
}

func TestEditable_48Horas(t *testing.T) {
	creada := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	assert.True(t, Editable(creada, creada.Add(48*time.Hour)))
	assert.False(t, Editable(creada, creada.Add(48*time.Hour+time.Second)))
}

func TestPromedio_Redondeo(t *testing.T) {
	assert.Equal(t, 0.0, Promedio(0, 0))
	assert.Equal(t, 4.5, Promedio(9, 2))
	assert.Equal(t, 4.3, Promedio(13, 3)) // 4.333…
	assert.Equal(t, 4.7, Promedio(14, 3)) // 4.666…
	assert.Equal(t, 4.2, Promedio(42, 10))
}

func TestChipsValidos(t *testing.T) {
	assert.True(t, ChipsValidos(nil))
	assert.True(t, ChipsValidos([]string{"buena_comida", "lento"}))
	assert.False(t, ChipsValidos([]string{"inventado"}))
	assert.False(t, ChipsValidos([]string{"lento", "lento"}))
	assert.False(t, ChipsValidos(Chips[:6]), "más de 5")
}

func TestAporte_YDiferencia(t *testing.T) {
	r := Resena{Calificacion: 4, Aspectos: Aspectos{5, 4, 3, 2, 1}}
	assert.True(t, r.Aporte().Cero(), "sin contar no aporta")
	r.Contada = true
	assert.Equal(t, negocios.Suma{Total: 1, Estrellas: 4, Sabor: 5, Atencion: 4, Ambiente: 3, Espera: 2, PrecioCalidad: 1}, r.Aporte())

	editada := r
	editada.Calificacion, editada.Aspectos.Sabor = 2, 3
	d := editada.Aporte().Menos(r.Aporte())
	assert.Equal(t, negocios.Suma{Estrellas: -2, Sabor: -2}, d, "editar solo mueve la diferencia")

	borrada := r
	borrada.Contada = false
	assert.Equal(t, int64(-1), borrada.Aporte().Menos(r.Aporte()).Total)
}

func TestInicioPeriodo(t *testing.T) {
	loc := negocios.MexicoCity()
	t1 := time.Date(2026, 11, 14, 3, 0, 0, 0, time.UTC) // 13 de noviembre local
	assert.Equal(t, time.Date(2026, 11, 1, 0, 0, 0, 0, loc), inicioPeriodo(t1, "mes", loc))
	assert.Equal(t, time.Date(2026, 10, 1, 0, 0, 0, 0, loc), inicioPeriodo(t1, "trimestre", loc))
	assert.Equal(t, time.Date(2026, 1, 1, 0, 0, 0, 0, loc), inicioPeriodo(time.Date(2026, 2, 20, 12, 0, 0, 0, time.UTC), "trimestre", loc))
}

func TestTextoCola(t *testing.T) {
	assert.Equal(t, "Muy rico", textoCola(Resena{Comentario: "Muy rico"}))
	assert.Contains(t, textoCola(Resena{Calificacion: 2, Chips: []string{"lento"}}), "2 estrellas")
}

func TestRespuestaIA_Validar(t *testing.T) {
	ok := respuestaIA{Texto: "  Gracias por visitarnos.  "}
	require.NoError(t, ok.Validar())
	assert.Equal(t, "Gracias por visitarnos.", ok.Texto)
	vacia := respuestaIA{Texto: " "}
	assert.Error(t, vacia.Validar())
	larga := respuestaIA{Texto: strings.Repeat("a", 501)}
	assert.Error(t, larga.Validar())
	correo := respuestaIA{Texto: "Escríbenos a dueno@ejemplo.com"}
	assert.Error(t, correo.Validar())
}

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	lat, lng, prec, fuera := 19.43, -99.13, 20, 91.0
	id := bson.NewObjectID().Hex()
	asp := AspectosRequest{5, 4, 3, 2, 1}
	ok := CrearRequest{CheckinID: id, Calificacion: 5, Aspectos: asp}
	with := func(f func(*CrearRequest)) CrearRequest { r := ok; f(&r); return r }
	cero, seis := 0, 6
	cases := []struct {
		name string
		dto  any
		ok   bool
	}{
		{"checkin por ubicación", CheckinRequest{NegocioID: id, Metodo: MetodoUbicacion, Lat: &lat, Lng: &lng, PrecisionM: &prec}, true},
		{"ubicación sin precisión", CheckinRequest{NegocioID: id, Metodo: MetodoUbicacion, Lat: &lat, Lng: &lng}, false},
		{"latitud fuera", CheckinRequest{NegocioID: id, Metodo: MetodoUbicacion, Lat: &fuera, Lng: &lng, PrecisionM: &prec}, false},
		{"checkin por qr", CheckinRequest{NegocioID: id, Metodo: MetodoQR, Codigo: "abc"}, true},
		{"qr sin código", CheckinRequest{NegocioID: id, Metodo: MetodoQR}, false},
		{"método raro", CheckinRequest{NegocioID: id, Metodo: "reserva", Codigo: "abc"}, false},
		{"negocio no es id", CheckinRequest{NegocioID: "x", Metodo: MetodoQR, Codigo: "abc"}, false},
		{"reseña mínima", ok, true},
		{"calificación 0", with(func(r *CrearRequest) { r.Calificacion = 0 }), false},
		{"aspecto 6", with(func(r *CrearRequest) { r.Aspectos.Espera = 6 }), false},
		{"sin aspectos", with(func(r *CrearRequest) { r.Aspectos = AspectosRequest{} }), false},
		{"comentario de 501", with(func(r *CrearRequest) { r.Comentario = strings.Repeat("a", 501) }), false},
		{"4 fotos", with(func(r *CrearRequest) { r.Fotos = []string{"a", "b", "c", "d"} }), false},
		{"6 chips", with(func(r *CrearRequest) { r.Chips = Chips[:6] }), false},
		{"editar vacío", EditarRequest{}, true},
		{"editar calificación 0", EditarRequest{Calificacion: &cero}, false},
		{"editar calificación 6", EditarRequest{Calificacion: &seis}, false},
		{"útil sin intención", UtilRequest{}, false},
		{"reporte con motivo", ReporteRequest{Motivo: "spam"}, true},
		{"reporte motivo raro", ReporteRequest{Motivo: "no_me_gusto"}, false},
		{"respuesta vacía", RespuestaRequest{}, false},
		{"orden raro", PublicaQuery{Orden: "mejores", Limite: 20}, false},
		{"aspecto raro", PublicaQuery{Orden: "utiles", Aspecto: "pantalla", Limite: 20}, false},
		{"periodo semana", ResumenQuery{Periodo: "semana"}, false},
		{"periodo trimestre", ResumenQuery{Periodo: "trimestre"}, true},
	}
	for _, c := range cases {
		err := v.Struct(c.dto)
		if c.ok {
			assert.NoError(t, err, c.name)
		} else {
			assert.Error(t, err, c.name)
		}
	}
}

func TestValidarAspectos_Patch(t *testing.T) {
	s := &Service{}
	require.NoError(t, s.validarAspectos(AspectosRequest{1, 2, 3, 4, 5}))
	assert.Error(t, s.validarAspectos(AspectosRequest{0, 2, 3, 4, 5}))
}
