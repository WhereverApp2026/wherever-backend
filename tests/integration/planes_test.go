package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
)

// planEnv inserta un negocio aprobado abierto toda la semana y uno en borrador.
func planEnv(t *testing.T) (*planes.Repository, *planes.Calculadora, bson.ObjectID, bson.ObjectID) {
	t.Helper()
	env := negociosEnv(t)
	ok := placeDoc(placeSpec{slug: "plan-abierto", category: "cafeteria", windows: weekdays()})
	draft := placeDoc(placeSpec{slug: "plan-borrador", category: "cafeteria", windows: weekdays(), state: negocios.StatusDraft, dLat: 0.009})
	_, err := env.Mongo.DB().Collection(negocios.Collection).InsertMany(context.Background(), []any{ok, draft})
	require.NoError(t, err)
	calc := planes.NewCalculadora(negocios.NewRepository(env.Mongo.DB()), planes.DefaultVelocidades())
	return planes.NewRepository(env.Mongo.DB()), calc, ok.ID, draft.ID
}

func onePlan(owner, business bson.ObjectID) *planes.Plan {
	return &planes.Plan{
		OwnerID: owner, Nombre: "Domingo", Fecha: lunesMediodia, Personas: 2, Movilidad: planes.MobilityWalk, Origen: planes.OriginManual,
		Paradas: []planes.Parada{{ID: bson.NewObjectID(), NegocioID: business, Llega: lunesMediodia, Minutos: 60, Precio: 150}},
	}
}

func TestPlanes_CalculadoraValidaContraNegocios(t *testing.T) {
	_, calc, ok, draft := planEnv(t)
	p := onePlan(bson.NewObjectID(), ok)
	require.NoError(t, calc.Aplicar(context.Background(), p))
	assert.Equal(t, 300, p.CostoTotal)

	p.Paradas = append(p.Paradas, planes.Parada{ID: bson.NewObjectID(), NegocioID: draft, Llega: lunesMediodia.Add(2 * time.Hour), Minutos: 30})
	var se *planes.StopError
	require.ErrorAs(t, calc.Aplicar(context.Background(), p), &se)
	assert.Equal(t, planes.ReasonUnavailable, se.Reason)
}

func TestPlanes_SoloElDuenoLoLee(t *testing.T) {
	repo, calc, ok, _ := planEnv(t)
	owner := bson.NewObjectID()
	p := onePlan(owner, ok)
	require.NoError(t, calc.Aplicar(context.Background(), p))
	require.NoError(t, repo.Create(context.Background(), p))

	got, err := repo.FindOwned(context.Background(), owner, p.ID)
	require.NoError(t, err)
	assert.Equal(t, 1, got.Version)
	_, err = repo.FindOwned(context.Background(), bson.NewObjectID(), p.ID)
	assert.ErrorIs(t, err, planes.ErrNotFound)
}

func TestPlanes_IfMatchGanaUno(t *testing.T) {
	repo, calc, ok, _ := planEnv(t)
	owner := bson.NewObjectID()
	p := onePlan(owner, ok)
	require.NoError(t, calc.Aplicar(context.Background(), p))
	require.NoError(t, repo.Create(context.Background(), p))

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := range errs {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			edit := *p
			edit.Nombre = "Edición"
			errs[i] = repo.UpdateIfVersion(context.Background(), &edit, 1)
		}(i)
	}
	wg.Wait()
	wins := 0
	for _, err := range errs {
		if err == nil {
			wins++
		} else {
			assert.ErrorIs(t, err, planes.ErrVersionConflict)
		}
	}
	assert.Equal(t, 1, wins)

	stranger := *p
	stranger.OwnerID = bson.NewObjectID()
	assert.ErrorIs(t, repo.UpdateIfVersion(context.Background(), &stranger, 2), planes.ErrNotFound)
}

func TestPlanes_ARCOExportaYBorraSoloLosDelDueno(t *testing.T) {
	repo, calc, ok, _ := planEnv(t)
	owner, other := bson.NewObjectID(), bson.NewObjectID()
	for _, o := range []bson.ObjectID{owner, owner, other} {
		p := onePlan(o, ok)
		require.NoError(t, calc.Aplicar(context.Background(), p))
		require.NoError(t, repo.Create(context.Background(), p))
	}
	privacy := planes.NewPrivacy(repo)
	exported, err := privacy.Export(context.Background(), owner)
	require.NoError(t, err)
	assert.Len(t, exported, 2)

	require.NoError(t, privacy.PurgeAccount(context.Background(), owner))
	left, err := repo.ListByOwner(context.Background(), owner)
	require.NoError(t, err)
	assert.Empty(t, left)
	kept, err := repo.ListByOwner(context.Background(), other)
	require.NoError(t, err)
	assert.Len(t, kept, 1)
}
