package insignias

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Collection guarda cuándo se ganó cada insignia.
const Collection = "insignias_usuarios"

// Colecciones que se cuentan. Son las de cada módulo (planes, resenas, postales, amistades);
// aquí van como constantes porque postales y amistades dependen de personas, que usa este
// paquete. TestInsignias_ColeccionesIgualesALosModulos (Integration) evita que se desalineen.
const (
	ColPlanes    = "planes"
	ColResenas   = "resenas"
	ColCheckins  = "checkins"
	ColPostales  = "postales"
	ColAmistades = "amistades"
)

// Estados que cuentan en cada colección.
const (
	planActivo      = "activo"
	resenaPublicada = "publicada"
	postalLista     = "lista"
	amistadAceptada = "aceptada"
)

// Registro es un documento de `insignias_usuarios`.
type Registro struct {
	ID        bson.ObjectID `bson:"_id"`
	UsuarioID bson.ObjectID `bson:"usuario_id"`
	Insignia  string        `bson:"insignia"`
	GanadaEn  time.Time     `bson:"ganada_en"`
}

// Indexes: una insignia por persona.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{Collection: Collection, Models: []mongo.IndexModel{
		{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "insignia", Value: 1}}, Options: options.Index().SetName("ux_insignia_usuario").SetUnique(true)},
	}}}
}

// Repository lee los conteos y guarda las insignias ganadas.
type Repository struct {
	db *mongo.Database
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository { return &Repository{db: db} }

// Conteos cuenta lo que la persona ya hizo, por fuente. Cada conteo usa el índice por
// persona de su colección.
func (r *Repository) Conteos(ctx context.Context, userID bson.ObjectID) (map[string]int, error) {
	out := map[string]int{}
	cuentas := []struct {
		fuente, col string
		filtro      bson.D
	}{
		{FuentePlanes, ColPlanes, bson.D{{Key: "dueno_id", Value: userID}, {Key: "estado", Value: planActivo}}},
		{FuenteResenas, ColResenas, bson.D{{Key: "usuario_id", Value: userID}, {Key: "estado", Value: resenaPublicada}}},
		{FuentePostales, ColPostales, bson.D{{Key: "usuario_id", Value: userID}, {Key: "estado", Value: postalLista}}},
		{FuenteAmigos, ColAmistades, bson.D{{Key: "estado", Value: amistadAceptada}, {Key: "$or", Value: bson.A{
			bson.D{{Key: "de", Value: userID}}, bson.D{{Key: "para", Value: userID}},
		}}}},
	}
	for _, c := range cuentas {
		n, err := r.db.Collection(c.col).CountDocuments(ctx, c.filtro)
		if err != nil {
			return nil, err
		}
		out[c.fuente] = int(n)
	}
	lugares, err := r.db.Collection(ColCheckins).Distinct(ctx, "negocio_id", bson.D{{Key: "usuario_id", Value: userID}, {Key: "valido", Value: true}}).Raw()
	if err != nil {
		return nil, err
	}
	valores, err := lugares.Values()
	if err != nil {
		return nil, err
	}
	out[FuenteLugares] = len(valores)
	return out, nil
}

// Ganadas devuelve la fecha de cada insignia ya guardada.
func (r *Repository) Ganadas(ctx context.Context, userID bson.ObjectID) (map[string]time.Time, error) {
	cur, err := r.db.Collection(Collection).Find(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	if err != nil {
		return nil, err
	}
	var regs []Registro
	if err := cur.All(ctx, &regs); err != nil {
		return nil, err
	}
	out := make(map[string]time.Time, len(regs))
	for _, g := range regs {
		out[g.Insignia] = g.GanadaEn
	}
	return out, nil
}

// Guardar registra una insignia recién ganada. Si otra petición ya la guardó, gana la
// primera fecha (índice único): no es error.
func (r *Repository) Guardar(ctx context.Context, userID bson.ObjectID, insignia string, ahora time.Time) error {
	_, err := r.db.Collection(Collection).InsertOne(ctx, Registro{ID: bson.NewObjectID(), UsuarioID: userID, Insignia: insignia, GanadaEn: ahora})
	if err != nil && !mongo.IsDuplicateKeyError(err) {
		return err
	}
	return nil
}

// BorrarDe borra las insignias de la persona (ARCO).
func (r *Repository) BorrarDe(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.db.Collection(Collection).DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}
