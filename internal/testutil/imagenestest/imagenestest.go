// Package imagenestest arma el módulo de imágenes (nv-45) para pruebas: un doble del bucket
// (S3 es una interfaz externa) y el moderador real de nv-42 contra el servidor local de
// Rekognition. MongoDB y Redis son reales.
package imagenestest

import (
	"context"
	"log/slog"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/testutil/rekognitiontest"
)

// URLPublica es la del CloudFront de imágenes en pruebas.
const URLPublica = "https://imagenes.test"

// Objeto es lo que quedó guardado en el doble.
type Objeto struct {
	Tipo  string
	Datos []byte
	Cache string
}

// Firma es una subida firmada que pidió el service.
type Firma struct {
	Clave    string
	Tipo     string
	MaxBytes int64
	Vence    time.Duration
}

// Almacen es el doble del bucket: guarda en memoria y puede fallar a pedido.
type Almacen struct {
	mu      sync.Mutex
	objetos map[string]Objeto
	firmas  []Firma
	falla   error
}

// NuevoAlmacen devuelve el doble vacío.
func NuevoAlmacen() *Almacen { return &Almacen{objetos: map[string]Objeto{}} }

// Fallar hace que las siguientes operaciones devuelvan err (nil vuelve a la normalidad).
func (a *Almacen) Fallar(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.falla = err
}

// Subir hace lo que haría la app con el formulario firmado: deja el original en cuarentena.
func (a *Almacen) Subir(id string, tipo string, datos []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.objetos["cuarentena/"+id] = Objeto{Tipo: tipo, Datos: datos}
}

// Objeto devuelve lo guardado en la clave.
func (a *Almacen) Objeto(clave string) (Objeto, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	o, ok := a.objetos[clave]
	return o, ok
}

// Claves devuelve las claves guardadas, ordenadas.
func (a *Almacen) Claves() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, 0, len(a.objetos))
	for k := range a.objetos {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// Firmas devuelve las subidas firmadas, en orden.
func (a *Almacen) Firmas() []Firma {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]Firma(nil), a.firmas...)
}

// FirmarSubida cumple imagenes.Almacen.
func (a *Almacen) FirmarSubida(_ context.Context, clave, tipo string, maxBytes int64, vence time.Duration) (imagenes.Subida, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.falla != nil {
		return imagenes.Subida{}, a.falla
	}
	a.firmas = append(a.firmas, Firma{Clave: clave, Tipo: tipo, MaxBytes: maxBytes, Vence: vence})
	return imagenes.Subida{URL: "https://bucket.s3.test/", Campos: map[string]string{"key": clave, "Content-Type": tipo, "policy": "prueba"}}, nil
}

// Leer cumple imagenes.Almacen.
func (a *Almacen) Leer(_ context.Context, clave string, max int64) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.falla != nil {
		return nil, a.falla
	}
	o, ok := a.objetos[clave]
	if !ok {
		return nil, imagenes.ErrSinObjeto
	}
	if int64(len(o.Datos)) > max {
		return nil, imagenes.ErrObjetoGrande
	}
	return append([]byte(nil), o.Datos...), nil
}

// Guardar cumple imagenes.Almacen.
func (a *Almacen) Guardar(_ context.Context, clave, tipo string, datos []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.falla != nil {
		return a.falla
	}
	a.objetos[clave] = Objeto{Tipo: tipo, Datos: append([]byte(nil), datos...), Cache: imagenes.CacheInmutable}
	return nil
}

// Borrar cumple imagenes.Almacen.
func (a *Almacen) Borrar(_ context.Context, claves ...string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.falla != nil {
		return a.falla
	}
	for _, k := range claves {
		delete(a.objetos, k)
	}
	return nil
}

// Modulo es lo que la prueba necesita para manejar el módulo.
type Modulo struct {
	Service     *imagenes.Service
	Privacy     *imagenes.Privacy
	Almacen     *Almacen
	Rekognition *rekognitiontest.Server
}

// Build arma el service con el doble del bucket y el Rekognition local.
func Build(t testing.TB, db *mongo.Database, log *slog.Logger) Modulo {
	t.Helper()
	rek := rekognitiontest.New(t)
	adaptador, err := moderacion.NewRekognition(context.Background(), moderacion.RekognitionConfig{
		Region: "us-east-2", Endpoint: rek.URL, Credenciales: rekognitiontest.Credenciales(),
	})
	require.NoError(t, err)
	almacen := NuevoAlmacen()
	repo := imagenes.NewRepository(db)
	mod := moderacion.New(adaptador, nil, moderacion.Fijas(moderacion.ReglasPorDefecto()), log)
	svc, err := imagenes.NewService(repo, almacen, mod, imagenes.Config{URLPublica: URLPublica}, log)
	require.NoError(t, err)
	return Modulo{Service: svc, Privacy: imagenes.NewPrivacy(repo, almacen, URLPublica), Almacen: almacen, Rekognition: rek}
}
