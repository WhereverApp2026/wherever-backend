package sincronizacion

import (
	"encoding/json"

	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
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

// MaxActions es el tamaño máximo de un lote (spec nv-47).
const MaxActions = 100

// Action es algo que la persona hizo sin conexión. `id_cliente` lo genera la app y es único
// por persona: repetirlo no repite la acción. `version` es la que la app tenía (If-Match).
type Action struct {
	ClientID string          `json:"id_cliente" validate:"required,min=8,max=64"`
	Type     string          `json:"tipo" validate:"required,oneof=plan.crear plan.editar plan.borrar parada.agregar parada.editar parada.quitar preferencias.editar privacidad.editar"`
	PlanID   string          `json:"plan_id" validate:"required_unless=Type plan.crear Type preferencias.editar Type privacidad.editar,omitempty,len=24,hexadecimal"`
	StopID   string          `json:"parada_id" validate:"required_if=Type parada.editar,required_if=Type parada.quitar,omitempty,len=24,hexadecimal"`
	Version  int             `json:"version" validate:"omitempty,min=1"`
	Data     json.RawMessage `json:"datos"`
}

// PendingRequest es el cuerpo de POST /sync/pendientes.
type PendingRequest struct {
	Actions []Action `json:"acciones" validate:"required,min=1,max=100,dive"`
}

// ActionError explica una acción rechazada, con el formato de los errores de la API.
type ActionError struct {
	Code    string              `json:"codigo"`
	Message string              `json:"mensaje"`
	Fields  []apperr.FieldError `json:"campos,omitempty"`
}

// ActionResult es el resultado de una acción, en el mismo orden del lote. En un conflicto
// viaja la copia del servidor (`plan` o `ajustes`) y la app decide.
type ActionResult struct {
	ClientID string       `json:"id_cliente"`
	Result   string       `json:"resultado"`
	Original string       `json:"resultado_original,omitempty"`
	PlanID   string       `json:"plan_id,omitempty"`
	Plan     *planes.Plan `json:"plan,omitempty"`
	Settings *Settings    `json:"ajustes,omitempty"`
	Error    *ActionError `json:"error,omitempty"`
}

// PendingResponse es la respuesta de POST /sync/pendientes.
type PendingResponse struct {
	Results []ActionResult `json:"resultados"`
}
