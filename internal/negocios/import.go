package negocios

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// CSVHeader es el encabezado exacto de la semilla (A2 §1). El orden importa.
var CSVHeader = []string{
	"slug", "nombre", "categoria", "descripcion", "nivel_precio", "precio_promedio",
	"latitud", "longitud", "calle", "numero", "colonia", "municipio", "estado_republica", "cp",
	"telefono", "lun", "mar", "mie", "jue", "vie", "sab", "dom",
	"etiquetas", "servicios", "accesibilidad", "opciones_dieta", "alergenos", "fotos",
	"propietario_correo", "calificacion_promedio", "total_resenas", "acepta_reservas",
}

var (
	csvIndex = func() map[string]int {
		index := make(map[string]int, len(CSVHeader))
		for i, name := range CSVHeader {
			index[name] = i
		}
		return index
	}()
	slugPattern  = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	cpPattern    = regexp.MustCompile(`^[0-9]{5}$`)
	phonePattern = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
)

// ImportRow es un renglón de la semilla. El JSON usa los nombres del modelo; el CSV los
// aplana en columnas.
type ImportRow struct {
	Slug                string        `json:"slug"`
	Name                string        `json:"nombre"`
	Category            string        `json:"categoria"`
	Description         string        `json:"descripcion"`
	PriceLevel          int           `json:"nivel_precio"`
	AveragePrice        int           `json:"precio_promedio"`
	Location            Location      `json:"ubicacion"`
	Address             Direccion     `json:"direccion"`
	Phone               string        `json:"telefono"`
	Schedules           []Horario     `json:"horarios"`
	Photos              []Foto        `json:"fotos"`
	Services            []string      `json:"servicios"`
	Accessibility       Accesibilidad `json:"accesibilidad"`
	DietOptions         []string      `json:"opciones_dieta"`
	Allergens           []string      `json:"alergenos"`
	Tags                []string      `json:"etiquetas"`
	AcceptsReservations bool          `json:"acepta_reservas"`
	Rating              Calificacion  `json:"calificacion"`
	OwnerEmail          string        `json:"propietario_correo"`
}

// parsedRow guarda el número de fila de datos (la primera después del encabezado es 1).
type parsedRow struct {
	number int
	row    ImportRow
}

// Parse lee un CSV o un arreglo JSON. Los errores de fila se devuelven en `[]RowError`;
// solo los errores de archivo (encabezado, formato, tope) son fatales.
func Parse(r io.Reader, format string) ([]parsedRow, []RowError, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "csv":
		return parseCSV(r)
	case "json":
		return parseJSON(r)
	default:
		return nil, nil, apperr.Validation("datos inválidos", apperr.FieldError{Field: "archivo", Message: "formato no soportado (csv o json)"})
	}
}

func parseJSON(r io.Reader) ([]parsedRow, []RowError, error) {
	var rows []ImportRow
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&rows); err != nil {
		return nil, nil, apperr.BadRequest("cuerpo JSON inválido").WithDetail(err.Error())
	}
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return nil, nil, apperr.BadRequest("cuerpo JSON inválido").WithDetail("hay datos después del arreglo")
	}
	if len(rows) == 0 {
		return nil, nil, apperr.Validation("datos inválidos", apperr.FieldError{Field: "archivo", Message: "el archivo no tiene negocios"})
	}
	if len(rows) > MaxImportRows {
		return nil, nil, apperr.Validation("datos inválidos", apperr.FieldError{Field: "archivo", Message: fmt.Sprintf("máximo %d negocios por archivo", MaxImportRows)})
	}
	out := make([]parsedRow, 0, len(rows))
	for i, row := range rows {
		out = append(out, parsedRow{number: i + 1, row: row})
	}
	return out, []RowError{}, nil
}

