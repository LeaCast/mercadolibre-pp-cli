<div align="center">

<img src="docs/hero.jpg" alt="mercadolibre-pp-cli — buscá MercadoLibre desde la terminal" width="100%" />

# mercadolibre-pp-cli

**Buscá productos en MercadoLibre desde la terminal — para vos, para tu agente de IA, o para integrar en cualquier script.**

Un solo comando para listar precios, comparar variantes, filtrar por presupuesto, ordenar por más barato, traer detalles de cualquier publicación, y muchas cosas más — sin abrir el browser, sin scrapear, sin pelearse con OAuth.

[![Plataformas](https://img.shields.io/badge/plataformas-Linux%20%7C%20macOS%20%7C%20Windows-blue)](https://github.com/LeaCast/mercadolibre-pp-cli/releases)
[![Release](https://img.shields.io/github/v/release/LeaCast/mercadolibre-pp-cli?label=release)](https://github.com/LeaCast/mercadolibre-pp-cli/releases/latest)
[![Licencia](https://img.shields.io/badge/licencia-MIT-green)](LICENSE)

</div>

---

## ¿Qué hace en una línea?

Le decís en la terminal: _"buscá los 5 Motorola Edge 60 más baratos bajo $1M con envío gratis en Argentina"_ y te devuelve la lista lista, ordenada, con links directos, en menos de 4 segundos.

## ¿Por qué existe?

Tres razones simples:

1. **Buscar en mercadolibre.com.ar con browser es lento y te marea.** Hay que abrir la página, esperar que cargue scripts, hacer click en filtros, scrollear, comparar a ojo. Esta CLI lo hace en un solo comando.
2. **Si usás Claude Code / Codex / Gemini CLI / Cursor para ayudarte, los agentes no pueden navegar bien MercadoLibre** — la página tiene anti-bot y devuelve 403. Esta CLI le da al agente datos limpios y estructurados sin pelear contra protecciones.
3. **Cuando un agente lee resultados de la web, "gasta" mucha información de su memoria** procesando HTML lleno de banners, menús y publicidad. Con esta CLI el agente recibe solo lo que importa: precio, título, link. Le sale **~6 veces más barato en tokens** (ver [comparativa más abajo](#cli-vs-buscar-igual-en-la-web)).

---

## Instalación

### Opción A — Bajar el binario listo (recomendado)

Andá a [Releases](https://github.com/LeaCast/mercadolibre-pp-cli/releases/latest) y bajá el archivo de tu sistema:

| Si tenés...    | Bajá esto                                   |
| -------------- | ------------------------------------------- |
| Windows        | `mercadolibre-pp-cli_*_windows_amd64.zip`   |
| Mac (Intel)    | `mercadolibre-pp-cli_*_darwin_amd64.tar.gz` |
| Mac (M1/M2/M3) | `mercadolibre-pp-cli_*_darwin_arm64.tar.gz` |
| Linux          | `mercadolibre-pp-cli_*_linux_amd64.tar.gz`  |

Descomprimís, copiás el archivo `mercadolibre-pp-cli` a una carpeta que esté en tu PATH (en Mac/Linux suele ser `/usr/local/bin/`, en Windows cualquier carpeta de tu sistema con `setx PATH`), y listo. Probalo:

```bash
mercadolibre-pp-cli --help
```

### Opción B — Compilar desde el repo (si tenés Go instalado)

```bash
go install github.com/LeaCast/mercadolibre-pp-cli/cmd/mercadolibre-pp-cli@latest
```

---

## Setup inicial (una sola vez, 3 minutos)

MercadoLibre te obliga a crear tu propia "app" para usar su API. Es gratis y rápido. La CLI tiene un wizard que te lleva de la mano:

### Paso 1 — Crear tu app en MercadoLibre

Andá a **https://developers.mercadolibre.com.ar/devcenter** (logueate con tu cuenta normal de MercadoLibre) y hacé click en **"Crear aplicación"**.

Completá así:

- **Nombre:** lo que quieras (ej. `mi-cli`)
- **Descripción:** una línea cualquiera
- **Redirect URI:** `https://httpbin.org/get` ← **literalmente esto, copialo tal cual**
- **Flujos OAuth:** marcá ✅ Authorization Code, ✅ Refresh Token
- **Unidad de negocio:** ✅ Mercado Libre
- **Permisos:** dejá los defaults

Guardás y te van a aparecer dos valores: **App ID** (un número largo) y **Client Secret** (un string). Anotalos.

### Paso 2 — Correr el wizard

```bash
mercadolibre-pp-cli auth login
```

El wizard:

1. Te abre el navegador en la pantalla de autorización de MercadoLibre.
2. Vos hacés click en **"Autorizar"**.
3. ML te redirige a una página que muestra un JSON con un campo `args.code`.
4. Copiás ese código y lo pegás en la terminal cuando el wizard te lo pide.
5. Listo. La CLI guarda todo y de ahí en adelante **mantiene la sesión sola** (no hay que volver a autenticarse hasta dentro de ~6 meses).

> **¿Por qué hay que pegar el código a mano?** MercadoLibre no permite usar `http://localhost` en la configuración (exige HTTPS). Otras APIs (Google, GitHub, etc.) sí lo permiten y por eso ahí el login es 100% automático. Es la única fricción del setup.

### Verificar que funciona

```bash
mercadolibre-pp-cli doctor
```

Si todo está OK vas a ver una línea verde de auth.

---

## Comandos principales

### `items search` — buscar publicaciones reales

El comando que más vas a usar. Busca publicaciones activas (con precio, vendedor, envío) y te las devuelve filtradas y ordenadas.

```
mercadolibre-pp-cli items search [flags]
```

| Flag                             | Para qué sirve                                                                                         | Default           |
| -------------------------------- | ------------------------------------------------------------------------------------------------------ | ----------------- |
| `--q "<texto>"`                  | Lo que buscás (palabra clave)                                                                          | obligatorio       |
| `--site-id <MLA\|MLB\|MLM\|...>` | País: MLA=Argentina, MLB=Brasil, MLM=México, MLC=Chile, MCO=Colombia, MLU=Uruguay                      | obligatorio       |
| `--sort <orden>`                 | `price_asc` (barato→caro), `price_desc` (caro→barato), `relevance`                                     | `price_asc`       |
| `--filter <clave>=<valor>`       | Filtros. Repetible — podés poner varios. Ver tabla abajo.                                              | (ninguno)         |
| `--limit <N>`                    | Cuántos resultados querés ver (máx. 50)                                                                | 10                |
| `--catalog-limit <N>`            | Cuántos modelos del catálogo escanea (sube esto si tu búsqueda es muy específica)                      | 20                |
| `--domain-id <id>`               | Restringir a un dominio (ej. `MLA-CELLPHONES`). Útil cuando aparecen accesorios                        | (sin restricción) |
| `--json`                         | Devuelve JSON estructurado (para scripts y agentes)                                                    | (devuelve tabla)  |
| `--plain`                        | Tab-separated (útil para `awk`, `cut`)                                                                 |                   |
| `--compact`                      | Solo precio + link (mínimo tokens, para agentes)                                                       |                   |

**Filtros disponibles** (todos repetibles con `--filter`):

| Filtro                | Ejemplo                       | Qué hace                       |
| --------------------- | ----------------------------- | ------------------------------ |
| `price=MIN-MAX`       | `--filter price=0-1000000`    | Rango de precio                |
| `condition=new\|used` | `--filter condition=new`      | Solo nuevos o solo usados      |
| `shipping_cost=free`  | `--filter shipping_cost=free` | Solo con envío gratis          |
| `seller=<id>`         | `--filter seller=123456`      | Solo de un vendedor específico |
| `currency=ARS\|USD`   | `--filter currency=ARS`       | Filtrar por moneda             |

### `items get <id>` — detalle completo de una publicación

```bash
mercadolibre-pp-cli items get MLA1234567890
```

> **Limitación importante:** desde 2024 MercadoLibre restringió el endpoint `/items/<id>` aunque tu token sea válido. Solo el **vendedor dueño de la publicación** puede leer su propio item por API. Si pedís el detalle de un listing de otro seller, devuelve 403 — la API te dice "permisos" pero en realidad es por dueño, no por scope.
>
> **Workarounds:**
>
> 1. **Abrí el link en tu browser personal.** La columna `url` que ya viene en cada resultado de `items search` te lleva directo a la página. 30 segundos, sin fricción.
> 2. **Verificá reputación del vendedor con `users get <seller_id>`** — sí funciona con cualquier token y te da nivel de reputación + cantidad de transacciones. Es la señal de confianza más importante antes de comprar (ver ejemplo abajo).

### `users get <seller_id>` — reputación pública de un vendedor

Cuando `items search` te devuelve listings, cada uno incluye un `seller_id`. Con ese ID podés ver la reputación pública del vendedor:

```bash
mercadolibre-pp-cli users get 554230752 --json
```

**Resultado real** (corrida 2026-05-26, 0,7 segundos):

```
nickname:        JOSIGNACIOCARRIZOMIRANDA
country:         AR
user_type:       normal
level_id:        4_light_green
transactions:    36 históricas
```

El campo `level_id` va de `1_red` (peor) a `5_green` (mejor). `4_light_green` con sólo 36 transacciones significa: reputación buena pero historial corto — andá con cuidado en compras grandes.

### `catalog search` — buscar en el catálogo canónico

Si en vez de listings con precio querés ver qué modelos existen (ej. "qué iPhones tiene ML en catálogo"):

```bash
mercadolibre-pp-cli catalog search --site-id MLA --q "iphone" --limit 10
```

### Otros comandos útiles

| Comando                                 | Para qué                                          |
| --------------------------------------- | ------------------------------------------------- |
| `auth login`                            | Login interactivo (setup inicial)                 |
| `auth status`                           | Ver si estás logueado                             |
| `auth logout`                           | Borrar credenciales                               |
| `doctor`                                | Diagnóstico de salud del CLI                      |
| `sites`                                 | Listar todos los países de ML con código y moneda |
| `countries list`                        | Igual pero más detalle                            |
| `categories list-by-site --site-id MLA` | Árbol de categorías de un país                    |

Ejecutá cualquier comando con `--help` para ver todos sus flags.

---

## Ejemplos reales

### Ejemplo 1 — Los 5 celulares más baratos bajo $1M con envío gratis

```bash
mercadolibre-pp-cli items search \
  --q "motorola edge 60" \
  --site-id MLA \
  --sort price_asc \
  --filter price=0-1000000 \
  --filter shipping_cost=free \
  --limit 5
```

**Resultado real** (corrida 2026-05-26, 1,2 segundos):

```
#  precio         variante                                 envío   condición  url
1  ARS 414729.27  Motorola Edge 60 Fusion 256gb + 8gb Ram  gratis  new        https://articulo.mercadolibre.com.ar/MLA-3133009504
2  ARS 439505.11  Motorola Edge 60 Fusion 256gb Amazonite  gratis  new        https://articulo.mercadolibre.com.ar/MLA-3133009084
3  ARS 489071.98  Motorola Edge 60 Gibraltar Sea           gratis  new        https://articulo.mercadolibre.com.ar/MLA-3131589844
4  ARS 500000.00  Motorola Edge 60 Fusion 256gb Amazonite  gratis  new        https://articulo.mercadolibre.com.ar/MLA-1799466271
5  ARS 510000.00  Motorola Edge 60 Fusion 256gb Amazonite  gratis  new        https://articulo.mercadolibre.com.ar/MLA-3253872120
```

Click directo a comprar.

### Ejemplo 2 — Smart TVs Samsung 55" en Brasil (cualquier precio)

```bash
mercadolibre-pp-cli items search \
  --q "smart tv samsung 55" \
  --site-id MLB \
  --sort price_asc \
  --limit 5
```

**Resultado real** (corrida 2026-05-26, 1,3 segundos):

```
#  precio        variante                                          envío   condición  url
1  BRL 3500.00   Smart TV Samsung UN55TU8300FXZX curvo 4K 55"      pago    new        https://articulo.mercadolibre.com.br/MLB-6196004362
2  BRL 10683.16  Samsung Smart Tv 85 Uhd 4k + Samsung Smart Tv 50  gratis  new        https://articulo.mercadolibre.com.br/MLB-5979485036
```

Devuelve precio en BRL (reales brasileños) y link directo a cada publicación.

### Ejemplo 3 — iPhone 15 más baratos en México

```bash
mercadolibre-pp-cli items search \
  --q "iphone 15" \
  --site-id MLM \
  --domain-id MLM-CELLPHONES \
  --sort price_asc \
  --limit 5
```

**¿Para qué sirve `--domain-id MLM-CELLPHONES`?** Para palabras muy populares como "iphone 15", el catálogo de ML está dominado por accesorios (fundas, vidrios, cables). Si querés el teléfono en sí, le pasás el dominio explícito y la búsqueda se restringe a celulares. Sin ese flag, traerías fundas para iPhone 15 en vez de iPhones.

**Resultado real** (corrida 2026-05-26, 1,1 segundos):

```
#  precio        variante                             envío   condición  url
1  MXN 11000.00  Apple Iphone 15 128 Gb Rosa          gratis  new        https://articulo.mercadolibre.com.mx/MLM-2865788069
2  MXN 11200.00  Apple iPhone 15 (128 GB) - Amarillo  gratis  new        https://articulo.mercadolibre.com.mx/MLM-2283337521
3  MXN 12333.00  Apple iPhone 15 (256 GB) - Negro     gratis  new        https://articulo.mercadolibre.com.mx/MLM-2288307097
4  MXN 12699.00  Apple Iphone 15 128 Gb Rosa          gratis  new        https://articulo.mercadolibre.com.mx/MLM-3484217888
5  MXN 12997.00  Apple iPhone 15 (128 GB) - Verde     gratis  new        https://articulo.mercadolibre.com.mx/MLM-2213612267
```

### Tip: cuando no aparecen los resultados esperados

Si tu búsqueda te trae sólo accesorios (fundas, pantallas, cables) en vez del producto principal:

1. **Agregá `--domain-id <id>`** para forzar la categoría correcta. Ejemplos comunes:
   - Celulares: `MLA-CELLPHONES`, `MLB-CELLPHONES`, `MLM-CELLPHONES`
   - Notebooks: `MLA-NOTEBOOKS`, `MLB-NOTEBOOKS`
   - Smart TVs: `MLA-TELEVISIONS`, `MLB-TELEVISIONS`
   - Auriculares: `MLA-HEADPHONES`
2. **Subí `--catalog-limit`** (default 20, máximo 50) si tu producto es muy específico y se pierde entre miles de variantes.
3. **Hacé el `--q` más específico**: en vez de `"iphone"` probá `"iphone 15 256gb"`.

---

## Pidiéndole a Claude (o cualquier agente IA)

Acá la CLI realmente brilla: en vez de aprender los flags vos, le hablás en español a tu agente y él traduce a los comandos correctos.

### Ejemplo NL #1

**Vos le decís a Claude:**

> "Necesito comprar un iPhone 13 en MercadoLibre Argentina. Tirame los 5 más baratos con envío gratis, todos nuevos, bajo $1.500.000."

**Claude ejecuta:**

```bash
mercadolibre-pp-cli items search \
  --q "iphone 13" \
  --site-id MLA \
  --domain-id MLA-CELLPHONES \
  --sort price_asc \
  --filter price=0-1500000 \
  --filter condition=new \
  --filter shipping_cost=free \
  --limit 5 \
  --json
```

**Resultado real** (corrida 2026-05-26, 2,6 segundos, ~460 tokens devueltos a Claude):

```
1. $500.000 ARS — Apple iPhone 13 128 GB Medianoche
2. $600.000 ARS — Apple iPhone 13 (128 GB) - Azul
3. $620.000 ARS — Apple iPhone 13 (256 GB) - Azul
4. $700.000 ARS — Apple iPhone 13 128 GB Medianoche
5. $747.000 ARS — Apple iPhone 13 128 GB Medianoche
```

Claude lee el JSON y te lo arma como tabla en español con los links clickeables. Si le pedís "y traeme la reputación del vendedor del #1", Claude encadena automáticamente con `users get <seller_id>` para sumarte la señal de confianza antes de comprar.

### Ejemplo NL #2

**Vos le decís a Claude:**

> "Comparame precios de iPhone 15 en Argentina vs Brasil vs México. Solo nuevos, los 3 más baratos de cada país."

**Claude ejecuta 3 comandos en paralelo:**

```bash
mercadolibre-pp-cli items search --q "iphone 15" --site-id MLA --domain-id MLA-CELLPHONES --sort price_asc --filter condition=new --limit 3 --json
mercadolibre-pp-cli items search --q "iphone 15" --site-id MLB --domain-id MLB-CELLPHONES --sort price_asc --filter condition=new --limit 3 --json
mercadolibre-pp-cli items search --q "iphone 15" --site-id MLM --domain-id MLM-CELLPHONES --sort price_asc --filter condition=new --limit 3 --json
```

**Claude te responde** con una tabla comparativa de los 9 resultados, convirtiendo todas las monedas a USD al cambio del día para que veas en qué país conviene más, y te marca las diferencias clave (capacidad, color, envío internacional posible).

### Ejemplo NL #3 — Tamaño de mercado por categoría

**Vos le decís a Claude:**

> "Quiero saber qué rubros son los más grandes en MercadoLibre Argentina. Dame las 10 categorías raíz ordenadas por cuántas publicaciones tienen."

**Claude procesa las 32 categorías raíz en paralelo:**

```bash
mercadolibre-pp-cli categories list-by-site MLA --json \
  | jq -r '.results[].id' | tr -d '\r' | while read CAT; do
    mercadolibre-pp-cli categories get "$CAT" --json 2>/dev/null \
      | jq -r '.results | [.name, .total_items_in_this_category] | @tsv'
  done | sort -t$'\t' -k2 -n -r | head -10
```

**Resultado real** (corrida 2026-05-26):

| Categoría                  | Publicaciones |
| -------------------------- | ------------- |
| Hogar, Muebles y Jardín    | 22.382.690    |
| Libros, Revistas y Cómics  | 22.217.618    |
| Ropa y Accesorios          | 14.858.016    |
| Accesorios para Vehículos  | 14.439.070    |
| Herramientas               | 5.903.857     |
| Deportes y Fitness         | 5.658.204     |
| Juegos y Juguetes          | 5.544.480     |
| Belleza y Cuidado Personal | 4.423.272     |
| Celulares y Teléfonos      | 3.637.397     |
| Arte, Librería y Mercería  | 3.586.821     |

La foto del mercado argentino en 30 segundos — útil si estás pensando en qué rubro entrar a vender.

### Ejemplo NL #4 — Saturación de oferta cross-país

**Vos le decís a Claude:**

> "Pensaba importar productos Dyson Airwrap para revender. ¿En qué país hay menos competencia? Dame el número de productos en MercadoLibre Argentina, Brasil, México, Colombia y Uruguay."

**Claude ejecuta una iteración sobre los 5 sites:**

```bash
for SITE in MLA MLB MLM MCO MLU; do
  TOTAL=$(mercadolibre-pp-cli catalog search --q "dyson airwrap" \
    --site-id "$SITE" --limit 1 --json | jq -r '.results.paging.total // 0')
  echo "$SITE: $TOTAL"
done
```

**Resultado real** (corrida 2026-05-26, ordenado de menor a mayor saturación):

| País      | Productos en catálogo  |
| --------- | ---------------------- |
| Brasil    | 1.217 ← menos saturado |
| Uruguay   | 1.399                  |
| Argentina | 1.401                  |
| Colombia  | 1.507                  |
| México    | 1.670                  |

Una decisión de arbitraje internacional en 4 segundos. **Caveat técnico:** `catalog search` capea el total en 10.000 — para keywords muy populares ("apple watch") todos los países te devuelven 10.000 y el ranking pierde sentido. Para que el signal sea útil, usá queries específicas (marca + modelo).

> **Límites del API que conviene saber:**
>
> - `items/{id}` (detalle individual de publicación) está restringido al dueño del listing — incluso con OAuth, no podés leer el item de otro seller.
> - `users/{id}/items/search` (listar todas las publicaciones de un seller ajeno) también restringido.
>
> Lo que sí funciona público con OAuth: `items search`, `catalog search`, `users get` (perfil + reputación), `categories list-by-site`, `categories get`. Suficiente para casos reales de comparación, due diligence y market research.

---

## CLI vs buscar igual en la web

Misma búsqueda — _5 Motorola Edge 60 bajo $1M ARS, del más barato al más caro_ — corrida desde tres lugares al mismo tiempo (2026-05-26):

| Método                                 |    Tiempo |    Tokens | Qué devolvió                                                                       |
| -------------------------------------- | --------: | --------: | ---------------------------------------------------------------------------------- |
| **`mercadolibre-pp-cli items search`** | **1,2 s** |  **~460** | 5 listings reales con precio, variante, link, condición, envío                     |
| WebFetch a `listado.mercadolibre.com.ar` |         — |         — | **HTTP 403 — ML lo bloquea** (anti-bot, ni siquiera carga la página)               |
| WebSearch genérico                     |      ~4 s |      ~500 | "Prices as low as $859,999" — el más barato real era **$414.729**                  |

Tres cosas que esto deja claro:

- **MercadoLibre bloquea scraping.** Cualquier intento de leer la página directo termina en 403. No es solucionable con headers ni user-agents — ML tiene anti-bot serio.
- **Las búsquedas web genéricas devuelven precios viejos o inventados.** No tienen forma de ver el precio del día porque indexan páginas estáticas y resúmenes.
- **La CLI usa la API oficial con tu token.** Datos del momento, estructurados, sin ban, sin inventar nada. Para un agente IA eso es **~6× menos tokens consumidos** y **0% de errores de precio**.

---

## Preguntas frecuentes

**¿Es gratis usar la API de MercadoLibre?**
Sí. Las consultas read-only (catálogo, items, categorías, búsquedas) son gratuitas con los rate limits estándar. No te cobran nada.

**¿Necesito ser vendedor de MercadoLibre para usar esto?**
No. Solo necesitás una cuenta normal de MercadoLibre (la que usás para comprar). El paso del "Crear app" usa esa misma cuenta.

**¿Es seguro? ¿Qué pasa con mi token?**
El token se guarda solo en tu compu, en `~/.config/mercadolibre-pp-cli/config.toml` (Linux/Mac) o `%USERPROFILE%\.config\mercadolibre-pp-cli\config.toml` (Windows). Nada se envía a ningún servidor externo. El token solo permite leer datos públicos del marketplace + tu propia cuenta — no permite vaciar la tarjeta ni nada por el estilo.

**¿Cada cuánto tengo que volver a loguearme?**
Idealmente nunca. El token de acceso vive 6 horas, pero la CLI tiene refresh automático: antes de cada consulta chequea si está por vencer y se renueva sola usando el refresh token (que dura 6 meses). Solo tendrías que correr `auth login` de nuevo si dejás la CLI sin usarla por más de 6 meses.

**¿Por qué no aparecen los Motorola Edge 60 "puros" (sin Fusion) en mi búsqueda?**
La búsqueda por keyword es amplia: `"motorola edge 60"` matchea cualquier listing con esas palabras, incluyendo Fusion y Pro. Para ser más específico: agregá palabras al `--q` (ej. `--q "motorola edge 60 12gb"` excluye Fusion porque Fusion tiene 8GB) o usá `--domain-id MLA-CELLPHONES` con un keyword más estricto.

**¿Esto funciona para vender, no solo para buscar?**
La API de MercadoLibre tiene endpoints de escritura (publicar items, responder preguntas, gestionar órdenes) y esta CLI los soporta — pero por seguridad están detrás de flags explícitos. Mirá `mercadolibre-pp-cli auth status` para ver qué permisos tiene tu token, y `mercadolibre-pp-cli --help` para la superficie completa.

**¿Y si MercadoLibre cambia la API?**
La CLI está construida sobre [Printing Press](https://printingpress.dev) — una factory que regenera el código cliente desde la spec OpenAPI cuando hay cambios. Si ML actualiza algo, sale una versión nueva con el fix.

---

## Resolución de problemas

**`Error: GET ... returned HTTP 401: invalid access token`**
Tu token venció y por algún motivo el refresh automático no se disparó. Solución: `mercadolibre-pp-cli auth login` para re-loguearte (toma 30 segundos).

**`Error: GET /products/search returned HTTP 403`**
Faltan permisos en tu app de ML. Andá al devcenter, abrí tu app, verificá que tenga ✅ Authorization Code + ✅ Refresh Token + ✅ Mercado Libre marcados.

**`auth login` no abre el browser**
Copiá manualmente la URL que aparece en la terminal y pegala en cualquier browser. El resto del flujo (pegar el code) funciona igual.

**Después de autorizar veo el JSON pero no encuentro `args.code`**
Es la primera línea adentro del campo `"args"`. Si el JSON es muy largo, hacé Ctrl+F y buscá `"code"`. Copiá solo el valor entre comillas (algo tipo `TG-abc123-xyz456`).

---

## Licencia y créditos

MIT License. Construido con [Printing Press](https://printingpress.dev) — la factory que genera CLIs nativos para cualquier API REST sin escribir glue code a mano.

Issues y feature requests: [GitHub Issues](https://github.com/LeaCast/mercadolibre-pp-cli/issues).
