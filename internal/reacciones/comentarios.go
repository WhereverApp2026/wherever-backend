package reacciones

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// vista arma un comentario para quien mira; el eliminado sale sin autor ni texto.
func vista(c Comentario, autores map[bson.ObjectID]usuarios.User, me bson.ObjectID) ComentarioView {
	out := ComentarioView{ID: c.ID.Hex(), Texto: c.Texto, Estado: c.Estado, Propio: c.De(me), PuedeBorrar: PuedeBorrar(c, me), CreadoEn: c.CreadoEn.UTC()}
	if c.PadreID != nil {
		p := c.PadreID.Hex()
		out.PadreID = &p
	}
	if c.Estado == EstadoEliminado {
		out.Texto, out.Propio = "", false
		return out
	}
	if c.AutorID != nil {
		if u, ok := autores[*c.AutorID]; ok {
			out.Autor = autorView(u)
		}
	}
	return out
}

// mostrable: visible para quien mira y, salvo eliminado, con autor activo.
func mostrable(c Comentario, m Mirada, autores map[bson.ObjectID]usuarios.User) bool {
	if !m.ComentarioVisible(c) {
		return false
	}
	if c.Estado == EstadoEliminado {
		return true
	}
	if c.AutorID == nil {
		return false
	}
	_, ok := autores[*c.AutorID]
	return ok
}

// Comentar es POST /{momentos|planes}/{id}/comentarios. Rechazado por la moderación = 422 sin
// guardar; dudoso = `en_moderacion` (solo lo ve su autor hasta que se decida en nv-43).
func (s *Service) Comentar(ctx context.Context, cl *middleware.Claims, tipo, raw string, req ComentarioRequest) (ComentarioView, error) {
	me, err := self(cl)
	if err != nil {
		return ComentarioView{}, err
	}
	r, err := s.objeto(ctx, me, tipo, raw)
	if err != nil {
		return ComentarioView{}, err
	}
	texto, ok := LimpiarTexto(req.Texto)
	if !ok {
		return ComentarioView{}, invalid("texto", "escribe de 1 a 300 caracteres")
	}
	padre, err := s.padre(ctx, me, r, req.PadreID)
	if err != nil {
		return ComentarioView{}, err
	}
	estado, res, err := s.textoEstado(ctx, texto)
	if err != nil {
		return ComentarioView{}, err
	}
	autor := me
	c := Comentario{ID: bson.NewObjectID(), ObjetoTipo: r.Tipo, ObjetoID: r.ID, AutorID: &autor, DuenoID: r.Dueno, PadreID: padre,
		Texto: texto, Estado: estado, CreadoEn: s.ahora()}
	if err := s.Repo.InsertarComentario(ctx, c); err != nil {
		return ComentarioView{}, errStore()
	}
	if estado == EstadoEnModeracion {
		s.encolar(ctx, c, res)
	} else {
		s.avisar(ctx, r, me, notificaciones.KindComment)
	}
	s.Log.InfoContext(ctx, "comentario creado", slog.String("evento", "comentario_creado"), slog.String("id", c.ID.Hex()),
		slog.String("objeto", r.Tipo), slog.String("estado", estado))
	us, err := s.Personas.FindPublicByIDs(ctx, []bson.ObjectID{me})
	if err != nil {
		return ComentarioView{}, errStore()
	}
	autores := map[bson.ObjectID]usuarios.User{}
	for _, u := range us {
		autores[u.ID] = u
	}
	return vista(c, autores, me), nil
}

// padre valida `padre_id`: comentario publicado de primer nivel del mismo objeto y visible
// para quien responde; si no, 422.
func (s *Service) padre(ctx context.Context, me bson.ObjectID, r Ref, raw string) (*bson.ObjectID, error) {
	if raw == "" {
		return nil, nil
	}
	id, _ := parseID(raw) // el DTO ya validó el formato
	p, err := s.Repo.Comentario(ctx, id)
	if errors.Is(err, ErrNoEncontrado) {
		return nil, invalid("padre_id", "el comentario no existe")
	}
	if err != nil {
		return nil, errStore()
	}
	m, err := s.mirada(ctx, me, r)
	if err != nil {
		return nil, err
	}
	if p.AutorID != nil {
		if _, err := s.personas(ctx, []bson.ObjectID{*p.AutorID}, &m); err != nil {
			return nil, err
		}
	}
	if !PadreValido(p, r.Tipo, r.ID, m) {
		return nil, invalid("padre_id", "solo se responde a un comentario publicado de primer nivel")
	}
	return &id, nil
}

