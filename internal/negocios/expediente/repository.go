package expediente

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ErrNoEncontrado: no hay registro vigente de ese tipo.
var ErrNoEncontrado = errors.New("expediente: no encontrado")

// Repository accede a `negocios_documentos` y `accesos_expediente`.
type Repository struct {
	docs    *mongo.Collection
	accesos *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{docs: db.Collection(Collection), accesos: db.Collection(AccesosCollection)}
}

// Reemplazar guarda el registro de (negocio, tipo) y devuelve el anterior, si había, para
// borrar su objeto. La versión sube con cada reemplazo.
func (r *Repository) Reemplazar(ctx context.Context, d Documento) (*Documento, error) {
	filter := bson.D{{Key: "negocio_id", Value: d.NegocioID}, {Key: "tipo", Value: d.Tipo}}
	var antes Documento
	err := r.docs.FindOne(ctx, filter).Decode(&antes)
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		d.Version = 1
		if d.ID.IsZero() {
			d.ID = bson.NewObjectID()
		}
		_, err = r.docs.InsertOne(ctx, d)
		return nil, err
	case err != nil:
		return nil, err
	}
	d.ID = antes.ID
	d.Version = antes.Version + 1
	res, err := r.docs.ReplaceOne(ctx, bson.D{{Key: "_id", Value: antes.ID}, {Key: "version", Value: antes.Version}}, d)
	if err != nil {
		return nil, err
	}
	if res.MatchedCount == 0 {
		return nil, ErrConcurrente
	}
	return &antes, nil
}

// ErrConcurrente: otra petición cambió el registro a la vez.
var ErrConcurrente = errors.New("expediente: cambio concurrente")

// Buscar trae el registro de (negocio, tipo) si no está borrado.
func (r *Repository) Buscar(ctx context.Context, negocio bson.ObjectID, tipo string) (Documento, error) {
	var d Documento
	err := r.docs.FindOne(ctx, bson.D{
		{Key: "negocio_id", Value: negocio}, {Key: "tipo", Value: tipo},
		{Key: "estado", Value: bson.D{{Key: "$ne", Value: EstadoBorrado}}},
	}).Decode(&d)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Documento{}, ErrNoEncontrado
	}
	return d, err
}

// Listar trae los registros no borrados del negocio.
func (r *Repository) Listar(ctx context.Context, negocio bson.ObjectID) ([]Documento, error) {
	return r.find(ctx, bson.D{{Key: "negocio_id", Value: negocio}, {Key: "estado", Value: bson.D{{Key: "$ne", Value: EstadoBorrado}}}})
}

// DePropietario trae los registros no borrados de la persona (ARCO).
func (r *Repository) DePropietario(ctx context.Context, owner bson.ObjectID) ([]Documento, error) {
	return r.find(ctx, bson.D{{Key: "propietario_id", Value: owner}, {Key: "estado", Value: bson.D{{Key: "$ne", Value: EstadoBorrado}}}})
}

func (r *Repository) find(ctx context.Context, filter bson.D) ([]Documento, error) {
	cur, err := r.docs.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "tipo", Value: 1}}).SetLimit(200))
	if err != nil {
		return nil, err
	}
	out := []Documento{}
	return out, cur.All(ctx, &out)
}

// Borrar deja el registro como `borrado` sin llave ni datos: el objeto ya se quitó de S3.
func (r *Repository) Borrar(ctx context.Context, id bson.ObjectID) error {
	_, err := r.docs.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{
		{Key: "$set", Value: bson.D{{Key: "estado", Value: EstadoBorrado}}},
		{Key: "$unset", Value: bson.D{{Key: "llave_s3", Value: ""}, {Key: "datos_cifrados", Value: ""}, {Key: "borrar_despues_de", Value: ""}, {Key: "conservar_hasta", Value: ""}}},
		{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
	})
	return err
}

// Bloquear deja los datos fiscales en solo lectura del administrador hasta `hasta`.
func (r *Repository) Bloquear(ctx context.Context, id bson.ObjectID, hasta time.Time) error {
	_, err := r.docs.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, bson.D{
		{Key: "$set", Value: bson.D{{Key: "estado", Value: EstadoBloqueado}, {Key: "conservar_hasta", Value: hasta}}},
		{Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}},
	})
	return err
}

// ProgramarBorrado fija `borrar_despues_de` de los tipos vigentes del negocio.
func (r *Repository) ProgramarBorrado(ctx context.Context, negocio bson.ObjectID, tipos []string, at time.Time) error {
	_, err := r.docs.UpdateMany(ctx, bson.D{
		{Key: "negocio_id", Value: negocio}, {Key: "tipo", Value: bson.D{{Key: "$in", Value: tipos}}},
		{Key: "estado", Value: EstadoVigente},
	}, bson.D{{Key: "$set", Value: bson.D{{Key: "borrar_despues_de", Value: at}}}, {Key: "$inc", Value: bson.D{{Key: "version", Value: 1}}}})
	return err
}

// Vencidos trae hasta limit registros cuyo plazo venció: vigentes con `borrar_despues_de` o
// bloqueados con `conservar_hasta` (ix_borrar e ix_conservar).
func (r *Repository) Vencidos(ctx context.Context, now time.Time, limit int64) ([]Documento, error) {
	out := []Documento{}
	for _, filter := range []bson.D{
		{{Key: "estado", Value: EstadoVigente}, {Key: "borrar_despues_de", Value: bson.D{{Key: "$lte", Value: now}}}},
		{{Key: "estado", Value: EstadoBloqueado}, {Key: "conservar_hasta", Value: bson.D{{Key: "$lte", Value: now}}}},
	} {
		cur, err := r.docs.Find(ctx, filter, options.Find().SetLimit(limit))
		if err != nil {
			return nil, err
		}
		var rows []Documento
		if err := cur.All(ctx, &rows); err != nil {
			return nil, err
		}
		out = append(out, rows...)
	}
	return out, nil
}

// RegistrarAcceso escribe la fila de la bitácora. Si falla, el archivo no se entrega.
func (r *Repository) RegistrarAcceso(ctx context.Context, a Acceso) error {
	if a.ID.IsZero() {
		a.ID = bson.NewObjectID()
	}
	_, err := r.accesos.InsertOne(ctx, a)
	return err
}

// Accesos trae las lecturas registradas de un negocio, de la más nueva a la más vieja.
func (r *Repository) Accesos(ctx context.Context, negocio bson.ObjectID) ([]Acceso, error) {
	cur, err := r.accesos.Find(ctx, bson.D{{Key: "negocio_id", Value: negocio}},
		options.Find().SetSort(bson.D{{Key: "leido_en", Value: -1}}).SetLimit(100))
	if err != nil {
		return nil, err
	}
	out := []Acceso{}
	return out, cur.All(ctx, &out)
}
