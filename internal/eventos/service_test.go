package eventos

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
)

type negociosFijos map[bson.ObjectID]negocios.Business

func (n negociosFijos) FindByID(_ context.Context, id bson.ObjectID) (negocios.Business, error) {
	b, ok := n[id]
	if !ok {
		return negocios.Business{}, negocios.ErrNotFound
	}
	return b, nil
}

var ahora = time.Date(2026, 11, 20, 18, 0, 0, 0, time.UTC)

func servicio(n negociosFijos) *Service {
	s := NewService(nil, n, nil, "https://imagenes.test", nil)
	s.now = func() time.Time { return ahora }
	return s
}

func TestFechas_BordesYDuracion(t *testing.T) {
	s := servicio(nil)
	h := time.Hour
	assert.NoError(t, s.fechas(ahora.Add(-30*time.Minute), ahora.Add(h)))
	assert.Error(t, s.fechas(ahora.Add(h), ahora.Add(h)), "fin = inicio")
	assert.Error(t, s.fechas(ahora.Add(-2*h), ahora.Add(h)), "más de una hora en el pasado")
	assert.Error(t, s.fechas(ahora.Add(181*24*h), ahora.Add(182*24*h)), "más de 180 días")
	assert.Error(t, s.fechas(ahora, ahora.Add(31*24*h)), "más de 30 días")
	assert.NoError(t, s.fechas(ahora, ahora.Add(30*24*h)))
}

func TestFoto_SoloImagenesPropias(t *testing.T) {
	s := servicio(nil)
	assert.NoError(t, s.foto(""))
	assert.NoError(t, s.foto("https://imagenes.test/eventos/a.webp"))
	assert.Error(t, s.foto("https://imagenes.test.evil.com/a.webp"))
	assert.Error(t, s.foto("https://otro.com/a.webp"))
}

func TestPermisos_Edicion(t *testing.T) {
	creador, dueno, otro := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	negocio := bson.NewObjectID()
	s := servicio(negociosFijos{negocio: {ID: negocio, OwnerID: dueno}})
	e := Evento{CreadoPor: &creador, NegocioID: &negocio}
	claims := func(id bson.ObjectID, roles ...string) *middleware.Claims {
		cl := &middleware.Claims{Roles: roles}
		cl.Subject = id.Hex()
		return cl
	}
	for _, c := range []struct {
		cl   *middleware.Claims
		want bool
	}{
		{claims(creador, middleware.RoleMerchant), true},
		{claims(dueno, middleware.RoleMerchant), true},
		{claims(otro, middleware.RoleAdmin), true},
		{claims(otro, middleware.RoleMerchant), false},
		{nil, false},
	} {
		ok, err := s.puedeEditar(context.Background(), c.cl, e)
		require.NoError(t, err)
		assert.Equal(t, c.want, ok)
	}
}

func TestNegocioPublicable_ComercianteSoloAprobadoPropio(t *testing.T) {
	me, otro := bson.NewObjectID(), bson.NewObjectID()
	ok, ajeno, revision, pausado := bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID(), bson.NewObjectID()
	s := servicio(negociosFijos{
		ok:       {ID: ok, OwnerID: me, State: negocios.StatusApproved},
		ajeno:    {ID: ajeno, OwnerID: otro, State: negocios.StatusApproved},
		revision: {ID: revision, OwnerID: me, State: negocios.StatusReview},
		pausado:  {ID: pausado, OwnerID: me, State: negocios.StatusApproved, Paused: &negocios.Pausa{}},
	})
	merchant := &middleware.Claims{Roles: []string{middleware.RoleMerchant}}
	_, err := s.negocioPublicable(context.Background(), merchant, me, ok.Hex())
	assert.NoError(t, err)
	for _, id := range []bson.ObjectID{ajeno, revision, pausado, bson.NewObjectID()} {
		_, err := s.negocioPublicable(context.Background(), merchant, me, id.Hex())
		assert.Error(t, err)
	}
	_, err = s.negocioPublicable(context.Background(), merchant, me, "")
	assert.Error(t, err, "el comerciante necesita negocio")
	admin := &middleware.Claims{Roles: []string{middleware.RoleAdmin}}
	got, err := s.negocioPublicable(context.Background(), admin, me, "")
	assert.NoError(t, err)
	assert.Nil(t, got)
}

func TestEstados_CanceladoNoSeReabre(t *testing.T) {
	e := Evento{Titulo: "Feria", Direccion: "Centro", Estado: EstadoCancelado}
	publicado := EstadoPublicado
	_, err := aplicarCampos(&e, EditarRequest{Estado: &publicado})
	require.NoError(t, err, "aplicarCampos solo copia; Editar rechaza antes con 409")
	assert.False(t, Evento{Estado: EstadoBorrador}.Visible())
	assert.False(t, Evento{Estado: EstadoPublicado, EnRevision: true}.Visible())
	assert.True(t, Evento{Estado: EstadoPublicado}.Visible())
}

func TestDTO_Validacion(t *testing.T) {
	v, err := validation.New()
	require.NoError(t, err)
	lat, lng, precio := 19.4, -99.1, int64(0)
	ok := CrearRequest{Titulo: "Tianguis", Tipo: TipoTianguis, Ubicacion: &Coordenadas{Lat: &lat, Lng: &lng}, Direccion: "Calle 1", Inicio: "x", Fin: "y", Precio: &precio}
	assert.NoError(t, v.Struct(ok))
	bad := ok
	bad.Tipo = "concierto"
	assert.Error(t, v.Struct(bad))
	bad = ok
	bad.Precio = nil
	assert.Error(t, v.Struct(bad))
	q := NuevaListaQuery()
	assert.Error(t, v.Struct(q), "lat y lng obligatorios")
	q.Lat, q.Lng = &lat, &lng
	assert.NoError(t, v.Struct(q))
	q.Dias = 31
	assert.Error(t, v.Struct(q))
}
