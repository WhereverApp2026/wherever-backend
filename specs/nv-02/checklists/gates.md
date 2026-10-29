# Checklist — nv-02 Molde

> Trazabilidad: `spec §2-§6 ↔ plan §3-§5 ↔ task T1`. Estado: Borrador sin ejecutar.

- [ ] `PageResult` con `items/siguiente_cursor/limite` y cursor opaco
- [ ] Registro único de validators; `PaginationQuery` válido (1,20) / inválido (0,500)
- [ ] Errores tipados de constitución §5 mapeados (400/401/403/404/408/409/410/422/423/429/503)
- [ ] `arch_test (6)` y `cursor_test (4)` verdes
- [ ] Ningún handler nuevo, ninguna colección, `endpoints.md` intacto
- [ ] `critic PASS`, `endpoints OK`, `coverage ≥45%`, `semgrep 0`
- [ ] Aprobación firmada: (pendiente de revisor)
