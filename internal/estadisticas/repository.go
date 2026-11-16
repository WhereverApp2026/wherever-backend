package estadisticas

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	platformmongo "github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
)

// Colecciones de otros módulos que el trabajo solo lee. Van por nombre para no importar sus
// paquetes de servicio; `fuentes_test.go` comprueba que sigan iguales.
const (
	colNegocios        = "negocios"
	colReservas        = "reservas"
	colCheckins        = "checkins"
	colResenas         = "resenas"
	colCupones         = "cupones"
	colCuponesUsuario  = "cupones_usuario"
	colCuponesMetricas = "cupones_metricas"
	colFavoritos       = "favoritos"
)

// Valores de otros módulos que filtran las agregaciones.
const (
	reservaBloqueada  = "bloqueada"
	reservaExpirada   = "expirada"
	resenaPublicada   = "publicada"
	favoritoNegocio   = "negocio"
	negocioAprobado   = "aprobado"
	negocioPausado    = "pausado"
	formatoDiaMongo   = "%Y-%m-%d"
	camposPorHora     = 24
	reintentosUpsert  = 2
	mesUltimoDiaFalso = "-31"
)

// Indexes son los índices de nv-74. El único creador es scripts/indexes.
func Indexes() []platformmongo.IndexSet {
	return []platformmongo.IndexSet{
		{Collection: Collection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "negocio_id", Value: 1}, {Key: "fecha", Value: -1}}, Options: options.Index().SetName("ux_negocio_fecha").SetUnique(true)},
		}},
		{Collection: CategoriaCollection, Models: []mongo.IndexModel{
			{Keys: bson.D{{Key: "categoria", Value: 1}, {Key: "mes", Value: 1}}, Options: options.Index().SetName("ux_categoria_mes").SetUnique(true)},
		}},
	}
}

// Repository lee y escribe las colecciones de estadísticas y lee, sin escribir, las de
// reservas, check-ins, reseñas, cupones, favoritos y negocios.
type Repository struct {
	db         *mongo.Database
	diarias    *mongo.Collection
	categorias *mongo.Collection
	relleno    *mongo.Collection
}

// NewRepository recibe la base por constructor.
func NewRepository(db *mongo.Database) *Repository {
	return &Repository{db: db, diarias: db.Collection(Collection), categorias: db.Collection(CategoriaCollection), relleno: db.Collection(RellenoCollection)}
}

func delDia(negocio bson.ObjectID, fecha string) bson.D {
	return bson.D{{Key: "negocio_id", Value: negocio}, {Key: "fecha", Value: fecha}}
}

// upsert reintenta una vez si dos escrituras crean el mismo día a la vez (choca con
// `ux_negocio_fecha`; la segunda ya encuentra el documento).
func (r *Repository) upsert(ctx context.Context, filter, update bson.D) error {
	var err error
	for range reintentosUpsert {
		_, err = r.diarias.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
		if !mongo.IsDuplicateKeyError(err) {
			return err
		}
	}
	return err
}

// Contar suma una vista a la ficha en el día local (y a su origen, si lo hay).
func (r *Repository) Contar(ctx context.Context, negocio bson.ObjectID, fecha, origen string, now time.Time) error {
	inc := bson.D{{Key: "vistas_ficha", Value: 1}}
	if origen != "" {
		inc = append(inc, bson.E{Key: "origen." + origen, Value: 1})
	}
	return r.upsert(ctx, delDia(negocio, fecha), bson.D{
		{Key: "$inc", Value: inc},
		{Key: "$set", Value: bson.D{{Key: "actualizado_en", Value: now}}},
		{Key: "$setOnInsert", Value: bson.D{{Key: "_id", Value: bson.NewObjectID()}}},
	})
}

