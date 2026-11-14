package postales

import (
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
)

// EstadoPostal combina el estado de la imagen (nv-45) y el del texto: cualquiera rechazado =
// `rechazada`; los dos aprobados = `lista`; si no, `en_moderacion`.
func EstadoPostal(imagenEstado, textoEstado string) string {
	switch {
	case imagenEstado == imagenes.EstadoRechazada || textoEstado == TextoRechazado:
		return EstadoRechazada
	case imagenEstado == imagenes.EstadoPublicada && textoEstado == TextoAprobado:
		return EstadoLista
	default:
		return EstadoEnModeracion
	}
}

// ImagenUsable dice si una imagen propia puede volverse postal: publicada o en revisión (la
// cuarentena de nv-45). Pendiente, en proceso o rechazada no.
func ImagenUsable(estado string) bool {
	return estado == imagenes.EstadoPublicada || estado == imagenes.EstadoEnRevision
}

// Relacion es lo que importa de quien mira frente al dueño.
type Relacion struct {
	Dueno     bool
	Amigos    bool
	Bloqueado bool
}

// PuedeVer es la matriz de visibilidad de §5: el dueño siempre; nadie entre bloqueados; los
// demás solo lo `lista` (o `publicado`), y según la visibilidad.
func PuedeVer(r Relacion, visibilidad string, listo bool) bool {
	switch {
	case r.Dueno:
		return true
	case r.Bloqueado || !listo:
		return false
	case visibilidad == VisPublica:
		return true
	case visibilidad == VisAmigos:
		return r.Amigos
	default:
		return false
	}
}

// rango ordena las visibilidades de la más cerrada a la más abierta.
func rango(v string) int {
	switch v {
	case VisPublica:
		return 2
	case VisAmigos:
		return 1
	default:
		return 0
	}
}

// Compatible dice si una postal con visibilidad `postal` admite un momento con `momento`: un
// momento no se abre más que su postal. Una postal privada sí, porque se cambia a la del momento.
func Compatible(postal, momento string) bool {
	return postal == VisPrivada || rango(momento) <= rango(postal)
}

// PlanPasado dice si el plan ya está en la pestaña `pasados` de nv-30: su fecha es anterior al
// inicio de hoy en la zona dada.
func PlanPasado(fecha, now time.Time, loc *time.Location) bool {
	local := now.In(loc)
	hoy := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	return fecha.Before(hoy)
}

// RangoMes convierte `AAAA-MM` en el inicio y el fin (excluido) del mes en la zona dada.
func RangoMes(mes string, loc *time.Location) (time.Time, time.Time, bool) {
	t, err := time.ParseInLocation("2006-01", mes, loc)
	if err != nil {
		return time.Time{}, time.Time{}, false
	}
	return t.UTC(), t.AddDate(0, 1, 0).UTC(), true
}

// Centavos pasa el costo del plan (pesos enteros, nv-30) a centavos.
func Centavos(pesos int) int64 { return int64(pesos) * 100 }
