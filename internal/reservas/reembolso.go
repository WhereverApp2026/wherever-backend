package reservas

import (
	"crypto/rand"
	"time"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
)

// Políticas de cancelación (A2).
const (
	PoliticaFlexible = "flexible"
	PoliticaModerada = "moderada"
	PoliticaEstricta = "estricta"
)

// Tipos de anticipo (A2).
const (
	AnticipoFijo       = "fijo"
	AnticipoPorcentaje = "porcentaje"
	AnticipoPorPersona = "por_persona"
)

// PoliticaDe da las reglas de cada política: flexible = 100 % hasta 2 h antes y 0 % después;
// moderada = 100 % hasta 24 h y 50 % después; estricta = 50 % hasta 48 h y 0 % después.
// Una desconocida se trata como flexible.
func PoliticaDe(tipo string) Politica {
	switch tipo {
	case PoliticaModerada:
		return Politica{Tipo: PoliticaModerada, HorasLimite: 24, PorcentajeAntes: 100, PorcentajeDespues: 50}
	case PoliticaEstricta:
		return Politica{Tipo: PoliticaEstricta, HorasLimite: 48, PorcentajeAntes: 50, PorcentajeDespues: 0}
	}
	return Politica{Tipo: PoliticaFlexible, HorasLimite: 2, PorcentajeAntes: 100, PorcentajeDespues: 0}
}

// Porcentaje es lo que se devuelve si se cancela en `now`. Si cancela el negocio, 100 %.
func Porcentaje(p Politica, fechaHora, now time.Time, porNegocio bool) int {
	if porNegocio {
		return 100
	}
	if fechaHora.Sub(now) >= time.Duration(p.HorasLimite)*time.Hour {
		return p.PorcentajeAntes
	}
	return p.PorcentajeDespues
}

// Reembolso es el monto en centavos: sin pago registrado (nv-60) es 0.
func Reembolso(r Reserva, now time.Time, porNegocio bool) int64 {
	if r.PagoID == "" || r.Anticipo.MontoCentavos <= 0 {
		return 0
	}
	return r.Anticipo.MontoCentavos * int64(Porcentaje(r.Politica, r.FechaHora, now, porNegocio)) / 100
}

// CalcularAnticipo da el anticipo en centavos. `valor` está en pesos para `fijo` y
// `por_persona`, y en porcentaje (0–100) de `precio_promedio × personas` para `porcentaje`.
func CalcularAnticipo(a negocios.Anticipo, precioPromedio, personas int) int64 {
	if a.Valor <= 0 {
		return 0
	}
	switch a.Tipo {
	case AnticipoFijo:
		return int64(a.Valor) * 100
	case AnticipoPorcentaje:
		return int64(a.Valor) * int64(precioPromedio) * int64(personas)
	case AnticipoPorPersona:
		return int64(a.Valor) * 100 * int64(personas)
	}
	return 0
}

// alfabeto son 32 símbolos sin O, I, 0 ni 1 (no se confunden al dictar el código).
const alfabeto = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// NuevoCodigo genera un código de 6 símbolos con crypto/rand. Como 256 es múltiplo de 32, el
// byte & 31 no sesga.
func NuevoCodigo() (string, error) {
	buf := make([]byte, CodigoLongitud)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = alfabeto[b&31]
	}
	return string(buf), nil
}
