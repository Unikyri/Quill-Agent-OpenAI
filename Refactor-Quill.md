# Refactor-Quill — Llevar el backend a Clean Architecture, feature por feature

> Un writeup para hacer *shadowing*: refactorizás vos, a mano, en **tu propia copia**, mientras un senior te explica el **por qué** de cada corte. No es "seguí esta estructura de carpetas porque sí". Es entender **por qué** un archivo se parte en varios, **por qué** una interfaz vive donde vive, y **cómo probar** cada funcionalidad a medida que avanzás.

**Cómo está pensada la guía:**
- Vamos **transversal**: una funcionalidad (vertical slice) completa por capítulo, de la más simple a la más compleja. Cada slice se mueve entero por todas las capas, se re-cablea, se prueba y se commitea. Nunca dejás el árbol roto.
- Cada capítulo trae **snippets representativos**, no la solución entera. El código lo escribís vos — ese es el ejercicio. Yo te muestro el patrón una vez y el "aha", después lo aplicás.
- El código, los nombres, los paths y los comandos van en **inglés** (así queda igual a un proyecto Go real). Las explicaciones, en criollo.

**Índice**
- Parte 0 — El mindset antes de tocar una línea
- Parte 1 — Diagnóstico + la arquitectura target
- Parte 2 — Preparación (la red de seguridad)
- Cap. 3 — Slice **Auth** (trivial) · la Regla de Dependencia en su forma pura
- Cap. 4 — Slice **Universe/Work/Chapter** (simple) · de-anemizar + el problema de las transacciones
- Interludio — Los tres *boundaries* compartidos: Vector, Graph, LLM
- Cap. 5 — Slice **Entity** (medium) · un caso de uso que orquesta cuatro backends
- Cap. 6 — Slice **Contradiction** (complejo) · el agente ReAct como caso de uso
- Cap. 7 — Slice **Memory** (muy complejo) · lógica pura testeable sin nada
- Cap. 8 — Slice **Analysis orchestrator** (capstone) · disolver la dependencia circular
- Parte 9 — Cierre honesto: qué ganás y qué te cuesta
- Apéndice — Mapa viejo→nuevo, diagrama antes/después, los sharp edges

---

## Parte 0 — El mindset antes de tocar una línea

Antes de la arquitectura, tres reglas. Si te saltás estas, no estás refactorizando: estás reescribiendo y rezando.

### Las 3 reglas de oro

| # | Regla | Por qué |
|---|-------|---------|
| 1 | **Tests verdes primero, siempre.** | Un refactor, por definición, **no cambia el comportamiento**. La única forma de *saber* que no lo cambiaste es una batería de tests que ya pasa. Sin red, no hay refactor: hay apuesta. |
| 2 | **Pasos chiquitos y reversibles.** | Movés una cosa, compilás, corrés los tests, seguís. Si algo se rompe, el culpable es el último paso de 20 líneas, no un diff de 2000. |
| 3 | **Un commit por slice (o menos).** | Cada commit deja el árbol **compilando y con los tests verdes**. Podés volver a cualquier punto. El historial cuenta la historia del refactor. |

> **La frase del senior:** "El refactor no es un big bang. Es cien pasitos aburridos, cada uno seguro. El día que sentís adrenalina refactorizando, parаste: perdiste la red."

### La Regla de Dependencia (el corazón de todo)

Clean Architecture es **una sola regla** con muchas consecuencias:

> **Las dependencias del código fuente apuntan SIEMPRE hacia adentro.** Una capa de adentro no sabe NADA de una capa de afuera.

```
        ┌─────────────────────────────────────────┐
        │  Frameworks & Drivers                    │   ← Fiber, pgx, AGE, Qwen HTTP, main.go
        │  ┌───────────────────────────────────┐   │
        │  │  Interface Adapters               │   │   ← handlers, repos (pgx), qwen client, ws hub
        │  │  ┌─────────────────────────────┐  │   │
        │  │  │  Application (Use Cases)    │  │   │   ← la lógica de "qué hace el sistema"
        │  │  │  ┌───────────────────────┐  │  │   │
        │  │  │  │  Domain (Entities)    │  │  │   │   ← reglas de negocio puras, sin frameworks
        │  │  │  └───────────────────────┘  │  │   │
        │  │  └─────────────────────────────┘  │   │
        │  └───────────────────────────────────┘   │
        └─────────────────────────────────────────┘

   Las flechas de import van → hacia el centro. NUNCA al revés.
```

¿Por qué esta regla y no otra? Porque lo que cambia seguido (el framework web, la base de datos, el proveedor de LLM) queda **afuera**, y lo que casi nunca cambia (las reglas de negocio) queda **adentro**, sin depender de lo volátil. Cuando Qwen se caiga de precio y quieras cambiar de LLM, tocás **un adapter**, no toda la lógica.

El truco técnico que hace posible que "adentro no dependa de afuera" pero igual pueda *usar* la base de datos se llama **Inversión de Dependencias**: el caso de uso define una **interfaz** (un *port*) que dice "necesito algo que sepa guardar entidades"; el adapter de Postgres **implementa** esa interfaz. La flecha de import va del adapter (afuera) hacia el port (adentro). Inversión.

**Guardate estos cuatro términos, los vas a usar todo el tiempo:**

| Término | Qué es | Ejemplo en Quill |
|---------|--------|------------------|
| **Domain** | Objetos y reglas de negocio puras. Cero imports de frameworks, DB o HTTP. | `Entity` con su método `Merge`, la regla "un label debe ser un identificador válido". |
| **Use case (Application)** | Orquesta el "qué hace el sistema", dependiendo **solo de ports**. | `ResolveOrCreate`: buscar-o-crear una entidad fusionando datos. |
| **Port** | Una interfaz **definida por adentro**, **implementada por afuera**. | `EntityRepository`, `LLMClient`, `GraphStore`. |
| **Adapter** | Traduce entre un port y el mundo real (pgx, Fiber, Qwen). | `postgres.EntityRepo`, `http.EntityHandler`, `qwen.Client`. |

---

## Parte 1 — Diagnóstico + la arquitectura target

### La foto actual: buen layering, cero inversión

El backend de Quill **ya tiene capas por carpeta** (`handlers/ → services/ → repositories/ → models/`). Eso está bien. El problema es que las flechas de dependencia apuntan para **cualquier lado menos hacia un centro estable**. Cinco síntomas concretos, todos verificables en el código de hoy:

| # | Síntoma | Evidencia (código actual) |
|---|---------|---------------------------|
| 1 | **Los services dependen del driver de la DB, no de una abstracción.** Guardan `*pgxpool.Pool` y `*repositories.XxxRepo` concretos. pgx se filtra a **11 archivos de service**. | `entity_service.go:10,17` (`import pgxpool`, campo `pool *pgxpool.Pool`); `universe_service.go:10,65`; `analysis_service.go:11,61`. |
| 2 | **`pgx.Tx` es parte de la API pública de los repos** → la transacción se maneja en el service. | `entity_repo.go:24` → `Create(ctx, tx pgx.Tx, e *models.Entity)`. El service abre y comitea: `universe_service.go:89-110`, `entity_service.go:94-170`. |
| 3 | **`models.go` es un archivo-Dios de 332 líneas.** Mezcla structs de dominio + DTOs de API + payloads de WebSocket. Solo tags `json`, cero `db`, cero métodos → **dominio anémico**. | `models.go:10-147` (dominio), `:151-247` (DTOs), `:249-332` (WS). |
| 4 | **Handlers que saltan directo a los repos**, salteando el service. | `main.go:145,148` (`NewContradictionHandler(contraSvc, contradictionRepo)`, `NewGraphHandler(graphRepo, ...)`); `contradiction.go` usa `contradictionRepo` directo. |
| 5 | **Ya hay interfaces sueltas** como *test seams* oportunistas, pero no como arquitectura (ninguna en la capa de repos). | `analysis_service.go:38,45,52` (`AnalysisHub`, `Reactivatr`, `EntityResolvr`); `agent_tools.go` (`ToolExecutor`); `ws/hub.go:19-59`. |

### Lo que YA está bien (para calibrar el ojo)

No todo es un desastre — y saber distinguir lo sano de lo enfermo es medio refactor:

- **Fiber está contenido.** Vive **solo** en `handlers/`, `middleware/` y `main.go`. NUNCA aparece en `services/`, `repositories/`, `models/` ni `ws/`. Esa es exactamente la pinta de **una frontera sana**: el framework web no contamina la lógica. Vas a usar esto como vara: "el objetivo es que pgx quede tan contenido como Fiber ya lo está".
- **Config está bien inyectado.** No hay singleton global; `config.Load()` se llama una vez en `main.go:29` y se inyecta. `os.Getenv` vive **solo** en `config.go`. Los services no leen el ambiente.

> **La frase del senior:** "Fiber ya te muestra cómo se ve un límite respetado. pgx te muestra cómo se ve uno reventado. Mismo proyecto, dos culturas. Tu trabajo es que pgx aprenda modales."

### La arquitectura target (Clean clásico, por capa)

Elegiste el layout **por capa** (el concéntrico de Uncle Bob) con **full clean** (ports & adapters + dominio rico + DTOs separados). Este es el árbol destino:

```
backend/
  cmd/server/main.go              # composition root: EL ÚNICO lugar que conoce todos los tipos concretos
  internal/
    domain/                       # reglas de negocio puras — cero imports de frameworks/db/http
      user.go  universe.go  work.go  chapter.go
      entity.go  contradiction.go  timeline.go  plot_hole.go
      ingestion.go  memory.go  relevance.go
      errors.go                   # sentinel errors del dominio (ErrInvalidIdentifier, ErrEntityNotFound, ...)
    app/                          # casos de uso + los ports que necesitan
      ports/                      # interfaces DEFINIDAS por app, IMPLEMENTADAS por adapters
        repository.go             # UserRepository, UniverseRepository, EntityRepository, ...
        gateway.go                # TxManager, LLMClient, Embedder, GraphStore, VectorStore, EventPublisher
      auth/       service.go
      universe/   service.go
      entity/     service.go
      contradiction/ service.go  tools.go
      memory/     service.go  fusion.go
      analysis/   service.go
    adapter/                      # traducen entre ports y el mundo exterior
      repository/postgres/        # implementaciones pgx de los ports *Repository / *Store
        user_repo.go  entity_repo.go  graph_repo.go  vector_repo.go
        tx.go                     # TxManager (pgx) — maneja la transacción por context
      http/                       # controllers Fiber + DTOs de request/response + mappers
        auth_handler.go  dto.go  mapper.go  ...
      qwen/                       # cliente HTTP de Qwen que implementa LLMClient + Embedder
        client.go
      ws/                         # adapter de WebSocket que implementa EventPublisher
        hub.go  protocol.go
    infra/                        # frameworks & drivers glue
      config/  config.go
      db/      pool.go            # pgxpool + bootstrap de AGE (AfterConnect)
```

