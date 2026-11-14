package cupones

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// tocaContenido dice si el PATCH cambia algo más que vigencia_fin, limite_usos y estado.
func (r EditarRequest) tocaContenido() bool {
	return r.Titulo != nil || r.Tipo != nil || r.Valor != nil || r.Condiciones != nil ||
		r.VigenciaInicio != nil || r.DiasValidos != nil || r.HorasValidas != nil
}

// editado es el cupón con el PATCH aplicado en memoria, antes de validar y guardar.
type editado struct {
	c     Cupon
	set   bson.D
	unset []string
}

func (e *editado) poner(campo string, v any) { e.set = append(e.set, bson.E{Key: campo, Value: v}) }

// contenido aplica título, tipo, valor, condiciones, días y horas. Si cambia el tipo sin
// valor, el valor anterior se descarta.
func (e *editado) contenido(r EditarRequest) error {
	if r.Titulo != nil {
		e.c.Titulo = *r.Titulo
		e.poner("titulo", e.c.Titulo)
	}
	if r.Condiciones != nil {
		e.c.Condiciones = *r.Condiciones
		e.poner("condiciones", e.c.Condiciones)
	}
	if err := e.tipoValor(r); err != nil {
		return err
	}
	if r.DiasValidos != nil {
		e.c.DiasValidos = *r.DiasValidos
		e.poner("dias_validos", e.c.DiasValidos)
	}
	if r.HorasValidas != nil {
		if err := ValidarHoras(r.HorasValidas); err != nil {
			return invalid("horas_validas", err.Error())
		}
		e.c.HorasValidas = r.HorasValidas
		e.poner("horas_validas", e.c.HorasValidas)
	}
	return nil
}

// tipoValor aplica tipo y valor; si cambia el tipo sin valor, el anterior se descarta.
func (e *editado) tipoValor(r EditarRequest) error {
	if r.Tipo == nil && r.Valor == nil {
		return nil
	}
	valor := r.Valor
	if r.Tipo != nil && *r.Tipo != e.c.Tipo && valor == nil {
		e.c.Valor = 0
	}
	if r.Tipo != nil {
		e.c.Tipo = *r.Tipo
	}
	if valor == nil && e.c.Valor != 0 {
		valor = &e.c.Valor
	}
	if err := ValidarValor(e.c.Tipo, valor); err != nil {
		return invalid("valor", err.Error())
	}
	e.c.Valor = valorGuardado(valor)
	e.poner("tipo", e.c.Tipo)
	e.poner("valor", e.c.Valor)
	return nil
}

// fechas aplica la vigencia y exige inicio < fin.
func (e *editado) fechas(r EditarRequest, now time.Time) error {
	if r.VigenciaInicio == nil && r.VigenciaFin == nil {
		return nil
	}
	if r.VigenciaInicio != nil {
		t, err := parseFecha("vigencia_inicio", *r.VigenciaInicio)
		if err != nil {
			return err
		}
		e.c.VigenciaInicio = t
		e.poner("vigencia_inicio", t)
	}
	if r.VigenciaFin != nil {
		t, err := parseFecha("vigencia_fin", *r.VigenciaFin)
		if err != nil {
			return err
		}
		if !t.After(now) {
			return invalid("vigencia_fin", "la vigencia ya terminó")
		}
		e.c.VigenciaFin = t
		e.poner("vigencia_fin", t)
	}
	if err := ValidarVigencia(e.c.VigenciaInicio, e.c.VigenciaFin); err != nil {
		return invalid("vigencia_fin", err.Error())
	}
	return nil
}

// limite aplica `limite_usos` (0 = sin límite; nunca menor que los usos hechos).
func (e *editado) limite(r EditarRequest) error {
	if r.LimiteUsos == nil {
		return nil
	}
	if *r.LimiteUsos > 0 && *r.LimiteUsos < e.c.UsosActuales {
		return invalid("limite_usos", "no puede ser menor que usos_actuales")
	}
	e.c.LimiteUsos = *r.LimiteUsos
	e.poner("limite_usos", e.c.LimiteUsos)
	return nil
}

// estado resuelve el estado final: el pedido (activo o pausado) o el que deja el cambio de
// límite y fechas (un agotado o vencido que vuelve a tener cupo o vigencia se activa).
func (e *editado) estado(r EditarRequest, previo string, now time.Time) error {
	deseado := previo
	if r.Estado != nil {
		if previo != EstadoActivo && previo != EstadoPausado && previo != EstadoProgramado {
			return apperr.Conflict("estado_no_permite: un cupón " + previo + " no se pausa ni se activa")
		}
		deseado = *r.Estado
	}
	if deseado == EstadoPausado {
		if r.Estado == nil {
			return nil // sigue en pausa (y conserva si la puso el negocio)
		}
		e.poner("estado", EstadoPausado)
		e.unset = append(e.unset, "pausado_por_negocio")
		return nil
	}
	deseado = e.c.estadoPorFechas(now)
	if deseado != previo || r.Estado != nil {
		e.poner("estado", deseado)
		e.unset = append(e.unset, "pausado_por_negocio")
	}
	return nil
}

// estadoPorFechas es el estado que toca a un cupón no pausado según usos y fechas.
func (c Cupon) estadoPorFechas(now time.Time) string {
	switch {
	case c.Agotado():
		return EstadoAgotado
	case !c.VigenciaFin.After(now):
		return EstadoVencido
	case c.VigenciaInicio.After(now):
		return EstadoProgramado
	}
	return EstadoActivo
}

// Editar aplica el PATCH con If-Match. Con canjes solo cambian vigencia_fin, limite_usos y
// estado (409). Las carteras ya guardadas conservan su copia del título.
func (s *Service) Editar(ctx context.Context, cl *middleware.Claims, rawNegocio, rawCupon string, version int, r EditarRequest) (CuponResponse, error) {
	b, c, err := s.cuponPropio(ctx, cl, rawNegocio, rawCupon)
	if err != nil {
		return CuponResponse{}, err
	}
	if c.Version != version {
		return CuponResponse{}, errStale()
	}
	if c.UsosActuales > 0 && r.tocaContenido() {
		return CuponResponse{}, errConCanjes()
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	e := &editado{c: c}
	for _, paso := range []func() error{
		func() error { return e.contenido(r) },
		func() error { return e.fechas(r, now) },
		func() error { return e.limite(r) },
		func() error { return e.estado(r, c.Estado, now) },
	} {
		if err := paso(); err != nil {
			return CuponResponse{}, err
		}
	}
	if len(e.set) == 0 {
		return duenoResponse(c), nil
	}
	out, err := s.repo.CambiarSi(ctx, c.ID, b.ID, version, bson.D{{Key: "usos_actuales", Value: c.UsosActuales}}, e.set, e.unset, now)
	if errors.Is(err, ErrNoAplica) {
		return CuponResponse{}, errStale()
	}
	if err != nil {
		return CuponResponse{}, errStore()
	}
	s.recalcular(ctx, b.ID)
	return duenoResponse(out), nil
}
