# task.md — NV-26 Cliente de IA (Gemini) y salida validada

## Fase 0 — Preparación
- [x] T01 Decisión del revisor (Gemini por cuota cero en Bedrock); rama `nv-26`.

## Fase 1 — Implementación
- [x] T02 Interfaz `ia.Modelo` y adaptador de Gemini (`generateContent`) con salida JSON con esquema.
- [x] T03 Llave en `GEMINI_API_KEY` (SSM o `.env`), en encabezado; modelo en `IA_MODELO_ID`; el logger oculta llaves de Google.
- [x] T04 Salida validada con un reintento; 429 como `ErrLimiteProveedor` sin reintento.
- [x] T05 Anonimizar el texto del usuario antes de enviarlo.
- [x] T06 Specs nv-27, nv-28, nv-31, nv-33 y nv-50 ajustadas; aviso de privacidad del proveedor de IA.

## Fase 2 — Cierre
- [x] T07 Llave real en SSM; prueba en vivo con las 20 frases; latencia y tasa de 429 en `gates.md`.
- [ ] T08 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR.