// Escribir guarda con `$set` lo calculado de cada día: recalcular dos veces da lo mismo. No
// toca `vistas_ficha` ni `origen`, que cuenta Contar.
func (r *Repository) Escribir(ctx context.Context, negocio bson.ObjectID, categoria string, dias map[string]Actividad, now time.Time) error {
	for fecha, a := range dias {
		set := bson.D{
			{Key: "reservas", Value: a.Reservas},
			{Key: "llegadas", Value: a.Llegadas},
			{Key: "cupones_canjeados", Value: a.CuponesCanjeados},
			{Key: "favoritos_nuevos", Value: a.FavoritosNuevos},
			{Key: "por_hora", Value: a.PorHora[:]},
			{Key: "resenas_dia", Value: a.Resenas},
			{Key: "aspectos_suma", Value: a.Aspectos},
			{Key: "actualizado_en", Value: now},
		}
		if categoria != "" {
			set = append(set, bson.E{Key: "categoria", Value: categoria})
		}
		update := bson.D{{Key: "$setOnInsert", Value: bson.D{{Key: "_id", Value: bson.NewObjectID()}}}}
		if a.Resenas > 0 {
			set = append(set, bson.E{Key: "calificacion_dia", Value: a.SumaCalificacion / float64(a.Resenas)})
		} else {
			update = append(update, bson.E{Key: "$unset", Value: bson.D{{Key: "calificacion_dia", Value: ""}}})
		}
		update = append(update, bson.E{Key: "$set", Value: set})
		if err := r.upsert(ctx, delDia(negocio, fecha), update); err != nil {
			return err
		}
	}
	return nil
}

// Rango trae los días del negocio entre desde y hasta (incluidos, `AAAA-MM-DD`).
func (r *Repository) Rango(ctx context.Context, negocio bson.ObjectID, desde, hasta string) ([]Diaria, error) {
	cur, err := r.diarias.Find(ctx, bson.D{
		{Key: "negocio_id", Value: negocio},
		{Key: "fecha", Value: bson.D{{Key: "$gte", Value: desde}, {Key: "$lte", Value: hasta}}},
	}, options.Find().SetSort(bson.D{{Key: "fecha", Value: 1}}))
	if err != nil {
		return nil, err
	}
	out := []Diaria{}
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// BorrarNegocio borra los días y la marca de relleno del negocio (baja, nv-56).
func (r *Repository) BorrarNegocio(ctx context.Context, negocio bson.ObjectID) error {
	if _, err := r.diarias.DeleteMany(ctx, bson.D{{Key: "negocio_id", Value: negocio}}); err != nil {
		return err
	}
	_, err := r.relleno.DeleteOne(ctx, bson.D{{Key: "_id", Value: negocio}})
	return err
}

// Limpiar borra los días anteriores a `antes` (`AAAA-MM-DD`).
func (r *Repository) Limpiar(ctx context.Context, antes string) (int64, error) {
	res, err := r.diarias.DeleteMany(ctx, bson.D{{Key: "fecha", Value: bson.D{{Key: "$lt", Value: antes}}}})
	if err != nil {
		return 0, err
	}
	return res.DeletedCount, nil
}

// Categoria trae los promedios de la categoría en el mes.
func (r *Repository) Categoria(ctx context.Context, categoria, mes string) (Categoria, bool, error) {
	var c Categoria
	err := r.categorias.FindOne(ctx, bson.D{{Key: "categoria", Value: categoria}, {Key: "mes", Value: mes}}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return c, false, nil
	}
	return c, err == nil, err
}

// CalcularCategorias resume el mes por categoría a partir de los días con reseñas: promedios
// ponderados por reseña y cuántos negocios tuvieron alguna. Devuelve cuántas categorías guardó.
func (r *Repository) CalcularCategorias(ctx context.Context, mes string, now time.Time) (int, error) {
	cur, err := r.diarias.Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "fecha", Value: bson.D{{Key: "$gte", Value: mes + "-01"}, {Key: "$lte", Value: mes + mesUltimoDiaFalso}}},
			{Key: "resenas_dia", Value: bson.D{{Key: "$gt", Value: 0}}},
			{Key: "categoria", Value: bson.D{{Key: "$exists", Value: true}}},
		}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$categoria"},
			{Key: "negocios", Value: bson.D{{Key: "$addToSet", Value: "$negocio_id"}}},
			{Key: "resenas", Value: bson.D{{Key: "$sum", Value: "$resenas_dia"}}},
			{Key: "calificacion", Value: bson.D{{Key: "$sum", Value: bson.D{{Key: "$multiply", Value: bson.A{"$calificacion_dia", "$resenas_dia"}}}}}},
			{Key: "sabor", Value: bson.D{{Key: "$sum", Value: "$aspectos_suma.sabor"}}},
			{Key: "atencion", Value: bson.D{{Key: "$sum", Value: "$aspectos_suma.atencion"}}},
			{Key: "ambiente", Value: bson.D{{Key: "$sum", Value: "$aspectos_suma.ambiente"}}},
			{Key: "espera", Value: bson.D{{Key: "$sum", Value: "$aspectos_suma.espera"}}},
			{Key: "precio_calidad", Value: bson.D{{Key: "$sum", Value: "$aspectos_suma.precio_calidad"}}},
		}}},
	})
	if err != nil {
		return 0, err
	}
	var filas []struct {
		Categoria    string          `bson:"_id"`
		Negocios     []bson.ObjectID `bson:"negocios"`
		Resenas      int             `bson:"resenas"`
		Calificacion float64         `bson:"calificacion"`
		Aspectos     Aspectos        `bson:",inline"`
	}
	if err := cur.All(ctx, &filas); err != nil {
		return 0, err
	}
	for _, f := range filas {
		n := float64(f.Resenas)
		_, err := r.categorias.UpdateOne(ctx, bson.D{{Key: "categoria", Value: f.Categoria}, {Key: "mes", Value: mes}}, bson.D{
			{Key: "$set", Value: bson.D{
				{Key: "aspectos", Value: f.Aspectos.entre(n)}, {Key: "calificacion", Value: f.Calificacion / n},
				{Key: "resenas", Value: f.Resenas}, {Key: "negocios", Value: len(f.Negocios)}, {Key: "actualizado_en", Value: now},
			}},
			{Key: "$setOnInsert", Value: bson.D{{Key: "_id", Value: bson.NewObjectID()}}},
		}, options.UpdateOne().SetUpsert(true))
		if err != nil && !mongo.IsDuplicateKeyError(err) {
			return 0, err
		}
	}
	return len(filas), nil
}

