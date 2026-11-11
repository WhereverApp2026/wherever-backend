package ia

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// VidaCache es la vida de una respuesta guardada (nv-27).
const VidaCache = time.Hour

// Resultados que se registran en usos_ia.
const (
	ResultadoOK        = "ok"
	ResultadoCache     = "cache"
	ResultadoTope      = "tope"
	ResultadoSinGasto  = "gasto_desconocido"
	ResultadoLimite    = "limite_proveedor"
	ResultadoTiempo    = "tiempo"
	ResultadoInvalida  = "invalida"
	ResultadoBloqueada = "bloqueada"
	ResultadoError     = "error"
	// nv-28: rechazos de los límites, sin llamar al proveedor.
	ResultadoLimiteUsuario = "limite_usuario"
	ResultadoLimiteGlobal  = "limite_global"
	ResultadoSinLimite     = "limite_desconocido"
)

// Contador es el gasto del mes en micro-USD (Redis). Leer dice si la llave existía.
type Contador interface {
	Leer(ctx context.Context, mes string) (int64, bool, error)
	Fijar(ctx context.Context, mes string, micro int64, vida time.Duration) error
	Sumar(ctx context.Context, mes string, micro int64, vida time.Duration) error
}

// Registro guarda cada llamada en usos_ia y suma el gasto de un mes para reconstruir el
// contador.
type Registro interface {
	Guardar(ctx context.Context, u Uso) error
	GastoDelMes(ctx context.Context, mes string) (int64, error)
}

// Cache guarda salidas ya validadas por la huella de la petición.
type Cache interface {
	Leer(ctx context.Context, huella string) ([]byte, bool, error)
	Guardar(ctx context.Context, huella string, valor []byte, vida time.Duration) error
}

// GuardiaConfig son el tope, los precios y los límites (IA_TOPE_USD_MES, IA_PRECIO_*,
// IA_LIMITE_*).
type GuardiaConfig struct {
	TopeUSDMes float64
	Precios    Precios
	Limites    Limites
	// Limitador cuenta los límites (Redis); sin él la guardia no se arma.
	Limitador Limitador
	// Ahora vacío usa time.Now; las pruebas fijan el reloj.
	Ahora func() time.Time
}

func (cfg GuardiaConfig) validar() error {
	if !(cfg.TopeUSDMes > 0) || cfg.Precios.EntradaUSDMTok < 0 || cfg.Precios.SalidaUSDMTok < 0 {
		return errors.New("ia: el tope debe ser positivo y los precios no negativos")
	}
	return cfg.Limites.validar()
}

// Guardia envuelve un Modelo con el tope de gasto mensual y la caché (nv-27) y con los límites
// por usuario y globales (nv-28). Es a su vez un Modelo: cada llamada al proveedor pasa por el
// tope y el límite global, incluido el reintento de GenerarValidado.
type Guardia struct {
	modelo    Modelo
	contador  Contador
	registro  Registro
	cache     Cache
	limitador Limitador
	limites   Limites
	zona      *time.Location
	tope      int64
	precios   Precios
	ahora     func() time.Time
	log       *slog.Logger
}

// NewGuardia valida el tope y arma la guardia.
func NewGuardia(m Modelo, c Contador, r Registro, k Cache, cfg GuardiaConfig, log *slog.Logger) (*Guardia, error) {
	if m == nil || c == nil || r == nil || k == nil || cfg.Limitador == nil || log == nil {
		return nil, errors.New("ia: la guardia necesita modelo, contador, registro, caché, limitador y logger")
	}
	if err := cfg.validar(); err != nil {
		return nil, err
	}
	zona, err := time.LoadLocation(zonaProveedor)
	if err != nil {
		return nil, fmt.Errorf("ia: zona del proveedor: %w", err)
	}
	ahora := cfg.Ahora
	if ahora == nil {
		ahora = time.Now
	}
	return &Guardia{modelo: m, contador: c, registro: r, cache: k, limitador: cfg.Limitador, limites: cfg.Limites,
		zona: zona, tope: MicroUSD(cfg.TopeUSDMes), precios: cfg.Precios, ahora: ahora, log: log}, nil
}

