package personas

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/contadores"
	"github.com/WhereverApp2026/wherever-backend/internal/insignias"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Pestañas del perfil de otra persona (nv-81). Sus rutas viven en postales, planes y reseñas
// (importar esos módulos aquí haría ciclo) y piden permiso con Service.Pestana.
const (
	PestanaMomentos = "momentos"
	PestanaPlanes   = "planes"
	PestanaResenas  = "resenas"
)

// Límite del perfil: las tres pestañas cuentan en la misma política (nv-70, nv-81).
const (
	PoliticaPerfil = profilePolicy
	LimitePerfil   = profileLimit
	VentanaPerfil  = profileWindow
)

// MaxDestacadas son las insignias que salen en el perfil («Explorador · Taquero · +3»).
const MaxDestacadas = 3

// VeActividad dice si quien mira ve la actividad (pestañas y contadores) según `actividad`
// (nv-82): `nadie`, nadie; `amigos`, solo los amigos; `todos`, cualquiera con sesión.
func VeActividad(soc usuarios.Social, amigo bool) bool {
	switch soc.ActivityLevel() {
	case usuarios.ActivityNobody:
		return false
	case usuarios.ActivityAll:
		return true
	default:
		return amigo
	}
}

// PestanaVisible suma a VeActividad el interruptor de cada pestaña.
func PestanaVisible(soc usuarios.Social, amigo bool, pestana string) bool {
	if !VeActividad(soc, amigo) {
		return false
	}
	switch pestana {
	case PestanaMomentos:
		return soc.SharesPostcards()
	case PestanaPlanes:
		return soc.SharesPlans()
	case PestanaResenas:
		return soc.SharesReviews()
	default:
		return false
	}
}

// Vista es el permiso de quien mira sobre una pestaña: de quién es, si son amigos y si la
// privacidad de la persona deja ver la pestaña (si no, la página va vacía).
type Vista struct {
	Autor   bson.ObjectID
	Amigo   bool
	Visible bool
}

// Pestana aplica las reglas del perfil a una pestaña: inexistente, inactiva, propia o con
// bloqueo en cualquier sentido = el mismo 404 (nv-70).
func (s *Service) Pestana(ctx context.Context, subject, rawID, pestana string) (Vista, error) {
	me, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return Vista{}, apperr.Unauthorized("credenciales inválidas")
	}
	id, err := bson.ObjectIDFromHex(rawID)
	if err != nil {
		return Vista{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "id", Message: "id inválido"})
	}
	u, rel, err := s.visiblePerson(ctx, me, id)
	if err != nil {
		return Vista{}, err
	}
	amigo := rel == RelationFriend
	return Vista{Autor: u.ID, Amigo: amigo, Visible: PestanaVisible(u.Privacy.Social, amigo, pestana)}, nil
}

// Contadores cuenta lo de una persona (lo cumple *contadores.Service).
type Contadores interface {
	De(ctx context.Context, id bson.ObjectID) (contadores.Contadores, error)
}

// WithContadores conecta los contadores del perfil; sin ellos salen en null.
func (s *Service) WithContadores(c Contadores) *Service {
	s.contadores = c
	return s
}

// ContadoresView son los contadores del perfil de otra persona; null = su privacidad no los
// deja ver. `planes` cuenta solo los públicos: lo que muestra su pestaña.
type ContadoresView struct {
	Planes  *int64 `json:"planes"`
	Amigos  *int64 `json:"amigos"`
	Resenas *int64 `json:"resenas"`
}

// contadoresDe arma los contadores visibles; sin servicio o sin permiso, en null.
func (s *Service) contadoresDe(ctx context.Context, u usuarios.User, amigo bool) (ContadoresView, error) {
	soc := u.Privacy.Social
	if s.contadores == nil || !VeActividad(soc, amigo) {
		return ContadoresView{}, nil
	}
	c, err := s.contadores.De(ctx, u.ID)
	if err != nil {
		return ContadoresView{}, errUnavailable(err)
	}
	out := ContadoresView{Amigos: &c.Amigos}
	if soc.SharesPlans() {
		out.Planes = &c.PlanesPublicos
	}
	if soc.SharesReviews() {
		out.Resenas = &c.Resenas
	}
	return out, nil
}

// destacadas da las primeras MaxDestacadas insignias ganadas y el total; con `actividad: nadie`
// o sin insignias conectadas, vacías.
func (s *Service) destacadas(ctx context.Context, u usuarios.User) ([]insignias.Ganada, int, error) {
	if s.insignias == nil || u.Privacy.Social.HidesActivity() {
		return []insignias.Ganada{}, 0, nil
	}
	ganadas, err := s.insignias.GanadasDe(ctx, u.ID)
	if err != nil {
		return nil, 0, err
	}
	return ganadas[:min(len(ganadas), MaxDestacadas)], len(ganadas), nil
}

// NivelView es el nivel de la persona en la tabla de nv-79.
type NivelView struct {
	Numero int    `json:"numero"`
	Nombre string `json:"nombre"`
}

// nivelDe lee `nivel_numero`; una cuenta previa a nv-79 (sin número) va en el nivel 1.
func nivelDe(u usuarios.User) *NivelView {
	n := insignias.NivelNumero(u.LevelNumber)
	return &NivelView{Numero: n.Numero, Nombre: n.Nombre}
}
