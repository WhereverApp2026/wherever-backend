// Package imagenes sube, valida, modera y publica fotos (spec nv-45). La persona sube el
// original directo a S3 con un formulario firmado (prefijo cuarentena/); al confirmar, la API
// lo lee, lo recodifica sin metadatos, lo modera con internal/moderacion y publica dos
// tamaños bajo i/, que es lo único que sirve el CloudFront de imágenes (nv-44).
package imagenes

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Collection guarda una fila por imagen.
const Collection = "imagenes"

// Estados de una imagen. Solo `publicada` tiene direcciones públicas.
const (
	// EstadoPendiente: formulario emitido, original aún no confirmado.
	EstadoPendiente = "pendiente"
	// EstadoProcesando: una petición la está procesando; evita moderar dos veces el original.
	EstadoProcesando = "procesando"
	// EstadoEnRevision: moderación dudosa o caída; el original sigue en cuarentena (nv-43).
	EstadoEnRevision = "en_revision"
	EstadoPublicada  = "publicada"
	EstadoRechazada  = "rechazada"
)

// Motivos de rechazo.
const (
	MotivoModeracion = "moderacion"
	MotivoArchivo    = "archivo_invalido"
)

// Tamaños publicados: lado mayor en píxeles. Nunca se amplía.
const (
	LadoGrande = 1080
	LadoChico  = 320
)

// Moderacion es lo que devolvió moderacion.Evaluar para el original.
type Moderacion struct {
	Veredicto string             `bson:"veredicto"`
	Puntajes  map[string]float64 `bson:"puntajes,omitempty"`
	Falla     string             `bson:"falla,omitempty"`
}

// Concluyente dice si ya hay un veredicto que no hay que repetir (se modera una vez por
// original; solo una falla del proveedor permite evaluar de nuevo).
func (m *Moderacion) Concluyente() bool { return m != nil && m.Falla == "" }

// Imagen es un documento de `imagenes`. El _id es aleatorio (32 hex) y también es el nombre
// en S3 y en CloudFront.
type Imagen struct {
	ID            string        `bson:"_id"`
	DuenoID       bson.ObjectID `bson:"dueno_id"`
	Tipo          string        `bson:"tipo"`
	Estado        string        `bson:"estado"`
	Motivo        string        `bson:"motivo,omitempty"`
	Moderacion    *Moderacion   `bson:"moderacion,omitempty"`
	CreadaEn      time.Time     `bson:"creada_en"`
	ActualizadaEn time.Time     `bson:"actualizada_en"`
	// ExpiraEn borra con TTL las pendientes que nunca se confirmaron (el original en
	// cuarentena/ también vence a 1 día por la regla del bucket).
	ExpiraEn *time.Time `bson:"expira_en,omitempty"`
}

// Claves de S3. cuarentena/ e i/ son los dos prefijos que el rol de la instancia puede tocar.
func claveOriginal(id string) string { return "cuarentena/" + id }

func clavePublicada(id string, lado int) string {
	if lado == LadoChico {
		return "i/" + id + "/320.jpg"
	}
	return "i/" + id + "/1080.jpg"
}

// claves son todas las que puede tener una imagen; borrar una inexistente no falla.
func claves(id string) []string {
	return []string{claveOriginal(id), clavePublicada(id, LadoGrande), clavePublicada(id, LadoChico)}
}
