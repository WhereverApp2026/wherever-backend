// Package auth es el módulo de autenticación, sesiones y recuperación de contraseña (hoja J1,
// spec nv-04). Molde nv-02: handler → DTO + validator → service → repository.
package auth

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mailer"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// Reglas de la hoja J1 y de la constitución §6.
const (
	MaxLoginFailures = 5
	LockoutWindow    = 15 * time.Minute
	PasswordResetTTL = time.Hour
	VerifyEmailTTL   = 24 * time.Hour
	ExportLinkTTL    = 24 * time.Hour
	ResendCooldown   = 60 * time.Second

	loginFailuresKey = "login_fallos:"
)

// Config son los parámetros del servicio, resueltos desde config.Config en main.
type Config struct {
	JWT        middleware.JWTConfig
	AccessTTL  time.Duration
	RefreshTTL time.Duration
}

// Service implementa los casos de uso de J1. Recibe todo por constructor y no guarda estado
// propio: sesiones en MongoDB, revocados y fallos de login en Redis (stateless, multi-instancia).
type Service struct {
	users      *usuarios.Repository
	repo       *Repository
	rdb        *goredis.Client
	mail       mailer.Mailer
	syncMail   mailer.Mailer
	issuer     *Issuer
	hasher     *Hasher
	accessTTL  time.Duration
	refreshTTL time.Duration
	log        *slog.Logger
}

// NewService arma el servicio; falla solo si no puede preparar el hash falso.
func NewService(users *usuarios.Repository, repo *Repository, rdb *goredis.Client, mail mailer.Mailer, cfg Config, log *slog.Logger) (*Service, error) {
	hasher, err := NewHasher()
	if err != nil {
		return nil, err
	}
	return &Service{
		users:      users,
		repo:       repo,
		rdb:        rdb,
		mail:       mail,
		syncMail:   mail,
		issuer:     NewIssuer(cfg.JWT, cfg.AccessTTL),
		hasher:     hasher,
		accessTTL:  cfg.AccessTTL,
		refreshTTL: cfg.RefreshTTL,
		log:        log,
	}, nil
}

// LoginInput son los datos de POST /auth/login ya validados.
type LoginInput struct {
	Email    string
	Password string
	Device   Device
	IP       string
}

// Login compara la contraseña con Argon2id. El contador `login_fallos:{correo}` corre exista
// o no la cuenta, y con 5 fallos bloquea 15 min (423). Sin Redis falla cerrado (503).
func (s *Service) Login(ctx context.Context, in LoginInput) (TokenResponse, error) {
	email := usuarios.NormalizeEmail(in.Email)
	if err := s.checkLockout(ctx, email); err != nil {
		return TokenResponse{}, err
	}
	user, err := s.users.FindByEmail(ctx, email)
	if err != nil && !errors.Is(err, usuarios.ErrNotFound) {
		return TokenResponse{}, dbError(err)
	}
	found := err == nil
	ok := s.hasher.Verify(in.Password, user.PasswordHash)
	if !found || !ok || !user.CanSignIn() {
		return TokenResponse{}, s.registerFailure(ctx, email)
	}
	if err := s.rdb.Del(ctx, loginFailuresKey+email).Err(); err != nil {
		return TokenResponse{}, redisError(err)
	}
	return s.CreateSession(ctx, user, MethodEmail, in.Device, in.IP)
}

func (s *Service) checkLockout(ctx context.Context, email string) error {
	failures, err := s.rdb.Get(ctx, loginFailuresKey+email).Int()
	if err != nil && !errors.Is(err, goredis.Nil) {
		return redisError(err)
	}
	if failures < MaxLoginFailures {
		return nil
	}
	ttl, err := s.rdb.TTL(ctx, loginFailuresKey+email).Result()
	if err != nil {
		return redisError(err)
	}
	return locked(ttl)
}

// registerFailure suma un fallo; al llegar a 5 reinicia la ventana para bloquear 15 min.
func (s *Service) registerFailure(ctx context.Context, email string) error {
	key := loginFailuresKey + email
	failures, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return redisError(err)
	}
	if failures >= MaxLoginFailures {
		if err := s.rdb.Expire(ctx, key, LockoutWindow).Err(); err != nil {
			return redisError(err)
		}
		return locked(LockoutWindow)
	}
	if err := s.rdb.ExpireNX(ctx, key, LockoutWindow).Err(); err != nil {
		return redisError(err)
	}
	return apperr.Unauthorized("correo o contraseña incorrectos")
}