func parseCSV(r io.Reader) ([]parsedRow, []RowError, error) {
	records, err := readCSV(r)
	if err != nil {
		return nil, nil, err
	}
	if err := checkRowCount(countDataRows(records[1:])); err != nil {
		return nil, nil, err
	}
	rows := make([]parsedRow, 0, len(records)-1)
	var errs []RowError
	for i, rec := range records[1:] {
		if allEmpty(rec) {
			continue
		}
		row, rowErrs := parseCSVRow(rec, i+1)
		if len(rowErrs) > 0 {
			errs = append(errs, rowErrs...)
			continue
		}
		rows = append(rows, parsedRow{number: i + 1, row: row})
	}
	return rows, errs, nil
}

// fileError es un 422 del archivo completo (encabezado, formato o tope).
func fileError(message string) error {
	return apperr.Validation("datos inválidos", apperr.FieldError{Field: "archivo", Message: message})
}

// readCSV lee todo el archivo, descarta el BOM y comprueba el encabezado exacto.
func readCSV(r io.Reader) ([][]string, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, apperr.BadRequest("no se pudo leer el archivo").WithDetail(err.Error())
	}
	reader := csv.NewReader(bytes.NewReader(bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})))
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fileError("CSV inválido: " + err.Error())
	}
	if len(records) == 0 {
		return nil, fileError("el archivo no tiene negocios")
	}
	if strings.Join(records[0], ",") != strings.Join(CSVHeader, ",") {
		return nil, fileError("encabezado esperado: " + strings.Join(CSVHeader, ","))
	}
	return records, nil
}

func countDataRows(records [][]string) int {
	n := 0
	for _, rec := range records {
		if !allEmpty(rec) {
			n++
		}
	}
	return n
}

func checkRowCount(n int) error {
	if n == 0 {
		return fileError("el archivo no tiene negocios")
	}
	if n > MaxImportRows {
		return fileError(fmt.Sprintf("máximo %d negocios por archivo", MaxImportRows))
	}
	return nil
}

// csvRow lee las columnas de un renglón y acumula sus errores de formato.
type csvRow struct {
	rec    []string
	number int
	errs   []RowError
}

func (r *csvRow) get(name string) string { return strings.TrimSpace(r.rec[csvIndex[name]]) }

func (r *csvRow) add(field, message string) {
	r.errs = append(r.errs, RowError{Row: r.number, Field: field, Message: message})
}

// intValue deja dst en cero si la celda viene vacía; la validación decide si falta.
func (r *csvRow) intValue(name string, dst *int) {
	value := r.get(name)
	if value == "" {
		return
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		r.add(name, "debe ser un número entero")
		return
	}
	*dst = n
}

func (r *csvRow) floatValue(name string, dst *float64) {
	value := r.get(name)
	if value == "" {
		return
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		r.add(name, "debe ser un número")
		return
	}
	*dst = f
}

// requiredFloat es para las coordenadas: vacías también son error.
func (r *csvRow) requiredFloat(name string) float64 {
	f, err := strconv.ParseFloat(r.get(name), 64)
	if err != nil {
		r.add(name, "debe ser un número")
	}
	return f
}

func (r *csvRow) boolValue(name string, dst *bool) {
	value := r.get(name)
	if value == "" {
		return
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		r.add(name, "debe ser true o false")
		return
	}
	*dst = b
}

// csvDays son las columnas de horario; `dom` es el día 0 de Go.
var csvDays = []struct {
	column string
	day    int
}{
	{"lun", 1}, {"mar", 2}, {"mie", 3}, {"jue", 4}, {"vie", 5}, {"sab", 6}, {"dom", 0},
}

func (r *csvRow) schedules() []Horario {
	out := make([]Horario, 0, len(csvDays))
	for _, item := range csvDays {
		horario, err := parseHorario(r.get(item.column), item.day)
		if err != nil {
			r.add(item.column, err.Error())
			continue
		}
		out = append(out, horario)
	}
	return out
}