// Nombre es el del modelo envuelto.
func (g *Guardia) Nombre() string { return g.modelo.Nombre() }

// Generar llama al proveedor solo si el gasto del mes no llegó al tope y la app no llegó a sus
// llamadas del minuto ni del día; suma el costo estimado con los tokens reales y registra la
// llamada.
func (g *Guardia) Generar(ctx context.Context, pet Peticion) (Respuesta, error) {
	ahora := g.ahora()
	mes := Mes(ahora)
	gastado, err := g.gastado(ctx, mes, ahora)
	if err != nil {
		g.registrar(ctx, ahora, Respuesta{}, 0, ResultadoSinGasto)
		return Respuesta{}, err
	}
	if gastado >= g.tope {
		g.registrar(ctx, ahora, Respuesta{}, 0, ResultadoTope)
		return Respuesta{}, ErrTope
	}
	if err := g.tomarGlobal(ctx, ahora); err != nil {
		g.registrar(ctx, ahora, Respuesta{}, 0, resultadoDe(err))
		return Respuesta{}, err
	}
	r, err := g.modelo.Generar(ctx, pet)
	costo := g.precios.CostoMicroUSD(r.TokensEntrada, r.TokensSalida)
	if costo > 0 {
		if errSuma := g.contador.Sumar(ctx, mes, costo, vidaMes(ahora)); errSuma != nil {
			g.log.Warn("ia: no se sumó el gasto en Redis", "parte", "gasto", "error", errSuma.Error())
		}
	}
	g.registrar(ctx, ahora, r, costo, resultadoDe(err))
	return r, err
}

// GenerarValidado responde desde la caché si la misma petición ya tiene una salida válida;
// si no, cuenta la generación en el límite del usuario, llama con el tope y la validación
// estricta, y guarda la salida por una hora. Un acierto de caché no cuenta: no llama al
// proveedor. Una salida que repite la frase del usuario no se guarda.
func (g *Guardia) GenerarValidado(ctx context.Context, pet Peticion, destino any) (Respuesta, error) {
	huella := Huella(g.modelo.Nombre(), pet)
	if datos, ok := g.leerCache(ctx, huella); ok && Decodificar(datos, destino) == nil {
		g.registrar(ctx, g.ahora(), Respuesta{}, 0, ResultadoCache)
		return Respuesta{JSON: datos}, nil
	}
	if err := g.tomarUsuario(ctx, g.ahora()); err != nil {
		g.registrar(ctx, g.ahora(), Respuesta{}, 0, resultadoDe(err))
		return Respuesta{}, err
	}
	r, err := GenerarValidado(ctx, g, pet, destino)
	if err == nil && !contieneTexto(r.JSON, pet.TextoUsuario) {
		if errCache := g.cache.Guardar(ctx, huella, r.JSON, VidaCache); errCache != nil {
			g.log.Warn("ia: no se guardó en caché", "parte", "cache", "error", errCache.Error())
		}
	}
	return r, err
}

// gastado lee el contador; si no existe lo reconstruye desde usos_ia, y si Redis falla usa
// usos_ia directamente. Sin ninguno de los dos se cierra: no se llama al proveedor.
func (g *Guardia) gastado(ctx context.Context, mes string, ahora time.Time) (int64, error) {
	valor, ok, err := g.contador.Leer(ctx, mes)
	if err == nil && ok {
		return valor, nil
	}
	total, errMes := g.registro.GastoDelMes(ctx, mes)
	if errMes != nil {
		return 0, ErrGastoDesconocido
	}
	if err == nil {
		if errFijar := g.contador.Fijar(ctx, mes, total, vidaMes(ahora)); errFijar != nil {
			g.log.Warn("ia: no se reconstruyó el contador", "parte", "gasto", "error", errFijar.Error())
		}
	}
	return total, nil
}

