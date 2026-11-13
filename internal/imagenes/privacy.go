package imagenes

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Privacy es la parte de imágenes en ARCO (nv-05): cumple privacidad.Purger y
// privacidad.Exporter. Sin almacén (imágenes apagadas) solo borra las filas.
type Privacy struct {
	repo    *Repository
	almacen Almacen
	base    string
}

// NewPrivacy recibe el repositorio, el almacén (puede ser nil) y la URL pública.
func NewPrivacy(repo *Repository, almacen Almacen, urlPublica string) *Privacy {
	return &Privacy{repo: repo, almacen: almacen, base: urlPublica}
}

// Name identifica al módulo en el registro y en el JSON exportado.
func (p *Privacy) Name() string { return Collection }

// PurgeHistory no toca las fotos: son contenido de la persona, no historial de uso.
func (p *Privacy) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra cada imagen en S3 y luego las filas; si S3 falla, el trabajo nocturno
// reintenta con las filas intactas.
func (p *Privacy) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	imgs, err := p.repo.ListarPorDueno(ctx, userID)
	if err != nil {
		return err
	}
	if p.almacen != nil {
		for _, img := range imgs {
			if err := p.almacen.Borrar(ctx, claves(img.ID)...); err != nil {
				return err
			}
		}
	}
	return p.repo.EliminarPorDueno(ctx, userID)
}

type exportada struct {
	ImagenResponse
	CreadaEn time.Time `json:"creada_en"`
}

// Export devuelve cada imagen con su estado y, si está publicada, sus direcciones.
func (p *Privacy) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	imgs, err := p.repo.ListarPorDueno(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]exportada, 0, len(imgs))
	for i := range imgs {
		out = append(out, exportada{ImagenResponse: *nuevaRespuesta(&imgs[i], p.base), CreadaEn: imgs[i].CreadaEn})
	}
	return out, nil
}
