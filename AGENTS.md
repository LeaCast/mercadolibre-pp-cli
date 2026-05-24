# Guía para agentes — mercadolibre-pp-cli

Este directorio es una CLI `mercadolibre-pp-cli` generada por [Printing Press](https://github.com/mvanhorn/cli-printing-press), así que los fixes sistémicos los tratás primero como fixes upstream de Printing Press. Mantené las ediciones locales acotadas y documentá por qué un patch al árbol generado vive acá.

## Contrato operativo local

Empezá pidiéndole a la CLI generada la verdad de runtime actual:

```bash
mercadolibre-pp-cli doctor --json
mercadolibre-pp-cli agent-context --pretty
```

Usá discovery en runtime en vez de depender de una lista de comandos copiada:

```bash
mercadolibre-pp-cli which "<capacidad>" --json
mercadolibre-pp-cli <comando> --help
```

Agregá `--agent` a las invocaciones para output JSON, compacto, defaults no-interactivos, sin color y scripting confirmation-safe:

```bash
mercadolibre-pp-cli <comando> --agent
```

Antes de correr un comando desconocido que puede mutar estado remoto, inspeccioná su help y preferí un dry run:

```bash
mercadolibre-pp-cli <comando> --help
mercadolibre-pp-cli <comando> --dry-run --agent
```

Usá `--yes --no-input` solo después de que el target, los argumentos y los side effects estén claros.

Para instalación, auth, ejemplos y guía de producto más extensa, leé `README.md` y `SKILL.md`. Este archivo se mantiene chico a propósito para que los agentes con scope local tengan guidance invariante sin duplicar los docs generados.

## Customizaciones locales

Si modificás esta CLI más allá de lo que produjo el generador, registrá cada customización en un `.printing-press-patches.json` en la raíz de esta CLI (al lado de `.printing-press.json`) para que el cambio no se pierda en la próxima regeneración y sea visible para el próximo lector.

Forma mínima:

```json
{
  "schema_version": 1,
  "applied_at": "YYYY-MM-DD",
  "base_run_id": "<copiá de .printing-press.json>",
  "base_printing_press_version": "<copiá de .printing-press.json>",
  "patches": [
    {
      "id": "identificador-corto",
      "summary": "Qué cambió (una oración).",
      "reason": "Por qué se necesitó esta customización (una o dos oraciones).",
      "files": ["internal/cli/foo.go"],
      "validated_outcome": "Opcional: resultado de test no-obvio que confirma el fix."
    }
  ]
}
```

Usá `deferred_to_upstream` cuando un patch local sea un puente temporal para un endpoint de API público faltante, un workaround de host no-oficial, drift de shape en respuesta live, o behavior que Printing Press debería generar correctamente eventualmente. Buscá primero en los issues de `mvanhorn/cli-printing-press`; reusá un issue que matchee o abrí uno, después seteá `upstream_issue` para que la próxima regen sepa qué tiene que superseder al patch:

```json
{
  "id": "puente-temporal",
  "summary": "Qué cambió (una oración).",
  "reason": "Por qué se necesitó esta customización (una o dos oraciones).",
  "files": ["internal/cli/foo.go"],
  "validated_outcome": "Opcional: resultado de test no-obvio que confirma el fix.",
  "deferred_to_upstream": [
    {
      "feature": "Behavior del generador o capacidad de API upstream que eventualmente debería superseder este patch",
      "reason": "Por qué el patch local es temporal o API-specific"
    }
  ],
  "upstream_issue": "https://github.com/mvanhorn/cli-printing-press/issues/<n>"
}
```

Este archivo es un **índice de customizaciones**, no una segunda copia del diff. Los diffs viven en `git`; el manifest es lo que le dice al próximo agente (o a la herramienta de regeneración) qué se customizó y por qué. Mantené `summary` y `reason` cortos — si te encontrás escribiendo tablas de renames de campos o transformaciones de código, ese detalle va en el commit message, no acá.

Los comentarios inline `// PATCH:` en el source son opcionales. Si te ayudan como aid de navegación (`grep -rn 'PATCH' .` saca a la luz los sitios customizados), agregalos — pero no son requeridos ni los enforce ningún CI.

## Customizaciones aplicadas en este repo

Desde la generación base, este repo tiene los siguientes patches manuales (todos documentados acá para no perderse en próximas regeneraciones):

1. **MCP companion removido** — `cmd/mercadolibre-pp-mcp/` eliminado y referencias removidas de `.goreleaser.yaml`. El proyecto prefiere distribución solo CLI; usuarios avanzados que quieran MCP pueden regenerar con `printing-press generate --spec spec/ml-spec.yaml --force`.
2. **Endpoint `/sites/{id}/search` no incluido** — ML lo cerró a apps no-certificadas en 2024. La spec usa `/products/search` (catálogo canónico) en su lugar, que devuelve data más rica y no requiere certificación.
3. **Endpoint `/orders/search` no incluido por default** — requiere scope `read orders` que no todos los usuarios habilitan. Documentado como caveat en README; usuarios avanzados pueden agregarlo editando `spec/ml-spec.yaml` y regenerando.
4. **Hero image en `docs/hero.jpg`** — pizarrón hand-drawn cargado en README para diferenciarlo visualmente.