func (g *Guardia) leerCache(ctx context.Context, huella string) ([]byte, bool) {
	datos, ok, err := g.cache.Leer(ctx, huella)
	if err != nil {
		// La caché no cierra (nv-28): sin Redis se llama al proveedor con el tope de usos_ia.
		g.log.Warn("ia: caché no disponible", "parte", "cache", "error", err.Error())
		return nil, false
	}
	return datos, ok
}

func (g *Guardia) registrar(ctx context.Context, ahora time.Time, r Respuesta, costo int64, resultado string) {
	u := Uso{
		UsuarioID:     usuarioDe(ctx),
		Modelo:        g.modelo.Nombre(),
		Mes:           Mes(ahora),
		TokensEntrada: r.TokensEntrada,
		TokensSalida:  r.TokensSalida,
		CostoMicroUSD: costo,
		Resultado:     resultado,
		DuracionMS:    r.Duracion.Milliseconds(),
		CreadoEn:      ahora.UTC(),
		ExpiraEn:      ahora.UTC().Add(VidaUso),
	}
	if err := g.registro.Guardar(ctx, u); err != nil {
		g.log.Warn("ia: no se registró el uso", "parte", "usos_ia", "error", err.Error())
	}
}

// resultados traduce cada error al resultado que se registra; el orden importa poco porque
// los errores no se envuelven entre sí.
var resultados = []struct {
	err       error
	resultado string
}{
	{ErrLimiteProveedor, ResultadoLimite},
	{ErrTiempo, ResultadoTiempo},
	{ErrRespuestaInvalida, ResultadoInvalida},
	{ErrBloqueada, ResultadoBloqueada},
	{ErrLimiteUsuario, ResultadoLimiteUsuario},
	{ErrLimiteGlobal, ResultadoLimiteGlobal},
	{ErrLimiteDesconocido, ResultadoSinLimite},
}

func resultadoDe(err error) string {
	if err == nil {
		return ResultadoOK
	}
	for _, r := range resultados {
		if errors.Is(err, r.err) {
			return r.resultado
		}
	}
	return ResultadoError
}

// Normalizar es la forma de la frase que entra a la huella: anonimizada, en minúsculas y con
// los espacios colapsados.
func Normalizar(texto string) string {
	return strings.Join(strings.Fields(strings.ToLower(Anonimizar(texto))), " ")
}

// Huella es el SHA-256 de modelo, instrucciones, contexto (la lista de candidatos), frase
// normalizada, esquema y tope de salida. Cada parte lleva su largo para que no se confundan.
func Huella(modelo string, pet Peticion) string {
	esquema, err := json.Marshal(pet.Esquema)
	if err != nil {
		esquema = nil
	}
	h := sha256.New()
	for _, parte := range []string{modelo, pet.Instrucciones, pet.Contexto, Normalizar(pet.TextoUsuario), string(esquema), strconv.Itoa(pet.MaxTokens)} {
		_, _ = h.Write([]byte(strconv.Itoa(len(parte)) + ":" + parte + "|"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// contieneTexto dice si la salida repite la frase del usuario; esa salida no va a la caché.
func contieneTexto(salida []byte, texto string) bool {
	frase := Normalizar(texto)
	if frase == "" {
		return false
	}
	return strings.Contains(strings.Join(strings.Fields(strings.ToLower(string(salida))), " "), frase)
}

// Uso es una llamada registrada en usos_ia. Sin texto del usuario ni salida del modelo.
type Uso struct {
	ID            bson.ObjectID `bson:"_id,omitempty"`
	UsuarioID     bson.ObjectID `bson:"usuario_id,omitempty"`
	Modelo        string        `bson:"modelo"`
	Mes           string        `bson:"mes"`
	TokensEntrada int           `bson:"tokens_entrada"`
	TokensSalida  int           `bson:"tokens_salida"`
	CostoMicroUSD int64         `bson:"costo_micro_usd"`
	Resultado     string        `bson:"resultado"`
	DuracionMS    int64         `bson:"duracion_ms"`
	CreadoEn      time.Time     `bson:"creado_en"`
	ExpiraEn      time.Time     `bson:"expira_en"`
}
