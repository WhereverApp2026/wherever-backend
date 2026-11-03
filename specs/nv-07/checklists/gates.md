# Checklist — nv-07 Despliegue de desarrollo en AWS

> Trazabilidad: `spec R1-R9 ↔ plan §3-§5 ↔ task T1-T7`. Estado: Borrador con evidencia (aplicado el 2026-11-03; faltan ciclo de apagado, `destroy` y firma).

- [x] Una sola instancia de tipo elegible; créditos de CPU `standard`; IMDSv2 (R1) — `verificar.sh`: `t3.micro` FreeTierEligible, `standard`, `required`
- [x] Disco gp3 cifrado de máximo 30 GB (R2) — 20 GB gp3 cifrado
- [ ] Apagado diario 22:00 y encendido L-V 08:00 en `America/Mexico_City`; fines de semana apagada (R3) — reglas `wherever-dev-apagar`/`-encender` ENABLED; falta ver un disparo real
- [x] Security Group: 80 y 443; 22 solo desde tu IP; app solo desde tu IP; sin puertos de bases de datos (R4) — 22 y 8080 solo desde `la IP de quien desarrolla (/32)`; salida 443/80/27017
- [ ] HTTPS con certificado válido que sobrevive al apagado (R5) — certificado de Let's Encrypt válido en `https://18-227-230-50.sslip.io`; falta probar tras el apagado
- [x] Secretos solo en Parameter Store; ninguno en Git ni en el estado (R6) — `MONGO_URI`, `JWT_KEY`, `PHONE_HMAC_KEY` SecureString en `/wherever/dev/`; `terraform.tfstate` sin coincidencias y fuera de Git
- [x] Presupuesto con alerta por correo (R7) — `wherever-dev-mensual` de **US$5** (decisión del usuario: la IP elástica sola cuesta ~US$3.6/mes), 50 % real y 100 % previsto, sin créditos
- [ ] `scripts/verificar.sh` en verde con la instancia encendida y apagada (R8) — encendida: OK con `/ready` por HTTPS; falta apagada
- [ ] `terraform destroy` y `apply` repetibles; `fmt`, `validate` y `trivy` limpios (R9) — `fmt`/`validate` OK, Trivy limpio con la excepción AWS-0104; `apply` hecho, un segundo `plan` sin cambios; `destroy` sin probar
- [x] Una IP elástica asociada, ninguna suelta, sin NAT ni balanceador — `verificar.sh`
- [ ] Ciclo de apagado y encendido probado: IP fija, contenedores vuelven solos, `/ready` responde
- [x] `Memoria.md` con fecha de fin del plan gratuito, excepción de Parameter Store y deuda de producción
- [ ] Aprobación firmada: (pendiente de revisor)

## Evidencia (2026-11-03, Terraform 1.16.6, AWS provider 6.x, perfil `wherever-deployer` en `816101201001`)
- `terraform apply`: 21 recursos (19 en el primer intento; 2 reglas fallaron por el apóstrofo en la descripción y se crearon tras corregirla). Un `plan` posterior: sin cambios.
- Instancia `i-056aba7efc9f57205`, IP elástica `18.227.230.50`; arranque terminado (Docker 25.0.14, Compose v2.39.4, swap 2 GB); SSM `Online`.
- `/ready` → 200 `{"mongodb":"ok","redis":"ok","neo4j":"ok","indices_mongodb":"ok"}` por HTTPS; `/health` 200; `http://` → 308 a `https://`; `/docs` 404 (staging); cabeceras HSTS, CSP y `X-Request-Id` presentes; `Via: 1.1 Caddy`.
- Memoria en la instancia con todo arriba: ~96 MB disponibles y 173 MB de swap en uso.
- Despliegue: el `docker build` local se colgó dos veces en `go mod download` (proxy con `unexpected EOF`); se compiló el binario Linux en local con la caché de módulos y se empaquetó sobre la misma base distroless. El resto de `desplegar.sh` (copiar imagen, `deploy-env.sh`, `compose up`) se corrió igual.
