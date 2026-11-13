package imagenes

import (
	"context"
	"errors"
	"log/slog"

	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
)

// TipoCola es el tipo con el que las fotos entran a la cola de revisión de nv-43.
const TipoCola = "imagen"

// Cola es lo que el service usa de la cola de revisión (nv-43).
type Cola interface {
	Encolar(ctx context.Context, e moderacion.Entrada) (moderacion.Item, error)
}

// WithCola manda a revisión lo que la moderación deja en cuarentena con veredicto (dudosa).
// Una falla del proveedor no se encola: la persona vuelve a confirmar y se modera de nuevo.
func (s *Service) WithCola(c Cola) *Service {
	s.cola = c
	return s
}

// encolar no falla la petición: si la cola no responde, la foto sigue en revisión sin
// publicarse y el original vence en cuarentena/ a 1 día.
func (s *Service) encolar(ctx context.Context, img *Imagen) {
	if s.cola == nil || img.Moderacion == nil || img.Moderacion.Falla != "" {
		return
	}
	_, err := s.cola.Encolar(context.WithoutCancel(ctx), moderacion.Entrada{
		Tipo: TipoCola, Referencia: img.ID, AutorID: img.DuenoID, ImagenClave: claveOriginal(img.ID),
		Resultado: moderacion.Resultado{Veredicto: moderacion.Veredicto(img.Moderacion.Veredicto), Puntajes: img.Moderacion.Puntajes},
	})
	if err != nil {
		s.log.WarnContext(ctx, "imagenes: no se pudo encolar para revisión", slog.String("evento", "imagen_cola_fallo"),
			slog.String("imagen", img.ID), slog.String("error", err.Error()))
	}
}

// Destino aplica la decisión del administrador (nv-43) a una foto en revisión: aprobada se
// recodifica y se publica; rechazada se borra el original.
type Destino struct{ s *Service }

// Destino devuelve el que se registra con moderacion.Service.WithDestino(TipoCola, ...).
func (s *Service) Destino() Destino { return Destino{s: s} }

// AlDecidir cumple moderacion.Destino. Si la foto ya no está en revisión (la borró su dueño o
// ya se decidió) no hace nada.
func (d Destino) AlDecidir(ctx context.Context, it moderacion.Item) error {
	s := d.s
	img, err := s.repo.TomarRevision(ctx, it.Referencia, it.AutorID, s.ahora().UTC())
	if errors.Is(err, ErrNoTomable) {
		return nil
	}
	if err != nil {
		return err
	}
	if it.Estado != moderacion.EstadoAprobado {
		s.borrar(ctx, claveOriginal(img.ID))
		img.Estado, img.Motivo = EstadoRechazada, MotivoModeracion
		return s.terminar(ctx, img)
	}
	datos, err := s.almacen.Leer(ctx, claveOriginal(img.ID), s.cfg.MaxBytes)
	if err != nil {
		s.devolver(ctx, img)
		return err // sin original (venció a 1 día) o S3 caído: sigue en revisión, sin publicarse
	}
	proc, err := s.procesar(ctx, datos, img.Tipo)
	if err == nil {
		err = s.publicar(ctx, img.ID, proc)
	}
	if err != nil {
		s.devolver(ctx, img)
		return err
	}
	s.borrar(ctx, claveOriginal(img.ID))
	img.Estado = EstadoPublicada
	return s.terminar(ctx, img)
}

// devolver deja la foto otra vez en revisión cuando no se pudo aplicar la aprobación.
func (s *Service) devolver(ctx context.Context, img *Imagen) {
	img.Estado, img.ActualizadaEn = EstadoEnRevision, s.ahora().UTC()
	if _, err := s.repo.Terminar(context.WithoutCancel(ctx), img); err != nil {
		s.log.ErrorContext(ctx, "imagenes: no se pudo devolver a revisión", slog.String("error", err.Error()))
	}
}
