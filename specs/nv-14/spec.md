# NV-14 — Infraestructura como paso configurado (crear, desplegar y eliminar)

## Contexto
La especificación del curso pide que la creación de la infraestructura y el despliegue sean un paso configurado, y que también lo sea la eliminación de lo desplegado y de la infraestructura. Hoy Terraform corre a mano y el CD solo actualiza la imagen; una instancia recreada ni siquiera se instalaría sola. El docente confirmó que bastan scripts versionados, así que esta spec los agrega, sin claves de AWS en GitHub. **Depende de**: NV-08, NV-11 y NV-13; el ciclo real se hace después de NV-12.

## Requisitos
1. `infra/aws-dev/scripts/crear.sh` aplica Terraform y deja la API corriendo: espera a Systems Manager, sube los archivos, arma `.env` desde Parameter Store, levanta Caddy y activa los timers. Es idempotente.
2. `infra/aws-dev/scripts/destruir.sh` elimina lo que creó Terraform.
3. Ambos muestran el plan antes de actuar y exigen escribir `CREAR` o `DESTRUIR`. Con `--solo-plan` no cambian nada. Solo aplican el plan guardado en esa misma corrida.
4. Credenciales: el perfil del deployer en la máquina del operador. Nada de claves de AWS en GitHub, como se decidió en NV-08.
5. El estado de Terraform sigue local, con respaldo antes y después de cada corrida en `~/wherever-dev-terraform-respaldo`.
6. **Primer arranque:** `actualizar.sh` no exige `/ready` 200 si aún no hay versión instalada. La imagen la instala el actualizador, igual que en un despliegue normal.
7. `crear.sh` imprime los pasos manuales con sus valores y espera hasta 20 minutos a que `/ready` dé 200 por HTTPS. Hasta que Atlas permita la IP nueva, `/indexes` falla y el actualizador reintenta cada minuto, sin marcar la imagen como fallida.
8. Los recursos externos (Parameter Store, Atlas, Aura, Upstash, GHCR, boundary y políticas) no se crean ni se destruyen aquí.
9. La infraestructura puede destruirse y recrearse de principio a fin sin pasos ocultos: el README lista cada paso manual.

## Diseño
- `crear.sh` y `destruir.sh` usan `terraform plan -out` y `apply` de ese archivo.
- `instalar-actualizador.sh` hace el primer arranque solo si no existe `.env`, así que es seguro correrlo en un servidor vivo. Espera a `cloud-init` antes de subir nada, porque `user_data` crea `/opt/wherever` al final.
- Las salidas de Terraform (`instance_ids`, `public_ips`, `target_group_arn`, `cloudfront_url`) alimentan los scripts.

## Contratos
Sin endpoints nuevos ni recursos nuevos de Terraform. Enmienda de la constitución §3: la infraestructura de desarrollo se crea y elimina con scripts versionados.

## Tests
- `go build ./...`, `go vet ./...` y las cinco suites sin cambios.
- `bash -n` de los scripts; `terraform fmt`, `validate`, `trivy config infra/`.
- `crear.sh --solo-plan`: sin cambios sobre la infraestructura actual. `destruir.sh --solo-plan`: lista todo sin borrar nada.
- Sin escribir la confirmación, ninguno de los dos cambia nada.
- `instalar-actualizador.sh` en el servidor vivo: no repite el primer arranque ni reinicia la API.
- `crear.sh` sobre una instancia nueva (la B de NV-12): primer arranque e instalación sola en cuanto Atlas permite su IP.
- **Ciclo completo:** `destruir.sh` y `crear.sh` recrean todo; `/ready` 200 por HTTPS; `/health` con el SHA de `main`; los pasos manuales bastan.

## Criterios
- Crear y eliminar la infraestructura es un paso versionado y reproducible.
- Sin claves de AWS en GitHub.
- Costo adicional cero.

## Límites
- El ciclo completo corta el servicio y cambia la URL, porque la IP elástica es otra: se hace una sola vez, después de NV-12 y antes de congelar la app móvil.
- Corre en la máquina del operador. El estado local depende de esa máquina y de sus respaldos: si se pierden, hay que borrar a mano en la consola.
- Si una imagen falla en el primer arranque, no hay versión anterior a la que regresar: el actualizador reintenta cada minuto.
- Atlas, SNS y la URL tienen pasos manuales al recrear.
- Permisos de borrado del deployer sin probar hasta el ciclo real.
- Solo desarrollo.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Implementada en la rama `nv-14-infra-scripts`, sin el ciclo real; sin firmar
- **Revisores:** Osmar Reséndiz (pendiente de firmar)
- **Fecha:** 2026-11-06 (ejecución)
- **Decisiones:** (a) respuesta del docente: bastan scripts versionados, según Osmar Reséndiz, 2026-11-06. Con eso salen el workflow `infra-dev.yml`, el usuario `wherever-dev-pipeline` y sus claves en GitHub. (b) El estado sigue local: el bucket de S3 solo hacía falta para correr en GitHub. Si se quiere remoto, es un cambio aparte.
- **Detalle:** la evidencia va en `specs/nv-14/checklists/gates.md` + `Memoria.md`.