**Qué vive en cada capa y por qué:**

| Capa | Qué mete | Qué NO mete | Importa a… |
|------|----------|-------------|-----------|
| `domain/` | Structs con **comportamiento** (métodos, invariantes, validación). | Framework, DB, HTTP, JSON tags. | Nada del proyecto (solo stdlib + `uuid`/`time`). |
| `app/ports/` | **Interfaces** que los casos de uso necesitan. | Implementaciones. | Solo `domain`. |
| `app/<feature>/` | La orquestación: "qué pasos da el sistema". | pgx, Fiber, Qwen concretos. | `domain` + `app/ports`. |
| `adapter/*` | Traducción: pgx↔port, Fiber↔use case, Qwen↔port. | Reglas de negocio. | `domain` + `app/ports` (+ `app/<feature>` en el caso de los controllers). |
| `infra/` | Config, pool de conexiones, bootstrap. | Lógica de negocio. | Nada de dominio. |
| `cmd/server/main.go` | El cableado: construye concretos y los inyecta. | Lógica. | **Todo.** |

**Verificación mental de la Regla de Dependencia:** todas las flechas de import apuntan hacia `domain/`. `main.go` es el único que importa todo lo concreto — y está bien, porque es el borde más externo, donde se ensambla la aplicación.

### Dónde van los ports: `app/ports/` vs "interfaz en el consumidor"

Elegiste un paquete **`app/ports/` compartido**, y para aprender está perfecto porque es **descubrible**: abrís `ports/` y ves todo el contrato del sistema de un saque. Pero seas consciente del tradeoff, porque un senior te lo va a marcar:

| Enfoque | A favor | En contra |
|---------|---------|-----------|
| **`app/ports/` compartido** (tu elección) | Todo el contrato en un lugar. Fácil de enseñar y de auditar. | Riesgo de paquete gordo; todos los features importan el mismo `ports`, acoplándolos por transitividad. |
| **Interfaz en el consumidor** (idiom Go: *"accept interfaces, return structs"*) | Cada use case declara solo la interfaz mínima que usa, al lado de donde la usa. Paquetes chicos, desacoplados. | La misma interfaz puede aparecer duplicada en dos features. |

> **La frase del senior:** "En Go idiomático, la interfaz la define **quien la consume**, no quien la implementa. `app/ports/` es una decisión de descubribilidad, no de idiom. La tomamos a propósito para que aprendas viendo el contrato entero. En un proyecto de producción, la mitad de esos ports terminarían viviendo al lado de su use case."

---

## Parte 2 — Preparación (la red de seguridad)

**No toques una sola línea de arquitectura hasta tener esto.**

### Paso 0 — ¿Está todo verde HOY?

```bash
cd backend
go build ./...
go test ./...
```

Los tests de integración (repos/handlers que tocan Postgres) necesitan `TEST_DATABASE_URL`; sin eso hacen `t.Skip`. Para la red completa querés esos también:

```bash
docker compose up -d postgres    # Postgres con pgvector + AGE
TEST_DATABASE_URL='postgres://quill:quill_dev_password@localhost:5432/quill?sslmode=disable' \
  go test ./...
```

> **Ojo con el estado de la DB:** el harness aplica migraciones directo y **no** escribe `schema_migrations`. Después de correr la suite contra el Postgres de compose, hacé `docker compose down -v` antes de levantar el stack completo, o el servicio `migrations` va a fallar con "relation already exists".

### Paso 1 — Entendé qué cubre la red (y parchá los agujeros)

Andá test por test y preguntate: **¿este test fija comportamiento, o fija implementación?** Un test que dice "el service llama a `pool.Begin`" no te sirve para refactorizar — se va a romper aunque el comportamiento no cambie. Un test que dice "registrar un usuario nuevo devuelve un token válido" es oro: sobrevive a cualquier reestructura interna.

Donde falte cobertura de **comportamiento observable**, agregá un **characterization test** (Michael Feathers, *Working Effectively with Legacy Code*): un test que documenta lo que el sistema hace HOY, aunque no sepas si es "correcto". No estás arreglando bugs; estás **clavando el comportamiento actual** para que el refactor no lo mueva sin que te enteres.

```go
// Ejemplo de characterization test: no juzga, solo fija.
// "Con este input, hoy el sistema devuelve exactamente esto."
func TestResolveOrCreate_ExactNameMatch_MergesAndReturnsExisting(t *testing.T) {
    // ... arrange: universe con una entity "Aragorn"
    // ... act: ResolveOrCreate con name "Aragorn" + nuevo alias
    // ... assert: isNew == false, el alias nuevo aparece, el ID es el mismo
}
```

### Paso 2 — El patrón estrangulador (strangler fig)

No vas a borrar `services/` y `repositories/` de un saque. Vas a hacer crecer la estructura nueva **al lado** de la vieja, slice por slice, hasta que la vieja quede vacía y la podás borrar.

```
internal/
  domain/        ← nace vacío, se llena slice por slice
  app/           ← idem
  adapter/       ← idem
  infra/         ← idem
  models/        ← sigue vivo, se vacía de a poco
  services/      ← sigue vivo, se vacía de a poco
  repositories/  ← sigue vivo, se vacía de a poco
  handlers/      ← sigue vivo, se vacía de a poco
```

Cada capítulo mueve **un slice** de lo viejo a lo nuevo, re-cablea `main.go` para que use la versión nueva de ese slice, corre los tests, commitea. El resto del sistema sigue andando sobre lo viejo. **El árbol nunca deja de compilar.**

> **La frase del senior:** "Refactor de big bang es la forma más elegante de romper producción un viernes. El estrangulador es aburrido y siempre funciona. Elegí aburrido."

Con la red puesta y la estructura vacía lista, arrancamos por el slice más simple que existe. Vamos.

---

## La plantilla de cada slice

Todos los capítulos siguen **la misma forma**. Cuando la internalices, cada slice se vuelve mecánico:

1. **Objetivo** — qué slice y por qué va en este orden.
2. **Radiografía actual** — los archivos de hoy y su acoplamiento (con refs reales).
3. **Diseño target** — qué archivos nuevos, en qué capa, y **por qué se parte así**.
4. **Paso a paso** — `domain → ports → app → adapter → re-cablear main.go`. Snippets representativos; el código lo escribís vos.
5. **El "por qué" del senior** — la decisión de diseño clave del slice.
6. **Probalo** — qué tests corren, qué *fake* podés escribir ahora que invertiste las deps, cómo verificar de punta a punta.
7. **Checkpoint** — compila, verde, commit.

---

## Cap. 3 — Slice Auth (trivial) · la Regla de Dependencia en su forma pura

### 3.1 Objetivo y por qué va primero

Auth es el slice **más autocontenido** del sistema: toca **solo Postgres** (la tabla `users`), hace hashing con `bcrypt` y firma JWT en proceso. **Sin Qwen, sin vector, sin graph, sin transacciones.** Es el lugar perfecto para aprender la Regla de Dependencia sin ruido: un dominio, un port, un adapter, un controller. Si entendés Auth, entendés el 80% del patrón.

### 3.2 Radiografía actual

Cadena: `handlers/auth.go → services/auth_service.go → repositories/user_repo.go`.

El `AuthService` de hoy depende del **repo concreto** (`auth_service.go:17-22`):

```go
type AuthService struct {
    userRepo   *repositories.UserRepo   // ← tipo CONCRETO. Acá está el acoplamiento.
    jwtSecret  string
    jwtExpiry  time.Duration
    bcryptCost int
}
```

¿Por qué es un problema si "anda igual"? Porque para **testear** `Register` necesitás un `*repositories.UserRepo` de verdad, que necesita un `*pgxpool.Pool` de verdad, que necesita un Postgres de verdad. Un test de una regla de negocio (¿se hashea el password? ¿se firma el token?) termina arrastrando una base de datos. Eso es el olor.

### 3.3 Diseño target

| Archivo nuevo | Capa | Qué contiene |
|---------------|------|--------------|
| `domain/user.go` | domain | El struct `User`, sin tags `json`. |
| `app/ports/repository.go` | ports | `UserRepository interface` (lo empezamos acá). |
| `app/auth/service.go` | app | El use case, dependiendo de `ports.UserRepository`. |
| `adapter/repository/postgres/user_repo.go` | adapter | La implementación pgx (mudanza de `repositories/user_repo.go`). |
| `adapter/http/auth_handler.go` + `dto.go` | adapter | El controller Fiber + los DTOs `RegisterRequest`/`LoginRequest`/`AuthResponse`. |

**Por qué se parte así:** el `User` (dato de negocio) no tiene por qué saber de JSON — eso es un detalle de cómo lo serializa la capa HTTP. Los DTOs (`RegisterRequest`, etc.) SÍ son HTTP y se van a `adapter/http`. La regla "hashear + firmar" es use case. El SQL es adapter. Cada cosa a su capa.

### 3.4 Paso a paso

**(a) Domain** — mudás `User` (de `models.go:10-17`) a `domain/user.go`, sin tags:

```go
// internal/domain/user.go
package domain

type User struct {
    ID          uuid.UUID
    Email       string
    DisplayName string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

> Fijate que `PasswordHash` **desaparece del struct de dominio**. El hash es un detalle de persistencia/seguridad, no un atributo del usuario de negocio. El repo lo maneja aparte (mirá la firma del port abajo: devuelve `(*User, hash, error)`).

**(b) Port** — la interfaz que el use case necesita. Es un **contrato mínimo**: solo los tres métodos que Auth usa.

```go
// internal/app/ports/repository.go
package ports

