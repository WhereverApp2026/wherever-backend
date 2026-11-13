// Package errores recibe los errores de la app que Crashlytics no ve (pagos, sincronización),
// los agrupa por huella sin datos personales y se los muestra al administrador (spec nv-64,
// hoja J9).
package errores

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/platform/logger"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Además del filtro del logger: teléfonos y tarjetas (7 o más dígitos, con separadores) y
// tokens con forma de JWT.
var (
	longDigits = regexp.MustCompile(`\+?[0-9](?:[ .\-]?[0-9]){6,}`)
	jwtLike    = regexp.MustCompile(`[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`)
)

// Store es lo que el service necesita del repositorio.
type Store interface {
	Upsert(ctx context.Context, rep Report) error
	List(ctx context.Context, f Filter, limit int) ([]Report, error)
}

// Service agrupa y lista errores de la app.
type Service struct {
	store  Store
	cursor *pagination.Signer
	now    func() time.Time
}

// NewService recibe el repositorio y el firmador de cursores (nv-17) por constructor.
func NewService(store Store, cursor *pagination.Signer) (*Service, error) {
	if store == nil || cursor == nil {
		return nil, errors.New("errores: faltan el repositorio o el firmador de cursores")
	}
	return &Service{store: store, cursor: cursor, now: time.Now}, nil
}

// Scrub quita del texto correos, llaves, credenciales en URI, secuencias de 7 o más dígitos y
// tokens con forma de JWT. Los JWT van primero: el filtro de dígitos partiría su firma.
func Scrub(text string) string {
	text = jwtLike.ReplaceAllString(text, logger.Redacted)
	text = logger.RedactText(text)
	return longDigits.ReplaceAllString(text, logger.Redacted)
}

// Fingerprint es el SHA-256 hex de módulo y mensaje ya filtrado (no es contraseña: §3 lo permite).
func Fingerprint(module, scrubbed string) string {
	sum := sha256.Sum256([]byte(module + "\x00" + scrubbed))
	return hex.EncodeToString(sum[:])
}

// Report suma una aparición del error; el mensaje se guarda ya filtrado.
func (s *Service) Report(ctx context.Context, req ReportRequest) error {
	msg := Scrub(strings.TrimSpace(req.Message))
	err := s.store.Upsert(ctx, Report{
		Origin: OriginApp, Version: req.Version, Module: req.Module, Message: msg, TraceID: req.TraceID,
		Fingerprint: Fingerprint(req.Module, msg), LastAt: s.now().UTC().Truncate(time.Millisecond),
	})
	if err != nil {
		return apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
	}
	return nil
}

// List devuelve una página de grupos del más reciente al más viejo. El cursor queda atado a
// los filtros: uno de otra consulta es 422.
func (s *Service) List(ctx context.Context, q ListQuery) (pagination.PageResult[View], error) {
	f := Filter{Module: q.Module}
	if q.Since != "" {
		since, err := time.Parse(time.RFC3339, q.Since)
		if err != nil {
			return pagination.PageResult[View]{}, apperr.Validation("datos inválidos", apperr.FieldError{Field: "desde", Message: "debe ser una fecha RFC 3339"})
		}
		since = since.UTC()
		f.Since = &since
	}
	scope := s.cursor.Scope("admin_errores", q.Module, q.Since)
	after, err := position(s.cursor, scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[View]{}, err
	}
	f.After = after
	rows, err := s.store.List(ctx, f, q.Limit)
	if err != nil {
		return pagination.PageResult[View]{}, apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		last := rows[len(rows)-1]
		next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: last.LastAt.UTC().Format(time.RFC3339Nano), Scope: scope})
		if err != nil {
			return pagination.PageResult[View]{}, apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
		}
	}
	return newPage(rows, next, q.Limit), nil
}

// position lee el cursor firmado; uno manipulado o de otros filtros es 422.
func position(signer *pagination.Signer, scope, raw string) (*Position, error) {
	c, ok, err := validation.Position(signer, scope, raw)
	if err != nil || !ok {
		return nil, err
	}
	bad := apperr.Validation("cursor inválido", apperr.FieldError{Field: "cursor", Message: "cursor inválido"})
	at, err := time.Parse(time.RFC3339Nano, c.Sort)
	if err != nil {
		return nil, bad
	}
	id, err := bson.ObjectIDFromHex(c.ID)
	if err != nil {
		return nil, bad
	}
	return &Position{LastAt: at, ID: id}, nil
}
