# task.md — NV-26 Cliente de Bedrock y salida validada

## Fase 0 — Preparación
- [ ] T01 Aprobar la spec; rama `nv-26`.

## Fase 1 — Implementación
- [ ] T02 Interfaz ia.Modelo con un adaptador de Amazon Bedrock (API Converse) que usa el rol…
- [ ] T03 Prueba previa de la organización: una llamada de prueba desde la instancia por…
- [ ] T04 Salida validada: el modelo responde JSON con esquema; una respuesta que no valida se…
- [ ] T05 Permisos mínimos: bedrock:InvokeModel solo sobre el modelo elegido en el rol de la…

## Fase 2 — Cierre
- [ ] T06 Pruebas, documentos y `Memoria.md`.
- [ ] T07 `go build`, `go vet`, cinco suites, `critic`, `check_sdd`, `check_endpoints`; PR; evidencia en `gates.md`.
