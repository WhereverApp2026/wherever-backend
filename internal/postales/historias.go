package postales

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
)

// Límites de las historias (nv-84): momentos de las últimas 24 h, hasta 30 personas.
const (
	VentanaHistorias = 24 * time.Hour
	MaxPersonasHist  = 30
	// MaxMomentosHist acota la consulta: de sobra para 30 personas con varias historias.
	MaxMomentosHist = 300
)

// Visita es un check-in buscado: persona, negocio y día de México (`2006-01-02`).
type Visita struct {
	Usuario bson.ObjectID
	Negocio bson.ObjectID
	Dia     string
}

// Visitas dice cuáles de las visitas tienen un check-in válido (lo cumple resenas.Repository).
type Visitas interface {
	Validas(ctx context.Context, vs []Visita) (map[Visita]bool, error)
}

// WithVisitas conecta «Verificado» de las historias; sin él ninguna sale verificada.
func (s *Service) WithVisitas(v Visitas) *Service {
	s.visitas = v
	return s
}

// NegocioRef es el lugar de una historia.
type NegocioRef struct {
	ID     string `json:"id"`
	Nombre string `json:"nombre"`
}

// HistoriaMomento es un momento dentro de la historia de una persona.
type HistoriaMomento struct {
	ID         string      `json:"id"`
	ImagenURL  string      `json:"imagen_url"`
	Negocio    *NegocioRef `json:"negocio"`
	Verificado bool        `json:"verificado"`
	CreadoEn   time.Time   `json:"creado_en"`
}

// HistoriaPersona son las historias de una persona, de la más vieja a la más nueva.
type HistoriaPersona struct {
	Autor    AutorView         `json:"autor"`
	Momentos []HistoriaMomento `json:"momentos"`
}

// HistoriasResponse es GET /momentos/historias.
type HistoriasResponse struct {
	Personas []HistoriaPersona `json:"personas"`
}

// amigosSinSilenciar son las amistades aceptadas que quien mira no silenció (nv-70).
func (s *Service) amigosSinSilenciar(ctx context.Context, me bson.ObjectID) ([]bson.ObjectID, error) {
	fs, err := s.Amigos.List(ctx, me, amistades.KindFriends)
	if err != nil {
		return nil, errStore()
	}
	out := make([]bson.ObjectID, 0, len(fs))
	for _, f := range fs {
		if len(out) == MaxAmigosFeed {
			break
		}
		if !f.MutedByUser(me) {
			out = append(out, f.Other(me))
		}
	}
	return out, nil
}

// Historias agrupa por persona los momentos de las últimas 24 h de las amistades (con postal
// lista y visibilidad `amigos` o `publica`), la persona con la historia más reciente primero.
// Fuera: silenciados, bloqueos (no son amistad aceptada) y quien no comparte sus postales u
// oculta su actividad (nv-82).
func (s *Service) Historias(ctx context.Context, cl *middleware.Claims) (HistoriasResponse, error) {
	out := HistoriasResponse{Personas: []HistoriaPersona{}}
	me, err := self(cl)
	if err != nil {
		return out, err
	}
	ids, err := s.amigosSinSilenciar(ctx, me)
	if err != nil {
		return out, err
	}
	autores, err := s.autores(ctx, ids, &me)
	if err != nil {
		return out, err
	}
	activos := make([]bson.ObjectID, 0, len(autores))
	for id := range autores {
		activos = append(activos, id)
	}
	if len(activos) == 0 {
		return out, nil
	}
	rows, err := s.Repo.Historias(ctx, activos, s.now().UTC().Add(-VentanaHistorias), MaxMomentosHist)
	if err != nil {
		return out, errStore()
	}
	rows, ps, err := s.conPostalLista(ctx, rows)
	if err != nil {
		return out, err
	}
	nombres, verificados, err := s.datosHistorias(ctx, rows)
	if err != nil {
		return out, err
	}
	out.Personas = AgruparHistorias(rows, autores, func(m Momento) HistoriaMomento {
		return s.historiaDe(m, ps[m.PostalID], nombres, verificados)
	})
	return out, nil
}

