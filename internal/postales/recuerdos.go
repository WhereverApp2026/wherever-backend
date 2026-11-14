package postales

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

func respuestaRecuerdo(r Recuerdo) RecuerdoResponse {
	ids := make([]string, 0, len(r.PostalesIDs))
	for _, id := range r.PostalesIDs {
		ids = append(ids, id.Hex())
	}
	paradas := r.Paradas
	if paradas == nil {
		paradas = []Parada{}
	}
	for i := range paradas {
		paradas[i].Hora = paradas[i].Hora.UTC()
	}
	return RecuerdoResponse{
		ID: r.ID.Hex(), PlanID: r.PlanID.Hex(), Titulo: r.Titulo, Fecha: r.Fecha.UTC(), Paradas: paradas, PostalesIDs: ids,
		GastoTotal: r.GastoTotal, PuntosGanados: r.PuntosGanados, TarjetaImagenID: r.TarjetaImagenID, TarjetaURL: r.TarjetaURL,
		CreadoEn: r.CreadoEn.UTC(), ActualizadoEn: r.ActualizadoEn.UTC(),
	}
}

// paradas copia las paradas del plan con el nombre y la portada de cada negocio.
func (s *Service) paradas(ctx context.Context, p planes.Plan) ([]Parada, error) {
	ids := make([]bson.ObjectID, 0, len(p.Paradas))
	for _, pa := range p.Paradas {
		ids = append(ids, pa.NegocioID)
	}
	bs, err := s.Negocios.FindFavoriteCards(ctx, ids)
	if err != nil {
		return nil, errStore()
	}
	porID := make(map[bson.ObjectID]negocios.Business, len(bs))
	for _, b := range bs {
		porID[b.ID] = b
	}
	out := make([]Parada, 0, len(p.Paradas))
	for _, pa := range p.Paradas {
		b := porID[pa.NegocioID]
		out = append(out, Parada{Nombre: b.Name, Hora: pa.Llega.UTC(), FotoURL: negocios.Cover(b.Photos)})
	}
	return out, nil
}

// tarjeta valida la tarjeta que dibujó la app: imagen propia ya publicada (422 si no).
func (s *Service) tarjeta(ctx context.Context, me bson.ObjectID, id string) (string, error) {
	if id == "" {
		return "", nil
	}
	if s.imagenes == nil {
		return "", errSinImagenes()
	}
	v, err := s.imagenes.Consultar(ctx, me, id)
	if errors.Is(err, imagenes.ErrNotFound) || (err == nil && v.Estado != imagenes.EstadoPublicada) {
		return "", invalid("tarjeta_imagen_id", "la tarjeta debe ser una imagen propia ya publicada")
	}
	if err != nil {
		return "", errStore()
	}
	return v.URL, nil
}

// GenerarRecuerdo arma (o rehace) el recuerdo de un plan pasado, propio o de un grupo donde la
// persona está confirmada. Devuelve si ya existía (200) o es nuevo (201).
func (s *Service) GenerarRecuerdo(ctx context.Context, cl *middleware.Claims, raw string, req RecuerdoRequest) (RecuerdoResponse, bool, error) {
	me, err := self(cl)
	if err != nil {
		return RecuerdoResponse{}, false, err
	}
	planID, ok := parseID(raw)
	if !ok {
		return RecuerdoResponse{}, false, errPlanNotFound()
	}
	p, err := s.planVisible(ctx, me, planID)
	if err != nil {
		return RecuerdoResponse{}, false, err
	}
	if !PlanPasado(p.Fecha, s.now(), s.loc) {
		return RecuerdoResponse{}, false, invalid("plan", "el plan aún no ha pasado")
	}
	url, err := s.tarjeta(ctx, me, req.TarjetaImagenID)
	if err != nil {
		return RecuerdoResponse{}, false, err
	}
	paradas, err := s.paradas(ctx, p)
	if err != nil {
		return RecuerdoResponse{}, false, err
	}
	postales, err := s.Repo.IDsDePlan(ctx, me, planID)
	if err != nil {
		return RecuerdoResponse{}, false, errStore()
	}
	now := s.ahora()
	rec, existia, err := s.Repo.GuardarRecuerdo(ctx, Recuerdo{
		ID: bson.NewObjectID(), UsuarioID: me, PlanID: planID, Titulo: p.Nombre, Fecha: p.Fecha, Paradas: paradas,
		PostalesIDs: postales, GastoTotal: Centavos(p.CostoTotal), TarjetaImagenID: req.TarjetaImagenID, TarjetaURL: url,
		CreadoEn: now, ActualizadoEn: now,
	})
	if err != nil {
		return RecuerdoResponse{}, false, errStore()
	}
	s.Log.InfoContext(ctx, "recuerdo generado", slog.String("evento", "recuerdo_generado"), slog.String("id", rec.ID.Hex()), slog.Bool("regenerado", existia))
	return respuestaRecuerdo(rec), existia, nil
}

// RecuerdoDePlan devuelve el recuerdo propio de un plan visible; si no hay, 404.
func (s *Service) RecuerdoDePlan(ctx context.Context, cl *middleware.Claims, raw string) (RecuerdoResponse, error) {
	me, err := self(cl)
	if err != nil {
		return RecuerdoResponse{}, err
	}
	planID, ok := parseID(raw)
	if !ok {
		return RecuerdoResponse{}, errPlanNotFound()
	}
	if _, err := s.planVisible(ctx, me, planID); err != nil {
		return RecuerdoResponse{}, err
	}
	r, err := s.Repo.RecuerdoDePlan(ctx, me, planID)
	if errors.Is(err, ErrNoEncontrado) {
		return RecuerdoResponse{}, errRecuerdoNotFound()
	}
	if err != nil {
		return RecuerdoResponse{}, errStore()
	}
	return respuestaRecuerdo(r), nil
}

// Recuerdos pagina los recuerdos propios por fecha del plan, del más nuevo al más viejo.
func (s *Service) Recuerdos(ctx context.Context, cl *middleware.Claims, q RecuerdosQuery) (pagination.PageResult[RecuerdoResponse], error) {
	var vacio pagination.PageResult[RecuerdoResponse]
	me, err := self(cl)
	if err != nil {
		return vacio, err
	}
	var desde, hasta time.Time
	if q.Mes != "" {
		desde, hasta, _ = RangoMes(q.Mes, s.loc)
	}
	scope := s.Cursor.Scope("recuerdos", me.Hex(), q.Mes)
	after, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return vacio, err
	}
	rows, err := s.Repo.Recuerdos(ctx, me, desde, hasta, after, q.Limite)
	if err != nil {
		return vacio, errStore()
	}
	rows, next, err := pagina(s, rows, q.Limite, scope, func(r Recuerdo) (bson.ObjectID, time.Time) { return r.ID, r.Fecha })
	if err != nil {
		return vacio, err
	}
	items := make([]RecuerdoResponse, 0, len(rows))
	for _, r := range rows {
		items = append(items, respuestaRecuerdo(r))
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

// BorrarRecuerdo borra el recuerdo propio; las postales se quedan. Ajeno o inexistente = 404.
func (s *Service) BorrarRecuerdo(ctx context.Context, cl *middleware.Claims, raw string) error {
	me, err := self(cl)
	if err != nil {
		return err
	}
	id, ok := parseID(raw)
	if !ok {
		return errRecuerdoNotFound()
	}
	borrado, err := s.Repo.BorrarRecuerdo(ctx, id, me)
	if err != nil {
		return errStore()
	}
	if !borrado {
		return errRecuerdoNotFound()
	}
	return nil
}
