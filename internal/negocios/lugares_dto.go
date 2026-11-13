package negocios

import (
	"slices"
	"time"
)

// Órdenes de GET /lugares.
const (
	OrderDistance = "distancia"
	OrderRating   = "calificacion"
	OrderPrice    = "precio"
)

// Límites de la búsqueda geográfica y del mapa (spec nv-21).
const (
	DefaultRadiusM = 5000
	MaxRadiusM     = 20000
	MaxMapPins     = 100
	MaxMapCells    = 400
	MinMapZoom     = 3
	MaxMapZoom     = 20
	// cellsPerTile parte cada mosaico de 256 px en 4×4 celdas (64 px por celda).
	cellsPerTile = 4
)

// ListQuery es `?categoria=&precio_max=&abierto_ahora=&lat=&lng=&radio_m=&orden=&cursor=&limite=`.
// La ubicación solo se usa en la consulta: no se guarda ni se registra.
type ListQuery struct {
	Category string   `query:"categoria" validate:"omitempty,max=40"`
	MaxPrice int      `query:"precio_max" validate:"omitempty,min=1,max=4"`
	OpenNow  bool     `query:"abierto_ahora"`
	Lat      *float64 `query:"lat" validate:"omitempty,latitude"`
	Lng      *float64 `query:"lng" validate:"omitempty,longitude"`
	RadiusM  int      `query:"radio_m" validate:"omitempty,min=1,max=20000"`
	Order    string   `query:"orden" validate:"omitempty,oneof=distancia calificacion precio"`
	Cursor   string   `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit    int      `query:"limite" validate:"min=1,max=50"`
}

// MapQuery es el área visible del mapa y su zoom.
type MapQuery struct {
	MinLat   *float64 `query:"min_lat" validate:"required,latitude"`
	MinLng   *float64 `query:"min_lng" validate:"required,longitude"`
	MaxLat   *float64 `query:"max_lat" validate:"required,latitude"`
	MaxLng   *float64 `query:"max_lng" validate:"required,longitude"`
	Zoom     int      `query:"zoom" validate:"required,min=3,max=20"`
	Category string   `query:"categoria" validate:"omitempty,max=40"`
}

// Point es una coordenada en la respuesta.
type Point struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

// RatingSummary es el resumen de calificación de la lista.
type RatingSummary struct {
	Average float64 `json:"promedio"`
	Reviews int     `json:"total_resenas"`
}

// PlaceItem es un negocio en GET /lugares.
type PlaceItem struct {
	ID           string        `json:"id"`
	Slug         string        `json:"slug"`
	Name         string        `json:"nombre"`
	Category     string        `json:"categoria"`
	PriceLevel   int           `json:"nivel_precio"`
	AveragePrice int           `json:"precio_promedio"`
	Rating       RatingSummary `json:"calificacion"`
	Cover        string        `json:"foto_portada"`
	Location     Point         `json:"ubicacion"`
	DistanceM    *float64      `json:"distancia_m,omitempty"`
	OpenNow      bool          `json:"abierto_ahora"`
}

// PlaceDetail es la ficha pública de GET /lugares/{id}. No lleva dueño, QR de check-in,
// plan, revisión ni versión.
type PlaceDetail struct {
	ID                  string        `json:"id"`
	Slug                string        `json:"slug"`
	Name                string        `json:"nombre"`
	Category            string        `json:"categoria"`
	Tags                []string      `json:"etiquetas"`
	Description         string        `json:"descripcion"`
	PriceLevel          int           `json:"nivel_precio"`
	AveragePrice        int           `json:"precio_promedio"`
	Location            Point         `json:"ubicacion"`
	Address             Direccion     `json:"direccion"`
	Phone               string        `json:"telefono,omitempty"`
	Schedules           []Horario     `json:"horarios"`
	Photos              []Foto        `json:"fotos"`
	Services            []string      `json:"servicios"`
	Accessibility       Accesibilidad `json:"accesibilidad"`
	DietOptions         []string      `json:"opciones_dieta"`
	Allergens           []string      `json:"alergenos"`
	AcceptsReservations bool          `json:"acepta_reservas"`
	Rating              Calificacion  `json:"calificacion"`
	Verified            bool          `json:"verificado"`
	ExcellenceSeal      bool          `json:"sello_excelencia"`
	OpenNow             bool          `json:"abierto_ahora"`
}

// MapPin es un negocio suelto en el mapa.
type MapPin struct {
	ID       string  `json:"id"`
	Name     string  `json:"nombre"`
	Category string  `json:"categoria"`
	Rating   float64 `json:"calificacion"`
	Location Point   `json:"ubicacion"`
}

// MapCell es una celda de la rejilla con su conteo y su centro (promedio de sus negocios).
type MapCell struct {
	Center Point `json:"centro"`
	Count  int   `json:"conteo"`
}

// MapResponse trae pines sueltos o, si hay más de MaxMapPins, celdas agrupadas.
type MapResponse struct {
	Grouped bool      `json:"agrupado"`
	Pins    []MapPin  `json:"pines"`
	Cells   []MapCell `json:"celdas"`
}

func pointOf(l Location) Point {
	if len(l.Coordinates) != 2 {
		return Point{}
	}
	return Point{Lat: l.Coordinates[1], Lng: l.Coordinates[0]}
}

// Cover es la primera foto por `orden`.
func Cover(photos []Foto) string {
	if len(photos) == 0 {
		return ""
	}
	first := slices.MinFunc(photos, func(a, b Foto) int { return a.Orden - b.Orden })
	return first.URL
}

func emptyIfNil[T any](items []T) []T {
	if items == nil {
		return []T{}
	}
	return items
}

func toItem(row placeRow, now time.Time) PlaceItem {
	return PlaceItem{
		ID:           row.ID.Hex(),
		Slug:         row.Slug,
		Name:         row.Name,
		Category:     row.Category,
		PriceLevel:   row.PriceLevel,
		AveragePrice: row.AveragePrice,
		Rating:       RatingSummary{Average: row.Rating.Promedio, Reviews: row.Rating.TotalResenas},
		Cover:        Cover(row.Photos),
		Location:     pointOf(row.Location),
		DistanceM:    row.DistanceM,
		OpenNow:      OpenAt(row.ScheduleMin, now),
	}
}

func toDetail(b Business, now time.Time) PlaceDetail {
	return PlaceDetail{
		ID:                  b.ID.Hex(),
		Slug:                b.Slug,
		Name:                b.Name,
		Category:            b.Category,
		Tags:                emptyIfNil(b.Tags),
		Description:         b.Description,
		PriceLevel:          b.PriceLevel,
		AveragePrice:        b.AveragePrice,
		Location:            pointOf(b.Location),
		Address:             b.Address,
		Phone:               b.Phone,
		Schedules:           emptyIfNil(b.Schedules),
		Photos:              emptyIfNil(b.Photos),
		Services:            emptyIfNil(b.Services),
		Accessibility:       b.Accessibility,
		DietOptions:         emptyIfNil(b.DietOptions),
		Allergens:           emptyIfNil(b.Allergens),
		AcceptsReservations: b.AcceptsReservations,
		Rating:              b.Rating,
		Verified:            b.Verified,
		ExcellenceSeal:      b.ExcellenceSeal,
		OpenNow:             OpenAt(b.ScheduleMin, now),
	}
}
