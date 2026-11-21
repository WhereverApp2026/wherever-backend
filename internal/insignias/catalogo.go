// Package insignias da las insignias, los puntos, el nivel y la racha de las personas (nv-78,
// nv-79). Todo se concilia al pedirlo (y cada noche) a partir de lo que ya hicieron en la app:
// check-ins válidos, reseñas publicadas, postales listas, planes y grupos. La primera vez que
// una insignia se gana se guarda su fecha; cada punto queda en una bitácora idempotente.
package insignias

// Fuentes de cada insignia: lo que se cuenta para saber si se ganó.
const (
	FuenteCafeterias = "cafeterias"
	FuenteResenas    = "resenas"
	FuenteGrupos     = "grupos"
	FuenteLugares    = "lugares"
	FuentePostales   = "postales"
	FuenteRacha      = "racha"
	FuenteMeGusta    = "me_gusta"
)

// Definicion es una insignia del catálogo. El catálogo es fijo y versionado en el código:
// agregar una no cambia las ya ganadas; quitar una solo deja de mostrarla (las de nv-78 siguen
// guardadas en `insignias_usuarios`, pero ya no se muestran).
type Definicion struct {
	ID          string
	Nombre      string
	Descripcion string
	Fuente      string
	Meta        int
}

// Catalogo es el de la maqueta (`83_Insignias`), en el orden en que la app lo muestra.
// `querido` cuenta los me gusta recibidos (nv-83); hasta entonces queda bloqueada.
var Catalogo = []Definicion{
	{ID: "catador_cafe", Nombre: "Catador de café", Descripcion: "Haz check-in en 5 cafeterías distintas", Fuente: FuenteCafeterias, Meta: 5},
	{ID: "resenista", Nombre: "Reseñista", Descripcion: "Publica tu primera reseña", Fuente: FuenteResenas, Meta: 1},
	{ID: "anfitrion_grupo", Nombre: "Anfitrión de grupo", Descripcion: "Organiza un grupo", Fuente: FuenteGrupos, Meta: 1},
	{ID: "viajero_pueblo", Nombre: "Viajero del pueblo", Descripcion: "Haz check-in en 10 lugares distintos", Fuente: FuenteLugares, Meta: 10},
	{ID: "cronista", Nombre: "Cronista", Descripcion: "Comparte 10 postales", Fuente: FuentePostales, Meta: 10},
	{ID: "racha_3", Nombre: "Racha de 3 semanas", Descripcion: "Visita un lugar 3 semanas seguidas", Fuente: FuenteRacha, Meta: 3},
	{ID: "sello_oro", Nombre: "Sello de oro", Descripcion: "Publica 20 reseñas", Fuente: FuenteResenas, Meta: 20},
	{ID: "querido", Nombre: "Querido", Descripcion: "Recibe 50 me gusta", Fuente: FuenteMeGusta, Meta: 50},
	{ID: "explorador_total", Nombre: "Explorador total", Descripcion: "Haz check-in en 30 lugares distintos", Fuente: FuenteLugares, Meta: 30},
}

// Ganada dice si la cuenta alcanza la meta.
func (d Definicion) Ganada(conteos map[string]int) bool { return conteos[d.Fuente] >= d.Meta }

// Progreso es el avance hacia la meta, sin pasarla.
func (d Definicion) Progreso(conteos map[string]int) int { return min(conteos[d.Fuente], d.Meta) }
