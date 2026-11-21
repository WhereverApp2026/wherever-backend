package insignias

// Nombres de los niveles (tabla aprobada en nv-79).
const (
	NivelExplorador = "Explorador"
	NivelAnfitrion  = "Anfitrión"
	NivelGuiaLocal  = "Guía local"
	NivelLeyenda    = "Leyenda"
)

// umbrales son los puntos para llegar a los niveles 1 a 8; del 8 en adelante, +700 por nivel.
var umbrales = []int{0, 150, 300, 500, 800, 1200, 1700, 2300}

const pasoLeyenda = 700

// Nivel es un escalón de la tabla: número, nombre y puntos con que se llega.
type Nivel struct {
	Numero int    `json:"numero"`
	Nombre string `json:"nombre"`
	Puntos int    `json:"puntos"`
}

// NivelNumero da el nivel de la tabla con ese número (≥ 1).
func NivelNumero(n int) Nivel {
	n = max(n, 1)
	return Nivel{Numero: n, Nombre: nombreNivel(n), Puntos: puntosNivel(n)}
}

// NivelDe da el nivel que corresponde a los puntos.
func NivelDe(puntos int) Nivel {
	n := 1
	for puntosNivel(n+1) <= puntos {
		n++
	}
	return NivelNumero(n)
}

func puntosNivel(n int) int {
	if n <= len(umbrales) {
		return umbrales[n-1]
	}
	return umbrales[len(umbrales)-1] + (n-len(umbrales))*pasoLeyenda
}

func nombreNivel(n int) string {
	switch {
	case n <= 3:
		return NivelExplorador
	case n <= 5:
		return NivelAnfitrion
	case n <= 7:
		return NivelGuiaLocal
	default:
		return NivelLeyenda
	}
}
