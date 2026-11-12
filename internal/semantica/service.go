package semantica

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"log/slog"
	"math"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendador"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/text"
)

// Límites de la búsqueda.
const (
	LimiteDefault = 10
	LimiteMax     = 30
	// VidaCache es lo que dura el vector de una frase en Redis (`cache:ia:emb_{huella}`).
	VidaCache = 24 * time.Hour
)

// Fuentes de la respuesta.
const (
	FuenteSemantica = "semantica"
	FuenteReglas    = "reglas"
)

const avisoReglas = "La búsqueda por significado no está disponible; te mostramos lugares por reglas."

var (
	errApagada     = errors.New("semantica: sin modelo de embeddings")
	errSinVectores = errors.New("semantica: ningún negocio aprobado tiene vector")
)

// Negocios da los negocios aprobados; lo implementa *Repository.
type Negocios interface {
	Visibles(ctx context.Context) ([]negocios.Business, error)
}

// Cache guarda el vector de una frase; lo implementa *ia.CacheRedis. Sin Redis se sigue sin
// caché (no cierra, como la de nv-27).
type Cache interface {
	Leer(ctx context.Context, huella string) ([]byte, bool, error)
	Guardar(ctx context.Context, huella string, valor []byte, vida time.Duration) error
}

// Config son las dependencias de valor.
type Config struct {
	Pesos recomendador.Pesos
	Ahora func() time.Time // nil = time.Now
}

// Service busca por significado. Sin vectorizador (sin llave o sin IA_EMBEDDINGS_ID)
// responde siempre el motor de reglas.
type Service struct {
	vect     ia.Vectorizador
	indice   Indice
	negocios Negocios
	cache    Cache
	cfg      Config
	log      *slog.Logger
}

// NewService valida los pesos al arrancar. vect puede ser nil.
func NewService(vect ia.Vectorizador, indice Indice, neg Negocios, cache Cache, cfg Config, log *slog.Logger) (*Service, error) {
	if err := cfg.Pesos.Validate(); err != nil {
		return nil, errors.Join(recomendador.ErrPesos, err)
	}
	if cfg.Ahora == nil {
		cfg.Ahora = time.Now
	}
	return &Service{vect: vect, indice: indice, negocios: neg, cache: cache, cfg: cfg, log: log}, nil
}

// Buscar devuelve los negocios aprobados más cercanos en significado a la frase. Si la IA no
// está, no tiene cupo o falla, responde el motor de reglas con las palabras de la frase como
// gustos (200 con `fuente: reglas`).
func (s *Service) Buscar(ctx context.Context, frase string, limite int) (Respuesta, error) {
	visibles, err := s.negocios.Visibles(ctx)
	if err != nil {
		s.log.ErrorContext(ctx, "semantica: negocios no disponibles")
		return Respuesta{}, apperr.Unavailable("servicio no disponible")
	}
	limite = limiteDe(limite)
	ahora := s.cfg.Ahora()
	afin, err := s.afinidad(ctx, frase, idsDe(visibles))
	if err != nil {
		s.log.WarnContext(ctx, "semantica: responde el motor de reglas", "causa", causa(err))
		return respuestaReglas(Reglas(visibles, frase, ahora, s.cfg.Pesos, limite), ahora), nil
	}
	return respuestaSemantica(PorAfinidad(visibles, afin, limite), afin, ahora), nil
}

// Afinidad es la cercanía en significado de cada negocio aprobado con la frase, de 0 a 1,
// para el factor `afinidad_semantica` del asistente (nv-31). nil si no se pudo: el factor
// queda neutro.
func (s *Service) Afinidad(ctx context.Context, frase string) map[bson.ObjectID]float64 {
	visibles, err := s.negocios.Visibles(ctx)
	if err != nil {
		return nil
	}
	afin, err := s.afinidad(ctx, frase, idsDe(visibles))
	if err != nil {
		s.log.WarnContext(ctx, "semantica: candidatos sin afinidad", "causa", causa(err))
		return nil
	}
	return afin
}

func (s *Service) afinidad(ctx context.Context, frase string, ids []bson.ObjectID) (map[bson.ObjectID]float64, error) {
	if s.vect == nil {
		return nil, errApagada
	}
	consulta, err := s.consulta(ctx, frase)
	if err != nil {
		return nil, err
	}
	sims, err := s.indice.Similitudes(ctx, consulta, ids)
	if err != nil {
		return nil, err
	}
	if len(sims) == 0 {
		return nil, errSinVectores
	}
	return Afinidades(sims), nil
}

// consulta es una llamada al proveedor por frase; el vector se guarda 24 h por huella.
func (s *Service) consulta(ctx context.Context, frase string) ([]float32, error) {
	huella := Huella(s.vect.Nombre(), frase)
	if raw, ok, err := s.cache.Leer(ctx, huella); err == nil && ok {
		if v, ok := decodificar(raw); ok {
			return v, nil
		}
	}
	v, err := s.vect.Consulta(ctx, frase)
	if err != nil {
		return nil, err
	}
	if err := s.cache.Guardar(ctx, huella, codificar(v), VidaCache); err != nil {
		s.log.WarnContext(ctx, "semantica: no se guardó la caché")
	}
	return v, nil
}