func locked(remaining time.Duration) error {
	minutes := int(math.Ceil(remaining.Minutes()))
	if minutes < 1 {
		minutes = 1
	}
	return apperr.Locked("demasiados intentos fallidos; intenta de nuevo en " + strconv.Itoa(minutes) + " min")
}

// CreateSession abre una sesión y emite el par de tokens. La reutiliza nv-05 al registrarse.
func (s *Service) CreateSession(ctx context.Context, user usuarios.User, method string, device Device, ip string) (TokenResponse, error) {
	now := time.Now().UTC()
	refresh, refreshHash, err := NewOpaque()
	if err != nil {
		return TokenResponse{}, err
	}
	family, err := newFamilyID()
	if err != nil {
		return TokenResponse{}, err
	}
	session := Session{
		ID:          bson.NewObjectID(),
		UserID:      user.ID,
		RefreshHash: refreshHash,
		FamilyID:    family,
		Method:      method,
		Device:      device,
		IP:          ip,
		LastUsed:    now,
		ExpiresAt:   now.Add(s.refreshTTL),
		CreatedAt:   now,
	}
	access, err := s.issuer.Issue(user.ID.Hex(), session.ID.Hex(), user.Roles, now)
	if err != nil {
		return TokenResponse{}, err
	}
	session.CurrentJTI = access.JTI
	if err := s.repo.InsertSession(ctx, session); err != nil {
		return TokenResponse{}, dbError(err)
	}
	return tokenResponse(access, refresh, user), nil
}

// Refresh rota el refresh token. Uno ya rotado revoca toda su familia (posible robo).
func (s *Service) Refresh(ctx context.Context, raw, ip string) (TokenResponse, error) {
	now := time.Now().UTC()
	hash := HashOpaque(raw)
	session, err := s.repo.FindActiveByRefreshHash(ctx, hash, now)
	if errors.Is(err, errNoDocument) {
		return TokenResponse{}, s.handleUnknownRefresh(ctx, hash, now)
	}
	if err != nil {
		return TokenResponse{}, dbError(err)
	}
	user, err := s.activeUser(ctx, session.UserID)
	if err != nil {
		return TokenResponse{}, err
	}
	refresh, refreshHash, err := NewOpaque()
	if err != nil {
		return TokenResponse{}, err
	}
	access, err := s.issuer.Issue(user.ID.Hex(), session.ID.Hex(), user.Roles, now)
	if err != nil {
		return TokenResponse{}, err
	}
	previous := session
	session.RefreshHash, session.CurrentJTI, session.IP = refreshHash, access.JTI, ip
	rotated, err := s.repo.Rotate(ctx, session, hash, now)
	if err != nil {
		return TokenResponse{}, dbError(err)
	}
	if !rotated {
		return TokenResponse{}, apperr.Unauthorized("sesión inválida")
	}
	if err := s.revokeAccess(ctx, now, previous); err != nil {
		return TokenResponse{}, err
	}
	return tokenResponse(access, refresh, user), nil
}

func (s *Service) handleUnknownRefresh(ctx context.Context, hash string, now time.Time) error {
	reused, err := s.repo.FindByPreviousHash(ctx, hash)
	if errors.Is(err, errNoDocument) {
		return apperr.Unauthorized("sesión inválida")
	}
	if err != nil {
		return dbError(err)
	}
	revoked, err := s.repo.RevokeFamily(ctx, reused.FamilyID, now)
	if err != nil {
		return dbError(err)
	}
	s.log.WarnContext(ctx, "refresh reutilizado: familia revocada", "familia_id", reused.FamilyID, "sesiones", len(revoked))
	if err := s.revokeAccess(ctx, now, revoked...); err != nil {
		return err
	}
	return apperr.Unauthorized("sesión inválida; inicia sesión de nuevo")
}

// Logout cierra la sesión del token y revoca su jti.
func (s *Service) Logout(ctx context.Context, claims *middleware.Claims) error {
	userID, sessionID, err := identity(claims)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	if _, err := s.repo.RevokeSession(ctx, userID, sessionID, now); err != nil && !errors.Is(err, errNoDocument) {
		return dbError(err)
	}
	return s.revokeClaims(ctx, now, claims)
}

// LogoutAll cierra todas las sesiones del usuario, la actual incluida.
func (s *Service) LogoutAll(ctx context.Context, claims *middleware.Claims) error {
	userID, _, err := identity(claims)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	revoked, err := s.repo.RevokeUserSessions(ctx, userID, bson.ObjectID{}, now)
	if err != nil {
		return dbError(err)
	}
	if err := s.revokeAccess(ctx, now, revoked...); err != nil {
		return err
	}
	return s.revokeClaims(ctx, now, claims)
}

