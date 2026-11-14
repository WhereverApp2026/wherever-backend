package postales

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

func TestEstadoPostal_MapeoDeImagenYTexto(t *testing.T) {
	cases := []struct{ imagen, texto, want string }{
		{imagenes.EstadoPublicada, TextoAprobado, EstadoLista},
		{imagenes.EstadoEnRevision, TextoAprobado, EstadoEnModeracion},
		{imagenes.EstadoPublicada, TextoEnRevision, EstadoEnModeracion},
		{imagenes.EstadoRechazada, TextoAprobado, EstadoRechazada},
		{imagenes.EstadoPublicada, TextoRechazado, EstadoRechazada},
		{imagenes.EstadoEnRevision, TextoRechazado, EstadoRechazada},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, EstadoPostal(c.imagen, c.texto), "%s + %s", c.imagen, c.texto)
	}
	assert.True(t, ImagenUsable(imagenes.EstadoPublicada))
	assert.True(t, ImagenUsable(imagenes.EstadoEnRevision), "la cuarentena de nv-45 cuenta")
	for _, e := range []string{imagenes.EstadoPendiente, imagenes.EstadoProcesando, imagenes.EstadoRechazada} {
		assert.False(t, ImagenUsable(e), e)
	}
}

func TestPuedeVer_Matriz(t *testing.T) {
	dueno := Relacion{Dueno: true}
	amigo := Relacion{Amigos: true}
	extrano := Relacion{}
	bloqueado := Relacion{Bloqueado: true}
	cases := []struct {
		name  string
		rel   Relacion
		vis   string
		listo bool
		want  bool
	}{
		{"dueño ve su privada sin moderar", dueno, VisPrivada, false, true},
		{"amigo no ve privada", amigo, VisPrivada, true, false},
		{"amigo ve amigos lista", amigo, VisAmigos, true, true},
		{"amigo no ve amigos en moderación", amigo, VisAmigos, false, false},
		{"extraño no ve amigos", extrano, VisAmigos, true, false},
		{"extraño ve pública lista", extrano, VisPublica, true, true},
		{"extraño no ve pública en moderación", extrano, VisPublica, false, false},
		{"bloqueado no ve pública", bloqueado, VisPublica, true, false},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, PuedeVer(c.rel, c.vis, c.listo), c.name)
	}
}

func TestCompatible_MomentoNoMasAbiertoQueLaPostal(t *testing.T) {
	assert.True(t, Compatible(VisPrivada, VisPublica), "la privada toma la del momento")
	assert.True(t, Compatible(VisPublica, VisAmigos))
	assert.True(t, Compatible(VisAmigos, VisAmigos))
	assert.False(t, Compatible(VisAmigos, VisPublica))
}

func TestPlanPasado_PestanaPasados(t *testing.T) {
	loc := negocios.MexicoCity()
	now := time.Date(2026, 11, 14, 1, 0, 0, 0, loc) // 1:00 en México
	assert.True(t, PlanPasado(time.Date(2026, 11, 13, 20, 0, 0, 0, loc), now, loc), "ayer")
	assert.False(t, PlanPasado(time.Date(2026, 11, 14, 0, 0, 0, 0, loc), now, loc), "hoy aún no pasa")
	assert.False(t, PlanPasado(time.Date(2026, 11, 20, 0, 0, 0, 0, loc), now, loc), "futuro")
}

func TestRangoMesYCentavos(t *testing.T) {
	loc := negocios.MexicoCity()
	desde, hasta, ok := RangoMes("2026-02", loc)
	require.True(t, ok)
	assert.Equal(t, time.Date(2026, 2, 1, 0, 0, 0, 0, loc).UTC(), desde)
	assert.Equal(t, time.Date(2026, 3, 1, 0, 0, 0, 0, loc).UTC(), hasta)
	_, _, ok = RangoMes("2026-13", loc)
	assert.False(t, ok)
	assert.Equal(t, int64(125000), Centavos(1250))
}

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	img := strings.Repeat("ab", 16)
	oid := bson.NewObjectID().Hex()
	etiq := func(n int) []string {
		out := make([]string, 0, n)
		for range n {
			out = append(out, bson.NewObjectID().Hex())
		}
		return out
	}
	largo := func(n int) *string { s := strings.Repeat("a", n); return &s }
	mal := "mal"
	cases := []struct {
		name string
		dto  any
		ok   bool
	}{
		{"postal mínima", CrearPostalRequest{ImagenID: img, Estilo: EstiloClasica, Visibilidad: VisPrivada}, true},
		{"postal completa", CrearPostalRequest{ImagenID: img, Estilo: EstiloPasaporte, Texto: *largo(80), PlanID: oid, NegocioID: oid, Visibilidad: VisPublica}, true},
		{"imagen corta", CrearPostalRequest{ImagenID: "abc", Estilo: EstiloClasica, Visibilidad: VisPrivada}, false},
		{"estilo desconocido", CrearPostalRequest{ImagenID: img, Estilo: "acuarela", Visibilidad: VisPrivada}, false},
		{"texto de 81", CrearPostalRequest{ImagenID: img, Estilo: EstiloClasica, Texto: *largo(81), Visibilidad: VisPrivada}, false},
		{"sin visibilidad", CrearPostalRequest{ImagenID: img, Estilo: EstiloClasica}, false},
		{"editar vacío", EditarPostalRequest{}, true},
		{"editar visibilidad mala", EditarPostalRequest{Visibilidad: &mal}, false},
		{"momento con 10 etiquetados", MomentoRequest{PostalID: oid, Visibilidad: VisAmigos, Etiquetados: etiq(10)}, true},
		{"momento con 11 etiquetados", MomentoRequest{PostalID: oid, Visibilidad: VisAmigos, Etiquetados: etiq(11)}, false},
		{"momento privado", MomentoRequest{PostalID: oid, Visibilidad: VisPrivada}, false},
		{"momento texto 281", MomentoRequest{PostalID: oid, Visibilidad: VisPublica, Texto: *largo(281)}, false},
		{"momento etiquetado repetido", MomentoRequest{PostalID: oid, Visibilidad: VisPublica, Etiquetados: []string{oid, oid}}, false},
		{"editar momento texto 281", EditarMomentoRequest{Texto: largo(281)}, false},
		{"álbum por mes", AlbumQuery{Mes: "2026-11", Limite: 20}, true},
		{"álbum mes malo", AlbumQuery{Mes: "11-2026", Limite: 20}, false},
		{"límite 51", ListaQuery{Limite: 51}, false},
		{"tarjeta mala", RecuerdoRequest{TarjetaImagenID: "zz"}, false},
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

func TestRespuestaPostal_SoloElDuenoVeImagenYPlan(t *testing.T) {
	plan := bson.NewObjectID()
	p := Postal{ID: bson.NewObjectID(), ImagenID: "x", ImagenEstado: imagenes.EstadoEnRevision, ImagenURL: "https://img/i/x/1080.jpg", PlanID: &plan, Estado: EstadoEnModeracion}
	propia := respuestaPostal(p, true, nil)
	assert.Equal(t, "x", propia.ImagenID)
	assert.NotNil(t, propia.PlanID)
	assert.Empty(t, propia.ImagenURL, "sin publicar no hay URL")
	p.ImagenEstado = imagenes.EstadoPublicada
	ajena := respuestaPostal(p, false, &AutorView{NombreUsuario: "ana"})
	assert.Empty(t, ajena.ImagenID)
	assert.Nil(t, ajena.PlanID)
	assert.Equal(t, "https://img/i/x/1080.jpg", ajena.ImagenURL)
}