type UserRepository interface {
    Create(ctx context.Context, u *domain.User, passwordHash string) error
    FindByEmail(ctx context.Context, email string) (user *domain.User, passwordHash string, err error)
    FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}
```

**(c) App** — el use case. **Un solo cambio conceptual** respecto de hoy: el campo pasa de `*repositories.UserRepo` (concreto) a `ports.UserRepository` (interfaz). La lógica de `Register`/`Login`/`generateToken` (hoy en `auth_service.go:33-118`) se muda **tal cual**.

```go
// internal/app/auth/service.go
package auth

type Service struct {
    users      ports.UserRepository   // ← interfaz, no *repositories.UserRepo
    jwtSecret  string
    jwtExpiry  time.Duration
    bcryptCost int
}

func New(users ports.UserRepository, cfg Config) *Service { ... }

// Register / Login / generateToken: idéntico a auth_service.go, pero s.users es una interfaz.
```

**(d) Adapter** — el repo pgx. Se muda de `repositories/user_repo.go` a `adapter/repository/postgres/user_repo.go`. Casi sin cambios: sigue guardando `*pgxpool.Pool`, sigue haciendo `rows.Scan`. **La clave:** ahora **satisface `ports.UserRepository`** por tipado estructural (Go no necesita un `implements` explícito). Agregá una línea de assert para que el compilador te avise si te desviás del contrato:

```go
// internal/adapter/repository/postgres/user_repo.go
var _ ports.UserRepository = (*UserRepo)(nil)   // compile-time check: implementa el port
```

Y el controller (`adapter/http/auth_handler.go`) se muda de `handlers/auth.go`, con los DTOs a `adapter/http/dto.go`.

**(e) Re-cablear `main.go`** — hoy dice (`main.go:72,91,136`):

```go
userRepo := repositories.NewUserRepo(pool)
authSvc  := services.NewAuthService(userRepo, cfg)
authH    := handlers.NewAuthHandler(authSvc)
```

Pasa a construir los tipos nuevos. La **firma no cambia** (Go inyecta `*postgres.UserRepo` donde se espera `ports.UserRepository` por estructura):

```go
userRepo := postgres.NewUserRepo(pool)          // implementa ports.UserRepository
authSvc  := auth.New(userRepo, authConfigFrom(cfg))
authH    := httpadapter.NewAuthHandler(authSvc)
```

### 3.5 El "por qué" del senior

> "¿Ves que el use case ahora dice `ports.UserRepository` y no `*repositories.UserRepo`? Esa línea es TODO Clean Architecture en miniatura. El use case ya no sabe que existe Postgres. No sabe que existe pgx. Sabe que existe *algo que puede crear y buscar usuarios*. Mañana ese algo puede ser Postgres, un mock, un archivo de texto o Redis. **El use case no se entera, y no debería.**"

### 3.6 Probalo — el payoff que hace que valga la pena

Acá se ve la ganancia por primera vez. **Ahora podés testear `Register` sin base de datos**, porque el use case acepta una interfaz y vos le pasás un *fake*:

```go
// internal/app/auth/service_test.go — CERO Postgres, CERO bcrypt lento de verdad
type fakeUsers struct {
    created *domain.User
    byEmail map[string]struct{ u *domain.User; hash string }
}
func (f *fakeUsers) Create(_ context.Context, u *domain.User, hash string) error {
    f.created = u; return nil
}
func (f *fakeUsers) FindByEmail(_ context.Context, e string) (*domain.User, string, error) {
    r, ok := f.byEmail[e]; if !ok { return nil, "", domain.ErrUserNotFound }
    return r.u, r.hash, nil
}
func (f *fakeUsers) FindByID(context.Context, uuid.UUID) (*domain.User, error) { return nil, nil }

func TestRegister_ReturnsValidToken(t *testing.T) {
    svc := auth.New(&fakeUsers{byEmail: map[string]...{}}, testConfig)
    user, token, err := svc.Register(ctx, "a@b.com", "pw123456", "Ana")
    require.NoError(t, err)
    require.NotEmpty(t, token)                 // se firmó el JWT
    // y validás el token con el mismo secret, sin tocar la DB
}
```

**Comandos:**
```bash
# unit del use case (sin DB, instantáneo):
go test ./internal/app/auth/...
# integración del adapter (con DB real, verifica el SQL):
TEST_DATABASE_URL=... go test ./internal/adapter/repository/postgres/ -run TestUserRepo
# end-to-end, servidor arriba:
curl -s localhost:8080/api/v1/auth/register -d '{"email":"a@b.com","password":"pw123456","display_name":"Ana"}'
```

Y la red original sigue siendo tu juez: el viejo `auth_service_test.go` (comportamiento) tiene que **seguir verde** hasta que lo migres.

### 3.7 Checkpoint

`go build ./... && go test ./...` verde → `git commit -m "refactor(auth): extract domain, port and adapter"`. Slice cerrado.

---

## Cap. 4 — Slice Universe/Work/Chapter (simple) · de-anemizar + transacciones

### 4.1 Objetivo

CRUD simple, pero con **dos lecciones grandes** que Auth no tenía:
1. **De-anemizar el dominio**: la validación de enums (`validateUniverseEnums`) es una **regla de negocio** que hoy vive suelta en el service. Va al dominio, como invariante.
2. **El problema de las transacciones**: hoy el service abre `pgx.Tx` y se lo pasa al repo. Eso filtra el detalle de persistencia al use case. Lo resolvemos con un port **`TxManager`** (Unit of Work).

Hacemos Universe en detalle; Work y Chapter son el mismo patrón (Work es aún más trivial —no toca el graph—; Chapter arrastra `RelevanceService`, lo dejás para después del interludio).

### 4.2 Radiografía actual

`UniverseService.Create` (`universe_service.go:78-121`) hace tres cosas mezcladas:

```go
func (s *UniverseService) Create(ctx, userID, input) (*models.Universe, error) {
    // 1) VALIDACIÓN (regla de negocio, pero acá está suelta)
    if input.Name == "" { return nil, fmt.Errorf("universe name is required") }
    if input.Format == "" { return nil, fmt.Errorf("universe format is required") }
    if err := validateUniverseEnums(input); err != nil { return nil, err }

    // 2) TRANSACCIÓN (detalle de persistencia, filtrado al use case)
    tx, err := s.pool.Begin(ctx)              // ← el service conoce pgx
    if err != nil { ... }
    defer tx.Rollback(ctx)
    u := &models.Universe{ ID: uuid.New(), ... }
    if err := s.universeRepo.Create(ctx, tx, u); err != nil { return nil, err }  // ← le pasa el tx
    if err := tx.Commit(ctx); err != nil { ... }

    // 3) GRAPH best-effort (fuera de la tx, no fatal)
    if s.graphRepo != nil { s.graphRepo.CreateGraph(ctx, u.ID.String()) }
    return u, nil
}
```

Y el repo expone el `tx` en su firma pública (`entity_repo.go:24`, mismo patrón en `universe_repo.go`):

```go
func (r *EntityRepo) Create(ctx context.Context, tx pgx.Tx, e *models.Entity) error
//                                        ^^^^^^^^^^ pgx.Tx en la API pública del repo
```

Detalle fino que confirma el olor: `Create` toma `tx pgx.Tx`, pero `FindByID` (`entity_repo.go:36-43`) usa `r.pool` directo. **Modelo mixto**: las escrituras piden un tx de afuera, las lecturas usan el pool. El repo no es dueño de su propia transaccionalidad — se la delega al service. Eso es lo que vamos a invertir.

### 4.3 Diseño target

| Archivo | Capa | Qué |
|---------|------|-----|
| `domain/universe.go` | domain | `Universe` + `NewUniverse(...) (*Universe, error)` con los invariantes de enum. |
| `domain/errors.go` | domain | `ErrInvalidGenre`, `ErrInvalidFormat`, `ErrUniverseNameRequired`, ... |
| `app/ports/repository.go` | ports | `UniverseRepository` (sin `pgx.Tx` en las firmas). |
| `app/ports/gateway.go` | ports | `TxManager` + `GraphStore` (este último lo definís acá y lo completás en el interludio). |
| `app/universe/service.go` | app | Use case que llama `domain.NewUniverse` y envuelve las escrituras en `TxManager.WithinTx`. |
| `adapter/repository/postgres/{universe_repo.go, tx.go}` | adapter | Repo pgx + la implementación de `TxManager`. |
| `adapter/http/universe_handler.go` + DTOs | adapter | Controller + `CreateUniverseRequest`. |

### 4.4 Paso a paso

**(a) De-anemizar: la validación es del dominio.** Los allowlists de género/formato (`universe_service.go:16-40`) y la validación (`:42-54`) describen **qué es un Universe válido**. Eso es una invariante de dominio, no una función suelta del service. Convertilo en un **constructor que no te deja crear un Universe inválido**:

```go
// internal/domain/universe.go
package domain

var (
    allowedGenres  = map[string]struct{}{"sci-fi": {}, "fantasy": {}, "mystery": {}, /* ... */}
    allowedFormats = map[string]struct{}{"novel": {}, "short-story": {}, "screenplay": {}, /* ... */}
)

