# NV-23 — Filtros duros y puntuación de recomendaciones

## Contexto
El motor recomienda desde el primer día sin datos de entrenamiento y es el plan B de la IA. **Origen**: US-166, US-025, US-037 y hoja A2 del Excel. **Depende de**: NV-20, NV-05.

## Requisitos
1. **Filtros duros** (nunca se recomienda lo que los incumple): negocio no aprobado, cerrado a esa hora, alergias y dieta del usuario, accesibilidad, mascotas, niños, y los marcados con «No me interesa».
2. **Puntuación ponderada** de 0 a 100 por presupuesto, distancia, horario, clima, gustos y calificación.
3. Cada recomendación incluye `razones[]` (los factores que más sumaron) y `por_que`, una línea en español legible (US-037).

## Diseño
- **Paquete** `internal/recomendador`: `Filtro` (reglas duras: `Condiciones()` para MongoDB y `Cumple()` en Go), `Puntaje` (una función por factor), `Explicador` (`Razones` y `PorQue`) y `Contexto` (lo arma quien llama).
- **Consulta:** `negocios.Repository.Candidates` reutiliza `List` de nv-21 por distancia (`$geoNear` con `VisibleFilter` y los filtros duros en `query`, antes del `$limit`), sin índices nuevos; `negocios.OpenAtCondition` expone `openAtFilter`. Hasta 200 candidatos en el radio (5 km por defecto, 20 km máx., como nv-21). `Ordenar` vuelve a aplicar `Filtro.Cumple` en Go como red de seguridad.
- **Pesos** en `internal/recomendador/pesos_recomendador.json`, embebido en el binario: presupuesto 0,20; distancia 0,20; horario 0,10; clima 0,10; gustos 0,25; calificación 0,15. `ParsePesos`/`Validate` exigen los seis factores en [0, 1] que sumen 1 (±0,001); `Recomendar` devuelve `ErrPesos` si no.
- **Factores (0–1):** presupuesto (1 dentro del rango o debajo; cae lineal hasta 0 al doble del máximo; sin presupuesto o sin `precio_promedio`, 0,5), distancia (1 − d/radio), horario (1 si sigue abierto 30 min después, si no 0), clima (0,5 fijo hasta nv-25), gustos (coincidencias de `gustos.categorias` con el giro, su nombre o una etiqueta, normalizadas; 2 o más = 1; sin gustos 0,5), calificación (promedio/5; sin reseñas 0,5). Total = 100 × Σ peso × factor, a un decimal.
- **Explicación:** `razones[]` son hasta 3 factores con valor ≥ 0,6, del que más aportó al que menos (el clima neutro nunca entra); `por_que` = «Te lo recomendamos porque a, b y c.» o, sin razones, «Está abierto y cumple con lo que pediste.» Sin alergias, dieta, cifras ni coordenadas.
- **Determinista:** misma entrada, mismo resultado; los empates de puntaje se rompen por calificación y luego por id.
- **Reutilizable:** `Recomendar(ctx, fuente, pesos, contexto, n)` es una función de paquete (`n` ≤ 50); el asistente (nv-31) y el inicio (nv-24) la usan.
- **Alérgenos (pendiente de nv-20):** `alergenos[]` lista lo que el negocio contiene; `["ninguno"]` (`negocios.AllergenNone`, va solo; la semilla lo valida) dice «declaró que no contiene ninguno»; una lista vacía es **sin dato**. Con alergias del catálogo pasa solo quien tiene dato y no contiene ninguna (`alergenos.0` existe y `$nin`); si el texto libre de `gustos.alergias` trae algo fuera del catálogo (`ParseAlergias` → `AlergiaSinCatalogo`), pasa solo `["ninguno"]`. Sin alergias declaradas el dato no importa.
- **Dieta, accesibilidad y «No me interesa»:** `DietasDe` toma de `gustos.restricciones` solo las del catálogo (`opciones_dieta $all`); silla de ruedas, niños y mascota llegan en el contexto y exigen la bandera de `accesibilidad`; los excluidos llegan como ids (`_id $nin`): todavía no hay colección de «No me interesa», la llena su spec.

## Contratos
Sin endpoints nuevos. Cambio de datos: `alergenos` acepta `ninguno` (solo) en la semilla de nv-20.

## Tests
- Unit `internal/recomendador/filtro_test.go` (11): cada filtro duro por separado y combinados, sin dato de alérgenos, alergia fuera de catálogo, `ParseAlergias`, `DietasDe`.
- Unit `internal/recomendador/puntaje_test.go` (11): casos fijos por factor, total fijo, desempates, pesos inválidos (`ErrPesos`).
- Unit `internal/recomendador/explicador_test.go` (5): elige los factores correctos; texto sin datos sensibles.
- Unit `internal/negocios/import_test.go`: `ninguno` va solo.
- Integration `tests/integration/recomendador_test.go` (3): 10 negocios fijos con orden y puntajes esperados, dos corridas iguales, ninguna alergia se cuela, sin alergias entra el sin dato.

## Criterios
- Con 10 negocios fijos el orden coincide con el esperado por el equipo.
- Ninguna recomendación incumple una alergia declarada.

## Límites
- Sin aprendizaje automático: los pesos son fijos (el ML llega con nv-50 y nv-51).
- La distancia es en línea recta.

## Aprobación y Control de Cambios
- **Estado:** 🚧 Borrador con evidencia (2026-11-10): implementada en la rama `nv-23`; falta la firma.
- **Revisores:** Osmar Reséndiz (pendiente de aprobar)
- **Fecha:** —
- **Decisiones para el revisor:** pesos propuestos; `["ninguno"]` contra lista vacía «sin dato»; los pesos se validan al llamar `Recomendar` (no al arrancar la API, porque aún no hay consumidor: nv-24 lo cablea al arrancar).
- **Detalle:** la evidencia irá en `specs/nv-23/checklists/gates.md` + `Memoria.md`.
