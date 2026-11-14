package negocios

import (
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// Suma es la contribución de reseñas a `calificacion_suma` (nv-62): enteros, para que sumar y
// restar sea exacto. Una reseña aporta Total 1 y sus estrellas y aspectos (1–5).
type Suma struct {
	Total         int64
	Estrellas     int64
	Sabor         int64
	Atencion      int64
	Ambiente      int64
	Espera        int64
	PrecioCalidad int64
}

// Menos devuelve s − o.
func (s Suma) Menos(o Suma) Suma {
	return Suma{
		Total: s.Total - o.Total, Estrellas: s.Estrellas - o.Estrellas, Sabor: s.Sabor - o.Sabor,
		Atencion: s.Atencion - o.Atencion, Ambiente: s.Ambiente - o.Ambiente, Espera: s.Espera - o.Espera,
		PrecioCalidad: s.PrecioCalidad - o.PrecioCalidad,
	}
}

// Cero dice si no cambia nada.
func (s Suma) Cero() bool { return s == Suma{} }

// camposSuma enlaza cada campo de `calificacion_suma` con su aspecto en `calificacion.aspectos`
// (vacío para total y estrellas).
var camposSuma = []struct{ suma, aspecto string }{
	{"total", ""}, {"estrellas", ""}, {"sabor", "sabor"}, {"atencion", "atencion"},
	{"ambiente", "ambiente"}, {"espera", "espera"}, {"precio_calidad", "precio_calidad"},
}

func (s Suma) valor(campo string) int64 {
	switch campo {
	case "total":
		return s.Total
	case "estrellas":
		return s.Estrellas
	case "sabor":
		return s.Sabor
	case "atencion":
		return s.Atencion
	case "ambiente":
		return s.Ambiente
	case "espera":
		return s.Espera
	default:
		return s.PrecioCalidad
	}
}

func num(path string) bson.D {
	return bson.D{{Key: "$ifNull", Value: bson.A{path, 0}}}
}

// inicialSemilla calcula `calificacion_suma` de un negocio de la semilla que solo trae
// promedio y total: suma = round(promedio × total).
func inicialSemilla() bson.D {
	total := num("$calificacion.total_resenas")
	inicial := bson.D{}
	for _, c := range camposSuma {
		switch c.suma {
		case "total":
			inicial = append(inicial, bson.E{Key: "total", Value: total})
		case "estrellas":
			inicial = append(inicial, bson.E{Key: "estrellas", Value: redondear(num("$calificacion.promedio"), total)})
		default:
			inicial = append(inicial, bson.E{Key: c.suma, Value: redondear(num("$calificacion.aspectos."+c.aspecto), total)})
		}
	}
	return inicial
}

func redondear(promedio, total bson.D) bson.D {
	return bson.D{{Key: "$round", Value: bson.A{bson.D{{Key: "$multiply", Value: bson.A{promedio, total}}}, 0}}}
}

func promedio(suma, total string) bson.D {
	return bson.D{{Key: "$cond", Value: bson.A{
		bson.D{{Key: "$gt", Value: bson.A{total, 0}}},
		bson.D{{Key: "$round", Value: bson.A{bson.D{{Key: "$divide", Value: bson.A{suma, total}}}, 1}}},
		0,
	}}}
}

// AplicarSuma suma d a `calificacion_suma` y recalcula `calificacion` en un solo updateOne de
// pipeline: atómico, sin leer antes. La primera vez inicializa la suma desde la semilla. No sube
// `version`: no es un cambio del dueño.
func (r *Repository) AplicarSuma(ctx context.Context, id bson.ObjectID, d Suma) error {
	sumar := bson.D{}
	for _, c := range camposSuma {
		sumar = append(sumar, bson.E{Key: "calificacion_suma." + c.suma, Value: bson.D{{Key: "$max", Value: bson.A{
			0, bson.D{{Key: "$add", Value: bson.A{"$calificacion_suma." + c.suma, d.valor(c.suma)}}},
		}}}})
	}
	total := "$calificacion_suma.total"
	recalc := bson.D{
		{Key: "calificacion.total_resenas", Value: total},
		{Key: "calificacion.promedio", Value: promedio("$calificacion_suma.estrellas", total)},
	}
	for _, c := range camposSuma[2:] {
		recalc = append(recalc, bson.E{Key: "calificacion.aspectos." + c.aspecto, Value: promedio("$calificacion_suma."+c.suma, total)})
	}
	pipeline := mongo.Pipeline{
		{{Key: "$set", Value: bson.D{{Key: "calificacion_suma", Value: bson.D{{Key: "$ifNull", Value: bson.A{"$calificacion_suma", inicialSemilla()}}}}}}},
		{{Key: "$set", Value: sumar}},
		{{Key: "$set", Value: recalc}},
	}
	res, err := r.col.UpdateOne(ctx, bson.D{{Key: "_id", Value: id}}, pipeline)
	if err != nil {
		return err
	}
	if res.MatchedCount == 0 {
		return ErrNotFound
	}
	return nil
}

// FindVisibleByQR busca el negocio visible dueño del `qr_checkin` (ux_qr_checkin).
func (r *Repository) FindVisibleByQR(ctx context.Context, codigo string) (Business, bool, error) {
	filter := append(VisibleFilter(), bson.E{Key: "qr_checkin", Value: codigo})
	var b Business
	err := r.col.FindOne(ctx, filter).Decode(&b)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Business{}, false, nil
	}
	if err != nil {
		return Business{}, false, err
	}
	return b, true, nil
}