// ListSessions devuelve los dispositivos con sesión abierta y marca la actual.
func (s *Service) ListSessions(ctx context.Context, claims *middleware.Claims) ([]SessionResponse, error) {
	userID, sessionID, err := identity(claims)
	if err != nil {
		return nil, err
	}
	sessions, err := s.repo.ListActive(ctx, userID, time.Now().UTC())
	if err != nil {
		return nil, dbError(err)
	}
	out := make([]SessionResponse, 0, len(sessions))
	for _, session := range sessions {
		out = append(out, SessionResponse{
			ID:       session.ID.Hex(),
			Device:   DeviceResponse{Model: session.Device.Model, System: session.Device.System, AppVersion: session.Device.AppVersion},
			LastUsed: session.LastUsed,
			Current:  session.ID == sessionID,
		})
	}
	return out, nil
}

// RevokeSession cierra una sesión del usuario; la de otro usuario o inexistente es 404.
func (s *Service) RevokeSession(ctx context.Context, claims *middleware.Claims, id string) error {
	userID, _, err := identity(claims)
	if err != nil {
		return err
	}
	target, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return apperr.NotFound("sesión no encontrada")
	}
	now := time.Now().UTC()
	revoked, err := s.repo.RevokeSession(ctx, userID, target, now)
	if errors.Is(err, errNoDocument) {
		return apperr.NotFound("sesión no encontrada")
	}
	if err != nil {
		return dbError(err)
	}
	return s.revokeAccess(ctx, now, revoked)
}

// RequestPasswordReset manda el enlace (1 h). Responde igual exista o no el correo y no
// reenvía antes de 60 s; por eso nunca devuelve un error que distinga los casos.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.users.FindByEmail(ctx, email)
	if errors.Is(err, usuarios.ErrNotFound) {
		return nil
	}
	if err != nil {
		return dbError(err)
	}
	if !user.CanSignIn() {
		return nil
	}
	if wait, err := s.cooldown(ctx, user.ID, mailer.KindPasswordReset); err != nil || wait > 0 {
		return err
	}
	return s.sendLink(ctx, user, mailer.KindPasswordReset, PasswordResetTTL)
}

// ResetInput son los datos de POST /auth/restablecer ya validados.
type ResetInput struct {
	Token       string
	NewPassword string
	Device      Device
	IP          string
}

// ResetPassword usa el enlace una sola vez, cambia la contraseña, cierra las demás sesiones
// y deja una sesión nueva abierta.
func (s *Service) ResetPassword(ctx context.Context, in ResetInput) (TokenResponse, error) {
	now := time.Now().UTC()
	link, err := s.repo.ConsumeToken(ctx, HashOpaque(in.Token), string(mailer.KindPasswordReset), now)
	if errors.Is(err, errNoDocument) {
		return TokenResponse{}, apperr.BadRequest("el enlace venció o ya se usó")
	}
	if err != nil {
		return TokenResponse{}, dbError(err)
	}
	user, err := s.activeUser(ctx, link.UserID)
	if err != nil {
		return TokenResponse{}, err
	}
	if err := s.setPassword(ctx, user.ID, in.NewPassword); err != nil {
		return TokenResponse{}, err
	}
	revoked, err := s.repo.RevokeUserSessions(ctx, user.ID, bson.ObjectID{}, now)
	if err != nil {
		return TokenResponse{}, dbError(err)
	}
	if err := s.revokeAccess(ctx, now, revoked...); err != nil {
		return TokenResponse{}, err
	}
	if err := s.rdb.Del(ctx, loginFailuresKey+user.Email).Err(); err != nil {
		return TokenResponse{}, redisError(err)
	}
	return s.CreateSession(ctx, user, MethodEmail, in.Device, in.IP)
}

// VerifyEmail usa el enlace de verificación una sola vez y marca el correo.
func (s *Service) VerifyEmail(ctx context.Context, raw string) error {
	link, err := s.repo.ConsumeToken(ctx, HashOpaque(raw), string(mailer.KindVerifyEmail), time.Now().UTC())
	if errors.Is(err, errNoDocument) {
		return apperr.BadRequest("el enlace venció o ya se usó")
	}
	if err != nil {
		return dbError(err)
	}
	if err := s.users.MarkEmailVerified(ctx, link.UserID); err != nil {
		return dbError(err)
	}
	return nil
}

