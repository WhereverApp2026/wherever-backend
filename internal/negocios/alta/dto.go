// Package alta es el alta del negocio por pasos y la revisión del administrador (spec nv-55,
// hoja A5): service, handler y DTO. Usa `internal/negocios` (modelo y repositorio) y
// `internal/negocios/expediente` (datos sensibles cifrados); ninguno de los dos lo usa a él.
package alta

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
)

// CrearRequest es el paso 1 (POST /negocios).
type CrearRequest struct {
	Nombre      string `json:"nombre" validate:"required,min=3,max=60"`
	Categoria   string `json:"categoria" validate:"required,max=30"`
	Descripcion string `json:"descripcion" validate:"max=300"`
}

// Ubicacion es el Point GeoJSON del paso 2.
type Ubicacion struct {
	Type        string    `json:"type" validate:"required"`
	Coordinates []float64 `json:"coordinates" validate:"required,len=2"`
}

// Paso2Request es la ubicación.
type Paso2Request struct {
	Direccion negocios.Direccion `json:"direccion"`
	Ubicacion Ubicacion          `json:"ubicacion"`
}

// Paso3Request son los horarios (hasta dos tramos por día).
type Paso3Request struct {
	Horarios []negocios.Horario `json:"horarios" validate:"required,min=1,max=14"`
}

// Paso5Request son precios y servicios.
type Paso5Request struct {
	NivelPrecio    int                    `json:"nivel_precio"`
	PrecioPromedio int                    `json:"precio_promedio"`
	Servicios      []string               `json:"servicios" validate:"max=20,dive,min=1,max=40"`
	OpcionesDieta  []string               `json:"opciones_dieta"`
	Alergenos      []string               `json:"alergenos"`
	Accesibilidad  negocios.Accesibilidad `json:"accesibilidad"`
}

// FotoRequest registra una imagen ya publicada por nv-45 en la galería.
type FotoRequest struct {
	ImagenID string `json:"imagen_id" validate:"required,max=64"`
	Tipo     string `json:"tipo" validate:"required,oneof=fachada interior producto"`
}

// OrdenRequest es el orden nuevo de la galería; la primera es la portada.
type OrdenRequest struct {
	FotosIDs []string `json:"fotos_ids" validate:"required,min=1,max=10,dive,required,max=64"`
}

// FiscalesRequest son los datos fiscales (A5 §1).
type FiscalesRequest struct {
	RFC           string `json:"rfc" validate:"required,max=20"`
	RazonSocial   string `json:"razon_social" validate:"required,max=254"`
	RegimenFiscal string `json:"regimen_fiscal" validate:"required,max=3"`
	UsoCFDI       string `json:"uso_cfdi" validate:"required,max=4"`
	CPFiscal      string `json:"cp_fiscal" validate:"required,max=5"`
}

// CuentaRequest es la cuenta de depósito (A5 §7).
type CuentaRequest struct {
	CLABE   string `json:"clabe" validate:"required,max=18"`
	Titular string `json:"titular" validate:"required,min=3,max=120"`
}

// DecisionRequest es la decisión del administrador.
type DecisionRequest struct {
	Decision string `json:"decision" validate:"required,oneof=aprobar rechazar"`
	Motivo   string `json:"motivo" validate:"max=500"`
}

// Decisiones.
const (
	DecisionAprobar  = "aprobar"
	DecisionRechazar = "rechazar"
)

// ColaQuery filtra la cola del administrador.
type ColaQuery struct {
	Estado string `query:"estado" validate:"omitempty,oneof=en_revision cambios_pendientes"`
	Cursor string `query:"cursor" validate:"omitempty,max=300,cursor"`
	Limit  int    `query:"limite" validate:"min=1,max=50"`
}

// LecturaQuery es el motivo obligatorio de cada lectura del administrador.
type LecturaQuery struct {
	Motivo string `query:"motivo" validate:"required,min=10,max=200"`
}

// FotoResponse es una foto de la galería con su id para borrarla u ordenarla.
type FotoResponse struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Tipo  string `json:"tipo"`
	Orden int    `json:"orden"`
}

// RevisionResponse es el estado de la revisión que ve el dueño.
type RevisionResponse struct {
	EnviadoEn     *time.Time `json:"enviado_en,omitempty"`
	RevisadoEn    *time.Time `json:"revisado_en,omitempty"`
	MotivoRechazo string     `json:"motivo_rechazo,omitempty"`
}

