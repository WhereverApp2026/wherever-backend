package privacidad

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// module es un Purger/Exporter de prueba: no es una base simulada, es un módulo registrado
// que anota en qué orden lo llamaron.
type module struct {
	name  string
	calls *[]string
	fail  error
	data  any
}

func (m module) Name() string { return m.name }

func (m module) PurgeHistory(context.Context, bson.ObjectID) error {
	*m.calls = append(*m.calls, m.name+":historial")
	return m.fail
}

func (m module) PurgeAccount(context.Context, bson.ObjectID) error {
	*m.calls = append(*m.calls, m.name+":cuenta")
	return m.fail
}

func (m module) Export(context.Context, bson.ObjectID) (any, error) { return m.data, m.fail }

func TestRegistry_RunsPurgersInRegistrationOrder(t *testing.T) {
	var calls []string
	r := NewRegistry()
	r.RegisterPurger(module{name: "sesiones", calls: &calls})
	r.RegisterPurger(module{name: "usuarios", calls: &calls})

	require.NoError(t, r.PurgeAccount(context.Background(), bson.NewObjectID()))
	require.NoError(t, r.PurgeHistory(context.Background(), bson.NewObjectID()))

	assert.Equal(t, []string{"sesiones:cuenta", "usuarios:cuenta", "sesiones:historial", "usuarios:historial"}, calls)
}

func TestRegistry_OneFailureDoesNotStopTheOthers(t *testing.T) {
	var calls []string
	r := NewRegistry()
	r.RegisterPurger(module{name: "planes", calls: &calls, fail: errors.New("sin conexión")})
	r.RegisterPurger(module{name: "usuarios", calls: &calls})

	err := r.PurgeAccount(context.Background(), bson.NewObjectID())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "planes: sin conexión")
	assert.Equal(t, []string{"planes:cuenta", "usuarios:cuenta"}, calls)
}

func TestRegistry_ExportKeepsWhatWorksAndReportsTheRest(t *testing.T) {
	r := NewRegistry()
	r.RegisterExporter(module{name: "usuarios", data: map[string]string{"nombre": "Ana"}})
	r.RegisterExporter(module{name: "resenas", fail: errors.New("caída")})

	out, err := r.Export(context.Background(), bson.NewObjectID())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "resenas")
	assert.Equal(t, map[string]any{"usuarios": map[string]string{"nombre": "Ana"}}, out)
}

func TestRegistry_EmptyIsNoOp(t *testing.T) {
	r := NewRegistry()
	assert.NoError(t, r.PurgeAccount(context.Background(), bson.NewObjectID()))
	out, err := r.Export(context.Background(), bson.NewObjectID())
	assert.NoError(t, err)
	assert.Empty(t, out)
}

// flaky falla en el primer intento y funciona en los siguientes; anota qué usuarios borró.
type flaky struct {
	name     string
	attempts *int
	purged   map[bson.ObjectID]bool
}

func (f flaky) Name() string { return f.name }

func (f flaky) PurgeHistory(_ context.Context, userID bson.ObjectID) error {
	*f.attempts++
	if *f.attempts == 1 {
		return errors.New("tiempo agotado")
	}
	f.purged[userID] = true
	return nil
}

func (f flaky) PurgeAccount(ctx context.Context, userID bson.ObjectID) error {
	return f.PurgeHistory(ctx, userID)
}

// DELETE /usuarios/yo/historial da 503 si un módulo falla a medias; reintentar termina el
// borrado porque cada Purger es idempotente (volver a borrar lo ya borrado no falla).
func TestRegistry_RetryAfterPartialFailureFinishes(t *testing.T) {
	var calls []string
	attempts := 0
	purged := map[bson.ObjectID]bool{}
	r := NewRegistry()
	r.RegisterPurger(module{name: "sesiones", calls: &calls})
	r.RegisterPurger(flaky{name: "planes", attempts: &attempts, purged: purged})
	r.RegisterPurger(module{name: "usuarios", calls: &calls})
	user := bson.NewObjectID()

	first := r.PurgeHistory(context.Background(), user)
	require.Error(t, first)
	assert.Contains(t, first.Error(), "planes: tiempo agotado")
	assert.False(t, purged[user])
	assert.Equal(t, []string{"sesiones:historial", "usuarios:historial"}, calls, "los demás módulos sí corrieron")

	require.NoError(t, r.PurgeHistory(context.Background(), user), "el reintento termina el trabajo")
	assert.True(t, purged[user])
	assert.Equal(t, []string{"sesiones:historial", "usuarios:historial", "sesiones:historial", "usuarios:historial"}, calls)
}
