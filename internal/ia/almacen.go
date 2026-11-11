package ia

import (
	"context"
	"errors"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// ColeccionUsos registra cada llamada a la IA (constitución §5: español, plural).
const ColeccionUsos = "usos_ia"

// VidaUso es lo que se conserva un registro: un año y un mes, para comparar con la factura.
const VidaUso = 400 * 24 * time.Hour

// Llaves de Redis (constitución §6), siempre con TTL.
const (
	prefijoGasto = "cuota:ia_gasto:"
	prefijoCache = "cache:ia:"
)

// IndexesUsos son los índices de usos_ia: suma por mes, ARCO por usuario y vida por TTL.
func IndexesUsos() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: ColeccionUsos,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "mes", Value: 1}}, Options: options.Index().SetName("ix_mes")},
			{Keys: bson.D{{Key: "usuario_id", Value: 1}}, Options: options.Index().SetName("ix_usuario").
				SetPartialFilterExpression(bson.D{{Key: "usuario_id", Value: bson.D{{Key: "$exists", Value: true}}}})},
			{Keys: bson.D{{Key: "expira_en", Value: 1}}, Options: options.Index().SetName("ttl_uso").SetExpireAfterSeconds(0)},
		},
	}}
}

// RegistroMongo guarda los usos en MongoDB. Cumple Registro, privacidad.Purger y
// privacidad.Exporter.
type RegistroMongo struct {
	col *mongo.Collection
}

// NewRegistroMongo recibe la base.
func NewRegistroMongo(db *mongo.Database) *RegistroMongo {
	return &RegistroMongo{col: db.Collection(ColeccionUsos)}
}

// Guardar inserta un uso.
func (r *RegistroMongo) Guardar(ctx context.Context, u Uso) error {
	_, err := r.col.InsertOne(ctx, u)
	return err
}

// GastoDelMes suma el costo estimado de todas las llamadas del mes.
func (r *RegistroMongo) GastoDelMes(ctx context.Context, mes string) (int64, error) {
	cur, err := r.col.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "mes", Value: mes}}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: nil}, {Key: "total", Value: bson.D{{Key: "$sum", Value: "$costo_micro_usd"}}}}}},
	})
	if err != nil {
		return 0, err
	}
	var filas []struct {
		Total int64 `bson:"total"`
	}
	if err := cur.All(ctx, &filas); err != nil {
		return 0, err
	}
	if len(filas) == 0 {
		return 0, nil
	}
	return filas[0].Total, nil
}

// Name identifica al módulo en el registro ARCO y en el JSON exportado.
func (r *RegistroMongo) Name() string { return ColeccionUsos }

// PurgeHistory desliga los usos del usuario: el costo se conserva para el tope del mes, sin
// saber de quién fue.
func (r *RegistroMongo) PurgeHistory(ctx context.Context, userID bson.ObjectID) error {
	_, err := r.col.UpdateMany(ctx, bson.D{{Key: "usuario_id", Value: userID}},
		bson.D{{Key: "$unset", Value: bson.D{{Key: "usuario_id", Value: ""}}}})
	return err
}

// PurgeAccount hace lo mismo que PurgeHistory: en usos_ia no queda nada del usuario.
func (r *RegistroMongo) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return r.PurgeHistory(ctx, userID)
}

type usoExportado struct {
	Modelo        string    `json:"modelo"`
	TokensEntrada int       `json:"tokens_entrada"`
	TokensSalida  int       `json:"tokens_salida"`
	CostoMicroUSD int64     `json:"costo_micro_usd"`
	Resultado     string    `json:"resultado"`
	DuracionMS    int64     `json:"duracion_ms"`
	CreadoEn      time.Time `json:"creado_en"`
}

// Export devuelve los usos del usuario, del más reciente al más antiguo.
func (r *RegistroMongo) Export(ctx context.Context, userID bson.ObjectID) (any, error) {
	cur, err := r.col.Find(ctx, bson.D{{Key: "usuario_id", Value: userID}}, options.Find().SetSort(bson.D{{Key: "creado_en", Value: -1}}))
	if err != nil {
		return nil, err
	}
	var usos []Uso
	if err := cur.All(ctx, &usos); err != nil {
		return nil, err
	}
	out := make([]usoExportado, 0, len(usos))
	for _, u := range usos {
		out = append(out, usoExportado{Modelo: u.Modelo, TokensEntrada: u.TokensEntrada, TokensSalida: u.TokensSalida,
			CostoMicroUSD: u.CostoMicroUSD, Resultado: u.Resultado, DuracionMS: u.DuracionMS, CreadoEn: u.CreadoEn})
	}
	return out, nil
}

// ContadorRedis es el gasto del mes en `cuota:ia_gasto:{mes}`. Cumple Contador.
type ContadorRedis struct {
	rdb *goredis.Client
}

// NewContadorRedis recibe el cliente.
func NewContadorRedis(rdb *goredis.Client) *ContadorRedis { return &ContadorRedis{rdb: rdb} }

// Leer es un GET; ok es falso si la llave no existe.
func (c *ContadorRedis) Leer(ctx context.Context, mes string) (int64, bool, error) {
	v, err := c.rdb.Get(ctx, prefijoGasto+mes).Int64()
	if errors.Is(err, goredis.Nil) {
		return 0, false, nil
	}
	return v, err == nil, err
}

// Fijar escribe el valor reconstruido solo si nadie lo escribió antes (SET NX).
func (c *ContadorRedis) Fijar(ctx context.Context, mes string, micro int64, vida time.Duration) error {
	return c.rdb.SetNX(ctx, prefijoGasto+mes, micro, vida).Err()
}

// Sumar es un INCRBY; solo la primera suma del mes, la que crea la llave, pone el TTL.
func (c *ContadorRedis) Sumar(ctx context.Context, mes string, micro int64, vida time.Duration) error {
	total, err := c.rdb.IncrBy(ctx, prefijoGasto+mes, micro).Result()
	if err != nil || total != micro {
		return err
	}
	return c.rdb.Expire(ctx, prefijoGasto+mes, vida).Err()
}

// CacheRedis guarda las salidas en `cache:ia:{huella}`. Cumple Cache.
type CacheRedis struct {
	rdb *goredis.Client
}

// NewCacheRedis recibe el cliente.
func NewCacheRedis(rdb *goredis.Client) *CacheRedis { return &CacheRedis{rdb: rdb} }

// Leer es un GET; ok es falso si no hay salida guardada.
func (c *CacheRedis) Leer(ctx context.Context, huella string) ([]byte, bool, error) {
	v, err := c.rdb.Get(ctx, prefijoCache+huella).Bytes()
	if errors.Is(err, goredis.Nil) {
		return nil, false, nil
	}
	return v, err == nil, err
}

// Guardar es un SET con vida.
func (c *CacheRedis) Guardar(ctx context.Context, huella string, valor []byte, vida time.Duration) error {
	return c.rdb.Set(ctx, prefijoCache+huella, valor, vida).Err()
}
