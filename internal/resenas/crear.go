package resenas

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

func errSinVisita() error {
	return apperr.Forbidden("necesitas un check-in válido propio de los últimos 30 días para reseñar")
}

// Crear guarda la reseña de una visita verificada propia. Sin texto o con texto aprobado se
// publica y cuenta; en cuarentena queda `en_moderacion` y entra a la cola; rechazada = 422 sin
// guardar (la visita sigue libre).
func (s *Service) Crear(ctx context.Context, cl *middleware.Claims, req CrearRequest) (ResenaResponse, error) {
	me, err := self(cl)
	if err != nil {
		return ResenaResponse{}, err
	}
	now := s.ahora()
	if !ChipsValidos(req.Chips) {
		return ResenaResponse{}, invalid("chips", "chips fuera del catálogo")
	}
	c, err := s.visitaLibre(ctx, me, req.CheckinID, now)
	if err != nil {
		return ResenaResponse{}, err
	}
	fotos, err := s.fotosDe(ctx, me, req.Fotos)
	if err != nil {
		return ResenaResponse{}, err
	}
	comentario := strings.TrimSpace(req.Comentario)
	veredicto := s.evaluar(ctx, comentario)
	if veredicto.Veredicto == moderacion.VeredictoRechazado {
		return ResenaResponse{}, errRechazado()
	}
	autor := me
	r := Resena{
		ID: bson.NewObjectID(), NegocioID: c.NegocioID, UsuarioID: &autor, CheckinID: c.ID, Calificacion: req.Calificacion,
		Aspectos: req.Aspectos.modelo(), Chips: noNil(req.Chips), Comentario: comentario, Fotos: fotos,
		Estado: EstadoEnModeracion, Reportes: []Reporte{}, Version: 1, CreadoEn: now, ActualizadoEn: now,
	}
	switch err := s.repo.Insertar(ctx, r); {
	case errors.Is(err, ErrDuplicado):
		return ResenaResponse{}, errVisitaResenada()
	case err != nil:
		return ResenaResponse{}, errStore()
	}
	s.alGuardar(ctx, r, veredicto)
	if guardada, err := s.repo.Buscar(ctx, r.ID); err == nil {
		r = guardada
	}
	return s.respuesta(r, now), nil
}

// visitaLibre exige un check-in válido propio de los últimos 30 días sin reseña, que la persona
// no haya reseñado ese negocio en 30 días y que el negocio esté visible y no sea suyo.
func (s *Service) visitaLibre(ctx context.Context, me bson.ObjectID, raw string, now time.Time) (Checkin, error) {
	cid, _ := parseID(raw)
	c, err := s.repo.Checkin(ctx, cid)
	switch {
	case errors.Is(err, ErrNoEncontrado):
		return Checkin{}, errSinVisita()
	case err != nil:
		return Checkin{}, errStore()
	case c.UsuarioID != me || !c.Valido || now.Sub(c.CreadoEn) > diasEn(DiasPorResenar):
		return Checkin{}, errSinVisita()
	case c.ResenaID != nil:
		return Checkin{}, errVisitaResenada()
	}
	n, err := s.repo.RecientesDe(ctx, me, c.NegocioID, now.Add(-diasEn(DiasEntreResenas)))
	if err != nil {
		return Checkin{}, errStore()
	}
	if n > 0 {
		return Checkin{}, apperr.Conflict("ya reseñaste este negocio en los últimos 30 días")
	}
	return c, s.negocioAjeno(ctx, c.NegocioID, me)
}

// negocioAjeno exige que el negocio esté visible y que no sea de quien reseña.
func (s *Service) negocioAjeno(ctx context.Context, id, me bson.ObjectID) error {
	b, ok, err := s.negocios.FindVisible(ctx, id)
	switch {
	case err != nil:
		return errStore()
	case !ok:
		return errNegocioNotFound()
	case b.OwnerID == me:
		return apperr.Forbidden("no puedes reseñar tu propio negocio")
	}
	return nil
}

// alGuardar enlaza el check-in y publica la reseña o la manda a la cola.
func (s *Service) alGuardar(ctx context.Context, r Resena, veredicto moderacion.Resultado) {
	if err := s.repo.MarcarResenado(ctx, r.CheckinID, r.ID); err != nil {
		s.log.WarnContext(ctx, "reseñas: el check-in no quedó enlazado", slog.String("evento", "resena_checkin_fallo"), slog.String("id", r.ID.Hex()))
	}
	if !veredicto.Publicable() {
		s.encolar(ctx, TipoColaResena, r.ID, *r.UsuarioID, textoCola(r), veredicto)
		return
	}
	if err := s.publicar(ctx, r.ID, EstadoEnModeracion); err != nil {
		// La reseña queda guardada: la decide un administrador en lugar de perderse.
		s.encolar(ctx, TipoColaResena, r.ID, *r.UsuarioID, textoCola(r), moderacion.Resultado{Falla: "recalculo"})
	}
}

func errVisitaResenada() error { return apperr.Conflict("esta visita ya tiene reseña") }
