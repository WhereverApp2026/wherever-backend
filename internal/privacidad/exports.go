package privacidad

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// ExportsCollection guarda las solicitudes de "Descargar mis datos" (spec nv-05, D-2).
const ExportsCollection = "exportaciones"

// Estados de una exportación.
const (
	ExportPending    = "pendiente"
	ExportReady      = "lista"
	ExportDownloaded = "descargada"
)

// Plazos de una exportación (nv-06): una por día; la pendiente se limpia sola a los 7 días;
// la lista vive como máximo 72 h desde que se armó, aunque el aviso no haya salido, y las
// 24 h de descarga cuentan desde que salió el aviso.
const (
	ExportCooldown   = 24 * time.Hour
	pendingRetention = 7 * 24 * time.Hour
	ReadyMaxAge      = 72 * time.Hour
	DownloadWindow   = 24 * time.Hour
)

// ErrNoExport indica que no hay exportación lista.
var ErrNoExport = errors.New("sin exportación lista")

// Export es un documento de `exportaciones`. `datos_json` es el archivo ya armado; se borra al
// descargarlo y el documento queda solo como registro para el límite de una por día.
// Límite conocido: un documento de MongoDB no pasa de 16 MB (ver Memoria.md).
type Export struct {
	ID           bson.ObjectID `bson:"_id"`
	UserID       bson.ObjectID `bson:"usuario_id"`
	Status       string        `bson:"estado"`
	Data         string        `bson:"datos_json,omitempty"`
	CreatedAt    time.Time     `bson:"creado_en"`
	ReadyAt      *time.Time    `bson:"lista_en,omitempty"`
	DownloadedAt *time.Time    `bson:"descargada_en,omitempty"`
	NoticeSentAt *time.Time    `bson:"aviso_enviado_en,omitempty"`
	ExpiresAt    time.Time     `bson:"expira_en"`
}

// Exports accede a `exportaciones`.
type Exports struct {
	col *mongo.Collection
}

// NewExports recibe la base por constructor.
func NewExports(db *mongo.Database) *Exports {
	return &Exports{col: db.Collection(ExportsCollection)}
}

// Indexes son los índices de `exportaciones` (colección nueva de nv-05; falta en el Excel).
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{{
		Collection: ExportsCollection,
		Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "estado", Value: 1}, {Key: "creado_en", Value: 1}}, Options: options.Index().SetName("ix_exportacion_estado")},
			{Keys: bson.D{{Key: "usuario_id", Value: 1}, {Key: "creado_en", Value: -1}}, Options: options.Index().SetName("ix_usuario_exportaciones")},
			{Keys: bson.D{{Key: "expira_en", Value: 1}}, Options: options.Index().SetName("ttl_exportacion").SetExpireAfterSeconds(0)},
		},
	}}
}

// LastRequestedAt dice cuándo pidió el usuario su última exportación; ok=false si nunca.
func (e *Exports) LastRequestedAt(ctx context.Context, userID bson.ObjectID) (time.Time, bool, error) {
	var doc Export
	err := e.col.FindOne(ctx, bson.D{{Key: "usuario_id", Value: userID}},
		options.FindOne().SetSort(bson.D{{Key: "creado_en", Value: -1}})).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, false, err
	}
	return doc.CreatedAt, true, nil
}

// Request crea una exportación pendiente; la arma el trabajo nocturno.
func (e *Exports) Request(ctx context.Context, userID bson.ObjectID, now time.Time) error {
	_, err := e.col.InsertOne(ctx, Export{
		ID: bson.NewObjectID(), UserID: userID, Status: ExportPending, CreatedAt: now, ExpiresAt: now.Add(pendingRetention),
	})
	return err
}

// Pending devuelve hasta `limit` exportaciones pendientes, las más viejas primero.
func (e *Exports) Pending(ctx context.Context, limit int64) ([]Export, error) {
	cur, err := e.col.Find(ctx, bson.D{{Key: "estado", Value: ExportPending}},
		options.Find().SetSort(bson.D{{Key: "creado_en", Value: 1}}).SetLimit(limit))
	if err != nil {
		return nil, err
	}
	var out []Export
	return out, cur.All(ctx, &out)
}

