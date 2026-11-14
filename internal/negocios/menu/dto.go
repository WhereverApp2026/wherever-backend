package menu

import "time"

// CrearRequest es un platillo nuevo. `imagen_id` es una imagen publicada por nv-45 del mismo
// dueño; `disponible` es true si no viene.
type CrearRequest struct {
	Seccion     string   `json:"seccion" validate:"required,max=30"`
	Nombre      string   `json:"nombre" validate:"required,max=60"`
	Descripcion string   `json:"descripcion" validate:"max=140"`
	Precio      float64  `json:"precio" validate:"gte=0,lte=100000"`
	Etiquetas   []string `json:"etiquetas" validate:"max=3,unique,dive,oneof=vegetariano picante sin_gluten"`
	ImagenID    string   `json:"imagen_id" validate:"omitempty,max=64"`
	Disponible  *bool    `json:"disponible"`
	Orden       int      `json:"orden" validate:"gte=0,lte=1000"`
}

// EditarRequest cambia solo los campos que vienen. `imagen_id` vacío quita la foto.
type EditarRequest struct {
	Seccion     *string   `json:"seccion" validate:"omitempty,min=1,max=30"`
	Nombre      *string   `json:"nombre" validate:"omitempty,min=1,max=60"`
	Descripcion *string   `json:"descripcion" validate:"omitempty,max=140"`
	Precio      *float64  `json:"precio" validate:"omitempty,gte=0,lte=100000"`
	Etiquetas   *[]string `json:"etiquetas" validate:"omitempty,max=3,unique,dive,oneof=vegetariano picante sin_gluten"`
	ImagenID    *string   `json:"imagen_id" validate:"omitempty,max=64"`
	Disponible  *bool     `json:"disponible"`
	Orden       *int      `json:"orden" validate:"omitempty,gte=0,lte=1000"`
}

// MenuQuery filtra el menú público.
type MenuQuery struct {
	Q       string `query:"q" validate:"omitempty,min=2,max=60"`
	Seccion string `query:"seccion" validate:"omitempty,max=30"`
}

// ItemResponse es un platillo como lo ve su dueño.
type ItemResponse struct {
	ID            string    `json:"id"`
	Seccion       string    `json:"seccion"`
	Nombre        string    `json:"nombre"`
	Descripcion   string    `json:"descripcion"`
	Precio        float64   `json:"precio"`
	Etiquetas     []string  `json:"etiquetas"`
	FotoURL       *string   `json:"foto_url"`
	Disponible    bool      `json:"disponible"`
	Orden         int       `json:"orden"`
	Version       int       `json:"version"`
	ActualizadoEn time.Time `json:"actualizado_en"`
}

// PlatilloPublico es un platillo del menú público, sin datos de control.
type PlatilloPublico struct {
	ID          string   `json:"id"`
	Nombre      string   `json:"nombre"`
	Descripcion string   `json:"descripcion"`
	Precio      float64  `json:"precio"`
	Etiquetas   []string `json:"etiquetas"`
	FotoURL     *string  `json:"foto_url"`
}

// Seccion agrupa los platillos públicos.
type Seccion struct {
	Nombre    string            `json:"nombre"`
	Platillos []PlatilloPublico `json:"platillos"`
}

// MenuResponse es el menú público agrupado por sección, en el orden del dueño.
type MenuResponse struct {
	Secciones []Seccion `json:"secciones"`
}

func foto(url string) *string {
	if url == "" {
		return nil
	}
	return &url
}

func etiquetas(e []string) []string {
	if e == nil {
		return []string{}
	}
	return e
}

func itemResponse(it Item) ItemResponse {
	return ItemResponse{ID: it.ID.Hex(), Seccion: it.Seccion, Nombre: it.Nombre, Descripcion: it.Descripcion, Precio: it.Precio,
		Etiquetas: etiquetas(it.Etiquetas), FotoURL: foto(it.FotoURL), Disponible: it.Disponible, Orden: it.Orden,
		Version: it.Version, ActualizadoEn: it.ActualizadoEn.UTC()}
}

// agrupar arma las secciones en el orden del primer platillo de cada una.
func agrupar(items []Item) MenuResponse {
	out := MenuResponse{Secciones: []Seccion{}}
	pos := map[string]int{}
	for _, it := range items {
		i, ok := pos[it.Seccion]
		if !ok {
			i = len(out.Secciones)
			pos[it.Seccion] = i
			out.Secciones = append(out.Secciones, Seccion{Nombre: it.Seccion, Platillos: []PlatilloPublico{}})
		}
		out.Secciones[i].Platillos = append(out.Secciones[i].Platillos, PlatilloPublico{ID: it.ID.Hex(), Nombre: it.Nombre,
			Descripcion: it.Descripcion, Precio: it.Precio, Etiquetas: etiquetas(it.Etiquetas), FotoURL: foto(it.FotoURL)})
	}
	return out
}
