package postales

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
)

// amigos son los amigos aceptados de la persona (hasta 500, nv-36).
func (s *Service) amigos(ctx context.Context, me bson.ObjectID) (map[bson.ObjectID]bool, error) {
	fs, err := s.Amigos.List(ctx, me, amistades.KindFriends)
	if err != nil {
		return nil, errStore()
	}
	out := make(map[bson.ObjectID]bool, len(fs))
	for _, f := range fs {
		if len(out) == MaxAmigosFeed {
			break
		}
		out[f.Other(me)] = true
	}
	return out, nil
}

// etiquetados valida que cada persona sea amiga aceptada del autor (si no, 422).
func (s *Service) etiquetados(ctx context.Context, me bson.ObjectID, raw []string) ([]bson.ObjectID, error) {
	out := make([]bson.ObjectID, 0, len(raw))
	if len(raw) == 0 {
		return out, nil
	}
	amigos, err := s.amigos(ctx, me)
	if err != nil {
		return nil, err
	}
	for _, r := range raw {
		id, _ := parseID(r)
		if !amigos[id] {
			return nil, invalid("etiquetados", "solo puedes etiquetar a tus amigos")
		}
		out = append(out, id)
	}
	return out, nil
}

// avisarEtiquetados avisa a quien entra a los etiquetados de un momento publicado.
func (s *Service) avisarEtiquetados(ctx context.Context, m Momento, antes []bson.ObjectID) {
	if m.Estado != MomentoPublicado {
		return
	}
	ya := make(map[bson.ObjectID]bool, len(antes))
	for _, id := range antes {
		ya[id] = true
	}
	for _, id := range m.Etiquetados {
		if !ya[id] {
			s.notifier.Emit(ctx, id, notificaciones.KindMomentTagged, map[string]string{"momento_id": m.ID.Hex()})
		}
	}
}

func idsHex(ids []bson.ObjectID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.Hex())
	}
	return out
}

func respuestaMomento(m Momento, p Postal, autor AutorView, propio bool) MomentoResponse {
	out := MomentoResponse{
		ID: m.ID.Hex(), Autor: autor, NegocioID: hexPtr(m.NegocioID), Texto: m.Texto, Visibilidad: m.Visibilidad, Estado: m.Estado,
		Etiquetados: idsHex(m.Etiquetados), ReaccionesTotal: m.ReaccionesTotal, Destacado: m.Destacado, Agradecido: m.Agradecido,
		Propio: propio, Version: m.Version, CreadoEn: m.CreadoEn.UTC(), ActualizadoEn: m.ActualizadoEn.UTC(),
		Postal: PostalCorta{ID: p.ID.Hex(), Estilo: p.Estilo, Texto: p.Texto},
	}
	if p.Estado == EstadoLista {
		out.Postal.ImagenURL = p.ImagenURL
	}
	if propio {
		out.PlanID = hexPtr(m.PlanID)
	}
	return out
}

// momentoNuevo valida postal, visibilidad, etiquetados, plan y texto, y arma el momento.
func (s *Service) momentoNuevo(ctx context.Context, me bson.ObjectID, p Postal, req MomentoRequest) (Momento, moderacion.Resultado, error) {
	var res moderacion.Resultado
	if p.Estado != EstadoLista {
		return Momento{}, res, invalid("postal_id", "la postal aún no está lista")
	}
	if !Compatible(p.Visibilidad, req.Visibilidad) {
		return Momento{}, res, invalid("visibilidad", "el momento no puede ser más abierto que su postal")
	}
	etiq, err := s.etiquetados(ctx, me, req.Etiquetados)
	if err != nil {
		return Momento{}, res, err
	}
	plan := p.PlanID
	if req.PlanID != "" {
		if plan, err = s.planPropio(ctx, me, req.PlanID); err != nil {
			return Momento{}, res, err
		}
	}
	texto := strings.TrimSpace(req.Texto)
	textoEstado, res, err := s.textoEstado(ctx, "texto", texto)
	if err != nil {
		return Momento{}, res, err
	}
	estado := MomentoPublicado
	if textoEstado != TextoAprobado {
		estado = MomentoEnModeracion
	}
	now := s.ahora()
	return Momento{
		ID: bson.NewObjectID(), AutorID: me, PostalID: p.ID, NegocioID: p.NegocioID, Texto: texto, Visibilidad: req.Visibilidad,
		Estado: estado, Etiquetados: etiq, PlanID: plan, Version: 1, CreadoEn: now, ActualizadoEn: now,
	}, res, nil
}