// ResendVerification reenvía el correo de verificación, como máximo 1 cada 60 s (429).
func (s *Service) ResendVerification(ctx context.Context, claims *middleware.Claims) error {
	userID, _, err := identity(claims)
	if err != nil {
		return err
	}
	user, err := s.activeUser(ctx, userID)
	if err != nil {
		return err
	}
	if user.EmailVerified {
		return nil
	}
	wait, err := s.cooldown(ctx, user.ID, mailer.KindVerifyEmail)
	if err != nil {
		return err
	}
	if wait > 0 {
		return apperr.RateLimited(int(math.Ceil(wait.Seconds())))
	}
	return s.sendLink(ctx, user, mailer.KindVerifyEmail, VerifyEmailTTL)
}

// SendVerification manda el enlace de verificación (24 h). Lo usa nv-05 al registrar.
func (s *Service) SendVerification(ctx context.Context, user usuarios.User) error {
	return s.sendLink(ctx, user, mailer.KindVerifyEmail, VerifyEmailTTL)
}

// CloseAllSessions cierra todas las sesiones del usuario y revoca sus access (lo usa nv-05 al
// pedir eliminar la cuenta).
func (s *Service) CloseAllSessions(ctx context.Context, userID bson.ObjectID) error {
	now := time.Now().UTC()
	revoked, err := s.repo.RevokeUserSessions(ctx, userID, bson.ObjectID{}, now)
	if err != nil {
		return dbError(err)
	}
	return s.revokeAccess(ctx, now, revoked...)
}

// WithSyncMailer fija el mailer síncrono (sin cola) que usa SendExportLink: el trabajo
// nocturno necesita saber si el aviso salió (nv-06). Por defecto es el mismo de las peticiones.
func (s *Service) WithSyncMailer(m mailer.Mailer) *Service {
	s.syncMail = m
	return s
}

// SendExportLink manda el enlace de un solo uso (24 h) para descargar "mis datos" (D-2, nv-05)
// y devuelve el error del envío. Cada intento invalida el enlace anterior y crea uno nuevo:
// solo guardamos el hash, así que no se puede reenviar el mismo (nv-06).
func (s *Service) SendExportLink(ctx context.Context, userID bson.ObjectID) error {
	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return dbError(err)
	}
	if err := s.repo.InvalidateTokens(ctx, userID, string(mailer.KindExport), time.Now().UTC()); err != nil {
		return dbError(err)
	}
	raw, err := s.issueLink(ctx, user, mailer.KindExport, ExportLinkTTL)
	if err != nil {
		return err
	}
	return s.syncMail.Send(ctx, mailer.Message{To: user.Email, Name: user.Name, Kind: mailer.KindExport, Token: raw})
}

// ConsumeExportLink usa el enlace de descarga una sola vez y devuelve de quién es.
func (s *Service) ConsumeExportLink(ctx context.Context, raw string) (bson.ObjectID, error) {
	link, err := s.repo.ConsumeToken(ctx, HashOpaque(raw), string(mailer.KindExport), time.Now().UTC())
	if errors.Is(err, errNoDocument) {
		return bson.ObjectID{}, apperr.BadRequest("el enlace venció o ya se usó")
	}
	if err != nil {
		return bson.ObjectID{}, dbError(err)
	}
	return link.UserID, nil
}

// ChangePassword exige la contraseña actual y cierra las otras sesiones; la actual sigue.
func (s *Service) ChangePassword(ctx context.Context, claims *middleware.Claims, current, next string) error {
	userID, sessionID, err := identity(claims)
	if err != nil {
		return err
	}
	user, err := s.activeUser(ctx, userID)
	if err != nil {
		return err
	}
	if !s.hasher.Verify(current, user.PasswordHash) {
		return apperr.Unauthorized("la contraseña actual no coincide")
	}
	if err := s.setPassword(ctx, user.ID, next); err != nil {
		return err
	}
	now := time.Now().UTC()
	revoked, err := s.repo.RevokeUserSessions(ctx, user.ID, sessionID, now)
	if err != nil {
		return dbError(err)
	}
	return s.revokeAccess(ctx, now, revoked...)
}

// activeUser carga al usuario de una sesión; si ya no existe o no está activo es 401.
func (s *Service) activeUser(ctx context.Context, id bson.ObjectID) (usuarios.User, error) {
	user, err := s.users.FindByID(ctx, id)
	if errors.Is(err, usuarios.ErrNotFound) || (err == nil && !user.CanSignIn()) {
		return usuarios.User{}, apperr.Unauthorized("sesión inválida")
	}
	if err != nil {
		return usuarios.User{}, dbError(err)
	}
	return user, nil
}

