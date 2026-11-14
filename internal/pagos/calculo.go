package pagos

import "time"

// Comision es el porcentaje del anticipo que retiene Wherever, redondeado al centavo.
func Comision(montoCentavos int64, pct int) int64 {
	if montoCentavos <= 0 || pct <= 0 {
		return 0
	}
	return (montoCentavos*int64(pct) + 50) / 100
}

// Desglose es lo que recibe el negocio de un pago: lo cobrado menos lo reembolsado y la comisión
// proporcional a lo que se quedó (un reembolso devuelve comisión en la misma proporción).
type Desglose struct {
	Anticipo   int64
	Reembolsos int64
	Comision   int64
	Neto       int64
}

// DesgloseDe calcula el desglose de un pago.
func DesgloseDe(p Pago) Desglose {
	reembolsado := p.Reembolsado()
	cobrado := p.MontoCentavos - reembolsado
	comision := int64(0)
	if p.MontoCentavos > 0 && cobrado > 0 {
		comision = p.ComisionCentavos * cobrado / p.MontoCentavos
	}
	return Desglose{Anticipo: p.MontoCentavos, Reembolsos: reembolsado, Comision: comision, Neto: cobrado - comision}
}

// Sumar acumula desgloses.
func (d Desglose) Sumar(o Desglose) Desglose {
	return Desglose{Anticipo: d.Anticipo + o.Anticipo, Reembolsos: d.Reembolsos + o.Reembolsos, Comision: d.Comision + o.Comision, Neto: d.Neto + o.Neto}
}

// SemanaInicio es el lunes 00:00 (hora de `loc`) de la semana de t, en UTC.
func SemanaInicio(t time.Time, loc *time.Location) time.Time {
	l := t.In(loc)
	dias := (int(l.Weekday()) + 6) % 7
	y, m, d := l.AddDate(0, 0, -dias).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, loc).UTC()
}

// ProximoLunes es el siguiente lunes 00:00 de `loc` después de t, en UTC.
func ProximoLunes(t time.Time, loc *time.Location) time.Time {
	return SemanaInicio(t, loc).In(loc).AddDate(0, 0, 7).UTC()
}

// ConservarHasta es la fecha de borrado por TTL: creado + 5 años.
func ConservarHasta(t time.Time) time.Time { return t.AddDate(Conservacion, 0, 0) }