// PublicarMomento comparte una postal `lista` propia. Una postal privada toma la visibilidad del
// momento; un momento no puede ser más abierto que su postal (422).
func (s *Service) PublicarMomento(ctx context.Context, cl *middleware.Claims, req MomentoRequest) (MomentoResponse, error) {
	me, err := self(cl)
	if err != nil {
		return MomentoResponse{}, err
	}
	p, err := s.propia(ctx, me, req.PostalID)
	if err != nil {
		return MomentoResponse{}, err
	}
	m, res, err := s.momentoNuevo(ctx, me, p, req)
	if err != nil {
		return MomentoResponse{}, err
	}
	estado, texto, now := m.Estado, m.Texto, m.CreadoEn
	if _, err := s.Repo.MomentoDePostal(ctx, p.ID); err == nil {
		return MomentoResponse{}, apperr.Conflict("la postal ya es un momento")
	}
	if p.Visibilidad == VisPrivada {
		if p, err = s.Repo.ActualizarPostal(ctx, p.ID, me, p.Version, bson.D{{Key: "visibilidad", Value: req.Visibilidad}, {Key: "actualizado_en", Value: now}}); err != nil {
			return MomentoResponse{}, errStale()
		}
	}
	if err := s.Repo.InsertarMomento(ctx, m); errors.Is(err, ErrDuplicado) {
		return MomentoResponse{}, apperr.Conflict("la postal ya es un momento")
	} else if err != nil {
		return MomentoResponse{}, errStore()
	}
	if estado == MomentoEnModeracion {
		s.encolar(ctx, TipoColaMomento, m.ID, me, texto, res)
	}
	s.avisarEtiquetados(ctx, m, nil)
	s.Log.InfoContext(ctx, "momento publicado", slog.String("evento", "momento_publicado"), slog.String("id", m.ID.Hex()),
		slog.String("visibilidad", m.Visibilidad), slog.String("estado", m.Estado))
	return respuestaMomento(m, p, AutorView{ID: me.Hex()}, true), nil
}

// Feed son los momentos de los amigos aceptados con cuenta activa, del más nuevo al más viejo.
func (s *Service) Feed(ctx context.Context, cl *middleware.Claims, q ListaQuery) (pagination.PageResult[MomentoResponse], error) {
	var vacio pagination.PageResult[MomentoResponse]
	me, err := self(cl)
	if err != nil {
		return vacio, err
	}
	scope := s.Cursor.Scope("momentos_feed", me.Hex())
	after, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return vacio, err
	}
	amigos, err := s.amigos(ctx, me)
	if err != nil {
		return vacio, err
	}
	ids := make([]bson.ObjectID, 0, len(amigos))
	for id := range amigos {
		ids = append(ids, id)
	}
	autores, err := s.autores(ctx, ids)
	if err != nil {
		return vacio, err
	}
	activos := make([]bson.ObjectID, 0, len(autores))
	for id := range autores {
		activos = append(activos, id)
	}
	if len(activos) == 0 {
		return pagination.NewPageResult([]MomentoResponse{}, "", q.Limite), nil
	}
	rows, err := s.Repo.Feed(ctx, activos, after, q.Limite)
	if err != nil {
		return vacio, errStore()
	}
	return s.paginaMomentos(ctx, rows, q.Limite, scope, autores, me)
}