func (r *csvRow) accessibility() Accesibilidad {
	var out Accesibilidad
	flags := map[string]*bool{"silla_ruedas": &out.SillaRuedas, "apto_ninos": &out.AptoNinos, "pet_friendly": &out.PetFriendly}
	for _, flag := range splitList(r.get("accesibilidad")) {
		dst, ok := flags[flag]
		if !ok {
			r.add("accesibilidad", "valor no válido: "+flag)
			continue
		}
		*dst = true
	}
	return out
}

func photosFromURLs(urls []string) []Foto {
	out := make([]Foto, 0, len(urls))
	for i, url := range urls {
		out = append(out, Foto{URL: url, Tipo: photoType(i), Orden: i + 1})
	}
	return out
}

func parseCSVRow(rec []string, number int) (ImportRow, []RowError) {
	r := &csvRow{rec: rec, number: number}
	if len(rec) != len(CSVHeader) {
		r.add("archivo", fmt.Sprintf("se esperaban %d columnas, llegaron %d", len(CSVHeader), len(rec)))
		return ImportRow{}, r.errs
	}
	row := ImportRow{
		Slug: r.get("slug"), Name: r.get("nombre"), Category: r.get("categoria"), Description: r.get("descripcion"),
		Address: Direccion{
			Calle: r.get("calle"), Numero: r.get("numero"), Colonia: r.get("colonia"),
			Municipio: r.get("municipio"), Estado: r.get("estado_republica"), CP: r.get("cp"),
		},
		Phone:       r.get("telefono"),
		OwnerEmail:  r.get("propietario_correo"),
		Tags:        splitList(r.get("etiquetas")),
		Services:    splitList(r.get("servicios")),
		DietOptions: splitList(r.get("opciones_dieta")),
		Allergens:   splitList(r.get("alergenos")),
		Photos:      photosFromURLs(splitList(r.get("fotos"))),
	}
	r.intValue("nivel_precio", &row.PriceLevel)
	r.intValue("precio_promedio", &row.AveragePrice)
	lat, lon := r.requiredFloat("latitud"), r.requiredFloat("longitud")
	row.Location = Location{Type: "Point", Coordinates: []float64{lon, lat}}
	r.floatValue("calificacion_promedio", &row.Rating.Promedio)
	r.intValue("total_resenas", &row.Rating.TotalResenas)
	r.boolValue("acepta_reservas", &row.AcceptsReservations)
	row.Schedules = r.schedules()
	row.Accessibility = r.accessibility()
	return row, r.errs
}

func parseHorario(value string, day int) (Horario, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return Horario{Dia: day, Cerrado: true}, nil
	}
	parts := strings.SplitN(value, "-", 2)
	if len(parts) != 2 {
		return Horario{}, fmt.Errorf("debe tener el formato HH:MM-HH:MM")
	}
	open, close := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	if _, err := time.Parse("15:04", open); err != nil {
		return Horario{}, fmt.Errorf("la hora de apertura debe ser HH:MM")
	}
	if _, err := time.Parse("15:04", close); err != nil {
		return Horario{}, fmt.Errorf("la hora de cierre debe ser HH:MM")
	}
	if open == close {
		return Horario{}, fmt.Errorf("la hora de apertura y cierre no pueden ser iguales")
	}
	return Horario{Dia: day, Abre: open, Cierra: close}, nil
}

func splitList(value string) []string {
	if strings.TrimSpace(value) == "" {
		return []string{}
	}
	out := make([]string, 0)
	seen := map[string]bool{}
	for _, item := range strings.Split(value, "|") {
		item = strings.TrimSpace(item)
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		out = append(out, item)
	}
	return out
}

func allEmpty(rec []string) bool {
	for _, value := range rec {
		if strings.TrimSpace(value) != "" {
			return false
		}
	}
	return true
}

func photoType(index int) string {
	switch index {
	case 0:
		return "fachada"
	case 1:
		return "interior"
	case 2:
		return "producto"
	default:
		return "interior"
	}
}

