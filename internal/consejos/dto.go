package consejos

// ConsejoResponse es la respuesta de GET /negocios/{id}/consejo. `fuente` dice si lo redactó la
// IA o la plantilla de la regla; `dia` es el día local del consejo.
type ConsejoResponse struct {
	Texto  string `json:"texto"`
	Accion string `json:"accion"`
	Fuente string `json:"fuente"`
	Dia    string `json:"dia"`
}
