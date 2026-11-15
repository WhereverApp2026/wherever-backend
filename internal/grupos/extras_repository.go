package grupos

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// ErrStale: la versión de If-Match ya no es la del grupo (nv-69).
var ErrStale = errors.New("el grupo cambió")

// lit protege un valor del usuario dentro de un pipeline: sin `$literal`, un texto que
// empieza con `$` se leería como un campo.
func lit(v any) bson.D { return bson.D{{Key: "$literal", Value: v}} }

// bump es `version + 1` contando como 1 la versión que falta (grupos de antes de nv-69).
func bump() bson.D {
	return bson.D{{Key: "$add", Value: bson.A{bson.D{{Key: "$ifNull", Value: bson.A{"$version", 1}}}, 1}}}
}

// versionIs filtra por la versión de If-Match; la 1 también acepta los grupos sin campo.
func versionIs(v int) bson.E {
	if v == 1 {
		return bson.E{Key: "$or", Value: bson.A{
			bson.D{{Key: "version", Value: 1}}, bson.D{{Key: "version", Value: bson.D{{Key: "$exists", Value: false}}}},
		}}
	}
	return bson.E{Key: "version", Value: v}
}

// pipelineSet aplica `fields` (ya protegidos), sube la versión y la fecha en una sola
// operación condicionada.
func (r *Repository) pipelineSet(ctx context.Context, filter bson.D, fields bson.D, now time.Time) (bool, error) {
	set := append(fields, bson.E{Key: "version", Value: bump()}, bson.E{Key: "actualizado_en", Value: lit(now)})
	res, err := r.col.UpdateOne(ctx, filter, bson.A{bson.D{{Key: "$set", Value: set}}})
	if err != nil {
		return false, err
	}
	return res.MatchedCount == 1, nil
}

// staleOr distingue versión vieja de grupo que ya no existe.
func (r *Repository) staleOr(ctx context.Context, id bson.ObjectID, ok bool, err error) error {
	if err != nil || ok {
		return err
	}
	if _, findErr := r.FindByID(ctx, id); findErr == nil {
		return ErrStale
	}
	return ErrNotFound
}

// Rename cambia el nombre del grupo con If-Match (solo el organizador).
func (r *Repository) Rename(ctx context.Context, id, organizer bson.ObjectID, version int, name string, now time.Time) error {
	ok, err := r.pipelineSet(ctx, bson.D{{Key: "_id", Value: id}, {Key: "organizador_id", Value: organizer}, alive(), versionIs(version)},
		bson.D{{Key: "nombre", Value: lit(name)}}, now)
	return r.staleOr(ctx, id, ok, err)
}

// Dissolve hace el borrado lógico con If-Match y quita el enlace: los integrantes pierden el
// acceso al plan, que sigue siendo del dueño.
func (r *Repository) Dissolve(ctx context.Context, id, organizer bson.ObjectID, version int, now time.Time) error {
	filter := bson.D{{Key: "_id", Value: id}, {Key: "organizador_id", Value: organizer}, alive(), versionIs(version)}
	res, err := r.col.UpdateOne(ctx, filter, bson.A{
		bson.D{{Key: "$set", Value: bson.D{{Key: "eliminado_en", Value: lit(now)}, {Key: "version", Value: bump()}, {Key: "actualizado_en", Value: lit(now)}}}},
		bson.D{{Key: "$unset", Value: bson.A{"codigo_hash", "codigo_vence"}}},
	})
	if err != nil {
		return err
	}
	return r.staleOr(ctx, id, res.MatchedCount == 1, nil)
}

// DropDissolved borra el documento del grupo disuelto de un plan para que el plan pueda tener
// otro grupo: el nuevo nace con otro id, así nadie hereda el chat, los votos ni La Vaca del
// grupo anterior (nv-69). `ux_plan` sigue resolviendo la carrera al crear.
func (r *Repository) DropDissolved(ctx context.Context, planID bson.ObjectID) error {
	_, err := r.col.DeleteOne(ctx, bson.D{{Key: "plan_id", Value: planID}, {Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: true}}}})
	return err
}

// DissolvedIDs son los grupos disueltos; el trabajo nocturno borra su chat.
func (r *Repository) DissolvedIDs(ctx context.Context) ([]bson.ObjectID, error) {
	cur, err := r.col.Find(ctx, bson.D{{Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: true}}}},
		options.Find().SetProjection(bson.D{{Key: "_id", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]bson.ObjectID, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ID)
	}
	return out, nil
}

// SetMeetingPoint guarda el punto de encuentro (el último gana) y sube la versión.
func (r *Repository) SetMeetingPoint(ctx context.Context, id bson.ObjectID, mp MeetingPoint, now time.Time) error {
	ok, err := r.pipelineSet(ctx, bson.D{{Key: "_id", Value: id}, alive()}, bson.D{{Key: "punto_encuentro", Value: lit(mp)}}, now)
	if err == nil && !ok {
		return ErrNotFound
	}
	return err
}

// SetSurprise guarda (o apaga) el modo sorpresa; solo el organizador.
func (r *Repository) SetSurprise(ctx context.Context, id, organizer bson.ObjectID, s Surprise, now time.Time) error {
	ok, err := r.pipelineSet(ctx, bson.D{{Key: "_id", Value: id}, {Key: "organizador_id", Value: organizer}, alive()},
		bson.D{{Key: "sorpresa", Value: lit(s)}}, now)
	if err == nil && !ok {
		return ErrNotFound
	}
	return err
}

// ShareLocation guarda la ubicación ya redondeada de un integrante con su vencimiento; con
// at == nil la retira.
func (r *Repository) ShareLocation(ctx context.Context, id, userID bson.ObjectID, at *Coords, expires time.Time) error {
	filter := bson.D{{Key: "_id", Value: id}, alive(), {Key: "integrantes.usuario_id", Value: userID}}
	update := bson.D{{Key: "$unset", Value: bson.D{{Key: "integrantes.$.ubicacion_compartida", Value: ""}, {Key: "integrantes.$.ubicacion_vence", Value: ""}}}}
	if at != nil {
		update = bson.D{{Key: "$set", Value: bson.D{{Key: "integrantes.$.ubicacion_compartida", Value: *at}, {Key: "integrantes.$.ubicacion_vence", Value: expires}}}}
	}
	_, err := r.col.UpdateOne(ctx, filter, update)
	return err
}

// PurgeExpiredLocations borra las ubicaciones compartidas vencidas de todos los grupos.
func (r *Repository) PurgeExpiredLocations(ctx context.Context, now time.Time) (int64, error) {
	res, err := r.col.UpdateMany(ctx,
		bson.D{{Key: "integrantes.ubicacion_vence", Value: bson.D{{Key: "$lte", Value: now}}}},
		bson.D{{Key: "$unset", Value: bson.D{{Key: "integrantes.$[m].ubicacion_compartida", Value: ""}, {Key: "integrantes.$[m].ubicacion_vence", Value: ""}}}},
		options.UpdateMany().SetArrayFilters([]any{bson.D{{Key: "m.ubicacion_vence", Value: bson.D{{Key: "$lte", Value: now}}}}}))
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}