// paginaMomentos arma la página: solo los momentos cuya postal sigue `lista`.
func (s *Service) paginaMomentos(ctx context.Context, rows []Momento, limite int, scope string, autores map[bson.ObjectID]AutorView, me bson.ObjectID) (pagination.PageResult[MomentoResponse], error) {
	var vacio pagination.PageResult[MomentoResponse]
	rows, next, err := pagina(s, rows, limite, scope, func(m Momento) (bson.ObjectID, time.Time) { return m.ID, m.CreadoEn })
	if err != nil {
		return vacio, err
	}
	pids := make([]bson.ObjectID, 0, len(rows))
	for _, m := range rows {
		pids = append(pids, m.PostalID)
	}
	ps, err := s.Repo.PostalesPorIDs(ctx, pids)
	if err != nil {
		return vacio, errStore()
	}
	items := make([]MomentoResponse, 0, len(rows))
	for _, m := range rows {
		p, ok := ps[m.PostalID]
		autor, activo := autores[m.AutorID]
		if !ok || !activo || p.Estado != EstadoLista {
			continue
		}
		items = append(items, respuestaMomento(m, p, autor, m.AutorID == me))
	}
	return pagination.NewPageResult(items, next, limite), nil
}

// momentoVisible trae el momento y su postal si quien mira puede verlos; si no, 404.
func (s *Service) momentoVisible(ctx context.Context, viewer *bson.ObjectID, raw string) (Momento, Postal, Relacion, error) {
	id, ok := parseID(raw)
	if !ok {
		return Momento{}, Postal{}, Relacion{}, errMomentoNotFound()
	}
	m, err := s.Repo.Momento(ctx, id)
	if errors.Is(err, ErrNoEncontrado) {
		return Momento{}, Postal{}, Relacion{}, errMomentoNotFound()
	}
	if err != nil {
		return Momento{}, Postal{}, Relacion{}, errStore()
	}
	p, err := s.Repo.Postal(ctx, m.PostalID)
	if err != nil {
		return Momento{}, Postal{}, Relacion{}, errMomentoNotFound()
	}
	rel, err := s.relacion(ctx, viewer, m.AutorID)
	if err != nil {
		return Momento{}, Postal{}, Relacion{}, err
	}
	if !PuedeVer(rel, m.Visibilidad, m.Estado == MomentoPublicado && p.Estado == EstadoLista) {
		return Momento{}, Postal{}, Relacion{}, errMomentoNotFound()
	}
	return m, p, rel, nil
}

// Momento es el detalle con sesión opcional: sin sesión solo los públicos.
func (s *Service) Momento(ctx context.Context, cl *middleware.Claims, raw string) (MomentoResponse, error) {
	viewer := quien(cl)
	m, p, rel, err := s.momentoVisible(ctx, viewer, raw)
	if err != nil {
		return MomentoResponse{}, err
	}
	autores, err := s.autores(ctx, []bson.ObjectID{m.AutorID})
	if err != nil {
		return MomentoResponse{}, err
	}
	autor, ok := autores[m.AutorID]
	if !ok && !rel.Dueno {
		return MomentoResponse{}, errMomentoNotFound()
	}
	if !ok {
		autor = AutorView{ID: m.AutorID.Hex()}
	}
	return respuestaMomento(m, p, autor, rel.Dueno), nil
}

// propio trae el momento del autor; ajeno o inexistente = 404.
func (s *Service) propio(ctx context.Context, me bson.ObjectID, raw string) (Momento, error) {
	id, ok := parseID(raw)
	if !ok {
		return Momento{}, errMomentoNotFound()
	}
	m, err := s.Repo.Momento(ctx, id)
	if errors.Is(err, ErrNoEncontrado) || (err == nil && m.AutorID != me) {
		return Momento{}, errMomentoNotFound()
	}
	if err != nil {
		return Momento{}, errStore()
	}
	return m, nil
}

