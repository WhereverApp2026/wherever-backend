package ia

import "regexp"

// Lo que se reconoce como dato personal en una frase libre. Un nombre propio no se puede
// distinguir con certeza; por eso la app pide no escribir datos personales (nv-28).
var anonimizadores = []struct {
	patron    *regexp.Regexp
	reemplazo string
}{
	{regexp.MustCompile(`(?i)\b(?:https?://|www\.)\S+`), "[enlace]"},
	{regexp.MustCompile(`[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}`), "[correo]"},
	{regexp.MustCompile(`@[A-Za-z0-9_.]{2,}`), "[usuario]"},
	{regexp.MustCompile(`-?\b\d{1,3}\.\d{4,}\b`), "[coordenada]"},
	{regexp.MustCompile(`\+?\(?\d(?:[\s().-]*\d){7,14}`), "[teléfono]"},
}

// Anonimizar quita de un texto libre correos, teléfonos, enlaces, usuarios y coordenadas
// antes de mandarlo al proveedor. Los montos cortos («500 pesos») y las horas se conservan.
func Anonimizar(texto string) string {
	for _, a := range anonimizadores {
		texto = a.patron.ReplaceAllString(texto, a.reemplazo)
	}
	return texto
}