// Huella identifica una frase para la caché sin guardarla: SHA-256 del modelo y de la frase
// anonimizada y normalizada. La llave no lleva usuario.
func Huella(modelo, frase string) string {
	sum := sha256.Sum256([]byte(modelo + "\n" + text.Normalize(ia.Anonimizar(strings.TrimSpace(frase)))))
	return "emb_" + hex.EncodeToString(sum[:])
}

func codificar(v []float32) []byte {
	out := make([]byte, 4*len(v))
	for i, x := range v {
		binary.LittleEndian.PutUint32(out[4*i:], math.Float32bits(x))
	}
	return out
}

func decodificar(raw []byte) ([]float32, bool) {
	if len(raw) != 4*ia.Dimensiones {
		return nil, false
	}
	out := make([]float32, ia.Dimensiones)
	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(raw[4*i:]))
	}
	return out, true
}

// PorAfinidad ordena por afinidad, luego calificación y luego id. Un negocio sin vector no
// sale: no se sabe qué tan cerca está.
func PorAfinidad(visibles []negocios.Business, afin map[bson.ObjectID]float64, limite int) []negocios.Business {
	out := make([]negocios.Business, 0, len(afin))
	for _, b := range visibles {
		if _, ok := afin[b.ID]; ok {
			out = append(out, b)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		ai, aj := afin[out[i].ID], afin[out[j].ID]
		if ai != aj {
			return ai > aj
		}
		if ri, rj := out[i].Rating.Promedio, out[j].Rating.Promedio; ri != rj {
			return ri > rj
		}
		return out[i].ID.Hex() < out[j].ID.Hex()
	})
	return out[:min(len(out), limite)]
}

// Reglas es la búsqueda sin semántica: el motor de nv-23 con las palabras de la frase como
// gustos, sin ubicación (la distancia vale lo mismo para todos) y sin filtros duros. Es la línea base
// de la evaluación (docs/ml-evaluacion.md).
func Reglas(visibles []negocios.Business, frase string, ahora time.Time, pesos recomendador.Pesos, limite int) []negocios.Business {
	return ordenar(visibles, recomendador.Contexto{Momento: ahora, Gustos: Palabras(frase)}, pesos, limite)
}

// Hibrido es el motor de reglas con el factor `afinidad_semantica`, como lo usa el asistente.
func Hibrido(visibles []negocios.Business, frase string, afin map[bson.ObjectID]float64, ahora time.Time, pesos recomendador.Pesos, limite int) []negocios.Business {
	return ordenar(visibles, recomendador.Contexto{Momento: ahora, Gustos: Palabras(frase), Afinidad: afin}, pesos, limite)
}

// ordenar puntúa con los factores y pesos del motor, sin sus filtros duros: la búsqueda no
// trae perfil ni hora pedida, y un lugar cerrado ahora puede ser el que se busca para luego
// (el horario solo resta puntos; la tarjeta dice `abierto_ahora`). Empates por calificación
// y luego id, como el motor.
func ordenar(visibles []negocios.Business, c recomendador.Contexto, pesos recomendador.Pesos, limite int) []negocios.Business {
	puntaje := recomendador.NewPuntaje(c)
	totales := make(map[bson.ObjectID]float64, len(visibles))
	for _, b := range visibles {
		totales[b.ID] = recomendador.Total(puntaje.Factores(negocios.Candidate{Business: b}), pesos)
	}
	out := slices.Clone(visibles)
	sort.SliceStable(out, func(i, j int) bool {
		if ti, tj := totales[out[i].ID], totales[out[j].ID]; ti != tj {
			return ti > tj
		}
		if ri, rj := out[i].Rating.Promedio, out[j].Rating.Promedio; ri != rj {
			return ri > rj
		}
		return out[i].ID.Hex() < out[j].ID.Hex()
	})
	return out[:min(len(out), limite)]
}

// Palabras son las palabras de la frase normalizadas, de 3 letras o más y sin repetir.
func Palabras(frase string) []string {
	campos := strings.FieldsFunc(text.Normalize(frase), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
	var out []string
	vistos := map[string]bool{}
	for _, c := range campos {
		if len([]rune(c)) >= 3 && !vistos[c] {
			vistos[c] = true
			out = append(out, c)
		}
	}
	return out
}

func idsDe(bs []negocios.Business) []bson.ObjectID {
	out := make([]bson.ObjectID, len(bs))
	for i, b := range bs {
		out[i] = b.ID
	}
	return out
}

func limiteDe(n int) int {
	if n <= 0 {
		return LimiteDefault
	}
	return min(n, LimiteMax)
}

// causas nombra la falla en el log sin texto del proveedor ni de la frase.
var causas = []struct {
	err   error
	causa string
}{
	{errApagada, "apagada"},
	{errSinVectores, "sin_vectores"},
	{ia.ErrLimiteGlobal, "limite"},
	{ia.ErrLimiteDesconocido, "limite_desconocido"},
	{ia.ErrLimiteProveedor, "proveedor_429"},
	{ia.ErrTiempo, "tiempo"},
	{context.DeadlineExceeded, "tiempo"},
}

func causa(err error) string {
	for _, c := range causas {
		if errors.Is(err, c.err) {
			return c.causa
		}
	}
	return "falla"
}
