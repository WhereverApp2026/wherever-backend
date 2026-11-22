package reacciones

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

// Relaciones de PersonaView (las de la búsqueda de nv-64 más `yo`).
const (
	RelacionYo       = "yo"
	RelacionAmigo    = "amigo"
	RelacionEnviada  = "solicitud_enviada"
	RelacionRecibida = "solicitud_recibida"
	RelacionNinguna  = "ninguna"
)

// van son los integrantes confirmados del plan sin su dueño (los que se apuntaron, nv-69).
func (s *Service) van(ctx context.Context, r Ref) ([]bson.ObjectID, error) {
	if r.Tipo != ObjetoPlan {
		return nil, nil
	}
	g, err := s.Grupos.FindByPlan(ctx, r.ID)
	if errors.Is(err, grupos.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, errStore()
	}
	out := []bson.ObjectID{}
	for _, m := range g.Members {
		if m.Status == grupos.StatusConfirmed && m.UserID != r.Dueno {
			out = append(out, m.UserID)
		}
	}
	return out, nil
}

// Reacciones es GET /{momentos|planes}/{id}/reacciones: quién dio me gusta y, en planes,
// quién va; los totales cuentan a todos y la lista solo a quien quien mira puede ver.
func (s *Service) Reacciones(ctx context.Context, cl *middleware.Claims, tipo, raw string, q ReaccionesQuery) (ReaccionesResponse, error) {
	me, err := self(cl)
	if err != nil {
		return ReaccionesResponse{}, err
	}
	r, err := s.objeto(ctx, me, tipo, raw)
	if err != nil {
		return ReaccionesResponse{}, err
	}
	likes, err := s.Repo.MeGustaDe(ctx, r.Tipo, r.ID, MaxPersonas)
	if err != nil {
		return ReaccionesResponse{}, errStore()
	}
	total, err := s.Repo.ContarMeGusta(ctx, r.Tipo, r.ID)
	if err != nil {
		return ReaccionesResponse{}, errStore()
	}
	van, err := s.van(ctx, r)
	if err != nil {
		return ReaccionesResponse{}, err
	}
	ambos, err := s.Repo.ContarMeGustaDe(ctx, r.Tipo, r.ID, van)
	if err != nil {
		return ReaccionesResponse{}, errStore()
	}
	orden, marcas := ordenar(q.Pestana, likes, van)
	personas, err := s.vistaPersonas(ctx, me, r, orden, marcas)
	if err != nil {
		return ReaccionesResponse{}, err
	}
	return ReaccionesResponse{Pestana: q.Pestana, Totales: Totales(total, int64(len(van)), ambos), Personas: personas}, nil
}

// marca dice por qué sale una persona en la lista.
type marca struct{ meGusta, va bool }

// ordenar arma la pestaña: primero los me gusta (del más reciente) y luego quienes van.
func ordenar(pestana string, likes []MeGusta, van []bson.ObjectID) ([]bson.ObjectID, map[bson.ObjectID]marca) {
	marcas := map[bson.ObjectID]marca{}
	orden := []bson.ObjectID{}
	agregar := func(id bson.ObjectID, f func(*marca)) {
		mk, ya := marcas[id]
		f(&mk)
		marcas[id] = mk
		if !ya {
			orden = append(orden, id)
		}
	}
	for _, l := range likes {
		agregar(l.UsuarioID, func(mk *marca) { mk.meGusta = true })
	}
	for _, id := range van {
		agregar(id, func(mk *marca) { mk.va = true })
	}
	out := orden[:0:0]
	for _, id := range orden {
		mk := marcas[id]
		if pestana == PestanaTodos || (pestana == PestanaMeGusta && mk.meGusta) || (pestana == PestanaVan && mk.va) {
			out = append(out, id)
		}
	}
	return out, marcas
}

// vistaPersonas arma la lista con nombre, relación y amigos en común; quita cuentas no
// activas, bloqueos y a quien oculta su actividad (salvo para el dueño del objeto).
func (s *Service) vistaPersonas(ctx context.Context, me bson.ObjectID, r Ref, ids []bson.ObjectID, marcas map[bson.ObjectID]marca) ([]PersonaView, error) {
	out := []PersonaView{}
	if len(ids) == 0 {
		return out, nil
	}
	m, err := s.mirada(ctx, me, r)
	if err != nil {
		return nil, err
	}
	us, err := s.personas(ctx, ids, &m)
	if err != nil {
		return nil, err
	}
	rels, err := s.Amigos.RelationsWith(ctx, me, ids)
	if err != nil {
		return nil, errStore()
	}
	comunes, err := s.comunes(ctx, me, ids)
	if err != nil {
		return nil, err
	}
	for _, id := range ids {
		u, ok := us[id]
		rel := relacion(me, id, rels)
		if !ok || rel == "" || !m.PersonaVisible(id) {
			continue
		}
		mk := marcas[id]
		out = append(out, PersonaView{ID: id.Hex(), Nombre: u.Name, NombreUsuario: u.Username, FotoURL: u.PhotoURL, Relacion: rel,
			AmigosEnComun: comunes[id], MeGusta: mk.meGusta, Va: mk.va})
	}
	return out, nil
}

// comunes cuenta los amigos en común de cada persona con quien mira (sin Neo4j: una consulta).
func (s *Service) comunes(ctx context.Context, me bson.ObjectID, ids []bson.ObjectID) (map[bson.ObjectID]int, error) {
	fs, err := s.Amigos.List(ctx, me, amistades.KindFriends)
	if err != nil {
		return nil, errStore()
	}
	mine := make([]bson.ObjectID, 0, len(fs))
	for _, f := range fs {
		mine = append(mine, f.Other(me))
	}
	out, err := s.Amigos.CommonWith(ctx, mine, ids)
	if err != nil {
		return nil, errStore()
	}
	return out, nil
}

// relacion traduce la amistad; un bloqueo da "" (no se muestra).
func relacion(me, id bson.ObjectID, rels map[bson.ObjectID]amistades.Friendship) string {
	if id == me {
		return RelacionYo
	}
	f, ok := rels[id]
	switch {
	case !ok:
		return RelacionNinguna
	case f.Status == amistades.StatusAccepted:
		return RelacionAmigo
	case f.Status == amistades.StatusPending && f.From == me:
		return RelacionEnviada
	case f.Status == amistades.StatusPending:
		return RelacionRecibida
	default:
		return ""
	}
}