// --- fuentes: lecturas de otros módulos ---

func diaLocal(campo string) bson.D {
	return bson.D{{Key: "$dateToString", Value: bson.D{{Key: "format", Value: formatoDiaMongo}, {Key: "date", Value: campo}, {Key: "timezone", Value: Zona}}}}
}

func entre(desde, hasta time.Time) bson.D {
	return bson.D{{Key: "$gte", Value: desde}, {Key: "$lt", Value: hasta}}
}

type conteo struct {
	Dia  string `bson:"_id"`
	Cant int    `bson:"n"`
}

// contarPorDia agrupa por día local los documentos de col que pasan el filtro.
func (r *Repository) contarPorDia(ctx context.Context, col, campoFecha string, filter bson.D) ([]conteo, error) {
	cur, err := r.db.Collection(col).Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: filter}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: diaLocal("$" + campoFecha)}, {Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}}}}},
	})
	if err != nil {
		return nil, err
	}
	out := []conteo{}
	return out, cur.All(ctx, &out)
}

// Actividad calcula, por día local entre desde y hasta (instantes, hasta excluido), lo que el
// negocio tuvo en reservas, check-ins, canjes, favoritos y reseñas. Solo trae días con algo.
func (r *Repository) Actividad(ctx context.Context, negocio bson.ObjectID, desde, hasta time.Time) (map[string]Actividad, error) {
	out := map[string]Actividad{}
	sumar := func(rows []conteo, fn func(*Actividad, int)) {
		for _, c := range rows {
			a := out[c.Dia]
			fn(&a, c.Cant)
			out[c.Dia] = a
		}
	}
	rango := entre(desde, hasta)
	reservas, err := r.contarPorDia(ctx, colReservas, "creado_en", bson.D{
		{Key: "negocio_id", Value: negocio}, {Key: "creado_en", Value: rango},
		{Key: "estado", Value: bson.D{{Key: "$nin", Value: bson.A{reservaBloqueada, reservaExpirada}}}},
	})
	if err != nil {
		return nil, err
	}
	sumar(reservas, func(a *Actividad, n int) { a.Reservas = n })
	canjes, err := r.contarPorDia(ctx, colCuponesUsuario, "canjeado_en", bson.D{{Key: "negocio_id", Value: negocio}, {Key: "canjeado_en", Value: rango}})
	if err != nil {
		return nil, err
	}
	sumar(canjes, func(a *Actividad, n int) { a.CuponesCanjeados = n })
	favoritos, err := r.contarPorDia(ctx, colFavoritos, "creado_en", bson.D{
		{Key: "tipo", Value: favoritoNegocio}, {Key: "objetivo_id", Value: negocio}, {Key: "creado_en", Value: rango},
	})
	if err != nil {
		return nil, err
	}
	sumar(favoritos, func(a *Actividad, n int) { a.FavoritosNuevos = n })
	if err := r.llegadas(ctx, negocio, rango, out); err != nil {
		return nil, err
	}
	if err := r.resenas(ctx, negocio, rango, out); err != nil {
		return nil, err
	}
	return out, nil
}

