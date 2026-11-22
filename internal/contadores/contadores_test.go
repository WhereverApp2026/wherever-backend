package contadores_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/contadores"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/votaciones"
)

// Los nombres se repiten aquí para no importar los módulos (ciclo); deben seguir iguales.
func TestNombres_IgualesALosDeCadaModulo(t *testing.T) {
	assert.Equal(t, planes.Collection, contadores.ColPlanes)
	assert.Equal(t, planes.StatusActive, contadores.PlanActivo)
	assert.Equal(t, planes.VisibilityPublic, contadores.PlanPublico)
	assert.Equal(t, amistades.Collection, contadores.ColAmistades)
	assert.Equal(t, amistades.StatusAccepted, contadores.AmistadAceptada)
	assert.Equal(t, resenas.Collection, contadores.ColResenas)
	assert.Equal(t, resenas.EstadoPublicada, contadores.ResenaVisible)
	assert.Equal(t, votaciones.Collection, contadores.ColVotaciones)
	assert.Equal(t, votaciones.StatusOpen, contadores.VotoAbierto)
	assert.Equal(t, grupos.Collection, contadores.ColGrupos)
	assert.Equal(t, grupos.StatusConfirmed, contadores.Confirmado)
}

func TestClave_EntidadDeLaConstitucion(t *testing.T) {
	id := bson.NewObjectID()
	assert.Equal(t, "cache:perfil_contadores:"+id.Hex(), contadores.Clave(id))
}

func TestInvalidar_NilNoFalla(t *testing.T) {
	var s *contadores.Service
	assert.NotPanics(t, func() { s.Invalidar(context.Background(), bson.NewObjectID()) })
	sinCache := contadores.NewService(&fuente{}, nil, nil)
	assert.NotPanics(t, func() { sinCache.Invalidar(context.Background(), bson.NewObjectID()) })
}

type fuente struct {
	desde, hasta time.Time
	contados     int
}

func (f *fuente) Contar(context.Context, bson.ObjectID) (contadores.Contadores, error) {
	f.contados++
	return contadores.Contadores{Planes: 1}, nil
}

func (f *fuente) DeAmigos(_ context.Context, _ []bson.ObjectID, desde, hasta, _ time.Time) (map[bson.ObjectID]contadores.Amigo, error) {
	f.desde, f.hasta = desde, hasta
	return map[bson.ObjectID]contadores.Amigo{}, nil
}

// «Hoy» es el día local de la zona dada, no el de UTC.
func TestDeAmigos_HoyEnLaZona(t *testing.T) {
	loc := negocios.MexicoCity()
	f := &fuente{}
	s := contadores.NewService(f, nil, loc)
	_, err := s.DeAmigos(context.Background(), []bson.ObjectID{bson.NewObjectID()})
	require.NoError(t, err)
	assert.Equal(t, loc, f.desde.Location())
	assert.Zero(t, f.desde.Hour())
	assert.Equal(t, 24*time.Hour, f.hasta.Sub(f.desde))
	n, err := s.De(context.Background(), bson.NewObjectID())
	require.NoError(t, err)
	assert.Equal(t, int64(1), n.Planes, "sin caché va directo a la fuente")
}
