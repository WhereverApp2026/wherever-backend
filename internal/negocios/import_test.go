package negocios

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

func validRowValues() map[string]string {
	return map[string]string{
		"slug":                  "taqueria-ejemplo",
		"nombre":                "Taquería Ejemplo",
		"categoria":             "restaurante",
		"descripcion":           "Tacos de ejemplo",
		"nivel_precio":          "2",
		"precio_promedio":       "150",
		"latitud":               "19.4326",
		"longitud":              "-99.1332",
		"calle":                 "Avenida Ejemplo",
		"numero":                "123",
		"colonia":               "Centro",
		"municipio":             "Cuauhtémoc",
		"estado_republica":      "CDMX",
		"cp":                    "06000",
		"telefono":              "+5215512345678",
		"lun":                   "09:00-18:00",
		"mar":                   "09:00-18:00",
		"mie":                   "09:00-18:00",
		"jue":                   "09:00-18:00",
		"vie":                   "09:00-18:00",
		"sab":                   "10:00-16:00",
		"dom":                   "",
		"etiquetas":             "tacos|mexicana",
		"servicios":             "terraza|wifi",
		"accesibilidad":         "silla_ruedas|apto_ninos",
		"opciones_dieta":        "vegetariano",
		"alergenos":             "gluten|lacteos",
		"fotos":                 "https://example.test/fachada.jpg|https://example.test/interior.jpg|https://example.test/producto.jpg",
		"propietario_correo":    "",
		"calificacion_promedio": "4.5",
		"total_resenas":         "10",
		"acepta_reservas":       "true",
	}
}

func csvLine(values map[string]string) string {
	cells := make([]string, len(CSVHeader))
	for i, name := range CSVHeader {
		cells[i] = values[name]
	}
	return strings.Join(cells, ",")
}

func csvBody(values map[string]string) string {
	return strings.Join(CSVHeader, ",") + "\n" + csvLine(values) + "\n"
}

func validImportRow() ImportRow {
	return ImportRow{
		Slug: "taqueria-ejemplo", Name: "Taquería Ejemplo", Category: "restaurante",
		Description: "Tacos de ejemplo", PriceLevel: 2, AveragePrice: 150,
		Location: Location{Type: "Point", Coordinates: []float64{-99.1332, 19.4326}},
		Address: Direccion{
			Calle: "Avenida Ejemplo", Numero: "123", Colonia: "Centro",
			Municipio: "Cuauhtémoc", Estado: "CDMX", CP: "06000",
		},
		Phone: "+5215512345678",
		Schedules: []Horario{
			{Dia: 1, Abre: "09:00", Cierra: "18:00"},
		},
		Photos: []Foto{
			{URL: "https://example.test/fachada.jpg", Tipo: "fachada", Orden: 1},
			{URL: "https://example.test/interior.jpg", Tipo: "interior", Orden: 2},
			{URL: "https://example.test/producto.jpg", Tipo: "producto", Orden: 3},
		},
		Services:            []string{"terraza", "wifi"},
		Accessibility:       Accesibilidad{SillaRuedas: true, AptoNinos: true},
		DietOptions:         []string{"vegetariano"},
		Allergens:           []string{"gluten", "lacteos"},
		Tags:                []string{"tacos", "mexicana"},
		AcceptsReservations: true,
		Rating:              Calificacion{Promedio: 4.5, TotalResenas: 10},
	}
}

func fieldsOf(errs []RowError) []string {
	out := make([]string, 0, len(errs))
	for _, err := range errs {
		out = append(out, err.Field)
	}
	return out
}

func TestParseCSV_ValidWithBOM(t *testing.T) {
	rows, parseErrs, err := Parse(strings.NewReader("\xEF\xBB\xBF"+csvBody(validRowValues())), "csv")

	require.NoError(t, err)
	assert.Empty(t, parseErrs)
	require.Len(t, rows, 1)
	row := rows[0].row
	assert.Equal(t, "taqueria-ejemplo", row.Slug)
	assert.Equal(t, []float64{-99.1332, 19.4326}, row.Location.Coordinates)
	assert.Len(t, row.Schedules, 7)
	assert.True(t, row.Schedules[6].Cerrado, "domingo vacío = cerrado")
	assert.Len(t, row.Photos, 3)
	assert.Equal(t, "fachada", row.Photos[0].Tipo)
	assert.Equal(t, []string{"tacos", "mexicana"}, row.Tags)
	assert.True(t, row.Accessibility.SillaRuedas)
	assert.Equal(t, 4.5, row.Rating.Promedio)
}

