package notificaciones

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/pagination"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

// Service atiende el centro de avisos y el registro de teléfonos, y cumple Notifier.
type Service struct {
	repo   *Repository
	pusher *Pusher
	cursor *pagination.Signer
	log    *slog.Logger
	now    func() time.Time
}

// NewService recibe sus dependencias por constructor; sin cursor la lista falla cerrada.
func NewService(repo *Repository, cursor *pagination.Signer, log *slog.Logger) *Service {
	return &Service{repo: repo, cursor: cursor, log: log, now: time.Now}
}

// WithPush manda además un push por FCM de cada aviso (nv-41). Sin él, solo el centro.
func (s *Service) WithPush(p *Pusher) *Service {
	if p != nil {
		s.pusher = p
	}
	return s
}

func errNoteNotFound() error { return apperr.NotFound("aviso no encontrado") }
func errStore() error        { return apperr.Unavailable("servicio no disponible") }

func self(subject string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(subject)
	if err != nil {
		return bson.ObjectID{}, apperr.Unauthorized("autenticación requerida")
	}
	return id, nil
}

// HashToken es el SHA-256 del token de FCM; basta porque el token es aleatorio de alta
// entropía.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// Emit guarda el aviso en el centro. Un fallo se registra (sin datos) y no se propaga: avisar
// nunca cambia la respuesta de la acción que lo originó.
func (s *Service) Emit(ctx context.Context, to bson.ObjectID, kind string, data map[string]string) {
	now := s.now().UTC()
	if data == nil {
		data = map[string]string{}
	}
	n := Notification{ID: bson.NewObjectID(), UserID: to, Kind: kind, Data: data, CreatedAt: now, ExpiresAt: now.Add(Retention)}
	if err := s.repo.Insert(ctx, &n); err != nil {
		s.log.Warn("notificaciones: el aviso no se guardó", "evento", "aviso_fallido", "tipo", kind)
		return // el centro y el push cuentan lo mismo: sin aviso guardado no hay push
	}
	if s.pusher != nil {
		s.pusher.Enqueue(to, kind, data)
	}
}

// List devuelve una página del centro, del aviso más nuevo al más viejo.
func (s *Service) List(ctx context.Context, subject string, q ListQuery) (pagination.PageResult[Item], error) {
	me, err := self(subject)
	if err != nil {
		return pagination.PageResult[Item]{}, err
	}
	scope := s.cursor.Scope("notificaciones", me.Hex())
	after, err := position(s.cursor, scope, q.Cursor)
	if err != nil {
		return pagination.PageResult[Item]{}, err
	}
	rows, err := s.repo.List(ctx, me, after, q.Limit)
	if err != nil {
		return pagination.PageResult[Item]{}, errStore()
	}
	next := ""
	if len(rows) > q.Limit {
		rows = rows[:q.Limit]
		last := rows[len(rows)-1]
		next, err = s.cursor.Encode(pagination.Cursor{ID: last.ID.Hex(), Sort: last.CreatedAt.UTC().Format(time.RFC3339Nano), Scope: scope})
		if err != nil {
			return pagination.PageResult[Item]{}, errStore()
		}
	}
	items := make([]Item, 0, len(rows))
	for _, n := range rows {
		items = append(items, newItem(n))
	}
	return pagination.NewPageResult(items, next, q.Limit), nil
}

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
	return &Position{CreatedAt: at, ID: id}, nil
}

// Unread cuenta los avisos sin leer.
func (s *Service) Unread(ctx context.Context, subject string) (UnreadResponse, error) {
	me, err := self(subject)
	if err != nil {
		return UnreadResponse{}, err
	}
	n, err := s.repo.CountUnread(ctx, me)
	if err != nil {
		return UnreadResponse{}, errStore()
	}
	return UnreadResponse{Total: n}, nil
}

func noteID(raw string) (bson.ObjectID, error) {
	id, err := bson.ObjectIDFromHex(raw)
	if err != nil {
		return bson.ObjectID{}, errNoteNotFound()
	}
	return id, nil
}

// Mark marca o desmarca un aviso propio; el de otra persona es 404.
func (s *Service) Mark(ctx context.Context, subject, rawID string, req MarkRequest) (Item, error) {
	me, err := self(subject)
	if err != nil {
		return Item{}, err
	}
	id, err := noteID(rawID)
	if err != nil {
		return Item{}, err
	}
	n, err := s.repo.SetRead(ctx, me, id, *req.Read, s.now().UTC())
	if errors.Is(err, ErrNotFound) {
		return Item{}, errNoteNotFound()
	}
	if err != nil {
		return Item{}, errStore()
	}
	return newItem(n), nil
}

// ReadAll marca todos los avisos como leídos.
func (s *Service) ReadAll(ctx context.Context, subject string) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	if err := s.repo.ReadAll(ctx, me, s.now().UTC()); err != nil {
		return errStore()
	}
	return nil
}

// Delete borra un aviso propio; el de otra persona es 404.
func (s *Service) Delete(ctx context.Context, subject, rawID string) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	id, err := noteID(rawID)
	if err != nil {
		return err
	}
	err = s.repo.Delete(ctx, me, id)
	if errors.Is(err, ErrNotFound) {
		return errNoteNotFound()
	}
	if err != nil {
		return errStore()
	}
	return nil
}

// RegisterDevice guarda el token del teléfono (hasta 5 por cuenta; el sexto es 422).
func (s *Service) RegisterDevice(ctx context.Context, subject string, req DeviceRequest) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	platform := req.Platform
	if platform == "" {
		platform = PlatformAndroid
	}
	d := Device{UserID: me, Token: req.Token, TokenHash: HashToken(req.Token), Platform: platform}
	err = s.repo.RegisterDevice(ctx, d, s.now().UTC())
	if errors.Is(err, ErrTooManyDevs) {
		return apperr.Validation("ya tienes 5 teléfonos registrados; quita uno primero",
			apperr.FieldError{Field: "token", Message: "máximo 5 dispositivos"})
	}
	if err != nil {
		return errStore()
	}
	return nil
}

// RemoveDevice quita el token del teléfono de la cuenta; si no era suyo, 404.
func (s *Service) RemoveDevice(ctx context.Context, subject string, req DeviceRemoveRequest) error {
	me, err := self(subject)
	if err != nil {
		return err
	}
	err = s.repo.RemoveDevice(ctx, me, HashToken(req.Token))
	if errors.Is(err, ErrNotFound) {
		return apperr.NotFound("dispositivo no encontrado")
	}
	if err != nil {
		return errStore()
	}
	return nil
}