// cambiosMomento valida el cuerpo del PATCH y arma el `$set`.
func (s *Service) cambiosMomento(ctx context.Context, me bson.ObjectID, m Momento, p Postal, req EditarMomentoRequest) (Momento, bson.D, moderacion.Resultado, error) {
	var res moderacion.Resultado
	var err error
	set := bson.D{{Key: "actualizado_en", Value: s.ahora()}}
	nuevo := m
	if req.Visibilidad != nil {
		if !Compatible(p.Visibilidad, *req.Visibilidad) {
			return m, nil, res, invalid("visibilidad", "el momento no puede ser más abierto que su postal")
		}
		nuevo.Visibilidad = *req.Visibilidad
		set = append(set, bson.E{Key: "visibilidad", Value: nuevo.Visibilidad})
	}
	if req.Etiquetados != nil {
		if nuevo.Etiquetados, err = s.etiquetados(ctx, me, *req.Etiquetados); err != nil {
			return m, nil, res, err
		}
		set = append(set, bson.E{Key: "etiquetados", Value: nuevo.Etiquetados})
	}
	if req.Texto == nil || strings.TrimSpace(*req.Texto) == m.Texto {
		return nuevo, set, res, nil
	}
	nuevo.Texto = strings.TrimSpace(*req.Texto)
	estado, res, err := s.textoEstado(ctx, "texto", nuevo.Texto)
	if err != nil {
		return m, nil, res, err
	}
	nuevo.Estado = MomentoPublicado
	if estado != TextoAprobado {
		nuevo.Estado = MomentoEnModeracion
	}
	return nuevo, append(set, bson.E{Key: "texto", Value: nuevo.Texto}, bson.E{Key: "estado", Value: nuevo.Estado}), res, nil
}

// EditarMomento cambia texto, etiquetados o visibilidad con If-Match. Avisa solo a los nuevos
// etiquetados; un texto nuevo vuelve a moderarse.
func (s *Service) EditarMomento(ctx context.Context, cl *middleware.Claims, raw string, version int, req EditarMomentoRequest) (MomentoResponse, error) {
	me, err := self(cl)
	if err != nil {
		return MomentoResponse{}, err
	}
	m, err := s.propio(ctx, me, raw)
	if err != nil {
		return MomentoResponse{}, err
	}
	if m.Version != version {
		return MomentoResponse{}, errStale()
	}
	p, err := s.Repo.Postal(ctx, m.PostalID)
	if err != nil {
		return MomentoResponse{}, errStore()
	}
	nuevo, set, res, err := s.cambiosMomento(ctx, me, m, p, req)
	if err != nil {
		return MomentoResponse{}, err
	}
	out, err := s.Repo.ActualizarMomento(ctx, m.ID, me, version, set)
	if errors.Is(err, ErrNoAplica) {
		return MomentoResponse{}, errStale()
	}
	if err != nil {
		return MomentoResponse{}, errStore()
	}
	if nuevo.Texto != m.Texto && nuevo.Estado == MomentoEnModeracion {
		s.encolar(ctx, TipoColaMomento, m.ID, me, nuevo.Texto, res)
	}
	s.avisarEtiquetados(ctx, out, m.Etiquetados)
	return respuestaMomento(out, p, AutorView{ID: me.Hex()}, true), nil
}

// BorrarMomento borra el momento con If-Match; la postal se queda.
func (s *Service) BorrarMomento(ctx context.Context, cl *middleware.Claims, raw string, version int) error {
	me, err := self(cl)
	if err != nil {
		return err
	}
	m, err := s.propio(ctx, me, raw)
	if err != nil {
		return err
	}
	if m.Version != version {
		return errStale()
	}
	if err := s.Repo.BorrarMomento(ctx, m.ID, me, version); errors.Is(err, ErrNoAplica) {
		return errStale()
	} else if err != nil {
		return errStore()
	}
	return nil
}

// negocioPropio trae el negocio del comerciante; ajeno, eliminado o inexistente = 404.
func (s *Service) negocioPropio(ctx context.Context, cl *middleware.Claims, raw string) (bson.ObjectID, error) {
	me, err := self(cl)
	if err != nil {
		return me, err
	}
	id, ok := parseID(raw)
	if !ok {
		return id, errNegocioNotFound()
	}
	b, err := s.Negocios.FindByID(ctx, id)
	if errors.Is(err, negocios.ErrNotFound) {
		return id, errNegocioNotFound()
	}
	if err != nil {
		return id, errStore()
	}
	if b.OwnerID != me || b.DeletedAt != nil {
		return id, errNegocioNotFound()
	}
	return id, nil
}

