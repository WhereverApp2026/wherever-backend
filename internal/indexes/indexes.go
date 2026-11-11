// Package indexes reúne los índices de MongoDB de todos los módulos. Lo usan
// scripts/indexes (único lugar que los crea, constitución §5) y la suite Database.
package indexes

import (
	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/ciudades"
	"github.com/WhereverApp2026/wherever-backend/internal/ia"
	"github.com/WhereverApp2026/wherever-backend/internal/negocios"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/privacidad"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// All son los índices de todos los módulos; cada módulo nuevo agrega aquí los suyos.
func All() []mongo.IndexSet {
	var all []mongo.IndexSet
	for _, sets := range [][]mongo.IndexSet{usuarios.Indexes(), auth.Indexes(), ciudades.Indexes(), negocios.Indexes(), privacidad.Indexes(), ia.IndexesUsos()} {
		all = append(all, sets...)
	}
	return all
}