// validateRows normaliza, detecta slugs repetidos y valida cada renglón. Devuelve solo
// los renglones válidos y los errores de los demás; no toca la base.
func validateRows(rows []parsedRow) ([]parsedRow, []RowError) {
	valid := make([]parsedRow, 0, len(rows))
	errs := make([]RowError, 0)
	seen := map[string]bool{}
	for _, item := range rows {
		normalizeRow(&item.row)
		var rowErrs []RowError
		if item.row.Slug != "" {
			if seen[item.row.Slug] {
				rowErrs = append(rowErrs, RowError{Row: item.number, Field: "slug", Message: "slug repetido en el archivo"})
			} else {
				seen[item.row.Slug] = true
			}
		}
		for _, fieldErr := range validateRow(item.row) {
			rowErrs = append(rowErrs, RowError{Row: item.number, Field: fieldErr.Field, Message: fieldErr.Message})
		}
		if len(rowErrs) > 0 {
			errs = append(errs, rowErrs...)
			continue
		}
		valid = append(valid, item)
	}
	if len(errs) == 0 {
		errs = nil
	}
	return valid, errs
}

func normalizeRow(row *ImportRow) {
	row.Slug = strings.TrimSpace(row.Slug)
	row.Name = strings.TrimSpace(row.Name)
	row.Category = strings.TrimSpace(row.Category)
	row.Description = strings.TrimSpace(row.Description)
	row.Phone = strings.TrimSpace(row.Phone)
	row.OwnerEmail = strings.ToLower(strings.TrimSpace(row.OwnerEmail))
	row.Address.Calle = strings.TrimSpace(row.Address.Calle)
	row.Address.Numero = strings.TrimSpace(row.Address.Numero)
	row.Address.Colonia = strings.TrimSpace(row.Address.Colonia)
	row.Address.Municipio = strings.TrimSpace(row.Address.Municipio)
	row.Address.Estado = strings.TrimSpace(row.Address.Estado)
	row.Address.CP = strings.TrimSpace(row.Address.CP)
	row.Location.Type = strings.TrimSpace(row.Location.Type)
	row.Tags = dedupeTrim(row.Tags)
	row.Services = dedupeTrim(row.Services)
	row.DietOptions = dedupeTrim(row.DietOptions)
	row.Allergens = dedupeTrim(row.Allergens)
	for i := range row.Photos {
		row.Photos[i].URL = strings.TrimSpace(row.Photos[i].URL)
		row.Photos[i].Tipo = strings.TrimSpace(row.Photos[i].Tipo)
		if row.Photos[i].Orden == 0 {
			row.Photos[i].Orden = i + 1
		}
		if row.Photos[i].Tipo == "" {
			row.Photos[i].Tipo = photoType(i)
		}
	}
	for i := range row.Schedules {
		row.Schedules[i].Abre = strings.TrimSpace(row.Schedules[i].Abre)
		row.Schedules[i].Cierra = strings.TrimSpace(row.Schedules[i].Cierra)
	}
	if row.Rating.TotalResenas > 0 && row.Rating.Promedio > 0 {
		fillAspects(&row.Rating.Aspectos, row.Rating.Promedio)
	}
}