// posicion lee el cursor firmado; uno de otra lista o manipulado = 422.
func (s *Service) posicion(scope, raw string) (*Pos, error) {
	c, ok, err := validation.Position(s.Cursor, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	id, okID := parseID(c.ID)
	t, errT := time.Parse(time.RFC3339Nano, c.Sort)
	if !okID || errT != nil {
		return nil, invalid("cursor", "cursor inválido")
	}
	return &Pos{Fecha: t, ID: id}, nil
}

// Comentarios es GET /{momentos|planes}/{id}/comentarios: los de primer nivel del más viejo al
// más nuevo, cada uno con sus respuestas.
func (s *Service) Comentarios(ctx context.Context, cl *middleware.Claims, tipo, raw string, q ListaQuery) (pagination.PageResult[HiloView], error) {
	var vacio pagination.PageResult[HiloView]
	me, err := self(cl)
	if err != nil {
		return vacio, err
	}
	r, err := s.objeto(ctx, me, tipo, raw)
	if err != nil {
		return vacio, err
	}
	scope := s.Cursor.Scope("comentarios", r.Tipo, r.ID.Hex(), me.Hex())
	after, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return vacio, err
	}
	rows, err := s.Repo.Principales(ctx, r.Tipo, r.ID, after, q.Limite)
	if err != nil {
		return vacio, errStore()
	}
	next := ""
	if len(rows) > q.Limite {
		rows = rows[:q.Limite]
		last := rows[len(rows)-1]
		if next, err = s.Cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: last.CreadoEn.UTC().Format(time.RFC3339Nano), Scope: scope}); err != nil {
			return vacio, errStore()
		}
	}
	items, err := s.hilos(ctx, me, r, rows)
	if err != nil {
		return vacio, err
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

// hilos arma la página con las respuestas y quita lo que quien mira no debe ver.
func (s *Service) hilos(ctx context.Context, me bson.ObjectID, r Ref, rows []Comentario) ([]HiloView, error) {
	ids := make([]bson.ObjectID, 0, len(rows))
	for _, c := range rows {
		ids = append(ids, c.ID)
	}
	resp, err := s.Repo.Respuestas(ctx, r.Tipo, r.ID, ids)
	if err != nil {
		return nil, errStore()
	}
	m, err := s.mirada(ctx, me, r)
	if err != nil {
		return nil, err
	}
	autores, err := s.personas(ctx, autoresDe(rows, resp), &m)
	if err != nil {
		return nil, err
	}
	return armarHilos(rows, resp, m, autores), nil
}

func autoresDe(listas ...[]Comentario) []bson.ObjectID {
	out := []bson.ObjectID{}
	for _, l := range listas {
		for _, c := range l {
			if c.AutorID != nil {
				out = append(out, *c.AutorID)
			}
		}
	}
	return out
}

// armarHilos cuelga cada respuesta visible de su comentario. Un eliminado sin respuestas
// visibles ya no aporta nada al hilo y no sale.
func armarHilos(rows, resp []Comentario, m Mirada, autores map[bson.ObjectID]usuarios.User) []HiloView {
	porPadre := map[bson.ObjectID][]ComentarioView{}
	for _, c := range resp {
		if c.Estado != EstadoEliminado && mostrable(c, m, autores) {
			porPadre[*c.PadreID] = append(porPadre[*c.PadreID], vista(c, autores, m.Yo))
		}
	}
	out := make([]HiloView, 0, len(rows))
	for _, c := range rows {
		hijos := porPadre[c.ID]
		if !mostrable(c, m, autores) || (c.Estado == EstadoEliminado && len(hijos) == 0) {
			continue
		}
		if hijos == nil {
			hijos = []ComentarioView{}
		}
		out = append(out, HiloView{ComentarioView: vista(c, autores, m.Yo), Respuestas: hijos})
	}
	return out
}

// BorrarComentario es DELETE /comentarios/{id}: el autor, o el dueño del objeto. Queda como
// «Comentario eliminado». Inexistente, ya eliminado o de un objeto que no veo = 404; visible
// pero ajeno en objeto ajeno = 403.
func (s *Service) BorrarComentario(ctx context.Context, cl *middleware.Claims, raw string) error {
	me, err := self(cl)
	if err != nil {
		return err
	}
	id, ok := parseID(raw)
	if !ok {
		return errComentarioNotFound()
	}
	c, err := s.Repo.Comentario(ctx, id)
	if errors.Is(err, ErrNoEncontrado) || (err == nil && c.Estado == EstadoEliminado) {
		return errComentarioNotFound()
	}
	if err != nil {
		return errStore()
	}
	if !PuedeBorrar(c, me) {
		return s.sinPermiso(ctx, me, c)
	}
	if err := s.Repo.Eliminar(ctx, c, s.ahora()); errors.Is(err, ErrNoEncontrado) {
		return errComentarioNotFound()
	} else if err != nil {
		return errStore()
	}
	return nil
}

// sinPermiso decide entre 404 (no lo veo) y 403 (lo veo, pero no es mío ni del dueño).
func (s *Service) sinPermiso(ctx context.Context, me bson.ObjectID, c Comentario) error {
	r, err := s.objeto(ctx, me, c.ObjetoTipo, c.ObjetoID.Hex())
	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr.Kind == apperr.KindNotFound {
		return errComentarioNotFound()
	}
	if err != nil {
		return err
	}
	m, err := s.mirada(ctx, me, r)
	if err != nil {
		return err
	}
	if c.AutorID != nil {
		if _, err := s.personas(ctx, []bson.ObjectID{*c.AutorID}, &m); err != nil {
			return err
		}
	}
	if !m.ComentarioVisible(c) {
		return errComentarioNotFound()
	}
	return apperr.Forbidden("solo el autor o el dueño pueden borrar el comentario")
}
