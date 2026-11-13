// Package negociostest arma los dobles de las interfaces externas del alta de negocios
// (nv-55): el bucket privado de S3 y, hasta que nv-45 llegue a main, las imágenes publicadas.
// La base, Redis y el cifrado son reales.
package negociostest

import (
	"context"
	"errors"
	"sort"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
)

// ErrObjeto: el objeto no existe en el doble del bucket.
var ErrObjeto = errors.New("negociostest: el objeto no existe")

// Almacen es el doble del bucket privado: guarda en memoria y puede fallar a pedido.
type Almacen struct {
	mu      sync.Mutex
	objetos map[string][]byte
	falla   error
}

// NuevoAlmacen devuelve el doble vacío.
func NuevoAlmacen() *Almacen { return &Almacen{objetos: map[string][]byte{}} }

// Fallar hace que todas las operaciones devuelvan err (nil lo quita).
func (a *Almacen) Fallar(err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.falla = err
}

// Objeto devuelve los bytes guardados con esa clave.
func (a *Almacen) Objeto(clave string) ([]byte, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	b, ok := a.objetos[clave]
	return b, ok
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

// Leer cumple expediente.Almacen.
func (a *Almacen) Leer(_ context.Context, clave string, max int64) ([]byte, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.falla != nil {
		return nil, a.falla
	}
	b, ok := a.objetos[clave]
	if !ok || int64(len(b)) > max {
		return nil, ErrObjeto
	}
	return append([]byte(nil), b...), nil
}

// Guardar cumple expediente.Almacen.
func (a *Almacen) Guardar(_ context.Context, clave, _ string, datos []byte) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.falla != nil {
		return a.falla
	}
	a.objetos[clave] = append([]byte(nil), datos...)
	return nil
}

// Borrar cumple expediente.Almacen.
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

// Imagenes es el doble de las imágenes publicadas de nv-45: la prueba publica una imagen para
// una persona y el alta la puede registrar en la galería.
type Imagenes struct {
	mu        sync.Mutex
	publicada map[string]bson.ObjectID
}

// NuevasImagenes devuelve el doble vacío.
func NuevasImagenes() *Imagenes { return &Imagenes{publicada: map[string]bson.ObjectID{}} }

// Publicar deja la imagen publicada a nombre de owner.
func (i *Imagenes) Publicar(owner bson.ObjectID, id string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.publicada[id] = owner
}

// Publicada cumple alta.Imagenes.
func (i *Imagenes) Publicada(_ context.Context, owner bson.ObjectID, id string) (string, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if dueno, ok := i.publicada[id]; !ok || dueno != owner {
		return "", alta.ErrImagenNoValida
	}
	return "https://img.wherever.test/i/" + id + "/800.webp", nil
}
