package alta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

func campos(fields []apperr.FieldError) []string {
	out := []string{}
	for _, f := range fields {
		out = append(out, f.Field)
	}
	return out
}

func TestFaltantes_ListaLoQueFalta(t *testing.T) {
	vacio := negocios.Business{}
	assert.Equal(t, []string{"ubicacion", "horarios", "fotos", "precios", "ine_frente", "ine_reverso", "comprobante_domicilio", "consentimiento"},
		campos(Faltantes(vacio, nil, false)))

	completo := negocios.Business{
		Location:    negocios.Location{Type: "Point", Coordinates: []float64{-99.13, 19.43}},
		ScheduleMin: []negocios.VentanaMin{{Dia: 1, Desde: 540, Hasta: 1080}},
		Photos:      []negocios.Foto{{ImagenID: "a"}, {ImagenID: "b"}, {ImagenID: "c"}},
		PriceLevel:  2,
	}
	docs := map[string]bool{expediente.TipoINEFrente: true, expediente.TipoINEReverso: true, expediente.TipoComprobante: true}
	assert.Empty(t, Faltantes(completo, docs, true), "fiscales y CLABE no frenan la revisión")

	delete(docs, expediente.TipoINEReverso)
	assert.Equal(t, []string{"ine_reverso"}, campos(Faltantes(completo, docs, true)))
}

func TestSlugDe_NormalizaYEsUnico(t *testing.T) {
	id := bson.NewObjectID()
	s := slugDe("Café  La Ñora!!", id)
	assert.Regexp(t, `^[a-z0-9-]+$`, s)
	assert.Contains(t, s, "cafe-la-nora-")
	assert.Equal(t, id.Hex()[16:], s[len(s)-8:])
	assert.Contains(t, slugDe("¡¡!!", id), "negocio-")
	assert.LessOrEqual(t, len(slugDe("a very long business name that keeps going and going beyond sixty chars", id)), 69)
}

func TestEditable_SoloBorradorORechazado(t *testing.T) {
	for _, st := range []string{negocios.StatusDraft, negocios.StatusRejected} {
		assert.NoError(t, editable(negocios.Business{State: st}))
	}
	for _, st := range []string{negocios.StatusReview, negocios.StatusApproved, negocios.StatusPaused} {
		err := editable(negocios.Business{State: st})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "alta_cerrada")
	}
}

func TestNegocioResponse_SinDatosSensibles(t *testing.T) {
	b := negocios.Business{
		ID: bson.NewObjectID(), Name: "Taquería", State: negocios.StatusDraft, Step: 2, Version: 3,
		Photos:         []negocios.Foto{{ImagenID: "img1", URL: "https://x/i/img1", Tipo: "fachada"}},
		CuentaDeposito: &negocios.CuentaMascara{Ultimos4: "9713", Banco: "BBVA MEXICO", Titular: "Ana", Estado: EstadoSinVerificar},
		Documentos:     negocios.Documentos{INE: true},
	}
	out := negocioResponse(b)
	assert.Equal(t, "img1", out.Fotos[0].ID)
	assert.Equal(t, "9713", out.CuentaDeposito.Ultimos4)
	assert.Nil(t, out.Ubicacion, "sin paso 2 no hay ubicación")
	assert.Equal(t, []string{}, out.Servicios)
	item := colaItem(b, b.CreatedAt)
	assert.Equal(t, 1, item.Fotos)
	assert.True(t, item.Documentos.INE)
}