func dedupeTrim(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func fillAspects(aspects *Aspectos, average float64) {
	if aspects.Sabor == 0 {
		aspects.Sabor = average
	}
	if aspects.Atencion == 0 {
		aspects.Atencion = average
	}
	if aspects.Ambiente == 0 {
		aspects.Ambiente = average
	}
	if aspects.Espera == 0 {
		aspects.Espera = average
	}
	if aspects.PrecioCalidad == 0 {
		aspects.PrecioCalidad = average
	}
}

func validateRow(row ImportRow) []apperr.FieldError {
	var errs []apperr.FieldError
	add := func(field, message string) {
		errs = append(errs, apperr.FieldError{Field: field, Message: message})
	}
	validateIdentity(row, add)
	validatePricing(row, add)
	validateLocation(row.Location, add)
	validateAddress(row.Address, add)
	if row.Phone != "" && !phonePattern.MatchString(row.Phone) {
		add("telefono", "debe estar en formato internacional, p. ej. +5215512345678")
	}
	validateSchedules(row.Schedules, add)
	validatePhotos(row.Photos, add)
	validateList("etiquetas", row.Tags, MaxTags, add)
	validateList("servicios", row.Services, MaxServices, add)
	validateCatalog("opciones_dieta", row.DietOptions, MaxDiet, ValidDietOption, add)
	validateCatalog("alergenos", row.Allergens, MaxAllergens, ValidAllergen, add)
	validateRating(row.Rating, add)
	validateOwnerEmail(row.OwnerEmail, add)
	return errs
}

func validateIdentity(row ImportRow, add func(string, string)) {
	if n := len(row.Slug); n < 3 || n > 80 || !slugPattern.MatchString(row.Slug) {
		add("slug", "debe tener 3 a 80 caracteres en minúsculas, números y guiones")
	}
	if n := len([]rune(row.Name)); n < 2 || n > 60 {
		add("nombre", "debe tener de 2 a 60 caracteres")
	}
	if n := len([]rune(row.Description)); n < 1 || n > 160 {
		add("descripcion", "debe tener de 1 a 160 caracteres")
	}
	if !ValidCategory(row.Category) {
		add("categoria", "debe ser uno de los 6 giros de A2")
	}
}

func validatePricing(row ImportRow, add func(string, string)) {
	if row.PriceLevel < 1 || row.PriceLevel > 4 {
		add("nivel_precio", "debe estar entre 1 y 4")
	}
	if row.AveragePrice < 0 || row.AveragePrice > 100000 {
		add("precio_promedio", "debe estar entre 0 y 100000 pesos")
	}
}

func validateOwnerEmail(email string, add func(string, string)) {
	if email == "" {
		return
	}
	address, err := mail.ParseAddress(email)
	if err != nil || strings.ToLower(address.Address) != email {
		add("propietario_correo", "no es un correo válido")
	}
}

func validateLocation(location Location, add func(string, string)) {
	if location.Type != "Point" || len(location.Coordinates) != 2 {
		add("ubicacion", "debe ser un Point GeoJSON [longitud, latitud]")
		return
	}
	lon, lat := location.Coordinates[0], location.Coordinates[1]
	if !finite(lat) || !finite(lon) || lat < 14 || lat > 33 || lon < -118 || lon > -86 {
		add("ubicacion", "las coordenadas deben estar dentro de México (lat 14–33, lng −118–−86)")
	}
}

func validateAddress(address Direccion, add func(string, string)) {
	for field, value := range map[string]string{
		"calle": address.Calle, "numero": address.Numero, "colonia": address.Colonia,
		"municipio": address.Municipio, "estado_republica": address.Estado,
	} {
		if n := len([]rune(value)); n < 1 || n > 80 {
			add(field, "debe tener de 1 a 80 caracteres")
		}
	}
	if !cpPattern.MatchString(address.CP) {
		add("cp", "debe tener 5 dígitos")
	}
}

func validateSchedules(schedules []Horario, add func(string, string)) {
	open := false
	for _, horario := range schedules {
		isOpen, message := checkHorario(horario)
		if message != "" {
			add("horarios", message)
		}
		open = open || isOpen
	}
	if !open {
		add("horarios", "al menos un día debe estar abierto")
	}
}

// checkHorario dice si el tramo está abierto y, si es inválido, por qué.
func checkHorario(horario Horario) (bool, string) {
	if horario.Dia < 0 || horario.Dia > 6 {
		return false, "el día debe estar entre 0 (domingo) y 6 (sábado)"
	}
	if horario.Cerrado {
		if horario.Abre != "" || horario.Cierra != "" {
			return false, "un día cerrado no lleva horas"
		}
		return false, ""
	}
	return checkOpenHours(horario.Abre, horario.Cierra)
}

func checkOpenHours(abre, cierra string) (bool, string) {
	if abre == "" || cierra == "" {
		return false, "un día abierto necesita apertura y cierre"
	}
	if _, err := time.Parse("15:04", abre); err != nil {
		return false, "la hora de apertura debe ser HH:MM"
	}
	if _, err := time.Parse("15:04", cierra); err != nil {
		return false, "la hora de cierre debe ser HH:MM"
	}
	if abre == cierra {
		return false, "la hora de apertura y cierre no pueden ser iguales"
	}
	return true, ""
}

func validatePhotos(photos []Foto, add func(string, string)) {
	if len(photos) < MinPhotos || len(photos) > MaxPhotos {
		add("fotos", fmt.Sprintf("deben ser de %d a %d URLs", MinPhotos, MaxPhotos))
		return
	}
	orders := map[int]bool{}
	for _, photo := range photos {
		if !strings.HasPrefix(photo.URL, "https://") || len(photo.URL) > 500 {
			add("fotos", "cada foto debe ser una URL https:// de hasta 500 caracteres")
		}
		switch photo.Tipo {
		case "fachada", "interior", "producto":
		default:
			add("fotos", "el tipo debe ser fachada, interior o producto")
		}
		if photo.Orden < 1 || photo.Orden > len(photos) || orders[photo.Orden] {
			add("fotos", "el orden debe ser único y estar entre 1 y el número de fotos")
		}
		orders[photo.Orden] = true
	}
}

func validateList(field string, values []string, max int, add func(string, string)) {
	if len(values) > max {
		add(field, fmt.Sprintf("no puede tener más de %d valores", max))
	}
	for _, value := range values {
		if n := len([]rune(value)); n < 1 || n > 40 || strings.Contains(value, "|") {
			add(field, "cada valor debe tener de 1 a 40 caracteres y no contener |")
			return
		}
	}
}

func validateCatalog(field string, values []string, max int, valid func(string) bool, add func(string, string)) {
	if len(values) > max {
		add(field, fmt.Sprintf("no puede tener más de %d valores", max))
	}
	for _, value := range values {
		if !valid(value) {
			add(field, "valor no válido: "+value)
			return
		}
	}
}

func validateRating(rating Calificacion, add func(string, string)) {
	if !finite(rating.Promedio) || rating.Promedio < 0 || rating.Promedio > 5 {
		add("calificacion", "el promedio debe estar entre 0 y 5")
	}
	if rating.TotalResenas < 0 {
		add("calificacion", "el total de reseñas no puede ser negativo")
	}
	if rating.TotalResenas == 0 && rating.Promedio != 0 {
		add("calificacion", "sin reseñas el promedio debe ser 0")
	}
	if rating.TotalResenas > 0 && rating.Promedio <= 0 {
		add("calificacion", "con reseñas el promedio debe ser mayor que 0")
	}
	validateAspects(rating.Aspectos, add)
}

func validateAspects(aspects Aspectos, add func(string, string)) {
	for _, aspect := range []float64{aspects.Sabor, aspects.Atencion, aspects.Ambiente, aspects.Espera, aspects.PrecioCalidad} {
		if !finite(aspect) || aspect < 0 || aspect > 5 {
			add("calificacion", "cada aspecto debe estar entre 0 y 5")
			return
		}
	}
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }

// deriveScheduleMin convierte los horarios legibles en ventanas de minutos. `Hasta`
// supera 1440 cuando el cierre es al día siguiente; lo consume NV-21.
func deriveScheduleMin(schedules []Horario) []VentanaMin {
	out := make([]VentanaMin, 0, len(schedules))
	for _, horario := range schedules {
		if horario.Cerrado || horario.Abre == "" || horario.Cierra == "" {
			continue
		}
		open, err := time.Parse("15:04", horario.Abre)
		if err != nil {
			continue
		}
		close, err := time.Parse("15:04", horario.Cierra)
		if err != nil {
			continue
		}
		from := open.Hour()*60 + open.Minute()
		to := close.Hour()*60 + close.Minute()
		if to < from {
			to += 24 * 60
		}
		if to == from {
			continue
		}
		out = append(out, VentanaMin{Dia: horario.Dia, Desde: from, Hasta: to})
	}
	return out
}
