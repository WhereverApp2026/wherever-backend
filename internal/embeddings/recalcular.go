package embeddings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
)

// Almacen es lo que el recálculo necesita de MongoDB (Repository lo cumple).
type Almacen interface {
	Fuentes(ctx context.Context) ([]Fuente, error)
	Versiones(ctx context.Context) (map[bson.ObjectID]string, error)
	Guardar(ctx context.Context, v Vector) error
	BorrarExcepto(ctx context.Context, vigentes []bson.ObjectID) (int64, error)
}

// Reporte son solo conteos: ni nombres ni ids.
type Reporte struct {
	Visibles   int
	Calculados int
	Borrados   int64
}

// Recalculador rehace los vectores desactualizados. Es idempotente: un negocio cuya versión
// (plantilla, modelo y texto) no cambió no se vuelve a mandar al proveedor.
type Recalculador struct {
	almacen Almacen
	vec     ia.Vectorizador
	lote    int
	ahora   func() time.Time
}

// NewRecalculador recibe el almacén, el vectorizador (ya con límites) y el tamaño de lote, que
// no pasa de ia.MaxLote ni del límite por minuto.
func NewRecalculador(almacen Almacen, vec ia.Vectorizador, lote int) (*Recalculador, error) {
	if almacen == nil || vec == nil {
		return nil, errors.New("embeddings: el recálculo necesita almacén y vectorizador")
	}
	if lote < 1 {
		return nil, errors.New("embeddings: el lote debe ser positivo")
	}
	return &Recalculador{almacen: almacen, vec: vec, lote: min(lote, ia.MaxLote), ahora: time.Now}, nil
}

type pendiente struct {
	id      bson.ObjectID
	doc     ia.Documento
	version string
}

// Recalcular vectoriza los negocios visibles sin vector o con versión vieja (todos si todos es
// true) y borra los vectores de negocios que dejaron de ser visibles. Si el proveedor falla a
// la mitad, lo ya guardado se queda y el siguiente recálculo sigue donde quedó.
func (r *Recalculador) Recalcular(ctx context.Context, todos bool) (Reporte, error) {
	fuentes, err := r.almacen.Fuentes(ctx)
	if err != nil {
		return Reporte{}, fmt.Errorf("embeddings: leer negocios: %w", err)
	}
	versiones, err := r.almacen.Versiones(ctx)
	if err != nil {
		return Reporte{}, fmt.Errorf("embeddings: leer versiones: %w", err)
	}
	rep := Reporte{Visibles: len(fuentes)}
	modelo := r.vec.Nombre()
	vigentes := make([]bson.ObjectID, 0, len(fuentes))
	var pendientes []pendiente
	for _, f := range fuentes {
		vigentes = append(vigentes, f.ID)
		doc := Documento(f)
		v := Version(modelo, doc)
		if todos || versiones[f.ID] != v {
			pendientes = append(pendientes, pendiente{id: f.ID, doc: doc, version: v})
		}
	}
	borrados, err := r.almacen.BorrarExcepto(ctx, vigentes)
	if err != nil {
		return rep, fmt.Errorf("embeddings: borrar vectores sin negocio: %w", err)
	}
	rep.Borrados = borrados
	for inicio := 0; inicio < len(pendientes); inicio += r.lote {
		n, err := r.calcularLote(ctx, pendientes[inicio:min(inicio+r.lote, len(pendientes))], modelo)
		rep.Calculados += n
		if err != nil {
			return rep, err
		}
	}
	return rep, nil
}

func (r *Recalculador) calcularLote(ctx context.Context, lote []pendiente, modelo string) (int, error) {
	docs := make([]ia.Documento, len(lote))
	for i, p := range lote {
		docs[i] = p.doc
	}
	vectores, err := r.vec.Documentos(ctx, docs)
	if err != nil {
		return 0, fmt.Errorf("embeddings: vectorizar: %w", err)
	}
	if len(vectores) != len(lote) {
		return 0, fmt.Errorf("embeddings: %w", ia.ErrVectorInvalido)
	}
	ahora := r.ahora().UTC()
	for i, p := range lote {
		v := Vector{NegocioID: p.id, Vector: vectores[i], Version: p.version, Modelo: modelo, ActualizadoEn: ahora}
		if err := r.almacen.Guardar(ctx, v); err != nil {
			return i, fmt.Errorf("embeddings: guardar: %w", err)
		}
	}
	return len(lote), nil
}

// Refrescar recalcula solo los desactualizados; es lo que llama la importación de negocios.
func (r *Recalculador) Refrescar(ctx context.Context) error {
	_, err := r.Recalcular(ctx, false)
	return err
}