type Universe struct {
    ID          uuid.UUID
    UserID      uuid.UUID
    Name        string
    Description string
    Genre       string
    Format      string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// NewUniverse es la ÚNICA forma de crear un Universe. Si compila y no dio error,
// es válido por construcción. Los invariantes viven acá, no en el service.
func NewUniverse(userID uuid.UUID, name, description, genre, format string) (*Universe, error) {
    if name == "" {
        return nil, ErrUniverseNameRequired
    }
    if format == "" {
        return nil, ErrUniverseFormatRequired
    }
    if genre != "" {
        if _, ok := allowedGenres[genre]; !ok {
            return nil, fmt.Errorf("%w: %q", ErrInvalidGenre, genre)
        }
    }
    if _, ok := allowedFormats[format]; !ok {
        return nil, fmt.Errorf("%w: %q", ErrInvalidFormat, format)
    }
    return &Universe{ID: uuid.New(), UserID: userID, Name: name, Description: description, Genre: genre, Format: format}, nil
}
```

> **El "por qué" del split:** ahora un `Universe` inválido **no se puede construir**. La regla vive pegada al dato que protege. Cualquier caso de uso — HTTP, ingestión, demo clone, un test — que quiera crear un Universe pasa por acá y hereda la validación gratis. En el modelo anémico, cada camino de entrada tenía que acordarse de llamar `validateUniverseEnums`. Uno se olvida → bug. Con la invariante en el constructor, **olvidarse es imposible**.

**(b) El port `TxManager` — la lección más jugosa del capítulo.** Queremos que el use case diga "hacé esto de forma atómica" **sin nombrar pgx**. El patrón Unit of Work: un port que recibe una función y la corre dentro de una transacción.

```go
// internal/app/ports/gateway.go
package ports

type TxManager interface {
    WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
```

Los repos **ya no reciben `tx`**. Sus firmas se limpian:

```go
// internal/app/ports/repository.go
type UniverseRepository interface {
    Save(ctx context.Context, u *domain.Universe) error      // ← sin pgx.Tx
    FindByID(ctx context.Context, id uuid.UUID) (*domain.Universe, error)
    ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]domain.Universe, int, error)
    Delete(ctx context.Context, id uuid.UUID) error
}
```

**El truco** está en el adapter: la transacción viaja **en el `context`**, y cada repo saca del context el ejecutor correcto (el `tx` si hay uno activo, o el `pool` si no). Como `pgx.Tx` y `*pgxpool.Pool` comparten los métodos `Exec/Query/QueryRow`, definís una interfaz mínima y ambos la satisfacen:

```go
// internal/adapter/repository/postgres/tx.go
package postgres

// Querier es el subconjunto común de *pgxpool.Pool y pgx.Tx.
type Querier interface {
    Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
    Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

type TxManager struct{ pool *pgxpool.Pool }

func (m *TxManager) WithinTx(ctx context.Context, fn func(context.Context) error) error {
    tx, err := m.pool.Begin(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback(ctx) // no-op si ya se comiteó
    if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
        return err // rollback vía el defer
    }
    return tx.Commit(ctx)
}

// querier: cada repo pide "el ejecutor de este context".
func querier(ctx context.Context, pool *pgxpool.Pool) Querier {
    if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
        return tx // dentro de WithinTx → usa la transacción
    }
    return pool // afuera → usa el pool directo
}
```

El repo pgx queda así (mismo SQL de hoy, pero sin `tx` en la firma):

```go
// internal/adapter/repository/postgres/universe_repo.go
func (r *UniverseRepo) Save(ctx context.Context, u *domain.Universe) error {
    _, err := querier(ctx, r.pool).Exec(ctx, `INSERT INTO universes (...) VALUES (...)`, u.ID, u.UserID, /* ... */)
    return err
}
```

**(c) El use case** ahora es narrativo — se lee como la regla de negocio, sin una línea de pgx:

```go
// internal/app/universe/service.go
func (s *Service) Create(ctx context.Context, userID uuid.UUID, in CreateInput) (*domain.Universe, error) {
    u, err := domain.NewUniverse(userID, in.Name, in.Description, in.Genre, in.Format) // valida
    if err != nil {
        return nil, err
    }
    if err := s.tx.WithinTx(ctx, func(ctx context.Context) error {                     // atómico
        return s.universes.Save(ctx, u)
    }); err != nil {
        return nil, err
    }
    if err := s.graph.CreateGraph(ctx, u.ID.String()); err != nil {                     // best-effort
        s.log.Warn("create AGE graph", "universe", u.ID, "err", err)
    }
    return u, nil
}
```

> Compará este `Create` con el `universe_service.go:78-121` original. Misma lógica, cero pgx, cero `Begin/Commit/Rollback`. La transacción es una **decisión de negocio** ("esto es atómico") expresada con `WithinTx`, no un detalle de driver desparramado en el método.

**(d) Re-cablear `main.go`:** construís `txMgr := postgres.NewTxManager(pool)` una vez y lo inyectás a todos los use cases que escriben.

### 4.5 El "por qué" del senior — y un freno de mano honesto

> "El `TxManager` es el patrón correcto cuando un caso de uso tiene que escribir en **varios repos de forma atómica**. Pero te aviso algo que la mayoría de los tutoriales de Clean Architecture te ocultan: **es el patrón más sobre-aplicado que existe.** Si tu caso de uso guarda **un solo agregado** (como este `Create`, que solo toca `universes`), un `TxManager` es casi ceremonia — el repo podría manejar su propia transacción internamente y listo. Lo hacemos acá porque **lo vas a necesitar de verdad en el slice de Entity** (que escribe entity + history + graph + embedding en un mismo flujo), y quiero que lo aprendas con un ejemplo simple antes de pegarte con el complejo. Aprendé el patrón; después aprendé a **no** usarlo cuando no hace falta."

### 4.6 Probalo

```bash
# El dominio se testea SOLO — es una función pura, sin nada:
go test ./internal/domain/ -run TestNewUniverse   # "fantasy" ok, "xxx" → ErrInvalidGenre
# El use case con fakes (fake UniverseRepository + fake TxManager que solo corre fn(ctx)):
go test ./internal/app/universe/...
# Integración del repo + TxManager con DB real (verifica rollback de verdad):
TEST_DATABASE_URL=... go test ./internal/adapter/repository/postgres/ -run TestUniverseRepo
# E2E:
curl -s -X POST localhost:8080/api/v1/universes -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"Middle-earth","format":"novel","genre":"fantasy"}'
```

El *fake* de `TxManager` en el test del use case es de una línea —`func(ctx, fn) error { return fn(ctx) }`— y ahí ves la belleza: el use case no distingue entre "transacción real" y "corré la función y ya". No le importa. Esa indiferencia **es** el desacople.

### 4.7 Checkpoint

Verde → `git commit -m "refactor(universe): domain invariants + TxManager port, drop pgx.Tx leak"`. Aplicá el mismo molde a Work. Chapter lo cerrás después del interludio (necesita `RelevanceService`, que a su vez toca los boundaries compartidos).

---

## Interludio — Los tres boundaries compartidos: Vector, Graph, LLM

### Por qué acá, y no dentro de cada slice

Los próximos cuatro slices (Entity, Contradiction, Memory, Analysis) comparten **tres dependencias de bajo nivel**:

| Boundary | Repo/service actual | Lo usan los slices… |
|----------|---------------------|---------------------|
| **VectorStore** (pgvector) | `repositories/vector_repo.go` | Entity, Contradiction, Memory, Analysis |
| **GraphStore** (Apache AGE) | `repositories/graph_repo.go` | Universe (provisión), Entity, Contradiction, Memory, Analysis |
| **LLMClient / Embedder** (Qwen) | `services/qwen_service.go` | Entity, Contradiction, Memory, Analysis |

Si invertís estos tres **ahora**, cada slice que sigue es casi mecánico: define su use case, pide los ports que necesita, listo. Si NO lo hacés, vas a re-tocar estos tres archivos **cuatro veces**. Economía de refactor pura: **el trabajo compartido se hace una vez, antes de los consumidores.**

### 4.i — VectorStore

Definís el port con los métodos que los use cases piden (juntá las firmas reales de `vector_repo.go`):

```go
// internal/app/ports/gateway.go
type VectorStore interface {
    FindSimilarEntity(ctx context.Context, universeID uuid.UUID, embedding []float32, threshold float64) (*uuid.UUID, float64, error)
    SaveEntityEmbedding(ctx context.Context, entityID uuid.UUID, embedding []float32) error
    FindSimilarParagraphs(ctx context.Context, universeID uuid.UUID, embedding []float32, k int) ([]domain.ParagraphMatch, error)
    KeywordSearch(ctx context.Context, universeID uuid.UUID, query string, k int) ([]domain.ParagraphMatch, error)
}
```

**El detalle Clean:** el port habla en `[]float32` (un tipo del dominio), **no** en `pgvector.Vector`. El adapter convierte `[]float32 ↔ pgvector.Vector` puertas adentro. pgvector no cruza la línea del port. Nadie arriba del adapter sabe que la base guarda vectores en un tipo especial.

### 4.ii — GraphStore (el sharp edge del proyecto)

Este es **el archivo más filoso del codebase** (`graph_repo.go`): AGE no soporta queries parametrizadas dentro de los bloques Cypher `$$ $$`, así que cada punto de interpolación se defiende a mano. La pregunta de arquitectura es: **¿qué de esto es dominio y qué es adapter?**

```
graph_repo.go, hoy:
  ├─ nombres de grafo (UUID-derived, seguros por construcción)   → ADAPTER (detalle AGE)
  ├─ escapeCypherString (escapeo de strings)                     → ADAPTER (detalle AGE)
  ├─ withAgeTx / withAgeConn (LOAD 'age', SET search_path,       → ADAPTER (detalle AGE)
  │  y RESTAURAR el search_path para no envenenar la conexión)
  └─ validCypherIdentifier: ^[A-Za-z_][A-Za-z0-9_]*$             → ¿DOMINIO o ADAPTER?
     (valida labels/rel-types que vienen del LLM)
