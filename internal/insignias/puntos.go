package insignias

import (
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Motivos de un movimiento de puntos y lo que vale cada uno (tabla aprobada en nv-79).
const (
	MotivoRegistro = "registro"
	MotivoCheckin  = "checkin"
	MotivoResena   = "resena"
	MotivoPostal   = "postal"
	MotivoPlan     = "plan"
	MotivoInsignia = "insignia"
)

// PuntosPor es lo que suma cada motivo.
var PuntosPor = map[string]int{
	MotivoRegistro: 50,
	MotivoCheckin:  20,
	MotivoResena:   30,
	MotivoPostal:   10,
	MotivoPlan:     5,
	MotivoInsignia: 50,
}

// PlanesPorSemana es el tope de planes que suman en una semana ISO.
const PlanesPorSemana = 5

// clave identifica un movimiento como lo hace `ux_movimiento` (la persona ya es fija).
type clave struct{ motivo, ref string }

// Pendiente es un movimiento que debería existir.
type Pendiente struct {
	Motivo string
	RefID  string
}

// Esperados son los movimientos que corresponden a la actividad y a las insignias ganadas.
// Cada uno lleva una referencia estable, así repetir la acción, o borrarla y crearla de nuevo,
// no suma otra vez:
//   - registro: la persona; los 50 puntos ya se dieron al crear la cuenta (no se vuelven a sumar).
//   - checkin: el check-in válido; `ux_checkin_dia` ya deja uno por lugar y día.
//   - resena: el check-in de la reseña (una por visita); recrearla no cambia la referencia.
//   - postal: la postal lista.
//   - plan: la semana ISO y el turno dentro de ella (`2026-W47#1` a `#5`): con el tope de 5,
//     borrar un plan y crear otro en la misma semana ocupa el mismo turno.
//   - insignia: el id de la insignia del catálogo vigente.
func Esperados(userID bson.ObjectID, a Actividad, ganadas map[string]time.Time) []Pendiente {
	out := []Pendiente{{MotivoRegistro, userID.Hex()}}
	for _, v := range a.Visitas {
		out = append(out, Pendiente{MotivoCheckin, v.ID.Hex()})
	}
	for _, c := range a.Resenas {
		out = append(out, Pendiente{MotivoResena, c.Hex()})
	}
	for _, p := range a.Postales {
		out = append(out, Pendiente{MotivoPostal, p.Hex()})
	}
	porSemana := map[string]int{}
	for _, p := range a.Planes {
		s := semanaISO(p.CreadoEn)
		if porSemana[s] >= PlanesPorSemana {
			continue
		}
		porSemana[s]++
		out = append(out, Pendiente{MotivoPlan, s + "#" + strconv.Itoa(porSemana[s])})
	}
	for _, d := range Catalogo {
		if _, ok := ganadas[d.ID]; ok {
			out = append(out, Pendiente{MotivoInsignia, d.ID})
		}
	}
	return out
}

// sumaAlCrear dice si el motivo suma al registrarse el movimiento. El de registro no: los 50
// puntos de bienvenida ya están en `usuarios.puntos` desde que se creó la cuenta.
func sumaAlCrear(motivo string) bool { return motivo != MotivoRegistro }