// MomentosDelNegocio es el panel del propietario: solo momentos públicos y publicados.
func (s *Service) MomentosDelNegocio(ctx context.Context, cl *middleware.Claims, raw string, q PanelQuery) (pagination.PageResult[MomentoResponse], error) {
	var vacio pagination.PageResult[MomentoResponse]
	id, err := s.negocioPropio(ctx, cl, raw)
	if err != nil {
		return vacio, err
	}
	scope := s.Cursor.Scope("momentos_negocio", id.Hex(), boolStr(q.Destacados))
	after, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return vacio, err
	}
	rows, err := s.Repo.DelNegocio(ctx, id, q.Destacados, after, q.Limite)
	if err != nil {
		return vacio, errStore()
	}
	ids := make([]bson.ObjectID, 0, len(rows))
	for _, m := range rows {
		ids = append(ids, m.AutorID)
	}
	autores, err := s.autores(ctx, ids)
	if err != nil {
		return vacio, err
	}
	me, _ := self(cl)
	return s.paginaMomentos(ctx, rows, q.Limite, scope, autores, me)
}

func boolStr(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func marcas(req MarcarRequest) bson.D {
	set := bson.D{}
	if req.Agradecido != nil {
		set = append(set, bson.E{Key: "agradecido_por_negocio", Value: *req.Agradecido})
	}
	if req.Destacado != nil {
		set = append(set, bson.E{Key: "destacado_por_negocio", Value: *req.Destacado})
	}
	return set
}

// avisarMarcas aplica las marcas al momento de antes y avisa al autor de las que se encienden.
func (s *Service) avisarMarcas(ctx context.Context, antes Momento, req MarcarRequest) Momento {
	m := antes
	datos := map[string]string{"momento_id": m.ID.Hex()}
	if req.Agradecido != nil {
		m.Agradecido = *req.Agradecido
		if m.Agradecido && !antes.Agradecido {
			s.notifier.Emit(ctx, m.AutorID, notificaciones.KindMomentThanked, datos)
		}
	}
	if req.Destacado != nil {
		m.Destacado = *req.Destacado
		if m.Destacado && !antes.Destacado {
			s.notifier.Emit(ctx, m.AutorID, notificaciones.KindMomentFeatured, datos)
		}
	}
	return m
}

// MarcarMomento agradece o destaca un momento público del negocio y avisa al autor cuando la
// marca se enciende.
func (s *Service) MarcarMomento(ctx context.Context, cl *middleware.Claims, raw, mraw string, req MarcarRequest) (MomentoResponse, error) {
	negocio, err := s.negocioPropio(ctx, cl, raw)
	if err != nil {
		return MomentoResponse{}, err
	}
	if req.Agradecido == nil && req.Destacado == nil {
		return MomentoResponse{}, invalid("agradecido", "manda agradecido o destacado")
	}
	mid, ok := parseID(mraw)
	if !ok {
		return MomentoResponse{}, errMomentoNotFound()
	}
	antes, err := s.Repo.MarcarMomento(ctx, mid, negocio, marcas(req))
	if errors.Is(err, ErrNoEncontrado) {
		return MomentoResponse{}, errMomentoNotFound()
	}
	if err != nil {
		return MomentoResponse{}, errStore()
	}
	m := s.avisarMarcas(ctx, antes, req)
	p, err := s.Repo.Postal(ctx, m.PostalID)
	if err != nil {
		return MomentoResponse{}, errStore()
	}
	autores, err := s.autores(ctx, []bson.ObjectID{m.AutorID})
	if err != nil {
		return MomentoResponse{}, err
	}
	autor := autores[m.AutorID]
	autor.ID = m.AutorID.Hex()
	return respuestaMomento(m, p, autor, false), nil
}
