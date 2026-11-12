// Package personas busca a otras personas por nombre de usuario o por teléfono (spec nv-35,
// hoja J2) sin revelar quién existe: la respuesta tiene la misma forma con o sin resultado,
// nunca lleva correo ni teléfono, y el teléfono solo se consulta por su HMAC.
package personas

import (
	"context"
	"errors"
	"log/slog"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Límites de la búsqueda (nv-35).
const (
	// MaxResults es el tope de resultados por prefijo; el teléfono da 0 o 1.
	MaxResults  = 10
	maxQueryLen = 32
	// localDigits: un número de 10 dígitos sin lada es de México (+52).
	localDigits    = 10
	defaultCountry = "+52"
)

var (
	usernamePrefix = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)
	e164           = regexp.MustCompile(`^\+[1-9][0-9]{7,14}$`)
	phoneChars     = regexp.MustCompile(`^\+?[0-9 ()\-.]+$`)
)

// Kind dice cómo se interpretó `q`.
type Kind int

// Tipos de búsqueda.
const (
	KindUsername Kind = iota + 1
	KindPhone
)

// Query es `q` ya interpretada y normalizada.
type Query struct {
	Kind  Kind
	Value string
}

// ParseQuery interpreta `q`: teléfono si empieza con `+` o si son 10 o más dígitos (con
// espacios, guiones, puntos o paréntesis), normalizado a E.164; si no, prefijo de nombre de
// usuario en minúsculas (`@` inicial opcional). Lo demás es 422.
func ParseQuery(raw string) (Query, error) {
	q := strings.TrimSpace(raw)
	if q == "" || len(q) > maxQueryLen {
		return Query{}, invalid("escribe un nombre de usuario o un teléfono")
	}
	if phone, ok, err := parsePhone(q); ok {
		return Query{Kind: KindPhone, Value: phone}, err
	}
	name := strings.ToLower(strings.TrimPrefix(q, "@"))
	if !usernamePrefix.MatchString(name) {
		return Query{}, invalid("el nombre de usuario lleva de 3 a 20 letras, números o guion bajo")
	}
	return Query{Kind: KindUsername, Value: name}, nil
}

// parsePhone dice si q es un teléfono (ok) y lo normaliza a E.164.
func parsePhone(q string) (phone string, ok bool, err error) {
	if !phoneChars.MatchString(q) {
		return "", false, nil
	}
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, q)
	plus := strings.HasPrefix(q, "+")
	if !plus && len(digits) < localDigits {
		return "", false, nil
	}
	phone = "+" + digits
	if !plus && len(digits) == localDigits {
		phone = defaultCountry + digits
	}
	if !e164.MatchString(phone) {
		return "", true, invalid("el teléfono debe estar en formato internacional, p. ej. +5215512345678")
	}
	return phone, true, nil
}

func invalid(msg string) error {
	return apperr.Validation("búsqueda inválida", apperr.FieldError{Field: "q", Message: msg})
}

// Users es lo que la búsqueda necesita del repositorio de usuarios.
type Users interface {
	SearchByUsernamePrefix(ctx context.Context, prefix string, exclude []bson.ObjectID, limit int64) ([]usuarios.User, error)
	FindByPhoneSearchable(ctx context.Context, phoneHash string, exclude []bson.ObjectID) (usuarios.User, bool, error)
}

// Blocks da las personas con relación `bloqueada` con el usuario, en cualquier sentido
// (amistades, nv-36): no aparecen en su búsqueda.
type Blocks interface {
	BlockedWith(ctx context.Context, userID bson.ObjectID) ([]bson.ObjectID, error)
}

// Service busca personas.
type Service struct {
	users    Users
	blocks   Blocks
	phoneKey []byte
	log      *slog.Logger
}

// NewService recibe los repositorios y la llave HMAC del teléfono (la misma de nv-05).
func NewService(users Users, blocks Blocks, phoneKey []byte, log *slog.Logger) (*Service, error) {
	if users == nil || blocks == nil || len(phoneKey) == 0 {
		return nil, errors.New("personas: faltan los repositorios o PHONE_HMAC_KEY")
	}
	return &Service{users: users, blocks: blocks, phoneKey: phoneKey, log: log}, nil
}

// Search busca por prefijo de nombre de usuario o por teléfono exacto, sin incluir a quien
// busca ni a quien tenga una relación bloqueada con él (nv-36). Un teléfono no registrado y
// uno registrado pero no buscable dan la misma respuesta.
func (s *Service) Search(ctx context.Context, subject, raw string) (SearchResponse, error) {
	self, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return SearchResponse{}, apperr.Unauthorized("credenciales inválidas")
	}
	q, err := ParseQuery(raw)
	if err != nil {
		return SearchResponse{}, err
	}
	exclude, err := s.blocks.BlockedWith(ctx, self)
	if err != nil {
		return SearchResponse{}, apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
	}
	exclude = append(exclude, self)
	var found []usuarios.User
	switch q.Kind {
	case KindPhone:
		u, ok, ferr := s.users.FindByPhoneSearchable(ctx, usuarios.HashPhone(s.phoneKey, q.Value), exclude)
		err = ferr
		if ok {
			found = []usuarios.User{u}
		}
	default:
		found, err = s.users.SearchByUsernamePrefix(ctx, q.Value, exclude, MaxResults)
	}
	if err != nil {
		return SearchResponse{}, apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
	}
	return newSearchResponse(found), nil
}
