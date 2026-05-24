---
name: pp-mercadolibre
description: "CLI Printing Press para MercadoLibre. CLI cross-platform para la API de MercadoLibre (catálogo, categorías, países, sitios, ítems, usuarios, preguntas) — Argentina, Brasil, México, Chile, Uruguay, Colombia, Perú y más."
author: "LeaCast"
license: "MIT"
argument-hint: "<comando> [args] | install cli"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - mercadolibre-pp-cli
---

# MercadoLibre — CLI Printing Press

CLI cross-platform para la API de MercadoLibre. Acceso a catálogo de productos, países, sitios, categorías, perfiles de usuario, publicaciones y preguntas. Cobertura LATAM (AR, BR, MX, CL, CO, UY, PE, etc.). Endpoints públicos no requieren auth; el resto vía OAuth 2.0 (token en `MERCADOLIBRE_ACCESS_TOKEN`).

## Prerrequisitos: instalar la CLI

Esta skill maneja el binario `mercadolibre-pp-cli`. **Tenés que verificar que la CLI esté instalada antes de invocar cualquier comando.** Si falta, instalala primero:

1. Instalar vía el installer de Printing Press:
   ```bash
   npx -y @mvanhorn/printing-press-library install mercadolibre --cli-only
   ```
2. Verificar: `mercadolibre-pp-cli --version`
3. Asegurate de que `$GOPATH/bin` (o `$HOME/go/bin`) esté en tu `$PATH`.

Si el `--version` reporta "command not found" después del install, el paso de instalación no puso el binario en `$PATH`. No procedas con los comandos de la skill hasta que la verificación funcione.

## Transporte HTTP

Esta CLI usa transporte HTTP compatible con Chrome para endpoints browser-facing. No requiere un proceso de browser residente para llamadas normales a la API.

## Referencia de comandos

**catalog** — Búsqueda y detalle del catálogo canónico de productos (universo cross-vendor). Requiere OAuth.

- `mercadolibre-pp-cli catalog get` — Detalle completo de un producto canónico (atributos, fotos, descripción)
- `mercadolibre-pp-cli catalog search` — Buscar productos canónicos por keyword en un sitio (ej: iphone en MLA)

**categories** — Operaciones sobre categorías (taxonomía, atributos requeridos)

- `mercadolibre-pp-cli categories get` — Detalle de categoría con path desde root, atributos requeridos, total de items
- `mercadolibre-pp-cli categories list-by-site` — Categorías raíz de un sitio (top-level)

**countries** — Operaciones sobre países (endpoint público, no requiere auth)

- `mercadolibre-pp-cli countries get` — Detalle de un país (estados, geografía)
- `mercadolibre-pp-cli countries list` — Lista todos los países soportados por MercadoLibre (público, sin OAuth)

**items** — Operaciones sobre publicaciones individuales (un item específico en el marketplace)

- `mercadolibre-pp-cli items <item_id>` — Detalle completo de una publicación (precio, stock, fotos, descripción, vendedor)

**questions** — Preguntas y respuestas en publicaciones (gestión de Q&A)

- `mercadolibre-pp-cli questions answer` — Responder una pregunta específica
- `mercadolibre-pp-cli questions list` — Listar preguntas en publicaciones de un vendedor

**sites** — Operaciones sobre sitios de MercadoLibre (AR, BR, MX, etc.)

- `mercadolibre-pp-cli sites` — Lista todos los sitios MercadoLibre (código de país + currency)

**users** — Operaciones sobre usuarios y vendedores (perfil público, reputación)

- `mercadolibre-pp-cli users get` — Perfil de usuario/vendedor (nick, reputación, fecha de registro)
- `mercadolibre-pp-cli users items` — Publicaciones de un vendedor (con filtros de estado)

### Encontrar el comando correcto

Cuando sabés qué querés hacer pero no qué comando lo hace, preguntale directamente a la CLI:

```bash
mercadolibre-pp-cli which "<capacidad en tus propias palabras>"
```

`which` resuelve una query de capacidad en lenguaje natural al comando que mejor matchea desde el índice de features curado de esta CLI. Exit code `0` significa al menos un match; exit code `2` significa que no hubo match con confianza — fallback a `--help` o usá una query más estrecha.

## Setup de autenticación

Corré `mercadolibre-pp-cli auth setup` para ver la URL y los pasos para obtener un token (agregá `--launch` para abrir la URL automáticamente). Después guardalo:

```bash
mercadolibre-pp-cli auth set-token TU_TOKEN_ACA
```

O seteá la env var `MERCADOLIBRE_ACCESS_TOKEN`.

Corré `mercadolibre-pp-cli doctor` para verificar el setup.