// llegadas cuenta los check-ins válidos por día y hora local.
func (r *Repository) llegadas(ctx context.Context, negocio bson.ObjectID, rango bson.D, out map[string]Actividad) error {
	cur, err := r.db.Collection(colCheckins).Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "negocio_id", Value: negocio}, {Key: "valido", Value: true}, {Key: "creado_en", Value: rango}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: bson.D{
				{Key: "dia", Value: diaLocal("$creado_en")},
				{Key: "hora", Value: bson.D{{Key: "$hour", Value: bson.D{{Key: "date", Value: "$creado_en"}, {Key: "timezone", Value: Zona}}}}},
			}},
			{Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	})
	if err != nil {
		return err
	}
	var rows []struct {
		ID struct {
			Dia  string `bson:"dia"`
			Hora int    `bson:"hora"`
		} `bson:"_id"`
		Cant int `bson:"n"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return err
	}
	for _, row := range rows {
		if row.ID.Hora < 0 || row.ID.Hora >= camposPorHora {
			continue
		}
		a := out[row.ID.Dia]
		a.Llegadas += row.Cant
		a.PorHora[row.ID.Hora] += row.Cant
		out[row.ID.Dia] = a
	}
	return nil
}

// resenas suma calificación y aspectos de las reseñas publicadas por día local.
func (r *Repository) resenas(ctx context.Context, negocio bson.ObjectID, rango bson.D, out map[string]Actividad) error {
	cur, err := r.db.Collection(colResenas).Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "negocio_id", Value: negocio}, {Key: "estado", Value: resenaPublicada}, {Key: "creado_en", Value: rango}}}},
		{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: diaLocal("$creado_en")},
			{Key: "n", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "calificacion", Value: bson.D{{Key: "$sum", Value: "$calificacion"}}},
			{Key: "sabor", Value: bson.D{{Key: "$sum", Value: "$aspectos.sabor"}}},
			{Key: "atencion", Value: bson.D{{Key: "$sum", Value: "$aspectos.atencion"}}},
			{Key: "ambiente", Value: bson.D{{Key: "$sum", Value: "$aspectos.ambiente"}}},
			{Key: "espera", Value: bson.D{{Key: "$sum", Value: "$aspectos.espera"}}},
			{Key: "precio_calidad", Value: bson.D{{Key: "$sum", Value: "$aspectos.precio_calidad"}}},
		}}},
	})
	if err != nil {
		return err
	}
	var rows []struct {
		Dia          string   `bson:"_id"`
		Cant         int      `bson:"n"`
		Calificacion float64  `bson:"calificacion"`
		Aspectos     Aspectos `bson:",inline"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return err
	}
	for _, row := range rows {
		a := out[row.Dia]
		a.Resenas = row.Cant
		a.SumaCalificacion = row.Calificacion
		a.Aspectos = row.Aspectos
		out[row.Dia] = a
	}
	return nil
}

// ConActividad junta los negocios con reservas, check-ins, canjes, favoritos o reseñas desde
// `desde` (las reseñas por `actualizado_en`, para ver las que se publicaron después).
func (r *Repository) ConActividad(ctx context.Context, desde time.Time) ([]bson.ObjectID, error) {
	desdeD := bson.D{{Key: "$gte", Value: desde}}
	fuentes := []struct {
		col, campo string
		filter     bson.D
	}{
		{colReservas, "negocio_id", bson.D{{Key: "creado_en", Value: desdeD}}},
		{colCheckins, "negocio_id", bson.D{{Key: "creado_en", Value: desdeD}}},
		{colCuponesUsuario, "negocio_id", bson.D{{Key: "canjeado_en", Value: desdeD}}},
		{colFavoritos, "objetivo_id", bson.D{{Key: "tipo", Value: favoritoNegocio}, {Key: "creado_en", Value: desdeD}}},
		{colResenas, "negocio_id", bson.D{{Key: "actualizado_en", Value: desdeD}}},
	}
	vistos := map[bson.ObjectID]bool{}
	out := []bson.ObjectID{}
	for _, f := range fuentes {
		var ids []bson.ObjectID
		if err := r.db.Collection(f.col).Distinct(ctx, f.campo, f.filter).Decode(&ids); err != nil {
			return nil, err
		}
		for _, id := range ids {
			if !vistos[id] {
				vistos[id] = true
				out = append(out, id)
			}
		}
	}
	return out, nil
}

