package asistente

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
)

const (
	// PlazoTotal es lo máximo que espera la persona antes del respaldo (nv-33, US-039).
	PlazoTotal = 8 * time.Second
	// reservaRespaldo es lo que se guarda del plazo para armar el plan por reglas.
	reservaRespaldo = time.Second
	// MaxExcluir es cuántos lugares ya propuestos acepta «otra idea».
	MaxExcluir = 30

	avisoReglas     = "La IA no está disponible ahora; armamos el plan con tus gustos y los lugares abiertos cerca. Revisa horarios y precios antes de salir."
	motivoCancelada = "cancelada"
)

// errIA marca una falla del modelo (no de la API): lleva al respaldo por reglas.
type errIA struct{ err error }

func (e errIA) Error() string { return e.err.Error() }
func (e errIA) Unwrap() error { return e.err }

// errApagada: sin proveedor configurado responde siempre el motor de reglas.
var errApagada = errors.New("ia: asistente sin proveedor")

// motivosIA traduce la falla a la etiqueta del log; una fila por error (gocyclo).
var motivosIA = []struct {
	err    error
	motivo string
}{
	{errApagada, "apagada"},
	{ia.ErrTiempo, "tiempo"}, {context.DeadlineExceeded, "tiempo"},
	{ia.ErrLimiteProveedor, "limite_proveedor"}, {ia.ErrTope, "tope"}, {ia.ErrGastoDesconocido, "gasto_desconocido"},
	{ia.ErrLimiteUsuario, "limite_usuario"}, {ia.ErrLimiteGlobal, "limite_global"}, {ia.ErrLimiteDesconocido, "limite_desconocido"},
	{ia.ErrRespuestaInvalida, "respuesta_invalida"}, {ia.ErrBloqueada, "bloqueada"}, {context.Canceled, motivoCancelada},
}

func motivoIA(err error) string {
	var lim *ia.LimiteError
	if errors.As(err, &lim) {
		return "limite_proveedor"
	}
	for _, m := range motivosIA {
		if errors.Is(err, m.err) {
			return m.motivo
		}
	}
	return "proveedor"
}

// validador es un modelo que trae su propia validación con caché y límite del usuario
// (ia.Guardia, nv-27/nv-28).
type validador interface {
	GenerarValidado(ctx context.Context, pet ia.Peticion, destino any) (ia.Respuesta, error)
}

// generarValidado pasa por la guardia si el modelo la tiene; si no, valida aquí.
func generarValidado(ctx context.Context, m ia.Modelo, pet ia.Peticion, destino any) (ia.Respuesta, error) {
	if v, ok := m.(validador); ok {
		return v.GenerarValidado(ctx, pet, destino)
	}
	return ia.GenerarValidado(ctx, m, pet, destino)
}

// interpretacionVacia es la de respaldo cuando la IA no entendió la frase: el motor usa solo
// el perfil y la hora pedida.
func interpretacionVacia() Interpretacion {
	return Interpretacion{Compania: SinDato, PalabrasClave: []string{}}
}

// idsExcluidos convierte los ids ya validados (objectid) de «otra idea»; uno ilegible se
// ignora: no puede coincidir con ningún negocio.
func idsExcluidos(hex []string) []bson.ObjectID {
	out := make([]bson.ObjectID, 0, len(hex))
	for _, h := range hex {
		if id, err := bson.ObjectIDFromHex(h); err == nil {
			out = append(out, id)
		}
	}
	return out
}