func TestParseCSV_BadHeader(t *testing.T) {
	_, _, err := Parse(strings.NewReader("slug,nombre\nx,y\n"), "csv")
	assert.Error(t, err)
}

func TestParseCSV_NoRows(t *testing.T) {
	_, _, err := Parse(strings.NewReader(strings.Join(CSVHeader, ",")+"\n\n"), "csv")
	assert.Error(t, err)
}

func TestParseCSV_BlankRowsIgnored(t *testing.T) {
	body := strings.Join(CSVHeader, ",") + "\n" + csvLine(validRowValues()) + "\n,,,\n"
	rows, parseErrs, err := Parse(strings.NewReader(body), "csv")
	require.NoError(t, err)
	assert.Empty(t, parseErrs)
	assert.Len(t, rows, 1)
}

func TestParseCSV_TooManyRows(t *testing.T) {
	body := strings.Join(CSVHeader, ",") + "\n" + strings.Repeat(csvLine(validRowValues())+"\n", MaxImportRows+1)
	_, _, err := Parse(strings.NewReader(body), "csv")
	assert.Error(t, err)
}

func TestParseCSV_RowErrors(t *testing.T) {
	values := validRowValues()
	values["nivel_precio"] = "dos"
	_, parseErrs, err := Parse(strings.NewReader(csvBody(values)), "csv")
	require.NoError(t, err)
	require.Len(t, parseErrs, 1)
	assert.Equal(t, "nivel_precio", parseErrs[0].Field)
	assert.Equal(t, 1, parseErrs[0].Row)

	values = validRowValues()
	values["lun"] = "25:00-18:00"
	_, parseErrs, err = Parse(strings.NewReader(csvBody(values)), "csv")
	require.NoError(t, err)
	require.Len(t, parseErrs, 1)
	assert.Equal(t, "lun", parseErrs[0].Field)

	values = validRowValues()
	values["accesibilidad"] = "otra"
	_, parseErrs, err = Parse(strings.NewReader(csvBody(values)), "csv")
	require.NoError(t, err)
	require.Len(t, parseErrs, 1)
	assert.Equal(t, "accesibilidad", parseErrs[0].Field)
}

func TestParseCSV_WrongColumnCount(t *testing.T) {
	body := strings.Join(CSVHeader, ",") + "\n" + "solo-un-campo\n"
	_, parseErrs, err := Parse(strings.NewReader(body), "csv")
	require.NoError(t, err)
	require.Len(t, parseErrs, 1)
	assert.Equal(t, "archivo", parseErrs[0].Field)
}

func TestParse_UnknownFormat(t *testing.T) {
	_, _, err := Parse(strings.NewReader(""), "xml")
	assert.Error(t, err)
}

func TestValidateRows_Valid(t *testing.T) {
	rows, _, err := Parse(strings.NewReader(csvBody(validRowValues())), "csv")
	require.NoError(t, err)
	valid, errs := validateRows(rows)
	assert.Empty(t, errs)
	require.Len(t, valid, 1)
	assert.Equal(t, "taqueria ejemplo", text.Normalize(valid[0].row.Name))
	assert.Equal(t, 4.5, valid[0].row.Rating.Aspectos.Sabor)
}

func TestValidateRows_CategoryPriceAndLocation(t *testing.T) {
	values := validRowValues()
	values["categoria"] = "otro"
	values["nivel_precio"] = "5"
	values["latitud"] = "40"
	rows, _, err := Parse(strings.NewReader(csvBody(values)), "csv")
	require.NoError(t, err)
	_, errs := validateRows(rows)
	assert.Contains(t, fieldsOf(errs), "categoria")
	assert.Contains(t, fieldsOf(errs), "nivel_precio")
	assert.Contains(t, fieldsOf(errs), "ubicacion")
}

func TestValidateRows_Photos(t *testing.T) {
	values := validRowValues()
	values["fotos"] = "https://example.test/una.jpg"
	rows, _, err := Parse(strings.NewReader(csvBody(values)), "csv")
	require.NoError(t, err)
	_, errs := validateRows(rows)
	assert.Contains(t, fieldsOf(errs), "fotos")
}

func TestValidateRows_DuplicateSlug(t *testing.T) {
	body := strings.Join(CSVHeader, ",") + "\n" + csvLine(validRowValues()) + "\n" + csvLine(validRowValues()) + "\n"
	rows, _, err := Parse(strings.NewReader(body), "csv")
	require.NoError(t, err)
	valid, errs := validateRows(rows)
	assert.Len(t, valid, 1)
	require.Len(t, errs, 1)
	assert.Equal(t, 2, errs[0].Row)
	assert.Equal(t, "slug", errs[0].Field)
}

