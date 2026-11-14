package cupones

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
)

// Motivos del rojo al validar.
const (
	MotivoYaUsado     = "ya_usado"
	MotivoOtroNegocio = "otro_negocio"
	MotivoNoExiste    = "no_existe"
)

func rojo(motivo string) ValidarResponse { return ValidarResponse{Valido: false, Motivo: motivo} }

// motivoDe convierte el 409 de motivoCupon en el motivo del rojo.
func motivoDe(err error) string {
	var e *apperr.Error
	if errors.As(err, &e) {
		return e.Message
	}
	return MotivoVencido
}

// Validar es el escaneo del negocio. Primero marca la cartera `canjeado` con un
// FindOneAndUpdate condicional (de varios escaneos simultáneos solo uno gana) y después toma
// el uso del cupón con un `updateOne` condicional; si el cupón ya no tenía usos, la cartera
// vuelve a `en_canje` y el rojo es `agotado`. Un escaneo repetido por el mismo usuario del
// negocio en los 5 minutos siguientes recibe el mismo verde con `repetido: true` sin contar.
func (s *Service) Validar(ctx context.Context, cl *middleware.Claims, rawNegocio string, req ValidarRequest) (ValidarResponse, error) {
	b, me, err := s.negocioPropio(ctx, cl, rawNegocio)
	if err != nil {
		return ValidarResponse{}, err
	}
	codigo := NormalizarCodigo(req.Codigo)
	if !CodigoValido(codigo) {
		return rojo(MotivoNoExiste), nil
	}
	now := s.now().UTC().Truncate(time.Millisecond)
	w, err := s.repo.PorCodigo(ctx, codigo)
	if errors.Is(err, ErrNotFound) {
		return s.yaUsado(ctx, b.ID, me, codigo, now)
	}
	if err != nil {
		return ValidarResponse{}, errStore()
	}
	if motivo, err := s.motivoAntes(ctx, b.ID, w, now); err != nil || motivo != "" {
		return rojo(motivo), err
	}
	return s.canjear(ctx, b.ID, me, codigo, now)
}

// motivoAntes revisa el código vivo antes de canjear: negocio, expiración, vigencia, usos y
// horario. Vacío = se puede canjear.
func (s *Service) motivoAntes(ctx context.Context, negocio bson.ObjectID, w Cartera, now time.Time) (string, error) {
	if w.NegocioID != negocio {
		return MotivoOtroNegocio, nil
	}
	if w.CanjeExpiraEn == nil || !w.CanjeExpiraEn.After(now) {
		return MotivoVencido, nil
	}
	c, err := s.repo.FindByID(ctx, w.CuponID)
	if errors.Is(err, ErrNotFound) {
		return MotivoVencido, nil
	}
	if err != nil {
		return "", errStore()
	}
	if err := motivoCupon(c, now, s.loc); err != nil {
		return motivoDe(err), nil
	}
	return "", nil
}

// canjear hace los dos pasos atómicos y compensa el primero si el segundo no cabe.
func (s *Service) canjear(ctx context.Context, negocio, me bson.ObjectID, codigo string, now time.Time) (ValidarResponse, error) {
	w, err := s.repo.Canjear(ctx, negocio, codigo, me, now)
	if errors.Is(err, ErrNoAplica) {
		// Otro escaneo ganó a la vez: es el reintento o ya se usó.
		return s.yaUsado(ctx, negocio, me, codigo, now)
	}
	if err != nil {
		return ValidarResponse{}, errStore()
	}
	if err := s.repo.TomarUso(ctx, w.CuponID, now); err != nil {
		if errUndo := s.repo.DeshacerCanje(context.WithoutCancel(ctx), w, now); errUndo != nil {
			s.log.Error("cupones: no se deshizo el canje", "evento", "cupones_canje_compensacion_fallo", "error", errUndo.Error())
		}
		if !errors.Is(err, ErrNoAplica) {
			return ValidarResponse{}, errStore()
		}
		return rojo(s.motivoSinUso(ctx, w.CuponID, now)), nil
	}
	s.contar(ctx, w.CuponID, "", "canjes")
	c, err := s.repo.FindByID(ctx, w.CuponID)
	if err != nil {
		return ValidarResponse{Valido: true}, nil
	}
	if c.Estado == EstadoAgotado {
		s.recalcular(ctx, negocio)
	}
	r := publicoResponse(c)
	return ValidarResponse{Valido: true, Cupon: &r}, nil
}

// motivoSinUso explica por qué TomarUso no encontró el cupón (agotado o vencido).
func (s *Service) motivoSinUso(ctx context.Context, id bson.ObjectID, now time.Time) string {
	c, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return MotivoVencido
	}
	if err := motivoCupon(c, now, s.loc); err != nil {
		return motivoDe(err)
	}
	return MotivoAgotado
}

// yaUsado responde el código sin canje vivo: el reintento del mismo validador (verde
// repetido), otro negocio, ya usado o inexistente.
func (s *Service) yaUsado(ctx context.Context, negocio, me bson.ObjectID, codigo string, now time.Time) (ValidarResponse, error) {
	w, err := s.repo.PorCodigoUsado(ctx, codigo)
	if errors.Is(err, ErrNotFound) {
		return rojo(MotivoNoExiste), nil
	}
	if err != nil {
		return ValidarResponse{}, errStore()
	}
	if w.NegocioID != negocio {
		return rojo(MotivoOtroNegocio), nil
	}
	reciente := w.CanjeadoEn != nil && w.CanjeadoEn.After(now.Add(-VidaCodigo))
	if w.Estado != CarteraCanjeado || !reciente || w.ValidadoPor == nil || *w.ValidadoPor != me {
		return rojo(MotivoYaUsado), nil
	}
	c, err := s.repo.FindByID(ctx, w.CuponID)
	if err != nil {
		return ValidarResponse{Valido: true, Repetido: true}, nil
	}
	r := publicoResponse(c)
	return ValidarResponse{Valido: true, Cupon: &r, Repetido: true}, nil
}
