// Package insignias da las insignias de las personas (nv-78): logros que se ganan con lo que
// ya hacen en la app (planes, reseñas, lugares visitados, amigos y postales). Se calculan al
// pedirlas a partir de esos datos; la primera vez que una se gana se guarda su fecha.
package insignias

// Fuentes de cada insignia: lo que se cuenta para saber si se ganó.
const (
	FuentePlanes   = "planes"
	FuenteResenas  = "resenas"
	FuenteLugares  = "lugares"
	FuenteAmigos   = "amigos"
	FuentePostales = "postales"
)

// Definicion es una insignia del catálogo. El catálogo es fijo y versionado en el código:
// agregar una no cambia las ya ganadas; quitar una solo deja de mostrarla.
type Definicion struct {
	ID          string
	Nombre      string
	Descripcion string
	Fuente      string
	Meta        int
}

// Catalogo es el orden en que la app las muestra (ganadas y bloqueadas).
var Catalogo = []Definicion{
	{ID: "primer_plan", Nombre: "Primer plan", Descripcion: "Crea tu primer plan", Fuente: FuentePlanes, Meta: 1},
	{ID: "planificador", Nombre: "Planificador", Descripcion: "Crea 10 planes", Fuente: FuentePlanes, Meta: 10},
	{ID: "primera_resena", Nombre: "Primera reseña", Descripcion: "Publica tu primera reseña", Fuente: FuenteResenas, Meta: 1},
	{ID: "critico", Nombre: "Crítico", Descripcion: "Publica 10 reseñas", Fuente: FuenteResenas, Meta: 10},
	{ID: "explorador", Nombre: "Explorador", Descripcion: "Haz check-in en 5 lugares distintos", Fuente: FuenteLugares, Meta: 5},
	{ID: "trotamundos", Nombre: "Trotamundos", Descripcion: "Haz check-in en 20 lugares distintos", Fuente: FuenteLugares, Meta: 20},
	{ID: "buena_compania", Nombre: "Buena compañía", Descripcion: "Ten 5 amigos", Fuente: FuenteAmigos, Meta: 5},
	{ID: "fotografo", Nombre: "Fotógrafo", Descripcion: "Comparte 5 postales", Fuente: FuentePostales, Meta: 5},
}

// Ganada dice si la cuenta alcanza la meta.
func (d Definicion) Ganada(conteos map[string]int) bool { return conteos[d.Fuente] >= d.Meta }

// Progreso es el avance hacia la meta, sin pasarla.
func (d Definicion) Progreso(conteos map[string]int) int { return min(conteos[d.Fuente], d.Meta) }
