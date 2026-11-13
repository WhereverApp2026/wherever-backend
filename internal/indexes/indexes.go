// Package indexes reúne los índices de MongoDB de todos los módulos. Lo usan
// scripts/indexes (único lugar que los crea, constitución §5) y la suite Database.
package indexes

import (
	"github.com/WhereverApp2026/wherever-backend/internal/amistades"
	"github.com/WhereverApp2026/wherever-backend/internal/asistente"
	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/busquedas"
	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/favoritos"
	"github.com/WhereverApp2026/wherever-backend/internal/grafo"
	"github.com/WhereverApp2026/wherever-backend/internal/grupos"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/moderacion"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios/expediente"
	"github.com/WhereverApp2026/wherever-backend/internal/notificaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/recomendaciones"
	"github.com/WhereverApp2026/wherever-backend/internal/sincronizacion"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/votaciones"
)

// All son los índices de todos los módulos; cada módulo nuevo agrega aquí los suyos.
func All() []mongo.IndexSet {
	var all []mongo.IndexSet
	for _, sets := range [][]mongo.IndexSet{usuarios.Indexes(), usuarios.ConsentIndexes(), auth.Indexes(), ciudades.Indexes(), negocios.Indexes(), expediente.Indexes(), privacidad.Indexes(), ia.IndexesUsos(), planes.Indexes(), asistente.Indexes(), recomendaciones.Indexes(), busquedas.Indexes(), amistades.Indexes(), grupos.Indexes(), favoritos.Indexes(), votaciones.Indexes(), moderacion.Indexes(), notificaciones.Indexes(), sincronizacion.Indexes(), sincronizacion.ActionIndexes(), grafo.Indexes()} {
		all = append(all, sets...)
	}
	return all
}
