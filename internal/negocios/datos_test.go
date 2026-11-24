package negocios

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

var ahora90 = time.Date(2026, 11, 23, 18, 0, 0, 0, time.UTC)

func TestValidateExperiencias(t *testing.T) {
	assert.Empty(t, ValidateExperiencias([]string{"desayuno", "romantico"}))
	assert.Empty(t, ValidateExperiencias([]string{"desayuno", "comida", "cena", "paseo"}))
	for _, mal := range [][]string{nil, {}, {"desayuno", "comida", "cena", "paseo", "amigos"}, {"brunch"}, {"cena", "cena"}, {"Cena"}} {
		fields := ValidateExperiencias(mal)
		require.Len(t, fields, 1, "%v", mal)
		assert.Equal(t, "experiencias", fields[0].Field)
	}
}

func TestValidateAnioApertura(t *testing.T) {
	assert.Empty(t, ValidateAnioApertura(1900, ahora90))
	assert.Empty(t, ValidateAnioApertura(2026, ahora90))
	assert.NotEmpty(t, ValidateAnioApertura(1899, ahora90))
	assert.NotEmpty(t, ValidateAnioApertura(2027, ahora90), "año futuro")
	// El 1 de enero a las 05:00 UTC en México todavía es el año anterior.
	assert.NotEmpty(t, ValidateAnioApertura(2027, time.Date(2027, 1, 1, 5, 0, 0, 0, time.UTC)))
}

func TestWhatsApp_NormalizaValidaYEnlaza(t *testing.T) {
	n := NormalizeWhatsApp("+52 (773) 123-45.67")
	assert.Equal(t, "+527731234567", n)
	assert.Empty(t, ValidateWhatsApp(n))
	assert.Empty(t, ValidateWhatsApp(""), "vacío lo quita")
	for _, mal := range []string{"7731234567", "+17731234567", "+52773123456", "+5277312345678", "+52773abc4567"} {
		assert.NotEmpty(t, ValidateWhatsApp(mal), mal)
	}
	assert.Equal(t, "https://wa.me/527731234567", WhatsAppURL(n))
	assert.Empty(t, WhatsAppURL(""))
}

func TestValidateRedes(t *testing.T) {
	assert.Empty(t, ValidateRedes(Redes{}))
	assert.Empty(t, ValidateRedes(Redes{Instagram: "cafe.la_esquina", SitioWeb: "https://cafe-esquina.mx/menu?x=1"}))
	casos := map[string]Redes{
		"instagram con @":       {Instagram: "@cafe"},
		"instagram con espacio": {Instagram: "cafe esquina"},
		"instagram largo":       {Instagram: strings.Repeat("a", 31)},
		"sitio http":            {SitioWeb: "http://cafe.mx"},
		"javascript":            {SitioWeb: "javascript:alert(1)"},
		"data":                  {SitioWeb: "data:text/html,hola"},
		"acortador":             {SitioWeb: "https://bit.ly/abc"},
		"subdominio acortador":  {SitioWeb: "https://www.tinyurl.com/abc"},
		"con usuario":           {SitioWeb: "https://cafe.mx@malo.com"},
		"sin dominio":           {SitioWeb: "https://localhost"},
		"largo":                 {SitioWeb: "https://cafe.mx/" + strings.Repeat("a", 190)},
	}
	for nombre, r := range casos {
		assert.NotEmpty(t, ValidateRedes(r), nombre)
	}
	assert.Equal(t, Redes{Instagram: "cafe", SitioWeb: "https://cafe.mx"}, NormalizeRedes(Redes{Instagram: " cafe ", SitioWeb: " https://cafe.mx "}))
}

type textoFijo struct {
	malo bool
	err  error
}

func (f textoFijo) Prohibido(context.Context, string) (bool, error) { return f.malo, f.err }

func TestRevisarRedes(t *testing.T) {
	ctx := context.Background()
	r := Redes{Instagram: "cafe"}
	assert.NoError(t, RevisarRedes(ctx, nil, r), "sin revisor no revisa")
	assert.NoError(t, RevisarRedes(ctx, textoFijo{}, r))
	assert.NoError(t, RevisarRedes(ctx, textoFijo{malo: true}, Redes{}), "vacías no se revisan")
	err := RevisarRedes(ctx, textoFijo{malo: true}, r)
	var appErr *apperr.Error
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, 422, appErr.Kind.Status())
	assert.Equal(t, "redes.instagram", appErr.Fields[0].Field)
	err = RevisarRedes(ctx, textoFijo{err: errors.New("caído")}, r)
	require.ErrorAs(t, err, &appErr)
	assert.Equal(t, 503, appErr.Kind.Status())
}

func TestExperienciaDeHoraYCompania(t *testing.T) {
	mx := MexicoCity()
	assert.Equal(t, ExperienciaDesayuno, ExperienciaDeHora(time.Date(2026, 11, 23, 8, 0, 0, 0, mx)))
	assert.Equal(t, ExperienciaComida, ExperienciaDeHora(time.Date(2026, 11, 23, 14, 0, 0, 0, mx)))
	assert.Equal(t, ExperienciaCena, ExperienciaDeHora(time.Date(2026, 11, 23, 20, 0, 0, 0, mx)))
	assert.Empty(t, ExperienciaDeHora(time.Date(2026, 11, 23, 3, 0, 0, 0, mx)))
	assert.Equal(t, ExperienciaRomantico, ExperienciaDeCompania("pareja"))
	assert.Equal(t, ExperienciaFamilia, ExperienciaDeCompania("familia"))
	assert.Equal(t, ExperienciaAmigos, ExperienciaDeCompania("amigos"))
	assert.Empty(t, ExperienciaDeCompania("solo"))
}

func TestToDetail_DatosDelNegocio(t *testing.T) {
	b := Business{ID: bson.NewObjectID(), Phone: "+527730000000", Experiencias: []string{"cena"}, AnioApertura: 1998,
		WhatsApp: "+527731234567", Redes: &Redes{Instagram: "cafe"}}
	d := toDetail(b, ahora90)
	assert.Equal(t, []string{"cena"}, d.Experiencias)
	assert.Equal(t, 1998, d.AnioApertura)
	assert.Equal(t, "https://wa.me/527731234567", d.WhatsAppURL)
	assert.Equal(t, &Redes{Instagram: "cafe"}, d.Redes)
	// Sin datos, los campos no salen.
	b2 := Business{ID: bson.NewObjectID(), Redes: &Redes{}}
	d2 := toDetail(b2, ahora90)
	assert.Nil(t, d2.Redes)
	assert.Empty(t, d2.WhatsAppURL)
}

func TestExploreFilters_Experiencia(t *testing.T) {
	app := fiber.New()
	var got ListQuery
	app.Get("/", func(c *fiber.Ctx) error { return c.QueryParser(&got) })
	_, err := app.Test(httptest.NewRequest("GET", "/?experiencia=romantico&limite=5", nil))
	require.NoError(t, err)
	assert.Equal(t, "romantico", got.Experience)
	v, err := validation.New()
	require.NoError(t, err)
	assert.NoError(t, v.Struct(got))
	got.Experience = "brunch"
	assert.Error(t, v.Struct(got))

	s := &Service{now: time.Now}
	match, err := s.matchConditions(context.Background(), ExploreFilters{Experience: "romantico"}, nil)
	require.NoError(t, err)
	assert.Contains(t, match, bson.D{{Key: "experiencias", Value: "romantico"}})
	assert.NotEqual(t, filtersScope(ExploreFilters{Experience: "cena"}, nil), filtersScope(ExploreFilters{}, nil), "el cursor firma la experiencia")
}