// NegocioResponse es el negocio como lo ve su dueño durante el alta. Nunca lleva datos del
// expediente: solo banderas y la CLABE enmascarada.
type NegocioResponse struct {
	ID              string                  `json:"id"`
	Nombre          string                  `json:"nombre"`
	Categoria       string                  `json:"categoria"`
	Descripcion     string                  `json:"descripcion"`
	Direccion       *negocios.Direccion     `json:"direccion,omitempty"`
	Ubicacion       *Ubicacion              `json:"ubicacion,omitempty"`
	Horarios        []negocios.Horario      `json:"horarios"`
	Fotos           []FotoResponse          `json:"fotos"`
	NivelPrecio     int                     `json:"nivel_precio"`
	PrecioPromedio  int                     `json:"precio_promedio"`
	Servicios       []string                `json:"servicios"`
	OpcionesDieta   []string                `json:"opciones_dieta"`
	Alergenos       []string                `json:"alergenos"`
	Accesibilidad   negocios.Accesibilidad  `json:"accesibilidad"`
	Estado          string                  `json:"estado"`
	PasoAlta        int                     `json:"paso_alta"`
	Revision        RevisionResponse        `json:"revision"`
	Documentos      negocios.Documentos     `json:"documentos"`
	CuentaDeposito  *negocios.CuentaMascara `json:"cuenta_deposito,omitempty"`
	Verificado      bool                    `json:"verificado"`
	Version         int                     `json:"version"`
	CreadoEn        time.Time               `json:"creado_en"`
	RequiereRefresh bool                    `json:"requiere_refresh,omitempty"`
}

// ColaItem es una fila de la cola del administrador: datos públicos y banderas, nada sensible.
type ColaItem struct {
	ID          string              `json:"id"`
	Nombre      string              `json:"nombre"`
	Categoria   string              `json:"categoria"`
	Descripcion string              `json:"descripcion"`
	Fotos       int                 `json:"fotos"`
	Documentos  negocios.Documentos `json:"documentos"`
	PasoAlta    int                 `json:"paso_alta"`
	EnviadoEn   *time.Time          `json:"enviado_en,omitempty"`
	DiasEspera  int                 `json:"dias_espera"`
	Version     int                 `json:"version"`
	// Cambios son los campos por revisar de un negocio ya aprobado (nv-56); con sus valores nuevos.
	Cambios *CambiosResponse `json:"cambios_pendientes,omitempty"`
}

// CambiosResponse son nombre, dirección o ubicación nuevos de un negocio aprobado (nv-56).
type CambiosResponse struct {
	Campos    []string            `json:"campos"`
	Nombre    string              `json:"nombre,omitempty"`
	Direccion *negocios.Direccion `json:"direccion,omitempty"`
	Ubicacion *Ubicacion          `json:"ubicacion,omitempty"`
	EnviadoEn time.Time           `json:"enviado_en"`
}

// DocumentosResponse es la lista de metadatos del expediente.
type DocumentosResponse struct {
	Items []expediente.Metadato `json:"items"`
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func negocioResponse(b negocios.Business) NegocioResponse {
	out := NegocioResponse{
		ID: b.ID.Hex(), Nombre: b.Name, Categoria: b.Category, Descripcion: b.Description,
		Horarios: nonNil(b.Schedules), Fotos: fotosResponse(b.Photos), NivelPrecio: b.PriceLevel, PrecioPromedio: b.AveragePrice,
		Servicios: nonNil(b.Services), OpcionesDieta: nonNil(b.DietOptions), Alergenos: nonNil(b.Allergens), Accesibilidad: b.Accessibility,
		Estado: b.State, PasoAlta: b.Step, Documentos: b.Documentos, CuentaDeposito: b.CuentaDeposito, Verificado: b.Verified,
		Version: b.Version, CreadoEn: b.CreatedAt.UTC(),
		Revision: RevisionResponse{EnviadoEn: b.Revision.EnviadoEn, RevisadoEn: b.Revision.RevisadoEn, MotivoRechazo: b.Revision.MotivoRechazo},
	}
	if b.Location.Type == "Point" {
		out.Ubicacion = &Ubicacion{Type: b.Location.Type, Coordinates: b.Location.Coordinates}
		addr := b.Address
		out.Direccion = &addr
	}
	return out
}

func fotosResponse(fotos []negocios.Foto) []FotoResponse {
	out := make([]FotoResponse, 0, len(fotos))
	for _, f := range fotos {
		out = append(out, FotoResponse{ID: f.ImagenID, URL: f.URL, Tipo: f.Tipo, Orden: f.Orden})
	}
	return out
}

func colaItem(b negocios.Business, now time.Time) ColaItem {
	sent := b.Revision.EnviadoEn
	var cambios *CambiosResponse
	if c := b.CambiosPendientes; c != nil {
		at := c.EnviadoEn
		sent = &at
		cambios = &CambiosResponse{Campos: nonNil(c.Campos), Nombre: c.Nombre, Direccion: c.Direccion, EnviadoEn: c.EnviadoEn.UTC()}
		if c.Ubicacion != nil {
			cambios.Ubicacion = &Ubicacion{Type: c.Ubicacion.Type, Coordinates: c.Ubicacion.Coordinates}
		}
	}
	dias := 0
	if sent != nil {
		dias = int(now.Sub(*sent).Hours() / 24)
	}
	return ColaItem{ID: b.ID.Hex(), Nombre: b.Name, Categoria: b.Category, Descripcion: b.Description, Fotos: len(b.Photos),
		Documentos: b.Documentos, PasoAlta: b.Step, EnviadoEn: sent, DiasEspera: dias, Version: b.Version, Cambios: cambios}
}