// MarkReady guarda el archivo; vence a las 72 h aunque el aviso no salga (el TTL la borra).
func (e *Exports) MarkReady(ctx context.Context, id bson.ObjectID, data string, now time.Time) error {
	_, err := e.col.UpdateByID(ctx, id, bson.D{{Key: "$set", Value: bson.D{
		{Key: "estado", Value: ExportReady}, {Key: "datos_json", Value: data},
		{Key: "lista_en", Value: now}, {Key: "expira_en", Value: now.Add(ReadyMaxAge)},
	}}})
	return err
}

// AwaitingNotice devuelve las exportaciones listas, sin vencer, cuyo aviso no ha salido.
func (e *Exports) AwaitingNotice(ctx context.Context, now time.Time, limit int64) ([]Export, error) {
	cur, err := e.col.Find(ctx, bson.D{
		{Key: "estado", Value: ExportReady},
		{Key: "aviso_enviado_en", Value: nil},
		{Key: "expira_en", Value: bson.D{{Key: "$gt", Value: now}}},
	}, options.Find().SetSort(bson.D{{Key: "lista_en", Value: 1}}).SetLimit(limit))
	if err != nil {
		return nil, err
	}
	var out []Export
	return out, cur.All(ctx, &out)
}

// MarkNotified anota el aviso: desde ahí corren las 24 h de descarga, sin pasar de las 72 h
// desde que se armó.
func (e *Exports) MarkNotified(ctx context.Context, exp Export, now time.Time) error {
	expires := now.Add(DownloadWindow)
	if exp.ReadyAt != nil && exp.ReadyAt.Add(ReadyMaxAge).Before(expires) {
		expires = exp.ReadyAt.Add(ReadyMaxAge)
	}
	_, err := e.col.UpdateByID(ctx, exp.ID, bson.D{{Key: "$set", Value: bson.D{
		{Key: "aviso_enviado_en", Value: now}, {Key: "expira_en", Value: expires},
	}}})
	return err
}

// TakeReady entrega la exportación lista más reciente y, en la misma operación, borra su
// archivo y la marca como descargada: los datos personales no se quedan las 24 h.
func (e *Exports) TakeReady(ctx context.Context, userID bson.ObjectID, now time.Time) (Export, error) {
	var doc Export
	err := e.col.FindOneAndUpdate(ctx,
		bson.D{{Key: "usuario_id", Value: userID}, {Key: "estado", Value: ExportReady}, {Key: "expira_en", Value: bson.D{{Key: "$gt", Value: now}}}},
		bson.D{
			{Key: "$set", Value: bson.D{{Key: "estado", Value: ExportDownloaded}, {Key: "descargada_en", Value: now}}},
			{Key: "$unset", Value: bson.D{{Key: "datos_json", Value: ""}}},
		},
		options.FindOneAndUpdate().SetSort(bson.D{{Key: "lista_en", Value: -1}}).SetReturnDocument(options.Before)).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Export{}, ErrNoExport
	}
	return doc, err
}

// DeleteByUser borra las exportaciones del usuario (Purger de esta colección).
func (e *Exports) DeleteByUser(ctx context.Context, userID bson.ObjectID) error {
	_, err := e.col.DeleteMany(ctx, bson.D{{Key: "usuario_id", Value: userID}})
	return err
}

// Name identifica a `exportaciones` en el registro.
func (e *Exports) Name() string { return ExportsCollection }

// PurgeHistory no borra nada: una exportación no es historial de uso.
func (e *Exports) PurgeHistory(context.Context, bson.ObjectID) error { return nil }

// PurgeAccount borra las exportaciones del usuario.
func (e *Exports) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return e.DeleteByUser(ctx, userID)
}