func TestValidateRows_ListSeparatorAndDedupe(t *testing.T) {
	values := validRowValues()
	values["etiquetas"] = "tacos|mexicana|tacos"
	rows, _, err := Parse(strings.NewReader(csvBody(values)), "csv")
	require.NoError(t, err)
	valid, errs := validateRows(rows)
	assert.Empty(t, errs)
	require.Len(t, valid, 1)
	assert.Equal(t, []string{"tacos", "mexicana"}, valid[0].row.Tags)
}

func TestValidateRows_RatingAndPhone(t *testing.T) {
	values := validRowValues()
	values["calificacion_promedio"] = "0"
	values["telefono"] = "123"
	rows, _, err := Parse(strings.NewReader(csvBody(values)), "csv")
	require.NoError(t, err)
	_, errs := validateRows(rows)
	assert.Contains(t, fieldsOf(errs), "calificacion")
	assert.Contains(t, fieldsOf(errs), "telefono")
}

func TestValidateRows_AllergenNoneGoesAlone(t *testing.T) {
	values := validRowValues()
	values["alergenos"] = "ninguno"
	rows, _, err := Parse(strings.NewReader(csvBody(values)), "csv")
	require.NoError(t, err)
	_, errs := validateRows(rows)
	assert.Empty(t, errs)

	values["alergenos"] = "ninguno|gluten"
	rows, _, err = Parse(strings.NewReader(csvBody(values)), "csv")
	require.NoError(t, err)
	_, errs = validateRows(rows)
	assert.Contains(t, fieldsOf(errs), "alergenos")
}

func TestValidateRows_OwnerEmail(t *testing.T) {
	values := validRowValues()
	values["propietario_correo"] = "no-es-correo"
	rows, _, err := Parse(strings.NewReader(csvBody(values)), "csv")
	require.NoError(t, err)
	_, errs := validateRows(rows)
	assert.Contains(t, fieldsOf(errs), "propietario_correo")
}

func TestParseJSON_MultipleScheduleSegments(t *testing.T) {
	row := validImportRow()
	row.Schedules = []Horario{
		{Dia: 1, Abre: "09:00", Cierra: "14:00"},
		{Dia: 1, Abre: "18:00", Cierra: "22:00"},
	}
	body, err := json.Marshal([]ImportRow{row})
	require.NoError(t, err)
	rows, parseErrs, err := Parse(strings.NewReader(string(body)), "json")
	require.NoError(t, err)
	assert.Empty(t, parseErrs)
	require.Len(t, rows, 1)
	valid, errs := validateRows(rows)
	assert.Empty(t, errs)
	require.Len(t, valid, 1)
	assert.Len(t, valid[0].row.Schedules, 2)
}

func TestParseJSON_TooManyRows(t *testing.T) {
	rows := make([]ImportRow, 0, MaxImportRows+1)
	for i := 0; i <= MaxImportRows; i++ {
		row := validImportRow()
		row.Slug = fmt.Sprintf("slug-%d", i)
		rows = append(rows, row)
	}
	body, err := json.Marshal(rows)
	require.NoError(t, err)
	_, _, err = Parse(strings.NewReader(string(body)), "json")
	assert.Error(t, err)
}

func TestParseJSON_Errors(t *testing.T) {
	for name, body := range map[string]string{
		"vacío":         "[]",
		"roto":          "[{",
		"campo extra":   `[{"slug":"x","desconocido":1}]`,
		"datos extra":   `[] {}`,
		"no es arreglo": `{}`,
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := Parse(strings.NewReader(body), "json")
			assert.Error(t, err)
		})
	}
}

func TestSeedTemplate_IsValid(t *testing.T) {
	file, err := os.Open("../../docs/semilla-negocios.csv")
	require.NoError(t, err)
	defer func() { _ = file.Close() }()

	rows, parseErrs, err := Parse(file, "csv")
	require.NoError(t, err)
	assert.Empty(t, parseErrs)
	valid, errs := validateRows(rows)
	assert.Empty(t, errs)
	assert.Len(t, valid, 2)
}

func TestDeriveScheduleMin_CrossesMidnight(t *testing.T) {
	got := deriveScheduleMin([]Horario{
		{Dia: 5, Abre: "22:00", Cierra: "02:00"},
		{Dia: 6, Cerrado: true},
	})

	assert.Equal(t, []VentanaMin{{Dia: 5, Desde: 22 * 60, Hasta: 26 * 60}}, got)
}