// historiaDe arma un momento de la historia con su imagen, lugar y «Verificado».
func (s *Service) historiaDe(m Momento, p Postal, nombres map[bson.ObjectID]string, verificados map[Visita]bool) HistoriaMomento {
	h := HistoriaMomento{ID: m.ID.Hex(), ImagenURL: p.ImagenURL, CreadoEn: m.CreadoEn.UTC()}
	if m.NegocioID == nil {
		return h
	}
	if n, ok := nombres[*m.NegocioID]; ok {
		h.Negocio = &NegocioRef{ID: m.NegocioID.Hex(), Nombre: n}
	}
	h.Verificado = verificados[s.visita(m)]
	return h
}

// conPostalLista deja los momentos cuya postal sigue `lista` (tiene imagen publicada).
func (s *Service) conPostalLista(ctx context.Context, rows []Momento) ([]Momento, map[bson.ObjectID]Postal, error) {
	pids := make([]bson.ObjectID, 0, len(rows))
	for _, m := range rows {
		pids = append(pids, m.PostalID)
	}
	ps, err := s.Repo.PostalesPorIDs(ctx, pids)
	if err != nil {
		return nil, nil, errStore()
	}
	kept := make([]Momento, 0, len(rows))
	for _, m := range rows {
		if p, ok := ps[m.PostalID]; ok && p.Estado == EstadoLista {
			kept = append(kept, m)
		}
	}
	return kept, ps, nil
}

func (s *Service) visita(m Momento) Visita {
	return Visita{Usuario: m.AutorID, Negocio: *m.NegocioID, Dia: m.CreadoEn.In(s.loc).Format(time.DateOnly)}
}

// datosHistorias trae el nombre de cada lugar y qué momentos vienen de un check-in válido ese día.
func (s *Service) datosHistorias(ctx context.Context, rows []Momento) (map[bson.ObjectID]string, map[Visita]bool, error) {
	nids := []bson.ObjectID{}
	vs := []Visita{}
	for _, m := range rows {
		if m.NegocioID != nil {
			nids = append(nids, *m.NegocioID)
			vs = append(vs, s.visita(m))
		}
	}
	nombres := map[bson.ObjectID]string{}
	if len(nids) > 0 {
		bs, err := s.Negocios.FindFavoriteCards(ctx, nids)
		if err != nil {
			return nil, nil, errStore()
		}
		for _, b := range bs {
			nombres[b.ID] = b.Name
		}
	}
	verificados := map[Visita]bool{}
	if s.visitas != nil && len(vs) > 0 {
		v, err := s.visitas.Validas(ctx, vs)
		if err != nil {
			return nil, nil, errStore()
		}
		verificados = v
	}
	return nombres, verificados, nil
}

// AgruparHistorias agrupa los momentos (del más nuevo al más viejo) por autora: la de la
// historia más reciente primero, hasta MaxPersonasHist, y dentro de cada una del más viejo al
// más nuevo. Quien no está en autores no sale.
func AgruparHistorias(rows []Momento, autores map[bson.ObjectID]AutorView, vista func(Momento) HistoriaMomento) []HistoriaPersona {
	out := []HistoriaPersona{}
	pos := map[bson.ObjectID]int{}
	for _, m := range rows {
		autor, ok := autores[m.AutorID]
		if !ok {
			continue
		}
		i, ya := pos[m.AutorID]
		if !ya {
			if len(out) == MaxPersonasHist {
				continue
			}
			i = len(out)
			pos[m.AutorID] = i
			out = append(out, HistoriaPersona{Autor: autor})
		}
		out[i].Momentos = append([]HistoriaMomento{vista(m)}, out[i].Momentos...)
	}
	return out
}
