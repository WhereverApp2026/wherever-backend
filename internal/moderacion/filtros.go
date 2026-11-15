package moderacion

import (
	"context"
	"errors"
	"fmt"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// FiltrosEditables son los filtros con reglas propias (nv-71); `palabra_bloqueada` no: sus
// palabras se ajustan con PATCH /reglas.
var FiltrosEditables = []string{FiltroDesnudez, FiltroViolencia, FiltroDrogas, FiltroOdio, FiltroSexual, FiltroAcoso, FiltroSpam, FiltroDatosPersonales}

// ReglaFiltro son los umbrales propios de un filtro; el que falta hereda el global. Un filtro
// con Activa en false no cuenta al decidir.
type ReglaFiltro struct {
	Cuarentena *float64 `bson:"cuarentena,omitempty"`
	Rechazo    *float64 `bson:"rechazo,omitempty"`
	Activa     bool     `bson:"activa"`
}

// ErrFiltroObligatorio: `datos_personales` no se puede desactivar.
var ErrFiltroObligatorio = errors.New("moderacion: el filtro de datos personales no se desactiva")

func editable(filtro string) bool {
	for _, f := range FiltrosEditables {
		if f == filtro {
			return true
		}
	}
	return false
}

// umbrales son los de un filtro: los propios o los globales.
func (r Reglas) umbrales(filtro string) (float64, float64, bool) {
	q, x := r.Cuarentena, r.Rechazo
	f, ok := r.Filtros[filtro]
	if !ok {
		return q, x, true
	}
	if f.Cuarentena != nil {
		q = *f.Cuarentena
	}
	if f.Rechazo != nil {
		x = *f.Rechazo
	}
	return q, x, f.Activa
}

// Decidir da el veredicto más severo entre los filtros con puntaje, cada uno con sus umbrales;
// los inactivos no cuentan. Sin reglas por filtro es Clasificar del puntaje más alto.
func (r Reglas) Decidir(puntajes map[string]float64) Veredicto {
	out := VeredictoAprobado
	for filtro, p := range puntajes {
		q, x, activa := r.umbrales(filtro)
		if !activa {
			continue
		}
		v := Reglas{Cuarentena: q, Rechazo: x}.Clasificar(p)
		if v == VeredictoRechazado {
			return v
		}
		if v == VeredictoCuarentena {
			out = v
		}
	}
	return out
}

// validarFiltros exige los mismos bordes en cada filtro (con el global para el que falte) y que
// `datos_personales` siga activo.
func (r Reglas) validarFiltros() error {
	for filtro, f := range r.Filtros {
		if !editable(filtro) {
			return fmt.Errorf("%w: filtro desconocido %q", ErrReglasInvalid, filtro)
		}
		if filtro == FiltroDatosPersonales && !f.Activa {
			return ErrFiltroObligatorio
		}
		q, x, _ := r.umbrales(filtro)
		if err := (Reglas{Cuarentena: q, Rechazo: x}).Validar(); err != nil {
			return fmt.Errorf("%w (filtro %s)", err, filtro)
		}
	}
	return nil
}

// FiltroPatch es el cuerpo de PATCH /admin/moderacion/reglas/{filtro}.
type FiltroPatch struct {
	Cuarentena *float64 `json:"cuarentena" validate:"omitempty,min=40,max=100"`
	Rechazo    *float64 `json:"rechazo" validate:"omitempty,min=40,max=100"`
	Activa     *bool    `json:"activa"`
}

// FiltroView es un filtro en GET /reglas; `propio: false` = hereda los umbrales globales.
type FiltroView struct {
	Cuarentena float64 `json:"cuarentena"`
	Rechazo    float64 `json:"rechazo"`
	Activa     bool    `json:"activa"`
	Propio     bool    `json:"propio"`
}

func filtrosView(r Reglas) map[string]FiltroView {
	out := make(map[string]FiltroView, len(FiltrosEditables))
	for _, filtro := range FiltrosEditables {
		q, x, activa := r.umbrales(filtro)
		f := r.Filtros[filtro]
		out[filtro] = FiltroView{Cuarentena: q, Rechazo: x, Activa: activa, Propio: f.Cuarentena != nil || f.Rechazo != nil}
	}
	return out
}

func errFiltroNoEncontrado() error { return apperr.NotFound("filtro no encontrado") }

// AjustarFiltro cambia los umbrales o la actividad de un filtro con la versión del documento
// de reglas (nv-71).
func (s *Service) AjustarFiltro(ctx context.Context, subject, filtro string, version int, req FiltroPatch) (ReglasView, error) {
	if req.Cuarentena == nil && req.Rechazo == nil && req.Activa == nil {
		return ReglasView{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "activa", Message: "manda cuarentena, rechazo o activa"})
	}
	return s.cambiarFiltro(ctx, subject, filtro, version, func(f *ReglaFiltro) {
		if req.Cuarentena != nil {
			f.Cuarentena = req.Cuarentena
		}
		if req.Rechazo != nil {
			f.Rechazo = req.Rechazo
		}
		if req.Activa != nil {
			f.Activa = *req.Activa
		}
	})
}

// DesactivarFiltro deja el filtro en `activa: false` y conserva sus umbrales.
func (s *Service) DesactivarFiltro(ctx context.Context, subject, filtro string, version int) error {
	_, err := s.cambiarFiltro(ctx, subject, filtro, version, func(f *ReglaFiltro) { f.Activa = false })
	return err
}

func (s *Service) cambiarFiltro(ctx context.Context, subject, filtro string, version int, cambio func(*ReglaFiltro)) (ReglasView, error) {
	who, err := revisor(subject)
	if err != nil {
		return ReglasView{}, err
	}
	if !editable(filtro) {
		return ReglasView{}, errFiltroNoEncontrado()
	}
	actual, err := s.repo.LeerReglas(ctx)
	if errors.Is(err, ErrNoEncontrado) {
		return ReglasView{}, apperr.NotFound("no hay reglas guardadas; créalas con POST")
	}
	if err != nil {
		return ReglasView{}, errStore()
	}
	r := actual.reglas()
	filtros := make(map[string]ReglaFiltro, len(r.Filtros)+1)
	for k, v := range r.Filtros {
		filtros[k] = v
	}
	f, ok := filtros[filtro]
	if !ok {
		f.Activa = true
	}
	cambio(&f)
	filtros[filtro] = f
	r.Filtros = filtros
	if errors.Is(r.validarFiltros(), ErrFiltroObligatorio) {
		return ReglasView{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "activa", Message: "el filtro de datos personales no se puede desactivar"})
	}
	return s.guardarReglas(ctx, who, version, r)
}
