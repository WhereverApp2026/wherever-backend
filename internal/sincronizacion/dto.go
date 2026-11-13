package sincronizacion

import (
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios/account"
)

// Límites de la descarga.
const (
	DefaultLimit = 100
	MaxLimit     = 200
)

// ChangesQuery es GET /sync/cambios.
type ChangesQuery struct {
	From  string `query:"desde" validate:"omitempty,max=300,cursor"`
	Limit int    `query:"limite" validate:"min=1,max=200"`
}

// NewChangesQuery trae el límite por defecto.
func NewChangesQuery() ChangesQuery { return ChangesQuery{Limit: DefaultLimit} }

// DeletedItem es algo que la app quita de su copia.
type DeletedItem struct {
	Kind string `json:"tipo"`
	ID   string `json:"id"`
}

// Settings son los ajustes que la app guarda: preferencias, privacidad y gustos, con la
// versión del perfil (para el If-Match de sus ediciones).
type Settings struct {
	Version     int                         `json:"version"`
	Preferences account.PreferencesResponse `json:"preferencias"`
	Privacy     account.PrivacyResponse     `json:"privacidad"`
	Tastes      *account.TastesResponse     `json:"gustos"`
}

// ChangesResponse es la respuesta: lo que cambió, el cursor para la siguiente vez y si
// quedan más páginas (`hay_mas`: volver a pedir con `siguiente` antes de dar por terminado).
type ChangesResponse struct {
	Plans    []planes.Plan `json:"planes"`
	Deleted  []DeletedItem `json:"borrados"`
	Settings *Settings     `json:"ajustes,omitempty"`
	Next     string        `json:"siguiente"`
	More     bool          `json:"hay_mas"`
}

func newSettings(u usuarios.User) Settings {
	tastes, prefs, privacy := account.SettingsOf(u)
	return Settings{Version: u.Version, Preferences: prefs, Privacy: privacy, Tastes: tastes}
}
