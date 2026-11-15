package eventos

import (
	"context"
	"errors"
	"strings"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

func errNoReabre() error {
	return apperr.Conflict("un evento cancelado no vuelve a publicarse; crea uno nuevo")
}

// editable trae el evento y exige permiso de edición: inexistente = 404, otro usuario = 403.
func (s *Service) editable(ctx context.Context, cl *middleware.Claims, raw string) (Evento, error) {
	if _, err := quien(cl); err != nil {
		return Evento{}, err
	}
	e, err := s.buscar(ctx, raw)
	if err != nil {
		return Evento{}, err
	}
	ok, err := s.puedeEditar(ctx, cl, e)
	if err != nil {
		return Evento{}, err
	}
	if !ok {
		return Evento{}, apperr.Forbidden("solo quien lo creó, el dueño del negocio o un administrador lo cambian")
	}
	return e, nil
}

// Editar aplica el cambio parcial con If-Match; `estado: cancelado` cancela y un cancelado no
// se reabre (409).
func (s *Service) Editar(ctx context.Context, cl *middleware.Claims, raw string, version int, req EditarRequest) (EventoView, error) {
	e, err := s.editable(ctx, cl, raw)
	if err != nil {
		return EventoView{}, err
	}
	if e.Version != version {
		return EventoView{}, errVersion()
	}
	res, err := s.cambiar(ctx, &e, req)
	if err != nil {
		return EventoView{}, err
	}
	e.ActualizadoEn = s.now().UTC()
	if err := s.repo.Reemplazar(ctx, &e, version); err != nil {
		return EventoView{}, guardarError(err)
	}
	me, _ := quien(cl)
	s.encolar(ctx, e, me, res)
	return nuevaVista(e), nil
}

// cambiar aplica el cambio al evento y modera el texto si cambió.
func (s *Service) cambiar(ctx context.Context, e *Evento, req EditarRequest) (moderacion.Resultado, error) {
	var res moderacion.Resultado
	if e.Estado == EstadoCancelado && req.Estado != nil && *req.Estado != EstadoCancelado {
		return res, errNoReabre()
	}
	if err := s.aplicarFechas(e, req); err != nil {
		return res, err
	}
	texto, err := aplicarCampos(e, req)
	if err != nil {
		return res, err
	}
	if err := s.foto(e.FotoURL); err != nil {
		return res, err
	}
	if !texto {
		return res, nil
	}
	if res, err = s.moderar(ctx, *e); err != nil {
		return res, err
	}
	e.EnRevision = res.Veredicto == moderacion.VeredictoCuarentena
	return res, nil
}

func guardarError(err error) error {
	switch {
	case errors.Is(err, ErrNoEncontrado):
		return errNoEncontrado()
	case errors.Is(err, ErrVersion):
		return errVersion()
	default:
		return errStore()
	}
}

// aplicarFechas cambia inicio o fin y las valida solo si cambian (un evento en curso puede
// editarse sin tocar sus horas).
func (s *Service) aplicarFechas(e *Evento, req EditarRequest) error {
	if req.Inicio == nil && req.Fin == nil {
		return nil
	}
	inicio, fin := e.Inicio, e.Fin
	var err error
	if req.Inicio != nil {
		if inicio, err = parseHora("inicio", *req.Inicio); err != nil {
			return err
		}
	}
	if req.Fin != nil {
		if fin, err = parseHora("fin", *req.Fin); err != nil {
			return err
		}
	}
	if err := s.fechas(inicio, fin); err != nil {
		return err
	}
	e.Inicio, e.Fin = inicio, fin
	return nil
}

// aplicarCampos copia el resto del cambio; dice si cambió texto que hay que moderar.
func aplicarCampos(e *Evento, req EditarRequest) (bool, error) {
	texto := false
	if req.Titulo != nil {
		e.Titulo, texto = strings.TrimSpace(*req.Titulo), true
	}
	if req.Descripcion != nil {
		e.Descripcion, texto = strings.TrimSpace(*req.Descripcion), true
	}
	if req.Direccion != nil {
		e.Direccion = strings.TrimSpace(*req.Direccion)
	}
	if e.Titulo == "" || e.Direccion == "" {
		return false, invalid("titulo", "titulo y direccion no pueden ir vacíos")
	}
	aplicarResto(e, req)
	return texto, nil
}

// aplicarResto copia los campos que no se moderan ni se validan aparte.
func aplicarResto(e *Evento, req EditarRequest) {
	if req.Tipo != nil {
		e.Tipo = *req.Tipo
	}
	if req.Ubicacion != nil {
		e.Ubicacion = nuevoPunto(*req.Ubicacion.Lat, *req.Ubicacion.Lng)
	}
	if req.Precio != nil {
		e.Precio = *req.Precio
	}
	if req.FotoURL != nil {
		e.FotoURL = *req.FotoURL
	}
	if req.Estado != nil {
		e.Estado = *req.Estado
	}
}

// Borrar borra el evento con If-Match (204).
func (s *Service) Borrar(ctx context.Context, cl *middleware.Claims, raw string, version int) error {
	e, err := s.editable(ctx, cl, raw)
	if err != nil {
		return err
	}
	if e.Version != version {
		return errVersion()
	}
	return guardarErrorNil(s.repo.Borrar(ctx, e.ID, version))
}

func guardarErrorNil(err error) error {
	if err == nil {
		return nil
	}
	return guardarError(err)
}

// Destino aplica la decisión del administrador sobre un evento en revisión (nv-43).
type Destino struct{ s *Service }

// Destino devuelve el receptor de decisiones de la cola de moderación.
func (s *Service) Destino() Destino { return Destino{s: s} }

// AlDecidir aprueba (lo muestra) o rechaza (lo deja en borrador) el evento.
func (d Destino) AlDecidir(ctx context.Context, it moderacion.Item) error {
	e, err := d.s.buscar(ctx, it.Referencia)
	if err != nil {
		return nil // ya no existe: no hay nada que decidir
	}
	return d.s.repo.Decidir(ctx, e.ID, it.Estado == moderacion.EstadoAprobado, d.s.now().UTC())
}
