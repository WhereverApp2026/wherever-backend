package ia

import (
	"context"
	"errors"
	"time"
)

// Llaves de los límites de embeddings (nv-50). El modelo de embeddings tiene su propia cuota en
// Gemini, separada de la del modelo de texto: por eso no comparte las ventanas de nv-28.
const (
	prefijoVectoresMin = "rate:ia_emb_min:"
	prefijoVectoresDia = "rate:ia_emb_dia:"
)

// Sumador suma n a varias ventanas de una vez (LimitadorRedis lo cumple).
type Sumador interface {
	Sumar(ctx context.Context, n int64, ventanas ...Ventana) ([]Cuenta, error)
}

// LimitesVectores son IA_EMB_LIMITE_MIN e IA_EMB_LIMITE_DIA: textos vectorizados por minuto y
// por día del proveedor para toda la app.
type LimitesVectores struct {
	PorMinuto int
	PorDia    int
}

// VectoresLimitados envuelve un Vectorizador con los límites globales. Cada texto cuenta uno,
// también dentro de un lote, y una llamada rechazada también cuenta. Sin Redis se cierra.
type VectoresLimitados struct {
	base    Vectorizador
	sumador Sumador
	limites LimitesVectores
	zona    *time.Location
	ahora   func() time.Time
}

// NewVectoresLimitados valida los límites y carga la zona del día del proveedor.
func NewVectoresLimitados(base Vectorizador, sumador Sumador, limites LimitesVectores) (*VectoresLimitados, error) {
	if base == nil || sumador == nil {
		return nil, errors.New("ia: los embeddings limitados necesitan vectorizador y contador")
	}
	if limites.PorMinuto < 1 || limites.PorDia < 1 {
		return nil, errors.New("ia: los límites de embeddings deben ser positivos")
	}
	zona, err := time.LoadLocation(zonaProveedor)
	if err != nil {
		return nil, err
	}
	return &VectoresLimitados{base: base, sumador: sumador, limites: limites, zona: zona, ahora: time.Now}, nil
}

// Nombre es el del vectorizador envuelto.
func (v *VectoresLimitados) Nombre() string { return v.base.Nombre() }

// Documentos cuenta len(docs) y vectoriza.
func (v *VectoresLimitados) Documentos(ctx context.Context, docs []Documento) ([][]float32, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	if err := v.tomar(ctx, int64(len(docs))); err != nil {
		return nil, err
	}
	return v.base.Documentos(ctx, docs)
}

// Consulta cuenta uno y vectoriza.
func (v *VectoresLimitados) Consulta(ctx context.Context, frase string) ([]float32, error) {
	if err := v.tomar(ctx, 1); err != nil {
		return nil, err
	}
	return v.base.Consulta(ctx, frase)
}

func (v *VectoresLimitados) tomar(ctx context.Context, n int64) error {
	ahora := v.ahora()
	cuentas, err := v.sumador.Sumar(ctx, n,
		Ventana{Llave: prefijoVectoresMin + ahora.UTC().Format("200601021504"), Vida: vidaMinuto},
		Ventana{Llave: prefijoVectoresDia + ahora.In(v.zona).Format("2006-01-02"), Vida: vidaDia})
	if err != nil || len(cuentas) != 2 {
		return ErrLimiteDesconocido
	}
	if cuentas[0].Total > int64(v.limites.PorMinuto) || cuentas[1].Total > int64(v.limites.PorDia) {
		return ErrLimiteGlobal
	}
	return nil
}