## Modo agente

Agregá `--agent` a cualquier comando. Se expande a: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON en stdout, errores en stderr
- **Filtrable** — `--select` deja solo un subset de campos. Paths con punto descienden a estructuras anidadas; arrays se traversan elemento a elemento. Crítico para mantener el contexto chico en APIs verbosas:

  ```bash
  mercadolibre-pp-cli catalog get mock-value --agent --select id,name,status
  ```
- **Previsualizable** — `--dry-run` muestra el request sin enviarlo
- **Offline-friendly** — los comandos sync/search pueden usar el store SQLite local cuando está disponible
- **No-interactivo** — nunca pide prompts, cada input es una flag
- **Reintentos explícitos** — usá `--idempotent` solo cuando un create de algo que ya existe debería contar como éxito

### Envelope de respuesta

Los comandos que leen del store local o de la API envuelven el output en un envelope de provenance:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parseá `.results` para los datos y `.meta.source` para saber si viene de live o local. Un resumen human-readable `N results (live)` se imprime a stderr solo cuando stdout es una terminal AND no hay flag de formato machine (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) seteada — los consumers piped/agent y las runs con formato explícito obtienen JSON puro en stdout.

## Feedback del agente

Cuando vos (o el agente) noten algo raro sobre esta CLI, registralo:

```bash
mercadolibre-pp-cli feedback "el flag --since es inclusivo pero los docs dicen exclusivo"
mercadolibre-pp-cli feedback --stdin < notas.txt
mercadolibre-pp-cli feedback list --json --limit 10
```

Las entradas se guardan localmente en `~/.local/share/mercadolibre-pp-cli/feedback.jsonl`. Nunca se hacen POST a ningún lado salvo que `MERCADOLIBRE_FEEDBACK_ENDPOINT` esté seteada AND se pase `--send` o `MERCADOLIBRE_FEEDBACK_AUTO_SEND=true`. El default es local-only.

Escribí lo que te *sorprendió*, no un bug report. Corto, específico, una línea: eso es lo que compone valor con el tiempo.

## Entrega del output

Cada comando acepta `--deliver <sink>`. El output va al sink nombrado además de (o en vez de) stdout, así los agentes pueden rutear resultados sin pipear a mano. Tres sinks soportados:

| Sink | Efecto |
|------|--------|
| `stdout` | Default; escribe solo a stdout |
| `file:<path>` | Escribe atómicamente el output a `<path>` (tmp + rename) |
| `webhook:<url>` | POST del body del output a la URL (`application/json` o `application/x-ndjson` cuando se usa `--compact`) |

Schemes desconocidos se rechazan con un error estructurado que nombra los soportados. Las fallas de webhook devuelven non-zero y loguean la URL + status HTTP en stderr.

## Perfiles nombrados

Un perfil es un set guardado de valores de flags, reusado entre invocaciones. Usalo cuando un agente programado llama al mismo comando en cada corrida con la misma configuración — patrón "Beacon" de HeyGen.

```bash
mercadolibre-pp-cli profile save briefing --json
mercadolibre-pp-cli --profile briefing catalog get mock-value
mercadolibre-pp-cli profile list --json
mercadolibre-pp-cli profile show briefing
mercadolibre-pp-cli profile delete briefing --yes
```

Las flags explícitas siempre ganan sobre los valores del perfil; los valores del perfil ganan sobre los defaults. `agent-context` lista todos los perfiles disponibles bajo `available_profiles` para que los agentes los descubran en runtime.

## Exit codes

| Code | Significado |
|------|-------------|
| 0 | Éxito |
| 2 | Error de uso (argumentos incorrectos) |
| 3 | Recurso no encontrado |
| 4 | Autenticación requerida |
| 5 | Error de API (problema upstream) |
| 7 | Rate limited (esperá y reintentá) |
| 10 | Error de config |

## Parseo de argumentos

Parseá `$ARGUMENTS`:

1. **Vacío, `help` o `--help`** → mostrar `mercadolibre-pp-cli --help`
2. **Empieza con `install`** → ver Prerrequisitos arriba
3. **Cualquier otra cosa** → uso directo (ejecutar como comando CLI con `--agent`)

## Uso directo

1. Verificar si está instalado: `which mercadolibre-pp-cli`. Si no está, ofrecer instalar (ver Prerrequisitos arriba).
2. Hacer match de la query del usuario con el mejor comando de la Referencia de comandos.
3. Ejecutar con el flag `--agent`:
   ```bash
   mercadolibre-pp-cli <comando> [subcomando] [args] --agent
   ```
4. Si es ambiguo, drill-in al help del subcomando: `mercadolibre-pp-cli <comando> --help`.
