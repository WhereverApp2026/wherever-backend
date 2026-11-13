package sincronizacion

import (
	"context"
	"encoding/json"
	"errors"

	"go.mongodb.org/mongo-driver/v2/bson"

	"github.com/WhereverApp2026/wherever-backend/internal/http/middleware"
	"github.com/WhereverApp2026/wherever-backend/internal/planes"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/apperr"
	"github.com/WhereverApp2026/wherever-backend/internal/shared/validation"
	"github.com/WhereverApp2026/wherever-backend/internal/usuarios/account"
)

// Tipos de acción que se aceptan sin conexión (nv-47).
const (
	ActionPlanCreate  = "plan.crear"
	ActionPlanEdit    = "plan.editar"
	ActionPlanDelete  = "plan.borrar"
	ActionStopAdd     = "parada.agregar"
	ActionStopEdit    = "parada.editar"
	ActionStopRemove  = "parada.quitar"
	ActionPrefsEdit   = "preferencias.editar"
	ActionPrivacyEdit = "privacidad.editar"
)

// Resultados de cada acción.
const (
	ResultApplied   = "aplicada"
	ResultDuplicate = "duplicada"
	ResultConflict  = "conflicto"
	ResultRejected  = "rechazada"
)

// PlanWriter es lo que la sincronización hace con los planes: lo mismo que la API (nv-30),
// así cada cambio se numera igual (nv-46).
type PlanWriter interface {
	Create(ctx context.Context, subject string, req planes.CreateRequest) (planes.Plan, error)
	Get(ctx context.Context, subject, rawID string) (planes.Plan, error)
	Patch(ctx context.Context, subject, rawID string, version int, req planes.PatchRequest) (planes.Plan, error)
	Delete(ctx context.Context, subject, rawID string, version int) error
	AddStop(ctx context.Context, subject, rawID string, version int, req planes.StopRequest) (planes.Plan, error)
	PatchStop(ctx context.Context, subject, rawID, rawStopID string, version int, req planes.PatchStopRequest) (planes.Plan, error)
	RemoveStop(ctx context.Context, subject, rawID, rawStopID string, version int) (planes.Plan, error)
}

// SettingsWriter cambia las preferencias y la privacidad (nv-05); ya suben la versión del
// perfil, que es lo que ve la descarga.
type SettingsWriter interface {
	PatchPreferences(ctx context.Context, claims *middleware.Claims, req account.PreferencesRequest) (account.PreferencesResponse, error)
	PatchPrivacy(ctx context.Context, claims *middleware.Claims, req account.PrivacyRequest) (account.PrivacyResponse, error)
}

// Pending aplica los lotes de acciones hechas sin conexión.
type Pending struct {
	actions   *Actions
	plans     PlanWriter
	settings  SettingsWriter
	users     Users
	validator *validation.Validator
}

// NewPending recibe sus dependencias por constructor.
func NewPending(actions *Actions, plans PlanWriter, settings SettingsWriter, users Users, v *validation.Validator) *Pending {
	return &Pending{actions: actions, plans: plans, settings: settings, users: users, validator: v}
}

// outcome es lo que deja una acción antes de registrarla.
type outcome struct {
	result   ActionResult
	planID   *bson.ObjectID
	infraErr bool
}

// Apply procesa el lote en orden. Cada acción se reclama primero (`id_cliente` único): si ya
// se vio, responde `duplicada` sin tocar nada. Un conflicto de versión no sobrescribe: trae la
// copia del servidor. Si la base falla a la mitad, lo ya aplicado queda registrado y la
// acción que falló se suelta para el siguiente intento (503).
func (p *Pending) Apply(ctx context.Context, claims *middleware.Claims, req PendingRequest) (PendingResponse, error) {
	me, err := bson.ObjectIDFromHex(claims.Subject)
	if err != nil {
		return PendingResponse{}, apperr.Unauthorized("autenticación requerida")
	}
	out := PendingResponse{Results: make([]ActionResult, 0, len(req.Actions))}
	for _, a := range req.Actions {
		rec, claimed, err := p.actions.Claim(ctx, me, a.ClientID)
		if err != nil {
			return PendingResponse{}, errStore()
		}
		if !claimed {
			out.Results = append(out.Results, duplicateOf(rec))
			continue
		}
		o := p.apply(ctx, claims, a)
		if o.infraErr {
			_ = p.actions.Release(ctx, rec.ID)
			return PendingResponse{}, errStore()
		}
		if err := p.actions.Finish(ctx, rec.ID, o.result.Result, o.planID); err != nil {
			return PendingResponse{}, errStore()
		}
		out.Results = append(out.Results, o.result)
	}
	return out, nil
}

func duplicateOf(rec ActionRecord) ActionResult {
	r := ActionResult{ClientID: rec.ClientID, Result: ResultDuplicate, Original: rec.Result}
	if rec.PlanID != nil {
		r.PlanID = rec.PlanID.Hex()
	}
	return r
}

// apply decodifica y valida los datos de la acción y la ejecuta.
func (p *Pending) apply(ctx context.Context, claims *middleware.Claims, a Action) outcome {
	switch a.Type {
	case ActionPlanCreate, ActionPlanEdit, ActionPlanDelete, ActionStopAdd, ActionStopEdit, ActionStopRemove:
		return p.applyPlan(ctx, claims.Subject, a)
	default:
		return p.applySettings(ctx, claims, a)
	}
}