```

La regla `validCypherIdentifier` es sutil. Viene del hecho de que **labels y rel-types salen del output del LLM** (extracción de entidades/relaciones). ¿Es "detalle de Cypher" o "regla de negocio"? Argumento del senior: **es las dos cosas, y por eso va en las dos capas** (defensa en profundidad):

- **En el dominio**, como invariante: un `domain.Entity` o `domain.Relationship` **se niega a existir con un tipo inválido**. El `ErrInvalidIdentifier` se muda a `domain/errors.go`. El dominio no sabe qué es Cypher — solo sabe que "un tipo de entidad debe ser un identificador con forma `[A-Za-z_][...]`", que resulta ser una regla de modelado razonable de por sí.
- **En el adapter**, como último cerrojo antes de interpolar en Cypher: aunque el dominio ya validó, el adapter **vuelve a chequear** en la frontera con AGE. Nunca confiés ciegamente en que el de arriba validó, cuando abajo estás construyendo SQL/Cypher a mano.

Todo lo demás —`escapeCypherString`, `withAgeTx`, el baile del `search_path`— es **100% adapter**. Se queda enterrado en `adapter/repository/postgres/graph_repo.go`. **El use case nunca se entera de que AGE existe.**

```go
// internal/app/ports/gateway.go
type GraphStore interface {
    CreateGraph(ctx context.Context, universeID string) error
    CreateNode(ctx context.Context, universeID, label string, props map[string]any) error
    CreateEdge(ctx context.Context, universeID, fromID, toID, relType string, props map[string]any) error
    GetNeighbors(ctx context.Context, universeID, entityID string) ([]domain.GraphNeighbor, error)
    GetNeighborsBatch(ctx context.Context, universeID string, entityIDs []string) (map[string][]domain.GraphNeighbor, error)
}
```

> **El "por qué" del senior:** "Este es el capítulo donde Clean Architecture se paga sola. El `search_path` de AGE es el bug más traicionero del proyecto —envenenás una conexión del pool y escribe tablas sombra en `ag_catalog`. Con el port, esa complejidad queda **sellada dentro del adapter**. El caso de uso pide `GetNeighbors` y recibe `[]domain.GraphNeighbor`. No sabe de `LOAD 'age'`, no sabe de `search_path`, no puede envenenar nada porque no tiene acceso. **La frontera no es burocracia: es una jaula para el código peligroso.**"

### 4.iii — LLMClient / Embedder

`qwen_service.go` hace HTTP contra la API de Qwen (function calling estilo OpenAI + embeddings). Hoy **todos los services embeben el `*QwenService` concreto**. Lo ponemos detrás de dos ports:

```go
// internal/app/ports/gateway.go
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
}

type LLMClient interface {
    // Chat soporta tool-calling: devuelve el texto y/o los tool calls que el modelo pidió.
    Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatReply, error)
    ExtractEntities(ctx context.Context, text string) ([]domain.ExtractedEntity, error)
    AnalyzeRelationships(ctx context.Context, text string, entities []domain.Entity) ([]domain.Relationship, error)
}
```

**La distinción fina (que se paga en el Cap. 6):** el **loop ReAct** (`RunAgentLoop`: el modelo propone tools → ejecutás → le devolvés el resultado → repite hasta que para) es **política de aplicación** — vive en `app`. El **formato de function-calling de OpenAI** (cómo se serializan los `tool_calls` en el JSON del request) es **detalle de adapter** — vive en `adapter/qwen`. El adapter expone un `Chat` limpio que devuelve `domain.ChatReply` con los tool calls ya parseados a un tipo del dominio; el loop de arriba orquesta sin tocar JSON de Qwen.

> **La frase del senior:** "El agente no es Qwen. El agente es un patrón: pensar, actuar, observar, repetir. Qwen es solo el cerebro que hoy le enchufás. Si mañana lo cambiás por otro modelo con function calling, el loop no se toca **una línea** — cambiás el adapter. Por eso el loop va en `app` y el HTTP va en `adapter`. Mezclarlos es el error que comete el 90% de la gente que 'integra un LLM'."

Con los tres boundaries ya invertidos, los slices 3–6 se vuelven casi mecánicos. Seguimos con Entity.

---

## Cap. 5 — Slice Entity (medium) · un caso de uso que orquesta cuatro backends

### 5.1 Objetivo

Primer slice que toca **los cuatro backends a la vez**: Postgres + pgvector + AGE graph + Qwen. Es el pago del interludio: como los tres ports ya existen, el use case solo los orquesta. Tres lecciones nuevas:
1. **Un service que se construye su propia dependencia** (`repositories.NewGraphRepo(s.pool)` inline) → inyectar.
2. **`mergeEntity`**, hoy un método del service que muta una copia, es **comportamiento de dominio** → `domain.Entity.Merge`.
3. **Tipos de la capa de repos que se filtran a las firmas** (`repositories.ExtractedEntity`, `repositories.EntityFilters`) → moverlos a `domain`/`app`.

### 5.2 Radiografía actual

`entity_service.go` tiene el olor #3 (se construye deps) bien a la vista, dentro de `ResolveOrCreate` (`entity_service.go:180-181`):

```go
// Create node in AGE graph
if s.pool != nil {
    graphRepo := repositories.NewGraphRepo(s.pool)   // ← el service se fabrica su propia dependencia
    ...
    graphRepo.CreateNode(ctx, graphName, newEntity.Type, props)
}
```

¿Por qué es malo? Porque esa dependencia es **invisible desde afuera**. Mirás el constructor `NewEntityService(pool, entityRepo, vectorRepo, qwenSvc)` (`entity_service.go:24`) y jamás sospecharías que también habla con el graph. No la podés mockear. No la podés sustituir. Está escondida adentro de un método. Y encima hay una **segunda** dependencia escondida: `historyRepo: repositories.NewEntityRelevanceHistoryRepo(pool)` se fabrica en el propio constructor (`:30`).

Y el `mergeEntity` (`entity_service.go:207-239`) es lógica de negocio pura disfrazada de método de service:

```go
func (s *EntityService) mergeEntity(existing *models.Entity, newData repositories.ExtractedEntity) *models.Entity {
    merged := *existing
    // union de aliases, "la descripción más larga gana", merge de properties, update de status...
    return &merged
}
```

No usa `s` para nada (ni el pool, ni un repo) — es una función que dado un entity y datos nuevos, produce el entity fusionado. **Eso grita "soy del dominio".**

### 5.3 Diseño target

| Archivo | Capa | Qué |
|---------|------|-----|
| `domain/entity.go` | domain | `Entity` + `Merge(ExtractedEntity)` + `AddAlias` + const `InitialRelevanceScore`. |
| `domain/entity.go` | domain | `ExtractedEntity` (era `repositories.ExtractedEntity` — es un concepto de dominio: "lo que el LLM extrajo"). |
| `app/entity/query.go` | app | `EntityQuery` (era `repositories.EntityFilters` — es un parámetro de caso de uso, no de repo). |
| `app/entity/service.go` | app | `ResolveOrCreate` orquestando `EntityRepository` + `VectorStore` + `GraphStore` + `Embedder` + `TxManager`, **todos inyectados**. |
| `adapter/repository/postgres/entity_repo.go` | adapter | Repo pgx (sin `pgx.Tx` en firmas). |
| `adapter/http/entity_handler.go` + DTOs | adapter | `UpdateEntityRequest`, `EntityBrief` → DTOs HTTP. |

### 5.4 Paso a paso

**(a) Domain rico — `Merge` como comportamiento.** Mudás `mergeEntity` a un método de `domain.Entity`. Fijate que el mismo código, movido de capa, **cambia de significado**: deja de ser "un helper del service" y pasa a ser "cómo un Entity absorbe información nueva" — una regla del negocio.

```go
// internal/domain/entity.go
package domain

const InitialRelevanceScore = 0.8   // era el 0.8 mágico en entity_service.go:161

type Entity struct {
    ID             uuid.UUID
    UniverseID     uuid.UUID
    Type           string
    Name           string
    Aliases        []string
    Description    string
    Properties     json.RawMessage
    Status         string
    RelevanceScore float64
    // ... sin tags json
}

// Merge absorbe datos nuevos de una mención. Regla de negocio:
// aliases se unen, la descripción más larga gana, el status nuevo pisa si viene.
func (e *Entity) Merge(in ExtractedEntity) {
    for _, a := range in.Aliases {
        if !e.hasAlias(a) {
            e.Aliases = append(e.Aliases, a)
        }
    }
    if len(in.Description) > len(e.Description) {
        e.Description = in.Description
    }
    if in.Status != "" {
        e.Status = in.Status
    }
    // properties: merge (el JSON-merge concreto puede quedar como helper)
}

func (e *Entity) hasAlias(a string) bool {
    for _, x := range e.Aliases {
        if x == a { return true }
    }
    return false
}
```

**(b) El use case — inyectá lo que antes se fabricaba.** El constructor ahora **declara todas** sus dependencias (nada escondido):

```go
// internal/app/entity/service.go
type Service struct {
    entities ports.EntityRepository
    vectors  ports.VectorStore
    graph    ports.GraphStore      // ← antes: repositories.NewGraphRepo(s.pool) escondido
    embedder ports.Embedder
    history  ports.HistoryRepository // ← antes: fabricado en el constructor
    tx       ports.TxManager
}

func New(entities ports.EntityRepository, vectors ports.VectorStore, graph ports.GraphStore,
    embedder ports.Embedder, history ports.HistoryRepository, tx ports.TxManager) *Service { ... }
