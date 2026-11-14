package postales

import (
	"context"
	"errors"
	"log/slog"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
)

// Destino aplica la decisión del administrador (nv-43) a los textos de postales y momentos.
type Destino struct{ s *Service }

// Destino devuelve el que se registra con WithDestino(TipoColaPostal y TipoColaMomento, ...).
func (s *Service) Destino() Destino { return Destino{s: s} }

// AlDecidir cumple moderacion.Destino. Lo ya decidido o borrado no cambia.
func (d Destino) AlDecidir(ctx context.Context, it moderacion.Item) error {
	s := d.s
	id, ok := parseID(it.Referencia)
	if !ok {
		return nil
	}
	aprobado := it.Estado == moderacion.EstadoAprobado
	now := s.ahora()
	switch it.Tipo {
	case TipoColaPostal:
		texto := TextoRechazado
		if aprobado {
			texto = TextoAprobado
		}
		ps, err := s.Repo.ModerarPostal(ctx, bson.D{{Key: "_id", Value: id}, {Key: "texto_estado", Value: TextoEnRevision}},
			bson.D{{Key: "texto_estado", Value: texto}})
		if err != nil {
			return err
		}
		return s.recalcular(ctx, ps)
	case TipoColaMomento:
		estado := MomentoRechazado
		if aprobado {
			estado = MomentoPublicado
		}
		if err := s.Repo.ModerarMomento(ctx, id, estado, now); err != nil {
			return err
		}
		if aprobado {
			if m, err := s.Repo.Momento(ctx, id); err == nil {
				s.avisarEtiquetados(ctx, m, nil)
			}
		}
	}
	return nil
}

// recalcular guarda el estado de cada postal tras un cambio de imagen o de texto. El filtro de
// ModerarPostal ya trae el valor nuevo; solo se guardan las que cambian de estado.
func (s *Service) recalcular(ctx context.Context, ps []Postal) error {
	now := s.ahora()
	for _, p := range ps {
		estado := EstadoPostal(p.ImagenEstado, p.TextoEstado)
		if estado == p.Estado {
			continue
		}
		if err := s.Repo.FijarEstado(ctx, p.ID, estado, now); err != nil {
			return err
		}
		s.Log.InfoContext(ctx, "postal moderada", slog.String("evento", "postal_moderada"), slog.String("id", p.ID.Hex()), slog.String("estado", estado))
	}
	return nil
}

// DestinoImagen envuelve el destino de imágenes de nv-45: primero publica o descarta la foto y
// luego pasa el nuevo estado a las postales que la usan.
type DestinoImagen struct {
	base moderacion.Destino
	s    *Service
}

// DestinoImagen devuelve el que se registra con WithDestino(imagenes.TipoCola, ...).
func (s *Service) DestinoImagen(base moderacion.Destino) DestinoImagen {
	return DestinoImagen{base: base, s: s}
}

// AlDecidir cumple moderacion.Destino.
func (d DestinoImagen) AlDecidir(ctx context.Context, it moderacion.Item) error {
	if d.base != nil {
		if err := d.base.AlDecidir(ctx, it); err != nil {
			return err
		}
	}
	return d.s.SincronizarImagen(ctx, it.AutorID, it.Referencia)
}

// SincronizarImagen copia a las postales el estado y la URL pública de su imagen.
func (s *Service) SincronizarImagen(ctx context.Context, dueno bson.ObjectID, imagenID string) error {
	if s.imagenes == nil {
		return nil
	}
	v, err := s.imagenes.Consultar(ctx, dueno, imagenID)
	if errors.Is(err, imagenes.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	ps, err := s.Repo.ModerarPostal(ctx, bson.D{{Key: "imagen_id", Value: imagenID}, {Key: "usuario_id", Value: dueno}},
		bson.D{{Key: "imagen_estado", Value: v.Estado}, {Key: "imagen_url", Value: v.URL}})
	if err != nil {
		return err
	}
	return s.recalcular(ctx, ps)
}