// PendientesRelleno trae hasta `limite` negocios aprobados o pausados, sin baja, que aún no
// tienen sus 90 días rellenados.
func (r *Repository) PendientesRelleno(ctx context.Context, limite int64) ([]bson.ObjectID, error) {
	cur, err := r.db.Collection(colNegocios).Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{
			{Key: "estado", Value: bson.D{{Key: "$in", Value: bson.A{negocioAprobado, negocioPausado}}}},
			{Key: "eliminado_en", Value: bson.D{{Key: "$exists", Value: false}}},
		}}},
		{{Key: "$lookup", Value: bson.D{{Key: "from", Value: RellenoCollection}, {Key: "localField", Value: "_id"}, {Key: "foreignField", Value: "_id"}, {Key: "as", Value: "hecho"}}}},
		{{Key: "$match", Value: bson.D{{Key: "hecho", Value: bson.D{{Key: "$size", Value: 0}}}}}},
		{{Key: "$limit", Value: limite}},
		{{Key: "$project", Value: bson.D{{Key: "_id", Value: 1}}}},
	})
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

// MarcarRelleno anota que el negocio ya tiene sus 90 días.
func (r *Repository) MarcarRelleno(ctx context.Context, negocio bson.ObjectID, now time.Time) error {
	_, err := r.relleno.UpdateOne(ctx, bson.D{{Key: "_id", Value: negocio}}, bson.D{{Key: "$set", Value: bson.D{{Key: "hecho_en", Value: now}}}}, options.UpdateOne().SetUpsert(true))
	return err
}

// TopCupones trae los 3 cupones del negocio con más canjes entre desde y hasta (días UTC de
// `cupones_metricas`, incluidos). Los cupones sin canjes no salen.
func (r *Repository) TopCupones(ctx context.Context, negocio bson.ObjectID, desde, hasta string) ([]TopCupon, error) {
	cur, err := r.db.Collection(colCupones).Find(ctx, bson.D{{Key: "negocio_id", Value: negocio}}, options.Find().SetProjection(bson.D{{Key: "titulo", Value: 1}}))
	if err != nil {
		return nil, err
	}
	var cupones []struct {
		ID     bson.ObjectID `bson:"_id"`
		Titulo string        `bson:"titulo"`
	}
	if err := cur.All(ctx, &cupones); err != nil {
		return nil, err
	}
	if len(cupones) == 0 {
		return []TopCupon{}, nil
	}
	titulos := make(map[bson.ObjectID]string, len(cupones))
	ids := make(bson.A, 0, len(cupones))
	for _, c := range cupones {
		titulos[c.ID] = c.Titulo
		ids = append(ids, c.ID)
	}
	cur, err = r.db.Collection(colCuponesMetricas).Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.D{{Key: "cupon_id", Value: bson.D{{Key: "$in", Value: ids}}}, {Key: "dia", Value: bson.D{{Key: "$gte", Value: desde}, {Key: "$lte", Value: hasta}}}}}},
		{{Key: "$group", Value: bson.D{{Key: "_id", Value: "$cupon_id"}, {Key: "canjes", Value: bson.D{{Key: "$sum", Value: "$canjes"}}}}}},
		{{Key: "$match", Value: bson.D{{Key: "canjes", Value: bson.D{{Key: "$gt", Value: 0}}}}}},
		{{Key: "$sort", Value: bson.D{{Key: "canjes", Value: -1}, {Key: "_id", Value: 1}}}},
		{{Key: "$limit", Value: 3}},
	})
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID     bson.ObjectID `bson:"_id"`
		Canjes int           `bson:"canjes"`
	}
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}
	out := make([]TopCupon, 0, len(rows))
	for _, row := range rows {
		out = append(out, TopCupon{Titulo: titulos[row.ID], Canjes: row.Canjes})
	}
	return out, nil
}
