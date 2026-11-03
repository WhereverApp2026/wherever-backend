// Package indexes reúne los índices de MongoDB de todos los módulos. Lo usan
// scripts/indexes (único lugar que los crea, constitución §5) y la suite Database.
package indexes

import (
	"github.com/WhereverApp2026/wherever-backend/internal/auth"
	"github.com/WhereverApp2026/wherever-backend/internal/platform/mongo"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
)

// All son los índices de todos los módulos; cada módulo nuevo agrega aquí los suyos.
func All() []mongo.IndexSet {
	return append(usuarios.Indexes(), auth.Indexes()...)
}