```

Y `ResolveOrCreate` (el funnel de 4 pasos de `entity_service.go:93-205`) se muda casi tal cual, pero:
- La transacción de las 4 ramas se envuelve en **un** `tx.WithinTx(...)` (adiós `Begin/Commit/Rollback` repetidos cuatro veces).
- `s.mergeEntity(existing, data)` pasa a `existing.Merge(data)` — el dominio hace el trabajo.
- `graphRepo := repositories.NewGraphRepo(s.pool)` desaparece: usás `s.graph.CreateNode(...)`, el port inyectado.

```go
func (s *Service) ResolveOrCreate(ctx context.Context, universeID uuid.UUID, data domain.ExtractedEntity) (*domain.Entity, string, bool, error) {
    var result *domain.Entity
    var prevStatus string
    var isNew bool

    err := s.tx.WithinTx(ctx, func(ctx context.Context) error {
        // Pasos 1-3: exact name / alias / semantic (vía s.embedder + s.vectors)
        if existing := s.findExisting(ctx, universeID, data); existing != nil {
            prevStatus = existing.Status
            existing.Merge(data)                    // ← dominio
            result = existing
            return s.entities.Save(ctx, existing)
        }
        // Paso 4: crear nuevo
        e := domain.NewEntity(universeID, data, domain.InitialRelevanceScore)
        result, isNew = e, true
        return s.entities.Save(ctx, e)
    })
    if err != nil { return nil, "", false, err }

    // efectos secundarios post-commit (best-effort): history + graph node + embedding
    if isNew {
        s.history.Append(ctx, result.ID)
        s.graph.CreateNode(ctx, universeID.String(), result.Type, result.GraphProps())
        if emb, err := s.embedder.Embed(ctx, result.Name); err == nil {
            s.vectors.SaveEntityEmbedding(ctx, result.ID, emb)
        }
    }
    return result, prevStatus, isNew, nil
}
```

**(c) Tipos que dejan de vivir en el repo.** `repositories.ExtractedEntity` aparecía en la firma del service (`entity_service.go:93`) y del `EntityResolvr` (`analysis_service.go:53`). Que un **tipo del repo** viaje por las firmas de la lógica es un acople invertido. `ExtractedEntity` es "lo que el LLM extrajo del texto" → **concepto de dominio** → `domain/entity.go`. `EntityFilters` es "cómo filtrás un listado" → **parámetro de caso de uso** → `app/entity/query.go`.

### 5.5 El "por qué" del senior

> "El `graphRepo := repositories.NewGraphRepo(s.pool)` adentro del método es el pecado más común de Go mal escrito: **una dependencia clandestina.** El constructor te miente sobre lo que el objeto necesita. Cuando inyectás TODO por el constructor, el constructor se vuelve la **declaración jurada** de las dependencias: leés la firma y sabés exactamente con qué habla este use case. Sin sorpresas, sin `new` escondidos, sin 'ah, pero también toca el graph'. Un objeto que se fabrica sus propias dependencias es un objeto que no podés testear ni razonar. Punto."

### 5.6 Probalo

```bash
# Merge es dominio puro — el test más lindo del slice, sin nada enchufado:
go test ./internal/domain/ -run TestEntity_Merge   # union de aliases, descripción más larga gana
# El use case con 5 fakes (entity/vector/graph/embedder/history) + fake TxManager:
go test ./internal/app/entity/...
# Integración con DB + AGE:
TEST_DATABASE_URL=... go test ./internal/adapter/repository/postgres/ -run 'TestEntityRepo|TestGraphRepo'
```

> El path de escritura de Entity **no tiene handler HTTP** propio (se dispara desde el pipeline de análisis, Cap. 8). Así que acá tu verificación es `go test` llamando al use case directo. El `handlers/entity.go` de hoy solo cubre lectura (`ListByUniverse`, `GetByID`, `Update`).

### 5.7 Checkpoint

Verde → `git commit -m "refactor(entity): rich domain Merge, inject graph/history, drop repo types from signatures"`.

---

## Cap. 6 — Slice Contradiction (complejo) · el agente ReAct como caso de uso

### 6.1 Objetivo

El call graph más profundo del proyecto. Dos caminos: **determinístico** (reglas, sin LLM) y **semántico** (el loop ReAct con tools). Lecciones:
1. **El agente como caso de uso**: el loop es política de `app`, el function-calling es detalle de `adapter/qwen` (ya lo adelantamos en el interludio; acá se concreta).
2. **`ToolExecutor` ya es una interfaz** (`agent_tools.go`) → promoverla a port y aprovecharla. Casi no hay trabajo.
3. **El `fingerprint` (SHA-256) para dedup** es una **invariante de dominio**.

### 6.2 Radiografía actual

Cadena: `contradiction_service.go` (`CheckDeterministic` + `CheckSemantic`) → `qwen_service.go` (`RunAgentLoop`) → `agent_tools.go` (`QuillExecutor.ExecuteTool` despacha `search_vector_memory` y `query_entity_graph`).

Lo bueno: **el proyecto ya te dejó media escalera puesta.** `agent_tools.go` ya define las interfaces por método (`vectorSearcher`, `graphQuerier`, `entityLister`) y `ToolExecutor`. Eso es exactamente un port, solo que "descubierto" ad hoc para testear, no elevado a arquitectura.

### 6.3 Diseño target

| Archivo | Capa | Qué |
|---------|------|-----|
| `domain/contradiction.go` | domain | `Contradiction` + `NewContradiction(...)` que **computa el fingerprint** + regla de igualdad por fingerprint. |
| `app/ports/gateway.go` | ports | `ToolExecutor` (promovido desde `agent_tools.go`). |
| `app/contradiction/service.go` | app | `CheckDeterministic` (reglas) + `CheckSemantic` (loop). |
| `app/contradiction/agent.go` | app | `RunAgentLoop` (la política ReAct, mudada desde `qwen_service.go`). |
| `app/contradiction/tools.go` | app | Las tools como use cases: `searchVectorMemory` (usa `VectorStore`+`Embedder`), `queryEntityGraph` (usa `EntityRepository`+`GraphStore`). |
| `adapter/qwen/client.go` | adapter | `Chat` con el wire-format de function calling. |

### 6.4 Paso a paso

**(a) Fingerprint como invariante.** Hoy el `Contradiction` tiene un campo `Fingerprint` (`models.go:97`) que alguien, en algún lado, tiene que acordarse de calcular. Movelo al constructor: **una contradicción sin fingerprint no existe.**

```go
// internal/domain/contradiction.go
func NewContradiction(universeID uuid.UUID, entityID *uuid.UUID, severity, description string, evA, evB string) *Contradiction {
    c := &Contradiction{
        ID: uuid.New(), UniverseID: universeID, EntityID: entityID,
        Severity: severity, Description: description, EvidenceA: evA, EvidenceB: evB,
        Status: "open",
    }
    c.Fingerprint = fingerprint(evA, evB, description)   // se calcula SIEMPRE, acá
    return c
}

// dedup: dos contradicciones son "la misma" si comparten fingerprint.
func fingerprint(parts ...string) string {
    h := sha256.Sum256([]byte(strings.Join(parts, "|")))
    return hex.EncodeToString(h[:])
}
```

**(b) El agente: política en `app`, protocolo en `adapter`.** El adapter de Qwen expone un `Chat` que ya te devuelve los tool calls **parseados a tipos de dominio**:

```go
// internal/app/ports/gateway.go
type LLMClient interface {
    Chat(ctx context.Context, req domain.ChatRequest) (domain.ChatReply, error)
}
type ToolExecutor interface {   // promovido desde agent_tools.go, casi igual
    Execute(ctx context.Context, call domain.ToolCall) (result string, err error)
}
```

El loop (mudado de `qwen_service.go:RunAgentLoop`) vive en `app` y **no toca una línea de JSON de Qwen**:

```go
// internal/app/contradiction/agent.go
func RunAgentLoop(ctx context.Context, llm ports.LLMClient, exec ports.ToolExecutor, req domain.ChatRequest, maxDepth int) (domain.ChatReply, error) {
    for depth := 0; depth < maxDepth; depth++ {
        reply, err := llm.Chat(ctx, req)
        if err != nil { return domain.ChatReply{}, err }
        if len(reply.ToolCalls) == 0 {
            return reply, nil                       // el modelo terminó
        }
        for _, call := range reply.ToolCalls {      // ejecutá cada tool...
            result, err := exec.Execute(ctx, call)
            if err != nil { result = "error: " + err.Error() }
            req.Messages = append(req.Messages, domain.ToolResult(call.ID, result)) // ...y devolvé el resultado
        }
    }
    return domain.ChatReply{}, domain.ErrAgentMaxDepth
}
```

**(c) Las tools como use cases.** `search_vector_memory` y `query_entity_graph` (hoy en `agent_tools.go`) pasan a ser casos de uso que dependen de ports, no de repos concretos:

```go
// internal/app/contradiction/tools.go
type toolExecutor struct {
    vectors  ports.VectorStore
    embedder ports.Embedder
    entities ports.EntityRepository
    graph    ports.GraphStore
}

