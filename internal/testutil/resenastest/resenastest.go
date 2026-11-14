// Package resenastest arma las reseñas (nv-62) para las pruebas: el moderador de texto de nv-42
// es la única interfaz externa simulada (Rekognition y el modelo de lenguaje).
package resenastest

import (
	"context"
	"errors"
	"sync"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/imagenes"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/alta"
)

// Moderador devuelve el veredicto fijado (aprobado por defecto) y cuenta las llamadas.
type Moderador struct {
	mu        sync.Mutex
	veredicto moderacion.Veredicto
	llamadas  int
}

// NuevoModerador aprueba todo hasta que la prueba fije otro veredicto.
func NuevoModerador() *Moderador { return &Moderador{veredicto: moderacion.VeredictoAprobado} }

// Fijar cambia el veredicto de las siguientes evaluaciones.
func (m *Moderador) Fijar(v moderacion.Veredicto) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.veredicto = v
}

// Llamadas dice cuántas veces se evaluó un texto.
func (m *Moderador) Llamadas() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.llamadas
}

// Evaluar cumple resenas.Moderador.
func (m *Moderador) Evaluar(_ context.Context, _ moderacion.Contenido) moderacion.Resultado {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.llamadas++
	return moderacion.Resultado{Veredicto: m.veredicto, Puntajes: map[string]float64{moderacion.FiltroAcoso: 50}}
}

// Publicadas son las imágenes publicadas de una prueba (el doble de nv-55).
type Publicadas interface {
	Publicada(ctx context.Context, dueno bson.ObjectID, id string) (string, error)
}

// Fotos adapta el doble de imágenes del alta a resenas.Fotos: una imagen ajena o sin publicar
// responde imagenes.ErrNoPublicada, como el módulo real.
type Fotos struct{ P Publicadas }

// Publicada cumple resenas.Fotos.
func (f Fotos) Publicada(ctx context.Context, dueno bson.ObjectID, id string) (string, error) {
	url, err := f.P.Publicada(ctx, dueno, id)
	if errors.Is(err, alta.ErrImagenNoValida) {
		return "", imagenes.ErrNoPublicada
	}
	return url, err
}
