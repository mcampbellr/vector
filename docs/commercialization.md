# Vector — Comercialización / Go-to-Market (análisis `/biz`)

> Estado: análisis estratégico en fase de concepto (sin usuarios ni landing aún). Estrategia
> de monetización y distribución desde el día 0, no optimización de un funnel existente.

## Resumen ejecutivo

Vector vive sobre Claude Code: audiencia nichada pero en crecimiento rápido y con alta
disposición a pagar (ya pagan API/Max). El núcleo (CLI + commands + rules) es trivialmente
clonable → no monetiza por sí solo. El foso real es doble: (1) la **capa de equipo** (board
hosteado, sync de estado, integraciones de tickets) y (2) el wedge diferencial: **eficiencia
de tokens medible**. Vector es el único en posición de decir "te ahorré $X de Claude este
mes", convirtiendo un dev-tool (difícil de monetizar) en un producto con **ROI cuantificable**.
Estrategia ganadora: **open-core** — CLI gratis para distribución viral, cobrar por equipo +
analytics de ahorro.

## Problemas clave (rankeados por impacto)

1. **El valor core es copiable** — skills/rules/commands en repo público se forkean en un día.
   Sin capa propietaria (sync/hosting/analytics) no hay qué cobrar.
2. **"Organizar specs" es vitamina, no analgésico** — nadie paga por orden; pagan por dolor
   (gasto de tokens descontrolado, caos de equipo). Liderar con eso, no con "kanban para devs".
3. **`/vector init` reorganiza el repo del usuario = fricción/miedo altísimos** — es el momento
   de mayor abandono y el paso más riesgoso de activación.
4. **El ahorro de tokens hoy es invisible** — si no se instrumenta y se muestra, el mejor
   argumento de venta no existe para el usuario.
5. **Dependencia de plataforma** — Anthropic puede absorber features. Mitigar con diseño
   agent/model-agnostic.

## Oportunidades y experimentos (priorizados)

1. **Token Savings Meter (máxima prioridad)** — instrumentar tokens/$ ahorrados por el ruteo a
   agentes baratos; mostrarlo en el dashboard y en `/vector:daily`. Es la métrica aha y el
   gancho central de pricing (loss aversion + ROI tangible).
2. **Open-core + Team tier hosteado** — CLI/commands/dashboard local = OSS gratis (motor de
   adquisición); pago = board hosteado, sync multi-máquina, seats, integraciones Jira/Linear/GitHub.
3. **Design-partner program (3–5 equipos en Claude Code)** — onboarding manual gratis a cambio
   de feedback + medición de ahorro real; valida pricing antes de construir billing.
4. **Reducir miedo de `/vector init`** — modo `--dry-run` que muestra el diff de reorganización
   sin aplicar + backup explícito. A/B: init directo vs preview-first (mide tasa de completar init).
5. **Landing/waitlist con test de mensaje** — A/B headline "Ahorra tokens de Claude en tu
   equipo" (analgésico) vs "Kanban de specs para devs" (vitamina). Telemetría opt-in del
   install-script = métrica de activación day 0.

## Recomendaciones estratégicas

- **Corto plazo:** token-meter desde el primer commit; waitlist + landing con test de mensaje;
  `--dry-run` en init.
- **Mediano:** open-core público (estrellas GitHub = distribución); Team tier con board hosteado
  + 1 integración de tickets (la que usen los design partners).
- **Largo:** agent/model-agnostic (hedge anti-plataforma); ser dueño de la categoría
  "agent-native spec tracking con economía de tokens" (hoy vacía: Linear/Jira = PM;
  GSD/OpenSpec = complementarios, no competidores).

## Insights avanzados

- **Loss aversion + anchoring:** anclar contra el gasto mensual de Claude API ("gastaste $180;
  Vector cuesta $12 y te ahorró $40") → el costo de Vector se vuelve trivial.
- **Lock-in benigno:** adoptar la estructura de Vector + board compartido hace doloroso migrar →
  retención por estructura, no por contrato.
- **Growth loop:** OSS → stars → `/vector init` estandariza el repo → teammate ve el orden →
  adopta → invita → expansión por seat. El **daily notes** crea el hábito diario que sostiene el loop.

## Pricing tentativo (validar con design partners)

| Tier | Precio aprox | Para quién | Incluye |
|------|--------------|------------|---------|
| **Free / OSS** | $0 | solo-dev | CLI, todos los `/vector:*`, dashboard local, estado por-spec |
| **Pro** | ~$8–12/mo | solo-dev avanzado | analytics de ahorro de tokens, sync personal multi-máquina |
| **Team** | ~$15–20/seat/mo | equipos | board hosteado, sync en tiempo real, roles/audit, integraciones de tickets |

## Métricas a instrumentar desde day 0

- Activación: % que completa `/vector init` tras instalar.
- Aha: tokens/$ ahorrados acumulados por usuario.
- Hábito: uso de `/vector:daily` (DAU del dashboard).
- Expansión: invitaciones por workspace, seats por equipo.