func (t *toolExecutor) Execute(ctx context.Context, call domain.ToolCall) (string, error) {
    switch call.Name {
    case "search_vector_memory":
        emb, _ := t.embedder.Embed(ctx, call.Arg("query"))
        matches, err := t.vectors.FindSimilarParagraphs(ctx, call.UniverseID, emb, 5)
        return formatMatches(matches), err
    case "query_entity_graph":
        // resolvé nombre→ID vía t.entities, después t.graph.GetNeighbors
    }
    return "", domain.ErrUnknownTool
}
```

### 6.5 El "por qué" del senior

> "Fijate lo que pasó con `ToolExecutor`: el proyecto **ya lo había descubierto** como interfaz para poder testear. Eso te dice algo hermoso — **la testabilidad y la buena arquitectura empujan en la misma dirección.** Cada vez que alguien creó una interfaz 'para poder mockear en un test', sin saberlo estaba dibujando un port. Nuestro trabajo en gran parte es **reconocer esos ports accidentales y elevarlos a decisión consciente.** El código ya te estaba pidiendo Clean Architecture; solo que a los gritos y desordenado."

### 6.6 Probalo

```bash
go test ./internal/domain/ -run TestContradiction_Fingerprint   # mismo evidence → mismo fingerprint (dedup)
go test ./internal/app/contradiction/...   # loop con fake LLMClient (scripted: "pedí tool, después respondé")
```

El *fake* de `LLMClient` para testear el loop es didáctico: le programás una secuencia ("primer `Chat` devolvé un tool call; segundo `Chat` devolvé la respuesta final") y verificás que el loop ejecutó la tool y paró. **Testeás el comportamiento del agente sin gastar un token de Qwen.** El `agent_tools_test.go` y `contradiction_service_test.go` actuales son tu red.

### 6.7 Checkpoint

Verde → `git commit -m "refactor(contradiction): agent loop as use case, fingerprint invariant, promote ToolExecutor port"`.

Con Entity y Contradiction hechos, ya viste el patrón completo aplicado a lógica compleja. Quedan los dos slices que integran todo.

---

## Cap. 7 — Slice Memory (muy complejo) · lógica pura testeable sin nada

### 7.1 Objetivo

El use case con **más fan-out de repos** del sistema (4 repos + budget + tokenizer). Pero adentro esconde la **joya pedagógica** de todo el refactor: la fusión RRF, que es **una función pura** — dado cinco listas rankeadas, produce una lista fusionada, sin tocar DB, LLM ni nada. Lecciones:
1. **Lógica pura al dominio**: `fuseRRF` no necesita ports; es matemática. Se testea con listas hechas a mano.
2. **Deps opcionales inyectadas por setter** (`SetConsolidationRepo`, `SetBudgetMgr`, `SetHistoryRepo`) → inyección explícita con *functional options* o *null object*.
3. **La concurrencia** (`WaitGroup`, goroutines de los 5 pipelines) es orquestación → vive en el use case, no baja al adapter.

### 7.2 Radiografía actual

`memory_service.go` corre cinco pipelines en paralelo (`runPipelines` con `sync.WaitGroup`) —vector, graph, recency, keyword, consolidated— y los combina con `fuseRRF`. Las deps opcionales se cablean **después** del constructor, por setters (`main.go:102-105`):

```go
memorySvc := services.NewMemoryService(graphRepo, entityRepo, vectorRepo)  // 3 deps requeridas
memorySvc.SetConsolidationRepo(consolidationRepo)   // ← opcional, por setter
memorySvc.SetBudgetMgr(budgetMgr)                   // ← opcional, por setter
memorySvc.SetHistoryRepo(...)                       // ← opcional, por setter
```

Los setters son otra semilla de refactor: alguien ya intuyó que estas deps son opcionales. Solo que "opcional por setter mutable" es frágil (podés olvidarte de llamarlo, y el objeto queda a medio construir).

### 7.3 Diseño target

| Archivo | Capa | Qué |
|---------|------|-----|
| `domain/memory.go` (o `app/memory/fusion.go`) | domain / app | `FuseRRF([]RankedList, k) []Fused` — **función pura**. Y `FuseRRFExplain` (el ledger por-pipeline). |
| `app/memory/service.go` | app | `Recall`/`RecallWithPipelines`/`RecallExplain` orquestando los ports + la fusión. |
| `app/memory/pipelines.go` | app | Los 5 pipelines, cada uno una función que produce una `RankedList` desde un port. |
| `app/ports/*` | ports | `ConsolidationRepository`, `Tokenizer` (contar tokens es lo único infra del budget). |

**Dónde poner `FuseRRF`:** es una decisión de gusto legítima. Argumento para **dominio**: es una regla de cómo el sistema combina evidencia, independiente de toda tecnología. Argumento para **`app/memory`**: es política de recuperación, no una entidad de negocio. Yo la pongo en `app/memory/fusion.go` como **domain service** (lógica sin estado que no pertenece a una entidad puntual), pero pura, sin ports. Lo importante no es la carpeta: es que **no dependa de nada de I/O**.

### 7.4 Paso a paso

**(a) La fusión pura — el test más satisfactorio del proyecto.** RRF (Reciprocal Rank Fusion): cada item recibe `1/(k+rank)` en cada pipeline donde aparece, y se suman. Cero dependencias.

```go
// internal/app/memory/fusion.go
package memory

type RankedList struct {
    Source string
    IDs    []string   // ordenados por rank (0 = mejor)
}

type Fused struct {
    ID    string
    Score float64
}

// FuseRRF combina listas rankeadas. Función PURA: sin ctx, sin ports, sin I/O.
func FuseRRF(lists []RankedList, k int) []Fused {
    scores := map[string]float64{}
    for _, l := range lists {
        for rank, id := range l.IDs {
            scores[id] += 1.0 / float64(k+rank+1)
        }
    }
    // ordenar por score desc → []Fused
    return sortByScore(scores)
}
```

```go
// internal/app/memory/fusion_test.go — sin DB, sin nada. Determinístico.
func TestFuseRRF_ItemInMultipleListsRanksHigher(t *testing.T) {
    out := FuseRRF([]RankedList{
        {Source: "vector", IDs: []string{"a", "b"}},
        {Source: "graph",  IDs: []string{"b", "c"}},
    }, 60)
    require.Equal(t, "b", out[0].ID)   // "b" aparece en ambas → gana
}
```

> **Dato que refuerza el argumento:** en el proyecto actual, los tests de fusión (`fuse_rrf_test.go`, `fuse_rrf_explain_test.go`) **ya corren sin base de datos** — son parte de los "metrics-only smoke tests". El código YA te está diciendo que esta lógica es pura. Nosotros solo la mudamos a la capa que refleja esa verdad.

**(b) Deps opcionales: de setter mutable a construcción explícita.** Cambiá los tres setters por *functional options* — el objeto se construye completo y de una:

```go
// internal/app/memory/service.go
type Option func(*Service)

func WithConsolidation(r ports.ConsolidationRepository) Option { return func(s *Service) { s.consolidation = r } }
func WithBudget(t ports.Tokenizer, max, reserve int) Option    { return func(s *Service) { s.budget = newBudget(t, max, reserve) } }

func New(graph ports.GraphStore, entities ports.EntityRepository, vectors ports.VectorStore, opts ...Option) *Service {
    s := &Service{graph: graph, entities: entities, vectors: vectors, consolidation: noopConsolidation{}}
    for _, o := range opts { o(s) }
    return s
}
```

Donde no haya consolidación, un **null object** (`noopConsolidation{}` que no hace nada) evita los `if s.consolidation != nil` desperdigados. El objeto siempre está completo; nunca a medio construir.

**(c) Concurrencia: se queda en el use case.** Los cinco pipelines corren en goroutines con `WaitGroup`. Eso es **orquestación de aplicación**, no I/O — vive en `app/memory`, no baja al adapter. Cada pipeline llama a **un port** (`s.vectors.FindSimilarParagraphs`, `s.graph.GetNeighborsBatch`, etc.) y produce una `RankedList`; `FuseRRF` las combina.

### 7.5 El "por qué" del senior

> "Cuando una función no tiene `ctx`, no tiene ports y no tiene efectos, **es la parte más valiosa de tu sistema** y probablemente la tenías enterrada. `FuseRRF` es EL algoritmo de recuperación de Quill —el corazón de la feature de memoria— y lo podés testear con dos listas de strings en un test que corre en un milisegundo. Clean Architecture no *crea* esa pureza; la **revela**, empujando el I/O hacia los bordes hasta que en el centro queda solo la lógica. El objetivo final del refactor es que tus reglas de negocio más importantes sean funciones puras rodeadas de una cáscara delgada de adapters."

### 7.6 Probalo

```bash
go test ./internal/app/memory/ -run TestFuseRRF   # puro, instantáneo, sin infra
go test ./internal/app/memory/...                 # use case con fakes de los 4 ports
TEST_DATABASE_URL=... QWEN_API_KEY=... go test ./internal/app/memory/ -run TestRecall_Integration
```

Relevance y Consolidation (parte del mismo subsistema) siguen el mismo molde: `RelevanceService` con su decaimiento exponencial (`DecayAll`) — donde el **cálculo de decaimiento** (`e^(-λt)`, umbral de archivado) es **regla de dominio pura** (otra función testeable sin nada), y las goroutines fire-and-forget de consolidación son orquestación.

### 7.7 Checkpoint

Verde → `git commit -m "refactor(memory): pure FuseRRF, functional options, ports for the 5 pipelines"`.

---

## Cap. 8 — Slice Analysis orchestrator (capstone) · disolver la dependencia circular

### 8.1 Objetivo

El slice final: el hub donde **convergen todas las dependencias**. Fan-out a cinco use cases (Entity, Contradiction, Relevance, Timeline, PlotHole) + Memory + LLM + WebSocket. Es el capstone porque integra todo lo anterior. La lección estrella:
- **Disolver la dependencia circular** `ws.Hub ↔ AnalysisService` que hoy se resuelve con el hack `nil` + `SetSubmitter`. Clean Architecture la **disuelve de raíz** separándola en dos ports.

### 8.2 Radiografía actual: el ciclo

Hoy hay una dependencia bidireccional entre el hub y el servicio de análisis:

```
ws.Hub  ──(necesita)──►  AnalysisService   : para enviarle paragraphs a analizar
                                              (hub.go:52 `submitter ParagraphSubmitter`,
                                               satisfecho por AnalysisService.SubmitParagraph)

AnalysisService  ──(necesita)──►  ws.Hub    : para emitir resultados al cliente
                                              (analysis_service.go:68 `hub AnalysisHub`,
                                               satisfecho por Hub.SendToUser)
```

Y hay un import **concreto** `services → ws` (`analysis_service.go:15`) por las constantes de tipo de mensaje (`ws.TypeAnalysisProgress`). Go **prohíbe los ciclos de import entre paquetes**, así que el ciclo se "resuelve" en tiempo de init con el truco de dos fases (`main.go:123,129`):

```go
hub := ws.NewHub(authSvc, nil, memorySvc, qwenSvc)  // ← submitter = nil (todavía no existe el service)
analysisSvc := services.NewAnalysisService(pool, ..., hub, memorySvc)
hub.SetSubmitter(analysisSvc)                        // ← back-patch: ahora sí, cableá el service al hub
```

Funciona, pero es frágil: el hub nace **a medio construir** (con un `submitter` nulo) y depende de que nadie se olvide del `SetSubmitter`.

### 8.3 Cómo Clean lo disuelve

La clave: **son dos relaciones distintas, no una circular.** El análisis (a) *recibe* trabajo y (b) *emite* eventos. Son dos direcciones que hoy están colapsadas en "el hub y el service se conocen". Las separás en dos ports, **ambos definidos en `app`**:

```go
// internal/app/ports/gateway.go
// OUTPUT port: el use case emite eventos. NO sabe qué es WebSocket.
type EventPublisher interface {
    Publish(ctx context.Context, userID uuid.UUID, ev domain.Event) error
}
// INPUT port: quien recibe paragraphs (el hub lo llama como controller).
type ParagraphSubmitter interface {
    SubmitParagraph(ctx context.Context, in domain.ParagraphSubmission) error
}
```

Y reasignás responsabilidades por capa:

- Las **constantes de tipo de mensaje** (`TypeAnalysisProgress`, etc.) y los **payloads** son detalle de transporte → viven en `adapter/ws/protocol.go`. **`app` deja de importarlas.** El use case emite `domain.Event` (un tipo del dominio); el adapter de WS traduce `domain.Event → WSMessage`. **El import `app → ws` desaparece.** Ciclo de compilación roto de verdad, no parcheado.
- El **hub es un controller** (como un handler HTTP): recibe el paragraph del socket y llama al input port `ParagraphSubmitter`. Import `adapter/ws → app/analysis`: de afuera hacia adentro. Legal.

```
ANTES (ciclo, resuelto con nil+setter):
   ws.Hub ⇄ AnalysisService   (services importa ws; ws recibe el service por setter)

DESPUÉS (sin ciclo, ambas flechas hacia adentro):
   adapter/ws ──► app/analysis        (hub llama al input port ParagraphSubmitter)
   app/analysis ──► app/ports.EventPublisher ◄── adapter/ws   (el hub implementa el output port)
```

**¿Y el `nil` + `SetSubmitter`?** Se puede eliminar del todo si querés: como ninguno de los dos paquetes se importa concretamente, `main` construye el use case pasándole el hub como `EventPublisher`, y construye/registra el hub apuntando al use case como `ParagraphSubmitter`. Sigue habiendo una referencia mutua **en runtime** (es inherente a la feature: análisis consume y emite), pero ya no hay ciclo de **compilación**, y el cableado es honesto. Si te molesta hasta el runtime mutuo, metés un **event dispatcher** in-process del que ambos dependen (el use case publica ahí, el hub se suscribe ahí) y ninguno referencia al otro — pero eso es refinamiento opcional, no lo necesitás para que esté limpio.

> **El "por qué" del senior:** "El hack `nil` + `SetSubmitter` no es un bug: es un **síntoma**. Es tu código gritándote que dos cosas están mal ubicadas. La dependencia circular casi nunca es real — casi siempre es **una responsabilidad mal repartida**. Cuando separás 'recibir trabajo' de 'emitir resultados' en dos ports, el ciclo se evapora, porque nunca fueron la misma relación. Cada vez que veas un `SetX` mutable para 'romper un ciclo de init', frená: ahí hay un port esperando nacer."

### 8.4 El resto del slice

- **El per-work queue** (goroutine por work, cola secuencial, `cancels map[uuid.UUID]context.CancelFunc` de `analysis_service.go:71-72`) es **orquestación de aplicación** → se queda en `app/analysis`. No lo abstraigas detrás de un port de "job queue" salvo que vayas a tener otra implementación; para un solo runner in-process es sobre-ingeniería.
- **El fan-out** a los cinco use cases: el orchestrator depende de los **input ports** de cada uno (o directamente de los use cases concretos — acá es defendible, son todos `app`). Los interfaces `EntityResolvr` y `Reactivatr` que ya existían (`analysis_service.go:45-53`) son, de nuevo, ports accidentales: promovelos.
- **El pipeline de dos pasadas** (core + enrichment) es política de negocio → `app`. El progreso se emite por `EventPublisher`.

### 8.5 Probalo

```bash
go test ./internal/app/analysis/...   # el orchestrator con fakes de los 5 use cases + fake EventPublisher
```

El *fake* de `EventPublisher` te deja **assertar qué eventos emitió el pipeline** (¿mandó `entity_discovered`? ¿`analysis_progress` en las 5 etapas?) sin levantar un WebSocket. El `analysis_service_test.go` actual es la red.

### 8.6 Checkpoint final

Verde → `git commit -m "refactor(analysis): dissolve ws↔analysis cycle via EventPublisher + ParagraphSubmitter ports"`.

**Y con esto, `services/`, `repositories/`, `models/` y `handlers/` quedaron vacíos.** Borralos. `git commit -m "chore: remove legacy layered packages"`. Terminaste.

---

## Parte 9 — Cierre honesto: qué ganás y qué te cuesta

### La foto final

```
Import graph, después del refactor (todas las flechas → hacia domain):

  main.go ──► adapter/{http, ws, qwen, postgres} ──► app/{feature} ──► app/ports ──► domain
     │                                                    │                            ▲
     └────────► infra/{config, db} ─────────────────────┘                            │
                                                    (adapters implementan ports) ──────┘
```

`main.go` es el **único** archivo que importa pgx, Fiber, Qwen y ws de forma concreta. Todo lo demás habla en ports y domain. La Regla de Dependencia se sostiene: **no existe una sola flecha que salga del centro hacia afuera.**

### El balance sin maquillaje

| Ganás | Te cuesta |
|-------|-----------|
| **Testabilidad**: cada use case se testea con fakes, sin DB ni LLM. Los tests corren en milisegundos. | **Más archivos**: un `models.go` se volvió ~15 archivos de dominio + DTOs + mappers. |
| **Swappability**: Postgres→otra DB, Qwen→otro LLM = tocás un adapter, cero lógica. | **Indirección**: para seguir un flujo saltás use case → port → adapter. Más "saltos de definición". |
| **Fronteras claras**: el sharp edge de AGE queda enjaulado; nadie de arriba lo puede envenenar. | **Boilerplate de mappers**: DTO↔domain a mano en cada borde. |
| **Reglas de negocio puras** en el centro (`FuseRRF`, `Merge`, invariantes), testeables sin nada. | **La pregunta "¿dónde va esto?"** aparece seguido al principio (¿dominio o app? ¿app o adapter?). |

### Cuándo Clean Architecture NO vale la pena

> "Te lo digo yo que la enseño: **el código pragmático original de Quill era defendible.** Para un MVP de hackathon, capas por carpeta + repos concretos es rápido y se entiende. Clean Architecture se paga cuando: tenés **múltiples adapters** (varios LLMs, varias DBs), el proyecto **vive años**, el **equipo crece**, o el **testing es crítico**. Si nada de eso aplica, Clean puede ser una tabla de indirección que ralentiza sin devolver nada. **Este ejercicio vale por el razonamiento, no por el dogma.** Aprendé a construirla para aprender a decidir cuándo. Aplicar Clean a todo por reflejo es el mismo error que no aplicarla nunca: dejar de pensar."

### Checklist de auto-evaluación (corré esto en cada slice)

- [ ] El use case, ¿importa `pgx`, `fiber`, o el cliente de Qwen? → Si sí, la dependencia no está invertida.
- [ ] El `domain`, ¿importa algo del proyecto que no sea `domain`? → Debe importar solo stdlib + `uuid`/`time`.
- [ ] ¿Podés testear el use case sin levantar Postgres? → Si no, todavía hay una dep concreta escondida.
- [ ] ¿Hay algún `new`/constructor de una dependencia **adentro** de un método? → Sacala al constructor.
- [ ] ¿Algún tipo del adapter (repo, pgx, pgvector) viaja por una firma del use case? → Movelo a `domain`/`app`.
- [ ] ¿Algún `SetX` mutable para "romper un ciclo de init"? → Ahí hay un port esperando.
- [ ] La lógica pura (fusión, merge, decay, fingerprint), ¿está en el centro y testeada sin I/O?

---

## Apéndice

### A. Mapa viejo → nuevo

| Archivo actual | Se parte en… |
|----------------|--------------|
| `internal/models/models.go` | `domain/*.go` (structs + comportamiento) · `adapter/http/dto.go` (RegisterRequest, CreateUniverseRequest, AuthResponse, ...) · `adapter/ws/protocol.go` (WSMessage, *Payload, RecallItem) |
| `internal/services/*_service.go` | `app/<feature>/service.go` (orquestación) + `domain/*.go` (reglas que estaban sueltas: `validateUniverseEnums`, `mergeEntity`, `fingerprint`, decay) |
| `internal/repositories/*_repo.go` | `adapter/repository/postgres/*_repo.go` (impl) + `app/ports/repository.go` (interfaces) |
| `internal/services/qwen_service.go` | `adapter/qwen/client.go` (HTTP + function-calling) + `app/.../agent.go` (loop ReAct) + `app/ports/gateway.go` (LLMClient, Embedder) |
| `internal/ws/hub.go` | `adapter/ws/hub.go` (impl) + `app/ports/gateway.go` (EventPublisher, ParagraphSubmitter) |
| `internal/config/config.go` | `infra/config/config.go` (sin cambios de fondo — ya estaba bien inyectado) |
| `cmd/server/main.go` (setup del pool + AGE) | `infra/db/pool.go` (pool + AfterConnect) + `main.go` (solo cableado) |

### B. Dónde sobreviven los sharp edges de ESTE repo

| Sharp edge | Antes | Después |
|------------|-------|---------|
| **AGE `search_path`** (envenenamiento de conexión) | disperso en `graph_repo.go`, riesgo project-wide | **sellado** en `adapter/repository/postgres/graph_repo.go` (`withAgeTx`); inalcanzable desde arriba del port |
| **`validCypherIdentifier`** (labels del LLM) | validación ad hoc en el repo | invariante en `domain` (`ErrInvalidIdentifier`) **+** cerrojo de defensa en el adapter |
| **Per-work sequential queue** | `analysis_service.go` (goroutine + cola) | orquestación en `app/analysis` (no se abstrae; es workflow, no I/O) |
| **Two-phase init (`nil` + `SetSubmitter`)** | hack en `main.go:123,129` | **disuelto** en dos ports (`EventPublisher` out + `ParagraphSubmitter` in) |
| **`pgx.Tx` en firmas de repo** | `entity_repo.go:24` etc. | reemplazado por `TxManager.WithinTx` (tx viaja por `context`) |

### C. Orden de ejecución recomendado (para pegar en tu board)

1. Preparación: red de tests verde + estructura vacía (`domain/ app/ adapter/ infra/`).
2. **Auth** → 3. **Universe** → **Work** (mismo molde).
3. **Interludio**: ports `VectorStore`, `GraphStore`, `LLMClient`/`Embedder`.
4. **Chapter** (ahora sí, ya está `RelevanceService` encaminado).
5. **Entity** → 6. **Contradiction** → 7. **Memory** (+ Relevance + Consolidation) → 8. **Analysis**.
6. Borrar `services/ repositories/ models/ handlers/` vacíos.

> **La última del senior:** "Vas a querer hacer dos slices en un día para 'avanzar'. No. Un slice, verde, commit, respirá. El refactor que se apura es el refactor que introduce el bug que no vas a encontrar hasta producción. Aburrido y seguro. Nos vemos del otro lado."