// decode lee `datos` en dst y lo valida con las mismas reglas de la API.
func (p *Pending) decode(a Action, dst any) error {
	if len(a.Data) == 0 {
		return apperr.Validation("faltan los datos de la acción", apperr.FieldError{Field: "datos", Message: "requerido"})
	}
	if err := json.Unmarshal(a.Data, dst); err != nil {
		return apperr.BadRequest("datos ilegibles")
	}
	return p.validator.Struct(dst)
}

func (p *Pending) applyPlan(ctx context.Context, subject string, a Action) outcome {
	plan, err := p.runPlan(ctx, subject, a)
	if err == nil {
		r := ActionResult{ClientID: a.ClientID, Result: ResultApplied}
		var id *bson.ObjectID
		if a.Type != ActionPlanDelete {
			r.Plan, r.PlanID, id = &plan, plan.ID.Hex(), &plan.ID
		}
		return outcome{result: r, planID: id}
	}
	return p.failed(ctx, subject, a, err)
}

func (p *Pending) runPlan(ctx context.Context, subject string, a Action) (planes.Plan, error) {
	switch a.Type {
	case ActionPlanCreate:
		var req planes.CreateRequest
		if err := p.decode(a, &req); err != nil {
			return planes.Plan{}, err
		}
		return p.plans.Create(ctx, subject, req)
	case ActionPlanEdit:
		var req planes.PatchRequest
		if err := p.decode(a, &req); err != nil {
			return planes.Plan{}, err
		}
		return p.plans.Patch(ctx, subject, a.PlanID, a.Version, req)
	case ActionPlanDelete:
		return planes.Plan{}, p.plans.Delete(ctx, subject, a.PlanID, a.Version)
	default:
		return p.runStop(ctx, subject, a)
	}
}

func (p *Pending) runStop(ctx context.Context, subject string, a Action) (planes.Plan, error) {
	switch a.Type {
	case ActionStopAdd:
		var req planes.StopRequest
		if err := p.decode(a, &req); err != nil {
			return planes.Plan{}, err
		}
		return p.plans.AddStop(ctx, subject, a.PlanID, a.Version, req)
	case ActionStopEdit:
		var req planes.PatchStopRequest
		if err := p.decode(a, &req); err != nil {
			return planes.Plan{}, err
		}
		return p.plans.PatchStop(ctx, subject, a.PlanID, a.StopID, a.Version, req)
	default:
		return p.plans.RemoveStop(ctx, subject, a.PlanID, a.StopID, a.Version)
	}
}

// failed traduce el error: versión vieja = conflicto con la copia del servidor; cualquier
// error de la petición = rechazada; un fallo de la base detiene el lote.
func (p *Pending) failed(ctx context.Context, subject string, a Action, err error) outcome {
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Kind == apperr.KindUnavailable || appErr.Kind == apperr.KindInternal {
		return outcome{infraErr: true}
	}
	r := ActionResult{ClientID: a.ClientID, Result: ResultRejected, Error: &ActionError{Code: appErr.Kind.Code(), Message: appErr.Message, Fields: appErr.Fields}}
	if appErr.Kind == apperr.KindConflict {
		r.Result, r.Error = ResultConflict, nil
		if server, getErr := p.plans.Get(ctx, subject, a.PlanID); getErr == nil {
			r.Plan, r.PlanID = &server, server.ID.Hex()
		}
	}
	return outcome{result: r}
}

// applySettings cambia preferencias o privacidad. Si la acción trae `version` y el perfil ya
// cambió, es conflicto y viajan los ajustes del servidor.
func (p *Pending) applySettings(ctx context.Context, claims *middleware.Claims, a Action) outcome {
	me, _ := bson.ObjectIDFromHex(claims.Subject) // ya validado en Apply
	u, err := p.users.FindByID(ctx, me)
	if err != nil {
		return outcome{infraErr: true}
	}
	if a.Version != 0 && a.Version != u.Version {
		settings := newSettings(u)
		return outcome{result: ActionResult{ClientID: a.ClientID, Result: ResultConflict, Settings: &settings}}
	}
	if err := p.runSettings(ctx, claims, a); err != nil {
		return p.failed(ctx, claims.Subject, a, err)
	}
	u, err = p.users.FindByID(ctx, me)
	if err != nil {
		return outcome{infraErr: true}
	}
	settings := newSettings(u)
	return outcome{result: ActionResult{ClientID: a.ClientID, Result: ResultApplied, Settings: &settings}}
}

func (p *Pending) runSettings(ctx context.Context, claims *middleware.Claims, a Action) error {
	if a.Type == ActionPrefsEdit {
		var req account.PreferencesRequest
		if err := p.decode(a, &req); err != nil {
			return err
		}
		_, err := p.settings.PatchPreferences(ctx, claims, req)
		return err
	}
	var req account.PrivacyRequest
	if err := p.decode(a, &req); err != nil {
		return err
	}
	_, err := p.settings.PatchPrivacy(ctx, claims, req)
	return err
}
