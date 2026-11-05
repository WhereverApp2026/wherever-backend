# Checklist — nv-15 Correcciones de la auditoría

> Trazabilidad: `spec R1-R5 ↔ plan §3-§5 ↔ task T1-T5`. Estado: Implementada, sin firmar.

- [ ] `JOB_HOUR` en 21 y el trabajo corre en el servidor (R1) — código y `TestLoad_JobHour` listos; falta ver la corrida a las 21:00
- [x] Sin consentimiento, las alergias dan 422 y no se guardan; sin alergias no hace falta (R2) — `TestConsentimiento_SinElLasAlergiasNoSeGuardan`, `TestConsentimiento_SinAlergiasNoHaceFalta`
- [x] Retirarlo borra las alergias, también si no hay gustos; las previas se ocultan (R3) — `TestConsentimiento_ConElSeGuardanYAlRetirarloSeBorran`, `TestConsentimiento_RetirarloSinGustos`
- [ ] `HIDE_INTERNALS` oculta `detalle` y sondas; prod siempre (R4) — `TestConfig_ExposeInternals`, `TestInternos_*`; falta comprobar en el servidor
- [x] `docs/endpoints.md` corregido sobre `/ready` (R5)
- [x] Capturas de contrato: solo se agrega `datos_sensibles` en 5 archivos
- [ ] Aprobación firmada: (pendiente de revisor)

## Evidencia local (2026-11-05, Go 1.27.3)
- Suites: Unit 583, Security 46, Database 4, Contract 3, sin fallos. Integration 111, con la nota de abajo.
- golangci-lint 0, Semgrep 0, `critic` PASS, `check_endpoints` OK.
- **Prueba intermitente:** `TestMailer_RegisterSendsVerification` falló 1 de 4 corridas de Integration en Windows: ningún correo en Mailpit en 10 s, en el primer envío de la suite. No toca este cambio. Sin causa confirmada; no ha fallado en CI.
