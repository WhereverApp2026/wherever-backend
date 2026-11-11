package ia

import (
	"context"
	"errors"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// Errores del tope de gasto (nv-27). Con cualquiera de ellos no se llama al proveedor y
// responde el motor de reglas.
var (
	// ErrTope: el gasto estimado del mes llegó al tope (IA_TOPE_USD_MES).
	ErrTope = errors.New("ia: tope de gasto mensual alcanzado")
	// ErrGastoDesconocido: no se pudo leer el gasto ni en Redis ni en usos_ia; se cierra.
	ErrGastoDesconocido = errors.New("ia: gasto del mes no disponible")
)

// Precios son los USD por millón de tokens del modelo. En la capa gratuita de Gemini son cero
// y el tope nunca se alcanza; protegen si se activa la facturación.
type Precios struct {
	EntradaUSDMTok float64
	SalidaUSDMTok  float64
}

// CostoMicroUSD estima el costo de una llamada con los tokens reales. Un token a P USD por
// millón cuesta P micro-USD, así que el costo es tokens × precio; se redondea hacia arriba
// para no quedar por debajo de la factura.
func (p Precios) CostoMicroUSD(entrada, salida int) int64 {
	costo := float64(entrada)*p.EntradaUSDMTok + float64(salida)*p.SalidaUSDMTok
	if costo <= 0 {
		return 0
	}
	return int64(math.Ceil(costo))
}

// MicroUSD convierte dólares a micro-dólares, la unidad entera del contador.
func MicroUSD(usd float64) int64 { return int64(math.Round(usd * 1e6)) }

// Mes es la clave del mes de gasto, en UTC («2026-11»).
func Mes(t time.Time) string { return t.UTC().Format("2006-01") }

// vidaMes es lo que le queda al mes de t más un día de margen: la vida del contador.
func vidaMes(t time.Time) time.Duration {
	u := t.UTC()
	siguiente := time.Date(u.Year(), u.Month()+1, 1, 0, 0, 0, 0, time.UTC)
	return siguiente.Sub(u) + 24*time.Hour
}

type claveUsuario struct{}

// ConUsuario marca en el contexto quién pide la generación, para registrar el uso. El id
// nunca viaja al proveedor (nv-28).
func ConUsuario(ctx context.Context, usuario bson.ObjectID) context.Context {
	return context.WithValue(ctx, claveUsuario{}, usuario)
}

func usuarioDe(ctx context.Context) bson.ObjectID {
	u, _ := ctx.Value(claveUsuario{}).(bson.ObjectID)
	return u
}
