package resenas

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// propia trae la reseña del autor; ajena, eliminada o inexistente = 404.
func (s *Service) propia(ctx context.Context, me bson.ObjectID, raw string) (Resena, error) {
	id, ok := parseID(raw)
	if !ok {
		return Resena{}, errResenaNotFound()
	}
	r, err := s.repo.Buscar(ctx, id)
	if errors.Is(err, ErrNoEncontrado) {
		return Resena{}, errResenaNotFound()
	}
	if err != nil {
		return Resena{}, errStore()
	}
	if r.UsuarioID == nil || *r.UsuarioID != me || r.Estado == EstadoEliminada {
		return Resena{}, errResenaNotFound()
	}
	return r, nil
}

// editada aplica el cuerpo sobre la reseña y valida lo que el DTO no puede (chips y fotos).
func (s *Service) editada(ctx context.Context, me bson.ObjectID, r Resena, req EditarRequest) (Resena, bool, error) {
	nueva := r
	if req.Calificacion != nil {
		nueva.Calificacion = *req.Calificacion
	}
	if req.Aspectos != nil {
		if err := s.validarAspectos(*req.Aspectos); err != nil {
			return r, false, err
		}
		nueva.Aspectos = req.Aspectos.modelo()
	}
	if req.Chips != nil {
		if !ChipsValidos(*req.Chips) {
			return r, false, invalid("chips", "chips fuera del catálogo")
		}
		nueva.Chips = noNil(*req.Chips)
	}
	if req.Fotos != nil {
		fotos, err := s.fotosDe(ctx, me, *req.Fotos)
		if err != nil {
			return r, false, err
		}
		nueva.Fotos = fotos
	}
	cambioTexto := false
	if req.Comentario != nil {
		nueva.Comentario = strings.TrimSpace(*req.Comentario)
		cambioTexto = nueva.Comentario != r.Comentario
	}
	return nueva, cambioTexto, nil
}

// validarAspectos revisa los aspectos de un PATCH: el validador no entra a punteros a struct
// sin `required`.
func (s *Service) validarAspectos(a AspectosRequest) error {
	for _, v := range []int{a.Sabor, a.Atencion, a.Ambiente, a.Espera, a.PrecioCalidad} {
		if v < 1 || v > 5 {
			return invalid("aspectos", "cada aspecto va de 1 a 5")
		}
	}
	return nil
}

func editablePor(r Resena, now time.Time, version int) error {
	switch {
	case !Editable(r.CreadoEn, now):
		return apperr.Forbidden("la reseña solo se edita en sus primeras 48 horas")
	case r.Estado != EstadoPublicada:
		return apperr.Conflict("la reseña está en revisión u oculta; no se puede editar")
	case r.Version != version:
		return errStale()
	}
	return nil
}

// revisada aplica el cambio y modera el texto nuevo: rechazado = 422; en cuarentena deja de
// contar y queda en moderación.
func (s *Service) revisada(ctx context.Context, me bson.ObjectID, r Resena, req EditarRequest) (Resena, moderacion.Resultado, error) {
	veredicto := moderacion.Resultado{Veredicto: moderacion.VeredictoAprobado}
	nueva, cambioTexto, err := s.editada(ctx, me, r, req)
	if err != nil {
		return r, veredicto, err
	}
	if cambioTexto {
		veredicto = s.evaluar(ctx, nueva.Comentario)
	}
	if veredicto.Veredicto == moderacion.VeredictoRechazado {
		return r, veredicto, errRechazado()
	}
	if !veredicto.Publicable() {
		nueva.Estado, nueva.Contada = EstadoEnModeracion, false
	}
	return nueva, veredicto, nil
}

// Editar cambia la reseña en sus primeras 48 h con If-Match. Un texto nuevo vuelve a moderarse:
// en cuarentena deja de contar y entra a la cola. En moderación u oculta = 409.
func (s *Service) Editar(ctx context.Context, cl *middleware.Claims, raw string, version int, req EditarRequest) (ResenaResponse, error) {
	me, err := self(cl)
	if err != nil {
		return ResenaResponse{}, err
	}
	r, err := s.propia(ctx, me, raw)
	if err != nil {
		return ResenaResponse{}, err
	}
	now := s.ahora()
	if err := editablePor(r, now, version); err != nil {
		return ResenaResponse{}, err
	}
	nueva, veredicto, err := s.revisada(ctx, me, r, req)
	if err != nil {
		return ResenaResponse{}, err
	}
	antes, err := s.repo.Transicion(ctx, bson.D{
		{Key: "_id", Value: r.ID}, {Key: "usuario_id", Value: me}, {Key: "version", Value: version}, {Key: "estado", Value: EstadoPublicada},
	}, bson.D{
		{Key: "calificacion", Value: nueva.Calificacion}, {Key: "aspectos", Value: nueva.Aspectos}, {Key: "chips", Value: nueva.Chips},
		{Key: "comentario", Value: nueva.Comentario}, {Key: "fotos", Value: nueva.Fotos}, {Key: "estado", Value: nueva.Estado},
		{Key: "contada", Value: nueva.Contada},
	}, now)
	if errors.Is(err, ErrNoAplica) {
		return ResenaResponse{}, errStale()
	}
	if err != nil {
		return ResenaResponse{}, errStore()
	}
	if err := s.aplicar(ctx, antes, nueva); err != nil {
		return ResenaResponse{}, err
	}
	if !veredicto.Publicable() {
		s.encolar(ctx, TipoColaResena, r.ID, me, textoCola(nueva), veredicto)
	}
	nueva.Version, nueva.ActualizadoEn = antes.Version+1, now
	return s.respuesta(nueva, now), nil
}