func (s *Service) setPassword(ctx context.Context, userID bson.ObjectID, password string) error {
	hash, err := s.hasher.Hash(password)
	if err != nil {
		return err
	}
	if err := s.users.SetPassword(ctx, userID, hash); err != nil {
		return dbError(err)
	}
	return nil
}

// cooldown dice cuánto falta para poder mandar otro enlace de ese tipo (0 = ya se puede).
func (s *Service) cooldown(ctx context.Context, userID bson.ObjectID, kind mailer.Kind) (time.Duration, error) {
	last, ok, err := s.repo.LastTokenAt(ctx, userID, string(kind))
	if err != nil {
		return 0, dbError(err)
	}
	if !ok {
		return 0, nil
	}
	return max(0, ResendCooldown-time.Since(last)), nil
}

// sendLink guarda el hash del enlace y lo encola. Un fallo del correo no cambia la
// respuesta (nv-06): con la cola nunca hay error; si el mailer devuelve uno, solo se registra.
func (s *Service) sendLink(ctx context.Context, user usuarios.User, kind mailer.Kind, ttl time.Duration) error {
	raw, err := s.issueLink(ctx, user, kind, ttl)
	if err != nil {
		return err
	}
	if err := s.mail.Send(ctx, mailer.Message{To: user.Email, Name: user.Name, Kind: kind, Token: raw}); err != nil {
		s.log.WarnContext(ctx, "no se pudo encolar el correo", "tipo", string(kind))
	}
	return nil
}

// issueLink crea el token de un solo uso y guarda solo su hash; devuelve el token en claro.
func (s *Service) issueLink(ctx context.Context, user usuarios.User, kind mailer.Kind, ttl time.Duration) (string, error) {
	raw, hash, err := NewOpaque()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	link := OneTimeToken{ID: bson.NewObjectID(), UserID: user.ID, Kind: string(kind), TokenHash: hash, ExpiresAt: now.Add(ttl), CreatedAt: now}
	if err := s.repo.InsertToken(ctx, link); err != nil {
		return "", dbError(err)
	}
	return raw, nil
}

// revokeAccess pone en `jwt_revocado:{jti}` el access token vigente de cada sesión, con TTL
// igual a su vida remanente. Sin Redis falla cerrado (503).
func (s *Service) revokeAccess(ctx context.Context, now time.Time, sessions ...Session) error {
	for _, session := range sessions {
		if err := s.revokeJTI(ctx, now, session.CurrentJTI, session.LastUsed.Add(s.accessTTL)); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) revokeClaims(ctx context.Context, now time.Time, claims *middleware.Claims) error {
	if claims.ExpiresAt == nil {
		return apperr.Unauthorized("credenciales inválidas")
	}
	return s.revokeJTI(ctx, now, claims.ID, claims.ExpiresAt.Time)
}

func (s *Service) revokeJTI(ctx context.Context, now time.Time, jti string, expiresAt time.Time) error {
	remaining := expiresAt.Sub(now)
	if jti == "" || remaining <= 0 {
		return nil
	}
	if err := s.rdb.Set(ctx, middleware.RevokedKey(jti), "1", remaining).Err(); err != nil {
		return redisError(err)
	}
	return nil
}

// identity extrae usuario y sesión de los claims; un `sub` o `sid` malformado es 401.
func identity(claims *middleware.Claims) (userID, sessionID bson.ObjectID, err error) {
	if claims == nil {
		return userID, sessionID, apperr.Unauthorized("autenticación requerida")
	}
	if userID, err = bson.ObjectIDFromHex(claims.Subject); err != nil {
		return userID, sessionID, apperr.Unauthorized("credenciales inválidas")
	}
	if sessionID, err = bson.ObjectIDFromHex(claims.SessionID); err != nil {
		return userID, sessionID, apperr.Unauthorized("credenciales inválidas")
	}
	return userID, sessionID, nil
}

func tokenResponse(access AccessToken, refresh string, user usuarios.User) TokenResponse {
	roles := user.Roles
	if roles == nil {
		roles = []string{}
	}
	return TokenResponse{
		AccessToken:  access.Raw,
		RefreshToken: refresh,
		ExpiresAt:    access.ExpiresAt,
		User:         UserResponse{ID: user.ID.Hex(), Name: user.Name, Email: user.Email, EmailVerified: user.EmailVerified, Roles: roles},
	}
}

func dbError(err error) error {
	return apperr.Unavailable("servicio no disponible").WithDetail(err.Error())
}

func redisError(err error) error {
	return apperr.Unavailable("no se pudo verificar la sesión").WithDetail(err.Error())
}
