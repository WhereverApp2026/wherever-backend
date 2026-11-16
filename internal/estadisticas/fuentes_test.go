package estadisticas

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/WhereverApp2026/wherever-backend/internal/cupones"
	"github.com/WhereverApp2026/wherever-backend/internal/favoritos"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/resenas"
	"github.com/WhereverApp2026/wherever-backend/internal/reservas"
)

// El repositorio lee otras colecciones por nombre para no importar sus paquetes; si alguna
// cambia de nombre o de valores, esta prueba avisa.
func TestFuentes_NombresIgualesALosModulos(t *testing.T) {
	assert.Equal(t, negocios.Collection, colNegocios)
	assert.Equal(t, reservas.Collection, colReservas)
	assert.Equal(t, resenas.CheckinsCollection, colCheckins)
	assert.Equal(t, resenas.Collection, colResenas)
	assert.Equal(t, cupones.Collection, colCupones)
	assert.Equal(t, cupones.CarteraCollection, colCuponesUsuario)
	assert.Equal(t, cupones.MetricasCollection, colCuponesMetricas)
	assert.Equal(t, favoritos.Collection, colFavoritos)
	assert.Equal(t, reservas.EstadoBloqueada, reservaBloqueada)
	assert.Equal(t, reservas.EstadoExpirada, reservaExpirada)
	assert.Equal(t, resenas.EstadoPublicada, resenaPublicada)
	assert.Equal(t, favoritos.TypeBusiness, favoritoNegocio)
	assert.Equal(t, negocios.StatusApproved, negocioAprobado)
	assert.Equal(t, negocios.StatusPaused, negocioPausado)
}