// Borrar pasa la reseña a `eliminada` (el check-in no se reutiliza) y la descuenta. Repetirlo
// responde igual (idempotente).
func (s *Service) Borrar(ctx context.Context, cl *middleware.Claims, raw string) error {
	me, err := self(cl)
	if err != nil {
		return err
	}
	id, ok := parseID(raw)
	if !ok {
		return errResenaNotFound()
	}
	r, err := s.repo.Buscar(ctx, id)
	if errors.Is(err, ErrNoEncontrado) || (err == nil && (r.UsuarioID == nil || *r.UsuarioID != me)) {
		return errResenaNotFound()
	}
	if err != nil {
		return errStore()
	}
	_, err = s.retirar(ctx, bson.D{{Key: "_id", Value: id}, {Key: "usuario_id", Value: me}, {Key: "estado", Value: bson.D{{Key: "$ne", Value: EstadoEliminada}}}}, EstadoEliminada)
	if errors.Is(err, ErrNoAplica) {
		return nil
	}
	return err
}

// publicada trae una reseña visible; las demás = 404.
func (s *Service) publicada(ctx context.Context, raw string) (Resena, error) {
	id, ok := parseID(raw)
	if !ok {
		return Resena{}, errResenaNotFound()
	}
	r, err := s.repo.Buscar(ctx, id)
	if errors.Is(err, ErrNoEncontrado) || (err == nil && r.Estado != EstadoPublicada) {
		return Resena{}, errResenaNotFound()
	}
	if err != nil {
		return Resena{}, errStore()
	}
	return r, nil
}

// Util marca o desmarca «me fue útil». Repetir la misma intención no cambia el conteo.
func (s *Service) Util(ctx context.Context, cl *middleware.Claims, raw string, req UtilRequest) (UtilResponse, error) {
	me, err := self(cl)
	if err != nil {
		return UtilResponse{}, err
	}
	r, err := s.publicada(ctx, raw)
	if err != nil {
		return UtilResponse{}, err
	}
	total, err := s.repo.MarcarUtil(ctx, r.ID, me, *req.Util, s.ahora())
	if err != nil {
		return UtilResponse{}, errStore()
	}
	return UtilResponse{Util: *req.Util, Total: total}, nil
}

// Reportar agrega el reporte; al tercero de personas distintas la reseña se oculta, deja de
// contar y entra a la cola de nv-43.
func (s *Service) Reportar(ctx context.Context, cl *middleware.Claims, raw string, req ReporteRequest) (ReporteResponse, error) {
	me, err := self(cl)
	if err != nil {
		return ReporteResponse{}, err
	}
	r, err := s.publicada(ctx, raw)
	if err != nil {
		return ReporteResponse{}, err
	}
	if r.UsuarioID != nil && *r.UsuarioID == me {
		return ReporteResponse{}, apperr.Forbidden("no puedes reportar tu propia reseña")
	}
	por := me
	despues, err := s.repo.AgregarReporte(ctx, r.ID, me, Reporte{Por: &por, Motivo: req.Motivo, Fecha: s.ahora()})
	switch {
	case errors.Is(err, ErrNoAplica):
		if _, errP := s.publicada(ctx, raw); errP != nil {
			return ReporteResponse{}, errP
		}
		return ReporteResponse{}, apperr.Conflict("ya reportaste esta reseña")
	case err != nil:
		return ReporteResponse{}, errStore()
	}
	if len(despues.Reportes) >= ReportesParaOcultar {
		if err := s.ocultarPorReportes(ctx, r.ID); err != nil {
			return ReporteResponse{}, err
		}
	}
	return ReporteResponse{Reportada: true}, nil
}

// ocultarPorReportes oculta la reseña, la descuenta y la manda a la cola de nv-43. Si otro
// reporte simultáneo ya la ocultó, no hace nada; anonimizada queda oculta (la cola exige autor).
func (s *Service) ocultarPorReportes(ctx context.Context, id bson.ObjectID) error {
	antes, err := s.retirar(ctx, bson.D{{Key: "_id", Value: id}, {Key: "estado", Value: EstadoPublicada}}, EstadoOculta)
	switch {
	case errors.Is(err, ErrNoAplica):
		return nil
	case err != nil:
		return err
	case antes.UsuarioID != nil:
		s.encolar(ctx, TipoColaResena, id, *antes.UsuarioID, textoCola(antes), moderacion.Resultado{Falla: FallaReportes})
	}
	return nil
}

// fechaCursor da el valor del cursor por fecha.
func fechaCursor(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }
