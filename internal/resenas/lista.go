package resenas

import (
	"context"
	"strconv"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Mias pagina las reseñas de la persona (sin las eliminadas).
func (s *Service) Mias(ctx context.Context, cl *middleware.Claims, q ListaQuery) (pagination.PageResult[ResenaResponse], error) {
	me, err := self(cl)
	if err != nil {
		return pagination.PageResult[ResenaResponse]{}, err
	}
	scope := s.cursor.Scope("resenas_mias", me.Hex())
	after, err := s.posicion(scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[ResenaResponse]{}, err
	}
	rs, err := s.repo.Mias(ctx, me, after, q.Limite)
	if err != nil {
		return pagination.PageResult[ResenaResponse]{}, errStore()
	}
	next := ""
	if len(rs) > q.Limite {
		rs = rs[:q.Limite]
		last := rs[len(rs)-1]
		if next, err = s.siguiente(scope, last.ID, fechaCursor(last.CreadoEn)); err != nil {
			return pagination.PageResult[ResenaResponse]{}, err
		}
	}
	now := s.ahora()
	items := make([]ResenaResponse, 0, len(rs))
	for _, r := range rs {
		items = append(items, s.respuesta(r, now))
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

func (s *Service) posPublica(scope, raw string, utiles bool) (*Pos, error) {
	if !utiles {
		return s.posicion(scope, raw)
	}
	c, ok, err := validation.Position(s.cursor, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	id, okID := parseID(c.ID)
	n, errN := strconv.Atoi(c.Sort)
	if !okID || errN != nil {
		return nil, invalid("cursor", "cursor inválido")
	}
	return &Pos{Num: n, ID: id}, nil
}

// Publicas es la lista pública de reseñas publicadas de un negocio visible. El autor sale con
// nombre de usuario y foto; anonimizada, «Usuario de Wherever». Nunca correo, teléfono ni
// check-in.
func (s *Service) Publicas(ctx context.Context, raw string, q PublicaQuery) (pagination.PageResult[PublicaResponse], error) {
	id, ok := parseID(raw)
	if !ok {
		return pagination.PageResult[PublicaResponse]{}, errNegocioNotFound()
	}
	if _, visible, err := s.negocios.FindVisible(ctx, id); err != nil {
		return pagination.PageResult[PublicaResponse]{}, errStore()
	} else if !visible {
		return pagination.PageResult[PublicaResponse]{}, errNegocioNotFound()
	}
	utiles := q.Orden == "utiles"
	scope := s.cursor.Scope("resenas_publicas", id.Hex(), q.Orden, strconv.Itoa(q.Estrellas), q.Aspecto, strconv.FormatBool(q.ConFotos), q.Q)
	after, err := s.posPublica(scope, q.Cursor, utiles)
	if err != nil {
		return pagination.PageResult[PublicaResponse]{}, err
	}
	rs, err := s.repo.Publicas(ctx, id, FiltroPublico{Estrellas: q.Estrellas, Aspecto: q.Aspecto, ConFotos: q.ConFotos, Q: q.Q, Utiles: utiles}, after, q.Limite)
	if err != nil {
		return pagination.PageResult[PublicaResponse]{}, errStore()
	}
	next := ""
	if len(rs) > q.Limite {
		rs = rs[:q.Limite]
		if next, err = s.siguientePublica(scope, rs[len(rs)-1], utiles); err != nil {
			return pagination.PageResult[PublicaResponse]{}, err
		}
	}
	autores, err := s.autoresDe(ctx, rs)
	if err != nil {
		return pagination.PageResult[PublicaResponse]{}, err
	}
	items := make([]PublicaResponse, 0, len(rs))
	for _, r := range rs {
		items = append(items, publica(r, autores))
	}
	return pagination.NewPageResult(items, next, q.Limite), nil
}

func (s *Service) siguientePublica(scope string, last Resena, utiles bool) (string, error) {
	if utiles {
		return s.siguiente(scope, last.ID, strconv.Itoa(last.Util))
	}
	return s.siguiente(scope, last.ID, fechaCursor(last.CreadoEn))
}

// publica arma la reseña de la lista pública; sin autor visible sale «Usuario de Wherever».
func publica(r Resena, autores map[bson.ObjectID]AutorView) PublicaResponse {
	autor := AutorView{NombreUsuario: AutorAnonimo}
	if r.UsuarioID != nil {
		if a, ok := autores[*r.UsuarioID]; ok {
			autor = a
		}
	}
	out := PublicaResponse{
		ID: r.ID.Hex(), Autor: autor, Calificacion: r.Calificacion, Aspectos: r.Aspectos, Chips: noNil(r.Chips),
		Comentario: r.Comentario, Fotos: fotosNoNil(r.Fotos), Util: r.Util, CreadoEn: r.CreadoEn.UTC(),
	}
	if rn := r.RespuestaNegocio; rn != nil && rn.Estado == EstadoPublicada {
		out.RespuestaNegocio = &RespuestaView{Texto: rn.Texto, Fecha: rn.Fecha.UTC()}
	}
	return out
}

// autoresDe trae nombre de usuario y foto de los autores; sin nombre de usuario sale anónimo.
func (s *Service) autoresDe(ctx context.Context, rs []Resena) (map[bson.ObjectID]AutorView, error) {
	out := map[bson.ObjectID]AutorView{}
	ids := []bson.ObjectID{}
	for _, r := range rs {
		if r.UsuarioID != nil {
			ids = append(ids, *r.UsuarioID)
		}
	}
	if len(ids) == 0 || s.autores == nil {
		return out, nil
	}
	us, err := s.autores.FindPublicByIDs(ctx, ids)
	if err != nil {
		return nil, errStore()
	}
	for _, u := range us {
		if u.Username != "" {
			out[u.ID] = AutorView{NombreUsuario: u.Username, FotoURL: u.PhotoURL}
		}
	}
	return out, nil
}
