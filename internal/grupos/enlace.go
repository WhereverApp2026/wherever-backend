package grupos

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Enlace de invitación (nv-38): 128 bits aleatorios, 7 días, revocable. En la base solo va
// el SHA-256; el código viaja en el fragmento del enlace y en el cuerpo de las peticiones,
// nunca en la ruta (la ruta queda en el log de acceso).
const (
	codeBytes = 16
	codeTTL   = 7 * 24 * time.Hour
	linkPath  = "/plan#codigo="
)

// Places trae las tarjetas públicas de los negocios del plan.
type Places interface {
	FindCards(ctx context.Context, ids []bson.ObjectID, now time.Time) ([]negocios.PlaceItem, error)
}

type shareDeps struct {
	places   Places
	linkBase string
}

// WithShare activa el enlace de invitación: tarjetas de negocios para la vista pública y
// APP_LINK_BASE para armar el enlace.
func (s *Service) WithShare(places Places, linkBase string) *Service {
	s.share = shareDeps{places: places, linkBase: strings.TrimRight(linkBase, "/")}
	return s
}

func newCode() (raw, hash string, err error) {
	buf := make([]byte, codeBytes)
	if _, err = rand.Read(buf); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(buf)
	return raw, hashCode(raw), nil
}

// hashCode es el SHA-256 en hexadecimal: basta porque el código es aleatorio de alta
// entropía, no una contraseña (constitución §6).
func hashCode(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func errLinkNotFound() error { return apperr.NotFound("enlace no encontrado") }

// Share crea (o reemplaza) el enlace del grupo de un plan propio; crea el grupo si falta.
func (s *Service) Share(ctx context.Context, subject, rawPlanID string) (ShareResponse, error) {
	me, err := self(subject)
	if err != nil {
		return ShareResponse{}, err
	}
	planID, err := bson.ObjectIDFromHex(rawPlanID)
	if err != nil {
		return ShareResponse{}, errPlanNotFound()
	}
	if err := s.ownPlan(ctx, me, planID); err != nil {
		return ShareResponse{}, err
	}
	g, err := s.groupForPlan(ctx, me, planID)
	if err != nil {
		return ShareResponse{}, errStore()
	}
	raw, hash, err := newCode()
	if err != nil {
		return ShareResponse{}, errStore()
	}
	now := s.now().UTC()
	expires := now.Add(codeTTL)
	if err := s.repo.SetCode(ctx, g.ID, me, hash, expires, now); err != nil {
		return ShareResponse{}, errStore()
	}
	return ShareResponse{Code: raw, Link: s.share.linkBase + linkPath + raw, GroupID: g.ID.Hex(), ExpiresAt: expires}, nil
}

// Revoke quita el enlace del grupo de un plan propio; sin enlace = 404.
func (s *Service) Revoke(ctx context.Context, subject, rawPlanID string) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	planID, err := bson.ObjectIDFromHex(rawPlanID)
	if err != nil {
		return errPlanNotFound()
	}
	if err := s.ownPlan(ctx, me, planID); err != nil {
		return err
	}
	g, err := s.repo.FindByPlan(ctx, planID)
	if errors.Is(err, ErrNotFound) {
		return errLinkNotFound()
	}
	if err != nil {
		return errStore()
	}
	ok, err := s.repo.ClearCode(ctx, g.ID, me, s.now().UTC())
	if err != nil {
		return errStore()
	}
	if !ok {
		return errLinkNotFound()
	}
	return nil
}

// byCode trae el grupo del enlace con su plan activo. Inexistente, revocado o de un plan
// borrado = 404; vencido = ErrExpired (la vista pública lo convierte en 404 y unirse en 410).
func (s *Service) byCode(ctx context.Context, raw string) (Group, planes.Plan, error) {
	g, err := s.repo.FindByCode(ctx, hashCode(raw))
	if errors.Is(err, ErrNotFound) {
		return Group{}, planes.Plan{}, errLinkNotFound()
	}
	if err != nil {
		return Group{}, planes.Plan{}, errStore()
	}
	if g.CodeExpires == nil || !s.now().Before(*g.CodeExpires) {
		return Group{}, planes.Plan{}, errExpired
	}
	p, err := s.plans.FindActive(ctx, g.PlanID)
	if errors.Is(err, planes.ErrNotFound) {
		return Group{}, planes.Plan{}, errLinkNotFound()
	}
	if err != nil {
		return Group{}, planes.Plan{}, errStore()
	}
	return g, p, nil
}

var errExpired = errors.New("enlace vencido")

// Preview es la vista pública del plan compartido: sin personas, sin ids de cuentas, sin
// notas ni origen. Vencido = 404, igual que inexistente.
func (s *Service) Preview(ctx context.Context, raw string) (PublicPlan, error) {
	g, p, err := s.byCode(ctx, raw)
	if errors.Is(err, errExpired) {
		return PublicPlan{}, errLinkNotFound()
	}
	if err != nil {
		return PublicPlan{}, err
	}
	ids := make([]bson.ObjectID, 0, len(p.Paradas))
	for _, st := range p.Paradas {
		ids = append(ids, st.NegocioID)
	}
	cards, err := s.share.places.FindCards(ctx, ids, s.now())
	if err != nil {
		return PublicPlan{}, errStore()
	}
	return newPublicPlan(g, p, cards), nil
}

// Join deja a quien llama confirmado en el grupo del enlace. No exige amistad (el enlace es
// la invitación), pero sí que el organizador y quien entra no se hayan bloqueado (404).
func (s *Service) Join(ctx context.Context, subject string, req JoinRequest) (View, error) {
	me, err := self(subject)
	if err != nil {
		return View{}, err
	}
	g, _, err := s.byCode(ctx, req.Code)
	if errors.Is(err, errExpired) {
		return View{}, apperr.Gone("el enlace venció; pide uno nuevo al organizador")
	}
	if err != nil {
		return View{}, err
	}
	if g.OrganizerID != me {
		if err := s.notBlocked(ctx, g.OrganizerID, me); err != nil {
			return View{}, err
		}
		if err := s.repo.Join(ctx, g.ID, me, s.now().UTC()); err != nil {
			if err := s.joinError(ctx, g.ID, me, err); err != nil {
				return View{}, err
			}
		}
	}
	return s.view(ctx, g.ID, me)
}

func (s *Service) notBlocked(ctx context.Context, organizer, me bson.ObjectID) error {
	f, ok, err := s.friends.FindByPair(ctx, organizer, me)
	if err != nil {
		return errStore()
	}
	if ok && f.Status == amistades.StatusBlocked {
		return errLinkNotFound()
	}
	return nil
}

// joinError: si en la carrera la misma persona ya quedó confirmada, no es error.
func (s *Service) joinError(ctx context.Context, groupID, me bson.ObjectID, err error) error {
	if !errors.Is(err, ErrFull) {
		return errStore()
	}
	g, findErr := s.repo.FindByID(ctx, groupID)
	if findErr == nil {
		if m, in := g.Member(me); in && m.Status == StatusConfirmed {
			return nil
		}
	}
	return errFull()
}
