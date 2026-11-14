package cupones

import (
	"crypto/rand"
	"errors"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// Alfabeto del código de canje: sin 0, O, 1 ni I, que se confunden al teclearlo. Son 32
// símbolos: 32⁶ ≈ 10⁹ códigos y un byte aleatorio & 31 no tiene sesgo.
const (
	Alfabeto    = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"
	LargoCodigo = 6
)

// Errores de las reglas puras; el service los traduce a 422.
var (
	errValorRequerido = errors.New("el tipo exige valor")
	errValorSobra     = errors.New("este tipo no lleva valor")
	errPorcentaje     = errors.New("el porcentaje va de 1 a 100")
	errMonto          = errors.New("el monto va en centavos y es mayor que 0")
	errVigencia       = errors.New("vigencia_inicio debe ser anterior a vigencia_fin")
	errHoras          = errors.New("desde y hasta no pueden ser iguales")
)

// ValidarValor revisa `valor` según el tipo: porcentaje 1–100, monto en centavos > 0;
// 2x1 y regalo sin valor.
func ValidarValor(tipo string, valor *int64) error {
	switch tipo {
	case TipoPorcentaje:
		if valor == nil {
			return errValorRequerido
		}
		if *valor < 1 || *valor > 100 {
			return errPorcentaje
		}
	case TipoMonto:
		if valor == nil {
			return errValorRequerido
		}
		if *valor < 1 {
			return errMonto
		}
	default:
		if valor != nil {
			return errValorSobra
		}
	}
	return nil
}

// ValidarVigencia exige inicio < fin.
func ValidarVigencia(inicio, fin time.Time) error {
	if !inicio.Before(fin) {
		return errVigencia
	}
	return nil
}

// ValidarHoras rechaza una ventana vacía (desde = hasta); el formato lo revisa el DTO.
func ValidarHoras(h *Horas) error {
	if h != nil && h.Desde == h.Hasta {
		return errHoras
	}
	return nil
}

func minutos(hhmm string) int {
	h, _ := strconv.Atoi(hhmm[:2])
	m, _ := strconv.Atoi(hhmm[3:])
	return h*60 + m
}

// EnHorario dice si el cupón se puede usar en `now` según días y horas en `loc`. Un rango que
// cruza medianoche (22:00–02:00) cuenta la madrugada como parte del día en que empezó.
func EnHorario(dias []int, horas *Horas, now time.Time, loc *time.Location) bool {
	local := now.In(loc)
	dia := int(local.Weekday())
	diaOK := func(d int) bool { return len(dias) == 0 || slices.Contains(dias, d) }
	if horas == nil {
		return diaOK(dia)
	}
	minuto := local.Hour()*60 + local.Minute()
	desde, hasta := minutos(horas.Desde), minutos(horas.Hasta)
	if desde < hasta {
		return diaOK(dia) && minuto >= desde && minuto < hasta
	}
	if minuto >= desde {
		return diaOK(dia)
	}
	return minuto < hasta && diaOK((dia+6)%7)
}

// GenerarCodigo da un código de LargoCodigo símbolos de Alfabeto con `r` (crypto/rand en la API).
func GenerarCodigo(r io.Reader) (string, error) {
	buf := make([]byte, LargoCodigo)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	var b strings.Builder
	for _, x := range buf {
		b.WriteByte(Alfabeto[int(x)&31])
	}
	return b.String(), nil
}

// NormalizarCodigo deja el código como se guarda (mayúsculas, sin espacios).
func NormalizarCodigo(s string) string { return strings.ToUpper(strings.TrimSpace(s)) }

// CodigoValido dice si el texto puede ser un código (largo y alfabeto).
func CodigoValido(s string) bool {
	if len(s) != LargoCodigo {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(Alfabeto, rune(s[i])) {
			return false
		}
	}
	return true
}

func codigoAleatorio() (string, error) { return GenerarCodigo(rand.Reader) }

// ResumenNormalizado es título + negocio sin acentos ni mayúsculas, para buscar por prefijo.
func ResumenNormalizado(titulo, negocio string) string {
	return text.Normalize(titulo + " " + negocio)
}

// CoincidePrefijo dice si alguna palabra del resumen (o el resumen entero) empieza con q.
func CoincidePrefijo(resumen, q string) bool {
	q = text.Normalize(q)
	return q == "" || strings.HasPrefix(resumen, q) || strings.Contains(resumen, " "+q)
}

// Dia es el día UTC de las métricas (`AAAA-MM-DD`).
func Dia(t time.Time) string { return t.UTC().Format("2006-01-02") }
