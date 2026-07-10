# Remake-Quill — Rehacer el backend desde cero con Clean Architecture (canónico) y TDD

> Esta no es una guía de refactor. Es un **remake**: vas a construir el backend de Quill **desde cero**, en un repo nuevo, escribiendo **el test primero** y el código después. Cada pieza aparece completa: no hay "y acá ponés lo tuyo".
>
> **Nunca escribiste un test.** Perfecto. La Parte 2 te enseña desde el `func TestXxx` hasta los fakes, y no avanzamos hasta que eso esté claro.
>
> **No vas a tocar el proyecto actual.** El remake vive aparte, en `quill-remake/`.

---

## Cómo leer esta guía

| Marca | Significado |
|-------|-------------|
| 🔴 **RED** | Escribís un test que **falla**. Sí, a propósito. |
| 🟢 **GREEN** | Escribís el código **mínimo** que lo hace pasar. |
| 🔵 **REFACTOR** | Limpiás, con el test cuidándote. |
| 📦 **Código completo** | El archivo entero. Copiá, pero **leé la explicación primero**. |
| ✍️ **Ejercicio** | Te doy los tests (la especificación). El código lo escribís vos. Solución al final. |
| ⚠️ **Trampa** | Un error clásico. Lo vas a cometer. Mejor que lo veas venir. |

**Índice**

- Parte 0 — Remake ≠ refactor (y por qué elegiste bien)
- Parte 1 — **Clean vs Hexagonal**: la diferencia que pediste
- Parte 2 — **TDD desde cero**: nunca escribiste un test
- Parte 3 — Setup del proyecto nuevo
- Slice 0 — **Health**: el flujo canónico completo, sin base de datos
- Slice 1 — **Auth / Register**: entidades, gateways y el spy presenter
- Slice 2 — **Auth / Login**: JWT y middleware
- Slice 3 — **Universe**: invariantes y transacciones
- Slice 4 — ✍️ **Work + Chapter** (ejercicio)
- Slice 5 — **Los tres gateways externos**: Qwen, pgvector, AGE
- Slice 6 — **Entity**: un interactor que orquesta cuatro gateways
- Slice 7 — **Contradiction**: el agente ReAct como caso de uso
- Slice 8 — **Memory / RRF**: la función pura
- Slice 9 — **Analysis + WebSocket**: el capstone
- Cierre — Lo que construiste, y dónde Go pelea con Clean
- Apéndice — Soluciones de los ejercicios

---

## Parte 0 — Remake ≠ refactor (y por qué elegiste bien)

Aclaremos los términos, porque confundirlos te hace perder meses.

| | **Refactor** | **Remake** (lo que vamos a hacer) |
|---|---|---|
| Punto de partida | El código que ya existe | Un repo vacío |
| Qué garantiza | El comportamiento **no cambia** | Nada. Estás creando |
| La red de seguridad | Los tests que **ya existen** | Los tests que **vas escribiendo** (TDD) |
| Qué aprendés | Operar código ajeno vivo sin romperlo | **Diseñar** desde cero |
| Riesgo | Bajo (pasos chiquitos, tests verdes) | Alto si no tenés disciplina |

Vos entendiste bien: refactorizar es copiar el proyecto, correr los tests como red, y mover cosas de lugar **sin cambiar lo que el sistema hace**. Eso es exactamente lo que es.

Y tu conclusión fue correcta: si lo que querés es **aprender a diseñar**, el remake enseña más. Cuando refactorizás, arrastrás las decisiones de otro. Cuando construís, cada decisión es tuya y tenés que justificarla.

**Lo único que perdés** (y quiero que lo sepas, no que te lo escondan): la habilidad de trabajar sobre código legacy vivo, que es la que vas a usar el 90% de tu carrera. Un día la aprendés. Hoy no es el día.

### Lo que necesitás instalado

```bash
go version          # 1.22 o superior
docker --version    # para Postgres
git --version
```

Nada más. Ni un framework de tests, ni un generador de mocks, ni una librería de assertions. **Todo va a ser Go de la biblioteca estándar.** Después te explico por qué.

### Cuando un test no pasa y no entendés

No borres el test. No comentes la assertion. Hacé esto, en orden:

1. Leé el mensaje de error **completo**. Go te dice `got X, want Y`. Ese es el 80% de la respuesta.
2. `go test ./... -v` para ver qué corrió y qué no.
3. Preguntate: *¿el test está mal, o el código está mal?* Las dos opciones son válidas. Un test puede estar mal escrito.
4. Si un test pasa **a la primera**, cuando esperabas que fallara: **desconfiá**. Probablemente no está probando lo que creés. Rompé el código a propósito y confirmá que el test se pone rojo.

> Ese punto 4 es la lección más importante de la Parte 2. Un test que nunca viste fallar **no es un test**: es una decoración.

---

## Parte 1 — Clean vs Hexagonal: la diferencia que pediste

Tenías razón, y quiero mostrarte exactamente por qué, porque aprender a detectar esto vale más que la guía entera.

### Los tres apellidos

| Arquitectura | Autor | Año |
|---|---|---|
| **Hexagonal** (*Ports & Adapters*) | Alistair Cockburn | 2005 |
| **Onion** | Jeffrey Palermo | 2008 |
| **Clean** | Robert C. Martin | 2012 (libro, 2017) |

Clean **no compite** con Hexagonal: la **absorbe**. Uncle Bob dice explícitamente que Clean sintetiza Hexagonal, Onion, DCI y **BCE** (Entity-Boundary-Interactor, de Ivar Jacobson, 1992 — de ahí sale la palabra **Interactor**).

Por eso se parecen. Y por eso confundirlas es tan fácil.

### Hexagonal: **una** frontera

```
                    ┌──────────────────────┐
   Driving          │                      │          Driven
   adapters   ─────►│   LA APLICACIÓN      │─────►   adapters
   (HTTP, CLI,      │   (un bloque         │        (DB, SMTP,
    tests)          │    indiviso)         │         otra API)
                    └──────────────────────┘
                     ▲                    ▲
              driving port          driven port
              (la API de la app)    (lo que la app necesita)
```

Hexagonal dice **una sola cosa**: separá la aplicación del mundo exterior con *ports* (interfaces) y *adapters* (implementaciones). Hay ports de dos lados: los que **manejan** la app (*driving*/primarios: HTTP, CLI) y los que la app **maneja** (*driven*/secundarios: base de datos, LLM).

**Lo que Hexagonal NO dice:** absolutamente nada sobre cómo se organiza el interior de la aplicación. Adentro del hexágono podés tener un despelote. Sigue siendo hexagonal.

### Clean: **cuatro** capas concéntricas y una ley

```
        ┌───────────────────────────────────────────────┐
        │  4. Frameworks & Drivers                      │  Fiber, pgx, la API de Qwen, main.go
        │  ┌─────────────────────────────────────────┐  │
        │  │  3. Interface Adapters                  │  │  Controllers, Presenters, Gateways
        │  │  ┌───────────────────────────────────┐  │  │
        │  │  │  2. Use Cases                     │  │  │  Interactors + los ports que definen
        │  │  │  ┌─────────────────────────────┐  │  │  │
        │  │  │  │  1. Entities                │  │  │  │  Reglas de negocio de la empresa
        │  │  │  └─────────────────────────────┘  │  │  │
        │  │  └───────────────────────────────────┘  │  │
        │  └─────────────────────────────────────────┘  │
        └───────────────────────────────────────────────┘

        LA REGLA DE DEPENDENCIA:
        los imports del código fuente apuntan SIEMPRE hacia adentro.
```

Clean **sí** prescribe el interior. Separa:
- **Entities** (capa 1): reglas que valdrían aunque el software no existiera. "Un universo no puede tener un género inventado."
- **Use Cases** (capa 2): reglas de **esta aplicación**. "Registrarse: validar, hashear, guardar, emitir token."

Y le pone nombre a cada pieza que cruza una frontera.

### La tabla que resuelve la discusión

| | **Hexagonal** | **Clean** |
|---|---|---|
| ¿Prescribe el interior del core? | **No.** Un bloque indiviso. | **Sí.** Entities más adentro que Use Cases. |
| Cantidad de fronteras | **Una**: adentro / afuera. | **Cuatro** círculos. |
| Cómo vuelve la respuesta | No lo prescribe. En la práctica: el adapter llama al port y **recibe un `return`**. | **Lo prescribe**: el Interactor **empuja** a un *Output Port* → **Presenter** → **ViewModel**. Nunca retorna. |
| Dónde vive la interfaz del repo | "Un driven port". | En el **paquete del use case que la necesita**. |
| Artefactos con nombre propio | Port, Adapter. | Entity, **Interactor**, Input Port, **Output Port**, Controller, **Presenter**, Gateway, **ViewModel**, Request/Response Model. |
| Simetría | Simétrica (izquierda/derecha). | Asimétrica: el control fluye hacia afuera, las dependencias hacia adentro. |

**En una frase:** *toda arquitectura Clean es compatible con Hexagonal; no toda Hexagonal es Clean.*

### La autopsia: por qué el otro documento NO era Clean

En este mismo repo hay un `Refactor-Quill.md` que escribí antes y que **decía** ser Clean Architecture. Vos oliste que no lo era. Corrí el grep sobre mis propias 1295 líneas:

```bash
$ rg -ci 'interactor|presenter|viewmodel|request model|response model|humble object' Refactor-Quill.md
0

$ rg -ci 'port|adapter' Refactor-Quill.md
171
```

**Cero.** En un documento entero sobre "Clean Architecture" no aparece **ni una vez** la palabra `Interactor`, `Presenter` ni `ViewModel`. Y `port`/`adapter` aparecen en 171 líneas.

Peor: en el capítulo 8 llegué a escribir que el orquestador *"puede depender directamente de los use cases concretos — acá es defendible"*, o sea que descarté el Input Port.

**Diagnóstico:** eso era **Ports & Adapters (Hexagonal)** con un árbol de carpetas de estética Clean. Es el error más común del mundo Go: se invierten los repositorios detrás de interfaces, se le pone `adapter/` a una carpeta, y se lo llama Clean.

Guardate el síntoma, porque lo vas a ver en el 90% de los repos que digan "Clean Architecture in Go":

> ⚠️ **Trampa.** Si el caso de uso hace `return output, nil` y el controller lo serializa, **no es Clean canónico**. Es Hexagonal. Puede estar perfecto — pero llamalo por su nombre.

### El concepto más difícil (y el más Clean): control vs dependencias

Este es EL punto. Leelo dos veces.

Cuando el usuario pide `POST /register`, el **control** viaja así:

```
Controller  ──►  Interactor  ──►  Presenter  ──►  HTTP response
   (afuera)       (adentro)        (afuera)
```

El control **entra** y después **vuelve a salir**. Pero las **dependencias** (los `import`) tienen que apuntar SIEMPRE hacia adentro. Si el Interactor (adentro) importara al Presenter (afuera), romperías la Regla de Dependencia.

**¿Cómo hacés que algo de adentro le hable a algo de afuera sin importarlo?**

El Interactor define **él mismo** una interfaz —el **Output Port**— que describe lo que necesita que alguien haga:

```go
// Vive en la capa de USE CASE (adentro).
type RegisterOutputPort interface {
    PresentRegistered(out RegisterOutput)
    PresentError(err error)
}
```

Y el Presenter, **afuera**, la implementa. El import va del Presenter (afuera) hacia el Output Port (adentro). ✅ Regla respetada.

```
        DEPENDENCIAS (imports)              CONTROL (llamadas en runtime)

   Presenter ───import──► OutputPort        Interactor ──llama──► Presenter
    (afuera)              (adentro)          (adentro)            (afuera)

        hacia ADENTRO ✅                     hacia AFUERA ✅
```

Eso se llama **Inversión de Dependencias**, y es literalmente la razón por la que Clean existe. El Interactor le habla al Presenter **sin saber que el Presenter existe**. Solo conoce una interfaz que él mismo escribió.

> **La frase para tatuarte:** el flujo de control va y viene. **El flujo de dependencias es de una sola mano: siempre hacia el centro.**

### Los seis artefactos que vas a escribir

| Artefacto | Capa | Qué es |
|---|---|---|
| **Entity** | 1 | Un objeto de negocio con sus reglas. `Universe` que se niega a existir con un género inválido. |
| **Request Model** | 2 | Datos planos que **entran** al caso de uso. `RegisterInput{Email, Password}`. Sin `json`. |
| **Input Port** | 2 | La interfaz que el Controller llama. `RegisterInputPort`. |
| **Interactor** | 2 | La implementación del Input Port. Orquesta entidades y gateways. |
| **Response Model** | 2 | Datos planos que **salen**. `RegisterOutput{User, Token}`. Sin `json`. |
| **Output Port** | 2 | La interfaz a la que el Interactor **empuja** el resultado. |
| **Presenter** | 3 | Implementa el Output Port. Convierte Response Model → **ViewModel**. |
| **ViewModel** | 3 | Lo que el cliente ve. **Acá sí** viven los tags `json`. |
| **Controller** | 3 | HTTP → Request Model → Input Port. |
| **Gateway** | 3 | Implementa una interfaz definida en la capa 2 (DB, LLM, reloj). |

> ⚠️ **Trampa (y corrección de mi documento anterior).** La interfaz del gateway (`UserRepository`) **NO va en un paquete `ports/` compartido**. Va en el paquete del use case que la necesita: `usecase/auth/repository.go`. En Clean, **la interfaz pertenece a quien la consume**. Un `ports/` global es un paquete-Dios que vuelve a acoplar todo.

---

## Parte 2 — TDD desde cero: nunca escribiste un test

Cerrá lo anterior. Esto es de cero absoluto.

### Qué es un test en Go

Un test es **una función normal** que:
1. vive en un archivo terminado en `_test.go`,
2. se llama `TestAlgo`,
3. recibe `t *testing.T`.

No hay framework. No hay `describe`, no hay `it`, no hay `expect`. Go trae los tests adentro.

```go
// wordcount.go
package text

func WordCount(s string) int {
    return 0   // todavía miente
}
```

```go
// wordcount_test.go
package text

import "testing"

func TestWordCount(t *testing.T) {
    got := WordCount("hola mundo cruel")
    want := 3

    if got != want {
        t.Errorf("WordCount() = %d, want %d", got, want)
    }
}
```

```bash
$ go test ./...
--- FAIL: TestWordCount (0.00s)
    wordcount_test.go:9: WordCount() = 0, want 3
FAIL
```

Eso es todo. **No existe `assert.Equal`.** Existe un `if`. Vos comparás, y si está mal, se lo contás a `t`.

### El ciclo: Red → Green → Refactor

TDD tiene tres pasos y **el orden no es negociable**.

#### 🔴 RED — escribí el test ANTES que el código

Borrá `wordcount.go`. Dejá solo el test. Corré:

```bash
$ go test ./...
# github.com/you/quill/internal/text [build failed]
./wordcount_test.go:6:9: undefined: WordCount
FAIL
```

> **Insight de principiante que casi nadie te dice:** en Go, **un test que no compila ES rojo**. Ese `undefined: WordCount` es tu primer RED legítimo. No lo "arregles" creando la función a las apuradas: ese error es la prueba de que el test está corriendo y de que el código todavía no existe.

#### 🟢 GREEN — el código más tonto que lo haga pasar

```go
// wordcount.go
package text

import "strings"

func WordCount(s string) int {
    return len(strings.Fields(s))
}
```

```bash
$ go test ./...
ok  github.com/you/quill/internal/text  0.002s
```

**No escribas de más.** ¿Querés manejar strings vacíos? Escribí primero el test que falla con string vacío. Sin test rojo, no se escribe código.

#### 🔵 REFACTOR — limpiá, con el test cuidándote

Acá no hay nada que limpiar (una línea). Pero cuando lo haya, refactorizás y volvés a correr. Si sigue verde, no rompiste nada. **Ese es el superpoder.**

### `t.Errorf` vs `t.Fatalf`

| Función | Qué hace |
|---|---|
| `t.Errorf(...)` | Marca el test como fallado **y sigue**. Usalo cuando querés ver todas las cosas que están mal. |
| `t.Fatalf(...)` | Marca el test como fallado **y corta ahí**. Usalo cuando seguir no tiene sentido (ej: el objeto es `nil`, todo lo que sigue va a explotar). |

Regla práctica: **`Fatalf` para errores inesperados, `Errorf` para assertions.**

```go
user, err := svc.Register(ctx, "a@b.com", "secret123")
if err != nil {
    t.Fatalf("Register() unexpected error: %v", err)   // si falló, no puedo seguir
}
if user.Email != "a@b.com" {
    t.Errorf("Email = %q, want %q", user.Email, "a@b.com")   // assertion: sigo revisando
}
```

### La convención `got` / `want`

Siempre, siempre, el mismo formato de mensaje:

```go
t.Errorf("NombreDeLoQueProbás = %v, want %v", got, want)
```

¿Por qué importa tanto? Porque dentro de seis meses vas a ver `Email = "" , want "a@b.com"` en un CI a las 3 de la mañana y vas a entender el bug sin abrir el código. Un mensaje como `t.Error("falló")` no te dice nada.

### Tests table-driven (el idiom de Go)

Cuando tenés muchos casos del mismo comportamiento, no escribas veinte funciones. Escribí una tabla.

```go
func TestWordCount(t *testing.T) {
    cases := []struct {
        name string
        in   string
        want int
    }{
        {name: "tres palabras",      in: "hola mundo cruel", want: 3},
        {name: "string vacío",       in: "",                 want: 0},
        {name: "espacios de sobra",  in: "  hola   mundo ",  want: 2},
        {name: "una sola palabra",   in: "hola",             want: 1},
    }

    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := WordCount(tc.in)
            if got != tc.want {
                t.Errorf("WordCount(%q) = %d, want %d", tc.in, got, tc.want)
            }
        })
    }
}
```

`t.Run` crea un **subtest** con nombre propio. Si falla el tercero, Go te dice `TestWordCount/espacios_de_sobra`. Podés correr uno solo:

```bash
go test ./internal/text/ -run 'TestWordCount/string_vacío' -v
```

**Nombrá los casos por el escenario, no por el input.** `"string vacío"`, no `"caso 2"`.

### Test doubles: fakes, stubs y spies

Acá está el puente entre TDD y Clean Architecture. Prestá atención.

Un **test double** es un objeto falso que ponés en lugar del real. Los tres que vas a usar:

| Tipo | Qué hace | Cuándo |
|---|---|---|
| **Stub** | Devuelve una respuesta enlatada. No tiene lógica. | "Que el LLM devuelva siempre este texto." |
| **Spy** | Igual que un stub, pero **registra las llamadas** que recibió. | "Quiero verificar que el interactor guardó al usuario." |
| **Fake** | Una implementación **real pero liviana** (ej: un repo en memoria). | "Un `UserRepository` con un `map`, sin Postgres." |

Los tres son **structs que escribís a mano**. Nada de generadores.

```go
// STUB: respuesta enlatada.
type stubTokenIssuer struct{}
func (stubTokenIssuer) Issue(*entity.User) (string, error) { return "fake-token", nil }

// SPY: registra lo que le pidieron.
type spyUserRepo struct {
    saved     []*entity.User
    savedHash string
}
func (s *spyUserRepo) Save(_ context.Context, u *entity.User, hash string) error {
    s.saved = append(s.saved, u)   // ← anoto la llamada
    s.savedHash = hash
    return nil
}

// FAKE: implementación real, liviana.
type fakeUserRepo struct {
    byEmail map[string]*entity.User
}
func (f *fakeUserRepo) ExistsByEmail(_ context.Context, email string) (bool, error) {
    _, ok := f.byEmail[email]
    return ok, nil
}
```

> **El proyecto original ya hace exactamente esto.** Si mirás `backend/internal/services/analysis_service_test.go:240` vas a encontrar un `spyReactivatr` que guarda `touchCalls []uuid.UUID`. Y en `contradiction_service_test.go:614`, un `mockExecutor` que devuelve `"mock tool result"`. Escritos a mano, sin librerías. Vas a escribir los tuyos igual.

### 🔑 La tesis que une TDD con Clean

Mirá el `fakeUserRepo` de arriba. **¿Por qué podés reemplazar el repositorio real por uno falso?**

Porque el interactor no depende de `*postgres.UserGateway`. Depende de una **interfaz** que él mismo definió: `UserRepository`. Esa interfaz es un **agujero** en el que enchufás lo que quieras.

Ahora dalo vuelta:

> Cuando escribís el test **primero**, todavía no existe Postgres. No podés depender de algo que no escribiste. Entonces te ves **obligado** a inventar la interfaz mínima que desearías tener. **Y esa interfaz es exactamente el port.**

**TDD no es un complemento de Clean Architecture. TDD la produce.** Si escribís los tests primero y honestamente, la inversión de dependencias aparece sola, porque es la única forma de que el test compile sin una base de datos.

### Cada capa de Clean tiene su técnica de test

Esto te va a organizar la cabeza para todo el resto de la guía:

| Capa | Qué testeás | Con qué | Necesita infra |
|---|---|---|---|
| **1. Entity** | Invariantes, comportamiento puro | Nada. Llamás la función. | ❌ |
| **2. Interactor** | Orquestación | **Fakes** de los gateways + **spy** del Output Port | ❌ |
| **3. Presenter** | Response Model → ViewModel | Nada. Es una función. | ❌ |
| **3. Controller** | HTTP → Input Port | **Stub** del Input Port + `app.Test(req)` | ❌ |
| **3. Gateway (DB)** | Que el SQL sea correcto | Postgres de verdad | ✅ |
| **3. Gateway (HTTP)** | Que hable bien con Qwen | `httptest.NewServer` | ❌ |
| **4. Todo junto** | Que el cableado funcione | Servidor + DB (e2e) | ✅ |

Fijate la columna de la derecha: **casi nada necesita infraestructura.** Esa es la recompensa de Clean. La pirámide de tests sale sola:

```
        ╱ e2e ╲            pocos, lentos, frágiles
      ╱─────────╲
    ╱ integración ╲        algunos (gateways contra Postgres)
  ╱─────────────────╲
╱   unitarios (1,2,3) ╲    muchísimos, instantáneos
──────────────────────
```

### Las tres herramientas de test que vas a usar

Ya conocés `testing`. Faltan dos, ambas de la stdlib o del framework:

**1. `httptest.NewServer` — para fakear HTTP saliente** (Qwen). Levantás un servidor falso y apuntás tu cliente ahí:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{"answer": 42})
}))
defer server.Close()

client := qwen.New(server.URL, "fake-key")   // ← el cliente ni se entera
```

**2. `app.Test(req)` — para probar handlers de Fiber sin abrir un puerto:**

```go
app := fiber.New()
app.Get("/api/v1/health", ctrl.Check)

req := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
resp, err := app.Test(req)   // ← in-memory, no hay socket, es instantáneo
```

**3. `t.Helper()` — para que los errores apunten al lugar correcto.** Si escribís una función auxiliar que hace assertions, marcala:

```go
func mustDecode(t *testing.T, r io.Reader, v any) {
    t.Helper()   // ← ahora el error se reporta en la línea que LLAMÓ a mustDecode
    if err := json.NewDecoder(r).Decode(v); err != nil {
        t.Fatalf("decode body: %v", err)
    }
}
```

### Antes de seguir: el contrato

No pases a la Parte 3 hasta que puedas responder esto sin mirar:

- [ ] ¿Por qué un test que no compila cuenta como RED?
- [ ] ¿Cuál es la diferencia entre `t.Errorf` y `t.Fatalf`?
- [ ] ¿Qué es un spy y en qué se diferencia de un stub?
- [ ] ¿Por qué escribir el test primero te *obliga* a crear una interfaz?
- [ ] ¿Qué capa de Clean necesita Postgres para testearse? (Solo una.)

Si alguna te trabó, volvé. **Esto no es opcional: el resto de la guía asume que lo tenés.**

---

## Parte 3 — Setup del proyecto nuevo

### El repo

```bash
mkdir quill-remake && cd quill-remake
git init
go mod init github.com/you/quill      # ← reemplazá "you" por tu usuario de GitHub
```

> A partir de acá, todos los imports de la guía dicen `github.com/you/quill/...`. Si pusiste otro módulo, ajustá.

### El esqueleto: las cuatro capas, vacías

```bash
mkdir -p cmd/api
mkdir -p internal/entity
mkdir -p internal/usecase
mkdir -p internal/adapter/controller
mkdir -p internal/adapter/presenter
mkdir -p internal/adapter/gateway
mkdir -p internal/infrastructure
```

```
quill-remake/
├── cmd/api/                     # Capa 4 — el composition root
└── internal/
    ├── entity/                  # Capa 1 — reglas de negocio
    ├── usecase/                 # Capa 2 — interactors + los ports que definen
    ├── adapter/                 # Capa 3 — controllers, presenters, gateways
    │   ├── controller/
    │   ├── presenter/
    │   └── gateway/
    └── infrastructure/          # Capa 4 — Fiber, pgx, config, reloj
```

**Regla que el compilador va a hacer cumplir por vos:** `internal/entity` no importa NADA del proyecto. Si algún día lo hace, rompiste Clean.

### La base de datos: copiar, no reinventar

El esquema de Quill son 19 migraciones SQL. **Re-tipearlas no te enseña nada de arquitectura.** Copialas del proyecto original:

```bash
cp -r ../Hackathon-QwenCloud/backend/migrations       ./migrations
cp    ../Hackathon-QwenCloud/backend/scripts/run-migrations.sh ./scripts/
cp    ../Hackathon-QwenCloud/docker-compose.yml       ./
```

> ⚠️ **Dato que te va a morder si no lo sabés:** las extensiones `vector` (pgvector) y `age` (Apache AGE) **no** las crean las migraciones. Las crea `run-migrations.sh` con `CREATE EXTENSION IF NOT EXISTS vector;` y `... age;` antes de aplicar nada. Si levantás Postgres a mano y corrés las migraciones sueltas, la 007 va a explotar con `type "vector" does not exist`.

Levantá solo la base:

```bash
docker compose up -d postgres
```

### Las dependencias: agregalas cuando un test te las pida

No hagas `go get` de todo ahora. **TDD también aplica a las dependencias:** cuando un test necesite Fiber, instalás Fiber.

Para el Slice 0 solo vas a necesitar una:

```bash
go get github.com/gofiber/fiber/v2@v2.52.6
```

### Primer commit

```bash
printf 'quill-remake\n*.env\n' > .gitignore
git add -A && git commit -m "chore: bootstrap clean architecture skeleton"
```

Listo. **Ahora sí, el primer slice.**

---

## Slice 0 — Health: el flujo canónico completo, sin base de datos

### La historia de usuario

> *Como operador, quiero pegarle a `GET /api/v1/health` y que me diga que el servicio está vivo y hace cuánto que arrancó.*

Ridículamente simple. **Ese es el punto.** Este slice no tiene base de datos, no tiene autenticación, no tiene nada. Solo el **flujo canónico de Clean, entero**:

```
GET /health
   │
   ▼
HealthController ──► [CheckInputPort] ──► CheckInteractor
   (capa 3)              (capa 2)            (capa 2)
                                                │  necesita saber la hora
                                                ├──► [Clock]  ← gateway (capa 2 define, capa 4 implementa)
                                                │
                                                │  construye HealthOutput (Response Model)
                                                ▼
                                          [CheckOutputPort]
                                                │
                                                ▼
                                         HealthPresenter ──► HealthViewModel ──► JSON
                                            (capa 3)
```

Si entendés este slice, entendés Clean. Todos los demás son este, más grande.

### 🔴 RED — el primer test

**Empezamos por el interactor**, que es donde vive la lógica. Y todavía no existe **nada**.

📦 `internal/usecase/health/interactor_test.go`

```go
package health

import (
	"context"
	"testing"
	"time"
)

// fakeClock es un FAKE: implementación real pero controlada.
// Gracias a esto el test es determinístico: no hay time.Now() de verdad,
// no hay sleeps, no hay flakiness.
type fakeClock struct{ now time.Time }

func (f fakeClock) Now() time.Time { return f.now }

// spyPresenter es un SPY: implementa el Output Port y registra lo que le empujaron.
// Como el interactor NO retorna nada, esta es la ÚNICA forma de testearlo.
type spyPresenter struct {
	called bool
	got    HealthOutput
}

func (s *spyPresenter) PresentHealth(out HealthOutput) {
	s.called = true
	s.got = out
}

func TestCheckInteractor_PresentsStatusAndUptime(t *testing.T) {
	startedAt := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	clock := fakeClock{now: startedAt.Add(90 * time.Second)}
	spy := &spyPresenter{}

	interactor := NewCheckInteractor(clock, startedAt)
	interactor.Execute(context.Background(), spy)

	if !spy.called {
		t.Fatal("interactor never pushed anything to the output port")
	}
	if spy.got.Status != "ok" {
		t.Errorf("Status = %q, want %q", spy.got.Status, "ok")
	}
	if spy.got.UptimeSeconds != 90 {
		t.Errorf("UptimeSeconds = %d, want %d", spy.got.UptimeSeconds, 90)
	}
}
```

```bash
$ go test ./internal/usecase/health/
# github.com/you/quill/internal/usecase/health [build failed]
./interactor_test.go:28:16: undefined: HealthOutput
./interactor_test.go:36:16: undefined: NewCheckInteractor
FAIL
```

**Eso es RED.** No compila, y está perfecto.

**Parate a mirar lo que acabás de hacer.** Escribiste el test primero y, sin darte cuenta, **diseñaste tres cosas**:

1. Que el interactor necesita algo con un método `Now()` → **inventaste el gateway `Clock`**.
2. Que el interactor **no retorna**: le empuja un `HealthOutput` a algo con `PresentHealth` → **inventaste el Output Port**.
3. La forma exacta del `HealthOutput`.

Nadie te dijo "hacé una interfaz". **El test te obligó.** Eso es lo que te prometí en la Parte 2.

### 🟢 GREEN — el código mínimo

Cuatro archivos chiquitos. Todos en la **capa 2**.

📦 `internal/usecase/health/model.go`

```go
package health

// HealthOutput es el RESPONSE MODEL: datos planos que salen del caso de uso.
//
// Fijate que NO tiene tags `json`. Este struct no sabe que existe HTTP.
// Convertirlo a JSON es trabajo del Presenter (capa 3).
type HealthOutput struct {
	Status        string
	UptimeSeconds int64
}
```

📦 `internal/usecase/health/port.go`

```go
package health

import "context"

// CheckInputPort es lo que el mundo exterior llama para ejecutar este caso de uso.
// El Controller depende de ESTA interfaz, nunca del CheckInteractor concreto.
type CheckInputPort interface {
	Execute(ctx context.Context, out CheckOutputPort)
}

// CheckOutputPort es donde el caso de uso EMPUJA su resultado.
// El Interactor depende de ESTA interfaz, nunca del Presenter concreto.
//
// Acá está la Inversión de Dependencias: el Presenter (capa 3, afuera)
// importa este paquete (capa 2, adentro) para implementar la interfaz.
// La flecha de import apunta hacia adentro. El control fluye hacia afuera.
type CheckOutputPort interface {
	PresentHealth(out HealthOutput)
}
```

📦 `internal/usecase/health/clock.go`

```go
package health

import "time"

// Clock es un GATEWAY. Leer el reloj del sistema es I/O: depende del mundo,
// no es determinístico, y no lo podés controlar desde un test.
//
// Por eso el caso de uso NO llama a time.Now() directamente: declara acá
// qué necesita, y deja que la capa 4 se lo provea.
type Clock interface {
	Now() time.Time
}
```

> **Este es el gateway más simple del universo, y enseña todo.** Un caso de uso que llama a `time.Now()` es un caso de uso que no podés testear sin esperar. Al invertirlo, tu test corre en microsegundos y siempre da lo mismo. **Todo I/O —la DB, el LLM, el reloj, el generador de UUIDs— se invierte igual.**

📦 `internal/usecase/health/interactor.go`

```go
package health

import (
	"context"
	"time"
)

// CheckInteractor implementa CheckInputPort.
type CheckInteractor struct {
	clock     Clock
	startedAt time.Time
}

func NewCheckInteractor(clock Clock, startedAt time.Time) *CheckInteractor {
	return &CheckInteractor{clock: clock, startedAt: startedAt}
}

// Assertion en tiempo de compilación: si CheckInteractor deja de cumplir
// el Input Port, el build falla acá y no en el cableado.
var _ CheckInputPort = (*CheckInteractor)(nil)

// Execute NO retorna el resultado. Lo EMPUJA al output port.
//
// Esta firma es la diferencia entre Clean y Hexagonal. Si dijera
// `Execute(ctx) (HealthOutput, error)` sería Hexagonal.
func (i *CheckInteractor) Execute(_ context.Context, out CheckOutputPort) {
	uptime := i.clock.Now().Sub(i.startedAt)

	out.PresentHealth(HealthOutput{
		Status:        "ok",
		UptimeSeconds: int64(uptime.Seconds()),
	})
}
```

```bash
$ go test ./internal/usecase/health/
ok  github.com/you/quill/internal/usecase/health  0.003s
```

🟢 **Verde.** Sin base de datos, sin servidor, sin esperar 90 segundos.

### 🔵 REFACTOR

Nada que limpiar todavía. Pero hacé el ejercicio de romperlo a propósito:

```go
Status: "okey",   // cambialo
```

```bash
$ go test ./internal/usecase/health/
--- FAIL: TestCheckInteractor_PresentsStatusAndUptime (0.00s)
    interactor_test.go:41: Status = "okey", want "ok"
```

Volvelo atrás. **Ahora sabés que el test funciona.** Un test que nunca viste fallar no es un test.

### Subiendo de capa: el Presenter

El Presenter vive en la **capa 3** e implementa el Output Port de la capa 2.

🔴 **RED primero:**

📦 `internal/adapter/presenter/health_presenter_test.go`

```go
package presenter

import (
	"testing"

	"github.com/you/quill/internal/usecase/health"
)

func TestHealthPresenter_BuildsViewModel(t *testing.T) {
	var p HealthPresenter

	p.PresentHealth(health.HealthOutput{Status: "ok", UptimeSeconds: 120})

	vm := p.ViewModel()
	if vm.Status != "ok" {
		t.Errorf("Status = %q, want %q", vm.Status, "ok")
	}
	if vm.UptimeSeconds != 120 {
		t.Errorf("UptimeSeconds = %d, want %d", vm.UptimeSeconds, 120)
	}
}
```

🟢 **GREEN:**

📦 `internal/adapter/presenter/health_presenter.go`

```go
package presenter

import "github.com/you/quill/internal/usecase/health"

// HealthViewModel ES el payload HTTP. Los tags `json` viven ACÁ,
// no en el Response Model. El caso de uso no sabe que existe JSON.
type HealthViewModel struct {
	Status        string `json:"status"`
	UptimeSeconds int64  `json:"uptime_seconds"`
}

// HealthPresenter implementa health.CheckOutputPort.
// Recibe el empujón del interactor y arma el ViewModel.
type HealthPresenter struct {
	vm HealthViewModel
}

var _ health.CheckOutputPort = (*HealthPresenter)(nil)

func (p *HealthPresenter) PresentHealth(out health.HealthOutput) {
	p.vm = HealthViewModel{
		Status:        out.Status,
		UptimeSeconds: out.UptimeSeconds,
	}
}

func (p *HealthPresenter) ViewModel() HealthViewModel { return p.vm }
```

> **¿Por qué duplicar el struct?** Porque `HealthOutput` y `HealthViewModel` **cambian por razones distintas**. Si mañana el frontend quiere `uptime` en formato `"2h 3m"`, cambia el ViewModel y el Presenter. El caso de uso ni se entera. Esa separación es el precio de entrada de Clean, y también su beneficio.

### Subiendo de capa: el Controller

🔴 **RED:**

📦 `internal/adapter/controller/health_controller_test.go`

```go
package controller

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/you/quill/internal/usecase/health"
)

// stubCheck es un STUB del Input Port: respuesta enlatada, cero lógica.
// El test del controller NO usa el interactor real. Cada capa se testea
// fakeando la capa de adentro.
type stubCheck struct{ out health.HealthOutput }

func (s stubCheck) Execute(_ context.Context, out health.CheckOutputPort) {
	out.PresentHealth(s.out)
}

func TestHealthController_Check_ReturnsJSON(t *testing.T) {
	ctrl := NewHealthController(stubCheck{
		out: health.HealthOutput{Status: "ok", UptimeSeconds: 42},
	})

	app := fiber.New()
	app.Get("/api/v1/health", ctrl.Check)

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("decode body %q: %v", body, err)
	}

	if got["status"] != "ok" {
		t.Errorf("status = %v, want %q", got["status"], "ok")
	}
	// Ojo: encoding/json decodifica TODO número a float64.
	if got["uptime_seconds"] != float64(42) {
		t.Errorf("uptime_seconds = %v, want 42", got["uptime_seconds"])
	}
}
```

🟢 **GREEN:**

📦 `internal/adapter/controller/health_controller.go`

```go
package controller

import (
	"github.com/gofiber/fiber/v2"

	"github.com/you/quill/internal/adapter/presenter"
	"github.com/you/quill/internal/usecase/health"
)

type HealthController struct {
	check health.CheckInputPort // ← la INTERFAZ, no el interactor concreto
}

func NewHealthController(check health.CheckInputPort) *HealthController {
	return &HealthController{check: check}
}

func (c *HealthController) Check(ctx *fiber.Ctx) error {
	// El presenter es por request: guarda el resultado de ESTA llamada.
	p := &presenter.HealthPresenter{}

	// Push-based: no hay valor de retorno. El interactor le habla al presenter.
	c.check.Execute(ctx.Context(), p)

	return ctx.JSON(p.ViewModel())
}
```

> ⚠️ **La adaptación honesta a Go.** Uncle Bob inyecta el Output Port en el **constructor** del Interactor. En Go eso te obliga a construir un interactor nuevo por cada request (porque el presenter es por request), lo cual es un desperdicio.
>
> La variante que usamos —**pasar el Output Port como parámetro de `Execute`**— conserva lo esencial: *el interactor empuja a una interfaz cuya implementación desconoce*. Los interactors quedan reusables y sin estado.
>
> Es una **adaptación consciente**, no un atajo. Decilo en voz alta cuando alguien te pregunte, y sabrás más de Clean que quien te preguntó.

### La capa 4: el reloj real y el cableado

📦 `internal/infrastructure/clock/system.go`

```go
package clock

import "time"

// System implementa health.Clock (por tipado estructural: Go no necesita
// que lo declares). Es la capa 4: el mundo real, impredecible.
type System struct{}

func (System) Now() time.Time { return time.Now() }
```

📦 `cmd/api/main.go`

```go
package main

import (
	"log"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/you/quill/internal/adapter/controller"
	"github.com/you/quill/internal/infrastructure/clock"
	"github.com/you/quill/internal/usecase/health"
)

func main() {
	startedAt := time.Now()

	// Capa 2 ← capa 4 (el reloj real entra por la interfaz Clock)
	checkInteractor := health.NewCheckInteractor(clock.System{}, startedAt)

	// Capa 3 ← capa 2 (el controller recibe el Input Port)
	healthCtrl := controller.NewHealthController(checkInteractor)

	app := fiber.New()
	app.Get("/api/v1/health", healthCtrl.Check)

	log.Fatal(app.Listen(":8080"))
}
```

**`main.go` es el único archivo que conoce todos los tipos concretos.** Es el borde más externo del sistema: acá se ensambla todo y no vive ninguna regla de negocio.

### Probalo de punta a punta

```bash
$ go test ./...
ok  github.com/you/quill/internal/adapter/controller  0.012s
ok  github.com/you/quill/internal/adapter/presenter   0.002s
ok  github.com/you/quill/internal/usecase/health      0.002s

$ go run ./cmd/api &
$ curl -s localhost:8080/api/v1/health
{"status":"ok","uptime_seconds":3}
```

### Lo que acabás de construir

```
internal/
├── usecase/health/            ← capa 2
│   ├── model.go               HealthOutput      (Response Model)
│   ├── port.go                CheckInputPort, CheckOutputPort
│   ├── clock.go               Clock             (gateway iface)
│   ├── interactor.go          CheckInteractor   (Interactor)
│   └── interactor_test.go     fakeClock + spyPresenter
├── adapter/
│   ├── controller/            ← capa 3
│   │   └── health_controller.go
│   └── presenter/             ← capa 3
│       └── health_presenter.go   HealthViewModel
└── infrastructure/clock/      ← capa 4
    └── system.go
```

Contá las flechas de import:

- `presenter` → `usecase/health` ✅ (afuera hacia adentro)
- `controller` → `usecase/health` ✅
- `usecase/health` → **nada del proyecto** ✅
- `main` → todo ✅ (es el borde externo)

**Cero flechas salen del centro.** Eso es la Regla de Dependencia, y ya la tenés funcionando.

### Checkpoint

```bash
go test ./... && git commit -am "feat(health): canonical clean slice — interactor, ports, presenter, controller"
```

> **El "por qué" del senior:** "Este slice es 100 líneas de código para devolver `{"status":"ok"}`. Un endpoint de health en Fiber son 3 líneas. **Y sin embargo lo hicimos así.** ¿Por qué? Porque el health check no importa: importa que ya escribiste un Interactor, un Input Port, un Output Port, un Presenter, un ViewModel, un gateway y un spy — y los viste correr verdes. **A partir de acá, todo es el mismo esqueleto con más carne.** Aprendiste la forma con el ejemplo que no te podía salir mal."

---

## Slice 1 — Auth / Register: entidades, gateways y el spy presenter

### La historia de usuario

> *Como escritor, quiero registrarme con email, contraseña y nombre, y recibir un token para poder usar la app.*

Ahora sí aparece todo: una **Entity** con reglas propias, **tres gateways** (base de datos, hasheo, tokens), y un **Presenter que mapea errores a códigos HTTP**.

### Las dependencias que este slice pide

```bash
go get github.com/google/uuid@v1.6.0
go get github.com/jackc/pgx/v5@v5.6.0
go get github.com/golang-jwt/jwt/v5@v5.2.1
go get golang.org/x/crypto@v0.31.0
```

### Capa 1 — La Entity: reglas que valen aunque no exista el software

🔴 **RED.** La entidad se testea sin absolutamente nada. Es una función.

📦 `internal/entity/user_test.go`

```go
package entity

import (
	"errors"
	"testing"
)

func TestNewUser(t *testing.T) {
	cases := []struct {
		name        string
		email       string
		displayName string
		wantErr     error
	}{
		{name: "usuario válido", email: "ana@quill.dev", displayName: "Ana"},
		{name: "email sin arroba", email: "ana.quill.dev", displayName: "Ana", wantErr: ErrInvalidEmail},
		{name: "email sin dominio", email: "ana@", displayName: "Ana", wantErr: ErrInvalidEmail},
		{name: "nombre vacío", email: "ana@quill.dev", displayName: "  ", wantErr: ErrDisplayNameRequired},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewUser(tc.email, tc.displayName)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("NewUser() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewUser() unexpected error: %v", err)
			}
			if got.ID.String() == "" {
				t.Error("NewUser() did not assign an ID")
			}
		})
	}
}

func TestNewUser_NormalizesEmail(t *testing.T) {
	got, err := NewUser("  ANA@Quill.DEV  ", "Ana")
	if err != nil {
		t.Fatalf("NewUser() unexpected error: %v", err)
	}
	if got.Email != "ana@quill.dev" {
		t.Errorf("Email = %q, want %q", got.Email, "ana@quill.dev")
	}
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("1234567"); !errors.Is(err, ErrPasswordTooShort) {
		t.Errorf("ValidatePassword(7 chars) = %v, want ErrPasswordTooShort", err)
	}
	if err := ValidatePassword("12345678"); err != nil {
		t.Errorf("ValidatePassword(8 chars) = %v, want nil", err)
	}
}
```

🟢 **GREEN.**

📦 `internal/entity/errors.go`

```go
package entity

import "errors"

// Errores de INVARIANTE: reglas que la entidad se niega a violar.
// No dependen de la base de datos ni de la aplicación. Son del negocio.
var (
	ErrInvalidEmail        = errors.New("invalid email")
	ErrDisplayNameRequired = errors.New("display name is required")
	ErrPasswordTooShort    = errors.New("password is too short")
)
```

📦 `internal/entity/user.go`

```go
package entity

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// MinPasswordLength es una regla de negocio, no una constante de configuración.
// Si mañana la empresa exige 12, se cambia acá y todo el sistema obedece.
const MinPasswordLength = 8

type User struct {
	ID          uuid.UUID
	Email       string
	DisplayName string
	CreatedAt   time.Time // lo llena la base de datos; la entidad no lee el reloj
}

// NewUser es la ÚNICA forma de construir un User.
// Si te devuelve un *User sin error, ese usuario es válido POR CONSTRUCCIÓN.
// No existe forma de crear un User con email inválido en todo el sistema.
func NewUser(email, displayName string) (*User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	displayName = strings.TrimSpace(displayName)

	if !looksLikeEmail(email) {
		return nil, ErrInvalidEmail
	}
	if displayName == "" {
		return nil, ErrDisplayNameRequired
	}

	return &User{
		ID:          uuid.New(),
		Email:       email,
		DisplayName: displayName,
	}, nil
}

// ValidatePassword vive acá y no en User porque el password en claro
// NUNCA es un campo de la entidad. Se valida, se hashea, y se olvida.
func ValidatePassword(plain string) error {
	if len([]rune(plain)) < MinPasswordLength {
		return ErrPasswordTooShort
	}
	return nil
}

func looksLikeEmail(s string) bool {
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	return strings.Contains(s[at+1:], ".")
}
```

```bash
$ go test ./internal/entity/
ok  github.com/you/quill/internal/entity  0.002s
```

> **Mirá bien el struct `User`.** No tiene tags `json`. No tiene tags `db`. No tiene `PasswordHash`. **El hash no es un atributo del usuario de negocio**: es un detalle de cómo lo guardamos. Por eso viaja aparte, como parámetro del gateway. Este es el primer archivo que se parte en varios: `User` (entity), `registerRequest` (controller), `UserViewModel` (presenter). Tres structs parecidos que **cambian por razones distintas**.

> ⚠️ **Trampa honesta.** `uuid.New()` adentro de la entidad es I/O disfrazado (usa el generador aleatorio del sistema). Un purista inyectaría un `IDGenerator`. En la práctica, el 99% de los proyectos Go lo hacen así y nadie se murió: el ID no es una regla de negocio, y ningún test asserta sobre su valor. **Sabelo, decidilo, no lo ignores.**

### Capa 2 — Los ports, el Request/Response Model y los gateways

Antes de escribir el interactor, escribí el **test del interactor**. Él te va a decir qué interfaces necesitás.

🔴 **RED.**

📦 `internal/usecase/auth/register_interactor_test.go`

```go
package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/you/quill/internal/entity"
)

// ── Test doubles ─────────────────────────────────────────────────────────

// fakeUserRepo: implementación real pero en memoria.
type fakeUserRepo struct {
	existing  map[string]bool
	saved     *entity.User
	savedHash string
	saveErr   error
}

func (f *fakeUserRepo) ExistsByEmail(_ context.Context, email string) (bool, error) {
	return f.existing[email], nil
}
func (f *fakeUserRepo) Save(_ context.Context, u *entity.User, hash string) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved, f.savedHash = u, hash
	return nil
}
func (f *fakeUserRepo) FindByEmail(context.Context, string) (*entity.User, string, error) {
	return nil, "", ErrUserNotFound
}

// stubHasher: respuesta enlatada, sin bcrypt (que es lento a propósito).
type stubHasher struct{}

func (stubHasher) Hash(plain string) (string, error) { return "hashed:" + plain, nil }
func (stubHasher) Compare(hash, plain string) error {
	if hash != "hashed:"+plain {
		return errors.New("mismatch")
	}
	return nil
}

// stubTokenIssuer
type stubTokenIssuer struct{}

func (stubTokenIssuer) Issue(*entity.User) (string, error) { return "fake-token", nil }

// spyPresenter: implementa RegisterOutputPort y registra qué le empujaron.
type spyPresenter struct {
	registered *RegisterOutput
	err        error
}

func (s *spyPresenter) PresentRegistered(out RegisterOutput) { s.registered = &out }
func (s *spyPresenter) PresentError(err error)               { s.err = err }

// ── Tests ────────────────────────────────────────────────────────────────

func TestRegisterInteractor_Success(t *testing.T) {
	repo := &fakeUserRepo{existing: map[string]bool{}}
	spy := &spyPresenter{}

	NewRegisterInteractor(repo, stubHasher{}, stubTokenIssuer{}).
		Execute(context.Background(), RegisterInput{
			Email: "ana@quill.dev", Password: "supersecret", DisplayName: "Ana",
		}, spy)

	if spy.err != nil {
		t.Fatalf("unexpected error presented: %v", spy.err)
	}
	if spy.registered == nil {
		t.Fatal("interactor never presented a success")
	}
	if spy.registered.Token != "fake-token" {
		t.Errorf("Token = %q, want %q", spy.registered.Token, "fake-token")
	}
	if repo.saved == nil {
		t.Fatal("user was never saved")
	}
	// El password NUNCA se guarda en claro. Esto es un test de seguridad.
	if repo.savedHash != "hashed:supersecret" {
		t.Errorf("savedHash = %q, want the hashed value", repo.savedHash)
	}
	if repo.saved.Email != "ana@quill.dev" {
		t.Errorf("saved email = %q, want normalized", repo.saved.Email)
	}
}

func TestRegisterInteractor_RejectsShortPassword(t *testing.T) {
	repo := &fakeUserRepo{existing: map[string]bool{}}
	spy := &spyPresenter{}

	NewRegisterInteractor(repo, stubHasher{}, stubTokenIssuer{}).
		Execute(context.Background(), RegisterInput{
			Email: "ana@quill.dev", Password: "corta", DisplayName: "Ana",
		}, spy)

	if !errors.Is(spy.err, entity.ErrPasswordTooShort) {
		t.Errorf("presented error = %v, want ErrPasswordTooShort", spy.err)
	}
	if repo.saved != nil {
		t.Error("must not save a user with an invalid password")
	}
}

func TestRegisterInteractor_RejectsDuplicateEmail(t *testing.T) {
	repo := &fakeUserRepo{existing: map[string]bool{"ana@quill.dev": true}}
	spy := &spyPresenter{}

	NewRegisterInteractor(repo, stubHasher{}, stubTokenIssuer{}).
		Execute(context.Background(), RegisterInput{
			Email: "ana@quill.dev", Password: "supersecret", DisplayName: "Ana",
		}, spy)

	if !errors.Is(spy.err, ErrEmailTaken) {
		t.Errorf("presented error = %v, want ErrEmailTaken", spy.err)
	}
	if repo.saved != nil {
		t.Error("must not save a duplicate user")
	}
}
```

**Leé ese test otra vez.** No hay Postgres. No hay bcrypt. No hay JWT. **Y sin embargo estás probando toda la lógica de registro**, incluida la regla de seguridad de que el password no se guarda en claro. Corre en 2 milisegundos.

Y de nuevo: el test te **dictó** las interfaces. `fakeUserRepo` tiene tres métodos → `UserRepository` tiene tres métodos. Ni uno más.

🟢 **GREEN.**

📦 `internal/usecase/auth/model.go`

```go
package auth

import "github.com/you/quill/internal/entity"

// RegisterInput es el REQUEST MODEL: datos planos que cruzan hacia adentro.
// Sin tags json: el caso de uso no sabe que existe HTTP.
type RegisterInput struct {
	Email       string
	Password    string
	DisplayName string
}

// RegisterOutput es el RESPONSE MODEL: datos planos que cruzan hacia afuera.
type RegisterOutput struct {
	User  *entity.User
	Token string
}
```

📦 `internal/usecase/auth/port.go`

```go
package auth

import "context"

type RegisterInputPort interface {
	Execute(ctx context.Context, in RegisterInput, out RegisterOutputPort)
}

type RegisterOutputPort interface {
	PresentRegistered(out RegisterOutput)
	PresentError(err error)
}
```

📦 `internal/usecase/auth/gateway.go`

```go
package auth

import (
	"context"

	"github.com/you/quill/internal/entity"
)

// ⚠️ ESTAS INTERFACES VIVEN ACÁ, en el paquete del caso de uso que las usa.
// NO en un paquete `ports/` compartido. La interfaz pertenece a quien la consume.
// Eso ES la Inversión de Dependencias.

// UserRepository: lo implementa adapter/gateway/postgres.
type UserRepository interface {
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	Save(ctx context.Context, u *entity.User, passwordHash string) error
	FindByEmail(ctx context.Context, email string) (u *entity.User, passwordHash string, err error)
}

// Hasher esconde bcrypt. bcrypt es un framework: va afuera.
type Hasher interface {
	Hash(plain string) (string, error)
	Compare(hash, plain string) error
}

// TokenIssuer esconde JWT. Idem.
type TokenIssuer interface {
	Issue(u *entity.User) (string, error)
}
```

📦 `internal/usecase/auth/errors.go`

```go
package auth

import "errors"

// Errores de APLICACIÓN, no de entidad.
// "El email ya existe" no es una regla que la entidad pueda verificar sola:
// necesita preguntarle a un repositorio. Por eso vive en la capa 2, no en la 1.
var (
	ErrEmailTaken         = errors.New("email already registered")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
)
```

📦 `internal/usecase/auth/register_interactor.go`

```go
package auth

import (
	"context"

	"github.com/you/quill/internal/entity"
)

type RegisterInteractor struct {
	users  UserRepository
	hasher Hasher
	tokens TokenIssuer
}

func NewRegisterInteractor(users UserRepository, hasher Hasher, tokens TokenIssuer) *RegisterInteractor {
	return &RegisterInteractor{users: users, hasher: hasher, tokens: tokens}
}

var _ RegisterInputPort = (*RegisterInteractor)(nil)

func (i *RegisterInteractor) Execute(ctx context.Context, in RegisterInput, out RegisterOutputPort) {
	// 1. Reglas de entidad (capa 1). No tocan el mundo.
	if err := entity.ValidatePassword(in.Password); err != nil {
		out.PresentError(err)
		return
	}
	user, err := entity.NewUser(in.Email, in.DisplayName)
	if err != nil {
		out.PresentError(err)
		return
	}

	// 2. Regla de aplicación: unicidad. Necesita el repositorio.
	taken, err := i.users.ExistsByEmail(ctx, user.Email)
	if err != nil {
		out.PresentError(err)
		return
	}
	if taken {
		out.PresentError(ErrEmailTaken)
		return
	}

	// 3. Efectos.
	hash, err := i.hasher.Hash(in.Password)
	if err != nil {
		out.PresentError(err)
		return
	}
	if err := i.users.Save(ctx, user, hash); err != nil {
		out.PresentError(err)
		return
	}

	token, err := i.tokens.Issue(user)
	if err != nil {
		out.PresentError(err)
		return
	}

	// 4. Empujar el resultado. No hay `return out, nil`.
	out.PresentRegistered(RegisterOutput{User: user, Token: token})
}
```

```bash
$ go test ./internal/usecase/auth/
ok  github.com/you/quill/internal/usecase/auth  0.003s
```

> **El orden de los pasos es una decisión de negocio, y está a la vista.** Se valida antes de tocar la base. Se chequea unicidad antes de hashear (bcrypt cuesta ~100ms: no lo gastes en un email duplicado). Se hashea antes de guardar. **Esa narrativa se lee de arriba abajo.** Eso es un Interactor.

### Capa 3 — El Presenter: acá viven los códigos HTTP

El interactor empujó un `error` de dominio. **¿Quién decide que `ErrEmailTaken` es un 409?** El Presenter. Porque "409" es un detalle de HTTP, y HTTP es un detalle de entrega.

🔴 **RED.**

📦 `internal/adapter/presenter/auth_presenter_test.go`

```go
package presenter

import (
	"net/http"
	"testing"

	"github.com/you/quill/internal/entity"
	"github.com/you/quill/internal/usecase/auth"
)

func TestAuthPresenter_MapsErrorsToStatus(t *testing.T) {
	cases := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "email inválido → 400", err: entity.ErrInvalidEmail, wantStatus: http.StatusBadRequest},
		{name: "password corta → 400", err: entity.ErrPasswordTooShort, wantStatus: http.StatusBadRequest},
		{name: "email tomado → 409", err: auth.ErrEmailTaken, wantStatus: http.StatusConflict},
		{name: "credenciales → 401", err: auth.ErrInvalidCredentials, wantStatus: http.StatusUnauthorized},
		{name: "desconocido → 500", err: errBoom, wantStatus: http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var p AuthPresenter
			p.PresentError(tc.err)

			if got := p.Status(); got != tc.wantStatus {
				t.Errorf("Status() = %d, want %d", got, tc.wantStatus)
			}
		})
	}
}

func TestAuthPresenter_HidesInternalErrorDetails(t *testing.T) {
	var p AuthPresenter
	p.PresentError(errBoom) // "connection refused: user=quill password=hunter2"

	vm, ok := p.Body().(ErrorViewModel)
	if !ok {
		t.Fatalf("Body() type = %T, want ErrorViewModel", p.Body())
	}
	if vm.Error != "internal error" {
		t.Errorf("Error = %q, want %q (no filtrar detalles internos)", vm.Error, "internal error")
	}
}
```

*(`errBoom` es un `errors.New("connection refused: user=quill password=hunter2")` declarado en el archivo de test.)*

🟢 **GREEN.**

📦 `internal/adapter/presenter/auth_presenter.go`

```go
package presenter

import (
	"errors"
	"net/http"

	"github.com/you/quill/internal/entity"
	"github.com/you/quill/internal/usecase/auth"
)

type UserViewModel struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type AuthViewModel struct {
	User  UserViewModel `json:"user"`
	Token string        `json:"token"`
}

type ErrorViewModel struct {
	Error string `json:"error"`
}

// AuthPresenter implementa auth.RegisterOutputPort (y después LoginOutputPort).
type AuthPresenter struct {
	status int
	body   any
}

var _ auth.RegisterOutputPort = (*AuthPresenter)(nil)

func (p *AuthPresenter) PresentRegistered(out auth.RegisterOutput) {
	p.status = http.StatusCreated
	p.body = AuthViewModel{
		User: UserViewModel{
			ID:          out.User.ID.String(),
			Email:       out.User.Email,
			DisplayName: out.User.DisplayName,
		},
		Token: out.Token,
	}
}

func (p *AuthPresenter) PresentError(err error) {
	switch {
	case errors.Is(err, entity.ErrInvalidEmail),
		errors.Is(err, entity.ErrDisplayNameRequired),
		errors.Is(err, entity.ErrPasswordTooShort):
		p.status = http.StatusBadRequest

	case errors.Is(err, auth.ErrEmailTaken):
		p.status = http.StatusConflict

	case errors.Is(err, auth.ErrInvalidCredentials):
		p.status = http.StatusUnauthorized

	default:
		// Error inesperado: NUNCA le mostramos el detalle al cliente.
		// Un error de pgx puede llevar la connection string adentro.
		p.status = http.StatusInternalServerError
		p.body = ErrorViewModel{Error: "internal error"}
		return
	}

	p.body = ErrorViewModel{Error: err.Error()}
}

func (p *AuthPresenter) Status() int { return p.status }
func (p *AuthPresenter) Body() any   { return p.body }
```

> **Este archivo es la razón por la que Clean vale la pena.** La tabla "qué error del negocio corresponde a qué código HTTP" está en **un solo lugar**, testeada, y el caso de uso ni se enteró. Si mañana exponés la misma lógica por gRPC, escribís otro Presenter que mapee a códigos gRPC. **El Interactor no se toca.**

### Capa 3 — El Controller

📦 `internal/adapter/controller/auth_controller.go`

```go
package controller

import (
	"net/http"

	"github.com/gofiber/fiber/v2"

	"github.com/you/quill/internal/adapter/presenter"
	"github.com/you/quill/internal/usecase/auth"
)

// registerRequest es el DTO de HTTP. Los tags `json` viven acá.
// NO es el Request Model: ese es auth.RegisterInput, y no tiene tags.
type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

type AuthController struct {
	register auth.RegisterInputPort
}

func NewAuthController(register auth.RegisterInputPort) *AuthController {
	return &AuthController{register: register}
}

func (c *AuthController) Register(ctx *fiber.Ctx) error {
	var req registerRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(http.StatusBadRequest).
			JSON(presenter.ErrorViewModel{Error: "invalid request body"})
	}

	p := &presenter.AuthPresenter{}

	c.register.Execute(ctx.Context(), auth.RegisterInput{
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	}, p)

	return ctx.Status(p.Status()).JSON(p.Body())
}
```

> **Tres structs, tres razones.** `registerRequest` cambia si cambia el contrato HTTP. `auth.RegisterInput` cambia si cambia el caso de uso. `entity.User` cambia si cambia el negocio. **Por eso se dividen los archivos.** No es burocracia: es que tienen dueños distintos y ciclos de vida distintos.

### Capa 3 — Los gateways reales

Estos son los únicos archivos donde aparecen bcrypt, JWT y pgx.

📦 `internal/adapter/gateway/crypto/bcrypt_hasher.go`

```go
package crypto

import (
	"golang.org/x/crypto/bcrypt"

	"github.com/you/quill/internal/usecase/auth"
)

type BcryptHasher struct{ cost int }

func NewBcryptHasher(cost int) BcryptHasher { return BcryptHasher{cost: cost} }

var _ auth.Hasher = BcryptHasher{}

func (h BcryptHasher) Hash(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), h.cost)
	return string(b), err
}

func (h BcryptHasher) Compare(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
```

📦 `internal/adapter/gateway/token/jwt_issuer.go`

```go
package token

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/you/quill/internal/entity"
	"github.com/you/quill/internal/usecase/auth"
)

type JWTIssuer struct {
	secret []byte
	expiry time.Duration
}

func NewJWTIssuer(secret string, expiry time.Duration) JWTIssuer {
	return JWTIssuer{secret: []byte(secret), expiry: expiry}
}

var _ auth.TokenIssuer = JWTIssuer{}

func (i JWTIssuer) Issue(u *entity.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": u.ID.String(),
		"email":   u.Email,
		"exp":     time.Now().Add(i.expiry).Unix(),
		"iat":     time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(i.secret)
}

// Validate lo vamos a usar en el Slice 2 (middleware).
func (i JWTIssuer) Validate(tokenString string) (uuid.UUID, error) {
	tok, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil || !tok.Valid {
		return uuid.Nil, auth.ErrInvalidCredentials
	}

	claims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return uuid.Nil, auth.ErrInvalidCredentials
	}
	raw, _ := claims["user_id"].(string)
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, auth.ErrInvalidCredentials
	}
	return id, nil
}
```

> ⚠️ **Ese chequeo de `SigningMethodHMAC` no es opcional.** Sin él, un atacante te manda un token firmado con `alg: none` o con RS256 usando tu clave pública como secreto HMAC. Es la vulnerabilidad de JWT más vieja del mundo. **Nunca la simplifiques.**

📦 `internal/adapter/gateway/postgres/user_gateway.go`

```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/you/quill/internal/entity"
	"github.com/you/quill/internal/usecase/auth"
)

type UserGateway struct{ pool *pgxpool.Pool }

func NewUserGateway(pool *pgxpool.Pool) *UserGateway { return &UserGateway{pool: pool} }

var _ auth.UserRepository = (*UserGateway)(nil)

func (g *UserGateway) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	var exists bool
	err := g.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)`, email,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("exists by email: %w", err)
	}
	return exists, nil
}

func (g *UserGateway) Save(ctx context.Context, u *entity.User, passwordHash string) error {
	_, err := g.pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, display_name) VALUES ($1, $2, $3, $4)`,
		u.ID, u.Email, passwordHash, u.DisplayName,
	)
	if err != nil {
		return fmt.Errorf("save user: %w", err)
	}
	return nil
}

func (g *UserGateway) FindByEmail(ctx context.Context, email string) (*entity.User, string, error) {
	var u entity.User
	var hash string

	err := g.pool.QueryRow(ctx,
		`SELECT id, email, display_name, password_hash, created_at FROM users WHERE email = $1`, email,
	).Scan(&u.ID, &u.Email, &u.DisplayName, &hash, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		// Devolvemos un error del CASO DE USO, no `pgx.ErrNoRows`.
		// El interactor no debe saber que existe pgx.
		return nil, "", auth.ErrUserNotFound
	}
	if err != nil {
		return nil, "", fmt.Errorf("find by email: %w", err)
	}
	return &u, hash, nil
}
```

> **Fijate la traducción de errores.** `pgx.ErrNoRows` es vocabulario de Postgres. Si el interactor tuviera que hacer `errors.Is(err, pgx.ErrNoRows)`, estaría importando pgx. **El gateway traduce el error del mundo exterior al vocabulario de adentro.** Eso es literalmente lo que significa "adapter".

### El test de integración del gateway

Este es el **único** test del slice que necesita Postgres.

📦 `internal/testutil/db.go`

```go
package testutil

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// SetupTestDB devuelve un pool contra TEST_DATABASE_URL.
// Si la variable no está, SALTEA el test en vez de fallar: así `go test ./...`
// sigue verde en una máquina sin Docker.
func SetupTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	pool, err := pgxpool.New(context.Background(), url)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("ping test db: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// Truncate deja las tablas vacías para que cada test arranque limpio.
// Los nombres de tabla vienen de código de test, nunca de input externo.
func Truncate(t *testing.T, pool *pgxpool.Pool, tables ...string) {
	t.Helper()
	for _, table := range tables {
		if _, err := pool.Exec(context.Background(), "TRUNCATE "+table+" CASCADE"); err != nil {
			t.Fatalf("truncate %s: %v", table, err)
		}
	}
}
```

📦 `internal/adapter/gateway/postgres/user_gateway_test.go`

```go
package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/you/quill/internal/entity"
	"github.com/you/quill/internal/testutil"
	"github.com/you/quill/internal/usecase/auth"
)

func TestUserGateway_SaveAndFindByEmail(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.Truncate(t, pool, "users")

	ctx := context.Background()
	g := NewUserGateway(pool)

	u, err := entity.NewUser("ana@quill.dev", "Ana")
	if err != nil {
		t.Fatalf("NewUser: %v", err)
	}

	if err := g.Save(ctx, u, "hashed-password"); err != nil {
		t.Fatalf("Save: %v", err)
	}

	exists, err := g.ExistsByEmail(ctx, "ana@quill.dev")
	if err != nil {
		t.Fatalf("ExistsByEmail: %v", err)
	}
	if !exists {
		t.Error("ExistsByEmail = false, want true after Save")
	}

	got, hash, err := g.FindByEmail(ctx, "ana@quill.dev")
	if err != nil {
		t.Fatalf("FindByEmail: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("ID = %v, want %v", got.ID, u.ID)
	}
	if hash != "hashed-password" {
		t.Errorf("hash = %q, want %q", hash, "hashed-password")
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt is zero; the DB should have filled it")
	}
}

func TestUserGateway_FindByEmail_NotFound(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.Truncate(t, pool, "users")

	_, _, err := NewUserGateway(pool).FindByEmail(context.Background(), "nadie@quill.dev")

	// El gateway traduce pgx.ErrNoRows al vocabulario del caso de uso.
	if !errors.Is(err, auth.ErrUserNotFound) {
		t.Errorf("err = %v, want auth.ErrUserNotFound", err)
	}
}
```

**Preparar la base de test** (una sola vez):

```bash
docker compose exec postgres psql -U quill -c 'CREATE DATABASE quill_test;'

DB_NAME=quill_test MIGRATIONS_DIR=./migrations sh ./scripts/run-migrations.sh

export TEST_DATABASE_URL='postgres://quill:quill_dev_password@localhost:5432/quill_test?sslmode=disable'
```

```bash
$ go test ./...           # sin TEST_DATABASE_URL: los de integración se saltean
$ TEST_DATABASE_URL=... go test ./...   # con la variable: corren todos
ok  github.com/you/quill/internal/adapter/gateway/postgres  0.089s
```

### Cableado y prueba de punta a punta

En `cmd/api/main.go`, agregá:

```go
pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
if err != nil {
	log.Fatalf("connect db: %v", err)
}
defer pool.Close()

userGateway := postgres.NewUserGateway(pool)
hasher      := crypto.NewBcryptHasher(12)
issuer      := token.NewJWTIssuer(os.Getenv("JWT_SECRET"), 24*time.Hour)

registerInteractor := auth.NewRegisterInteractor(userGateway, hasher, issuer)
authCtrl           := controller.NewAuthController(registerInteractor)

app.Post("/api/v1/auth/register", authCtrl.Register)
```

```bash
$ curl -s -X POST localhost:8080/api/v1/auth/register \
    -H 'Content-Type: application/json' \
    -d '{"email":"ana@quill.dev","password":"supersecret","display_name":"Ana"}' | jq
{
  "user": { "id": "…", "email": "ana@quill.dev", "display_name": "Ana" },
  "token": "eyJhbGciOiJIUzI1NiIs…"
}

$ # y de nuevo, mismo email:
$ curl -s -o /dev/null -w '%{http_code}\n' -X POST localhost:8080/api/v1/auth/register \
    -H 'Content-Type: application/json' \
    -d '{"email":"ana@quill.dev","password":"supersecret","display_name":"Ana"}'
409
```

### Checkpoint

```bash
go test ./... && git commit -am "feat(auth): register slice — entity, interactor, gateways, presenter"
```

### El "por qué" del senior

> "Contá los tests que escribiste: **cuatro de entidad, tres de interactor, dos de presenter, dos de gateway.** De los once, **nueve corren sin Postgres**. Y los dos que lo necesitan solo prueban una cosa: *que el SQL sea correcto*. Ninguna regla de negocio necesitó una base de datos para ser verificada.
>
> Eso no salió porque seas prolijo. Salió porque **escribiste el test primero y no tenías Postgres a mano.** La arquitectura fue la consecuencia, no la causa. **Ese es el secreto que nadie te cuenta de Clean Architecture: no la diseñás, la descubrís escribiendo tests.**"

---

## Slice 2 — Auth / Login: JWT, middleware y una lección de seguridad

> *Como escritor registrado, quiero entrar con mi email y contraseña y recibir un token; y quiero que las rutas privadas rechacen a quien no lo tenga.*

Desde acá **bajo el andamiaje**: ya conocés el ciclo 🔴🟢🔵 y las cinco piezas. Te muestro el código completo, pero la narración se concentra en **lo nuevo**.

Lo nuevo de este slice son **dos cosas**, y la segunda es la importante.

### Lo nuevo #1 — El interactor de Login

El test primero. Fijate en el **tercer caso**: es la lección del slice.

📦 `internal/usecase/auth/login_interactor_test.go`

```go
package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/you/quill/internal/entity"
)

// fakeLoginRepo devuelve un usuario con su hash, o ErrUserNotFound.
type fakeLoginRepo struct {
	user *entity.User
	hash string
}

func (f *fakeLoginRepo) FindByEmail(_ context.Context, email string) (*entity.User, string, error) {
	if f.user == nil || f.user.Email != email {
		return nil, "", ErrUserNotFound
	}
	return f.user, f.hash, nil
}
func (f *fakeLoginRepo) ExistsByEmail(context.Context, string) (bool, error) { return false, nil }
func (f *fakeLoginRepo) Save(context.Context, *entity.User, string) error    { return nil }

type spyLoginPresenter struct {
	out *LoginOutput
	err error
}

func (s *spyLoginPresenter) PresentLoggedIn(out LoginOutput) { s.out = &out }
func (s *spyLoginPresenter) PresentError(err error)          { s.err = err }

func TestLoginInteractor_Success(t *testing.T) {
	u, _ := entity.NewUser("ana@quill.dev", "Ana")
	repo := &fakeLoginRepo{user: u, hash: "hashed:supersecret"}
	spy := &spyLoginPresenter{}

	NewLoginInteractor(repo, stubHasher{}, stubTokenIssuer{}).
		Execute(context.Background(), LoginInput{Email: "ana@quill.dev", Password: "supersecret"}, spy)

	if spy.err != nil {
		t.Fatalf("unexpected error: %v", spy.err)
	}
	if spy.out.Token != "fake-token" {
		t.Errorf("Token = %q, want %q", spy.out.Token, "fake-token")
	}
}

func TestLoginInteractor_WrongPassword(t *testing.T) {
	u, _ := entity.NewUser("ana@quill.dev", "Ana")
	repo := &fakeLoginRepo{user: u, hash: "hashed:supersecret"}
	spy := &spyLoginPresenter{}

	NewLoginInteractor(repo, stubHasher{}, stubTokenIssuer{}).
		Execute(context.Background(), LoginInput{Email: "ana@quill.dev", Password: "otra-cosa"}, spy)

	if !errors.Is(spy.err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials", spy.err)
	}
}

// ⚠️ LA LECCIÓN DEL SLICE.
// Un email que no existe y una contraseña equivocada deben producir
// EXACTAMENTE el mismo error. Si no, un atacante enumera tus usuarios.
func TestLoginInteractor_UnknownEmail_LooksIdenticalToWrongPassword(t *testing.T) {
	repo := &fakeLoginRepo{} // sin usuarios
	spy := &spyLoginPresenter{}

	NewLoginInteractor(repo, stubHasher{}, stubTokenIssuer{}).
		Execute(context.Background(), LoginInput{Email: "nadie@quill.dev", Password: "loquesea"}, spy)

	if !errors.Is(spy.err, ErrInvalidCredentials) {
		t.Errorf("err = %v, want ErrInvalidCredentials (NUNCA ErrUserNotFound)", spy.err)
	}
	if errors.Is(spy.err, ErrUserNotFound) {
		t.Error("filtró que el email no existe: eso permite enumerar usuarios")
	}
}
```

📦 `internal/usecase/auth/login_interactor.go`

```go
package auth

import "context"

type LoginInput struct {
	Email    string
	Password string
}

type LoginOutput struct {
	User  *entity.User
	Token string
}

type LoginInputPort interface {
	Execute(ctx context.Context, in LoginInput, out LoginOutputPort)
}

type LoginOutputPort interface {
	PresentLoggedIn(out LoginOutput)
	PresentError(err error)
}

type LoginInteractor struct {
	users  UserRepository
	hasher Hasher
	tokens TokenIssuer
}

func NewLoginInteractor(users UserRepository, hasher Hasher, tokens TokenIssuer) *LoginInteractor {
	return &LoginInteractor{users: users, hasher: hasher, tokens: tokens}
}

var _ LoginInputPort = (*LoginInteractor)(nil)

func (i *LoginInteractor) Execute(ctx context.Context, in LoginInput, out LoginOutputPort) {
	user, hash, err := i.users.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(in.Email)))
	if err != nil {
		// El repo dijo ErrUserNotFound. Nosotros decimos ErrInvalidCredentials.
		// El cliente NUNCA distingue "no existe" de "clave mal".
		out.PresentError(ErrInvalidCredentials)
		return
	}

	if err := i.hasher.Compare(hash, in.Password); err != nil {
		out.PresentError(ErrInvalidCredentials)
		return
	}

	token, err := i.tokens.Issue(user)
	if err != nil {
		out.PresentError(err)
		return
	}

	out.PresentLoggedIn(LoginOutput{User: user, Token: token})
}
```

*(Agregá `"strings"` a los imports, y `PresentLoggedIn` al `AuthPresenter` — mapea a `200 OK` con el mismo `AuthViewModel`.)*

> **Por qué esto es arquitectura y no solo seguridad.** El gateway devuelve `ErrUserNotFound` (es la verdad técnica). El interactor decide **no revelarla** (es la política del negocio). Cada capa dice la verdad que le corresponde. Si el gateway devolviera directamente `ErrInvalidCredentials`, estarías metiendo una decisión de seguridad dentro de una consulta SQL.

### Lo nuevo #2 — El middleware

El middleware protege rutas. Necesita **validar** un token, no emitirlo. ¿Dónde va la interfaz?

**Pregunta clave: ¿quién la consume?** El middleware. Y el middleware es capa 3 (adapter). Entonces la interfaz **vive en el middleware**.

📦 `internal/adapter/controller/middleware.go`

```go
package controller

import (
	"net/http"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"github.com/you/quill/internal/adapter/presenter"
)

// TokenValidator: la interfaz la define QUIEN LA USA. Acá el consumidor
// es el middleware (capa 3), así que la interfaz vive acá.
// token.JWTIssuer la satisface sin saberlo (tipado estructural).
type TokenValidator interface {
	Validate(token string) (uuid.UUID, error)
}

const userIDKey = "userID"

func AuthMiddleware(v TokenValidator) fiber.Handler {
	return func(c *fiber.Ctx) error {
		header := c.Get("Authorization")
		raw, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || raw == "" {
			return c.Status(http.StatusUnauthorized).
				JSON(presenter.ErrorViewModel{Error: "missing bearer token"})
		}

		userID, err := v.Validate(raw)
		if err != nil {
			return c.Status(http.StatusUnauthorized).
				JSON(presenter.ErrorViewModel{Error: "invalid token"})
		}

		c.Locals(userIDKey, userID)
		return c.Next()
	}
}

// UserIDFrom saca el userID que el middleware dejó en el context de Fiber.
// Es el ÚNICO lugar del proyecto que conoce la clave "userID".
func UserIDFrom(c *fiber.Ctx) uuid.UUID {
	id, _ := c.Locals(userIDKey).(uuid.UUID)
	return id
}
```

Y su test, con un stub del validador:

📦 `internal/adapter/controller/middleware_test.go`

```go
package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type stubValidator struct {
	id  uuid.UUID
	err error
}

func (s stubValidator) Validate(string) (uuid.UUID, error) { return s.id, s.err }

func TestAuthMiddleware(t *testing.T) {
	wantID := uuid.New()

	cases := []struct {
		name       string
		header     string
		validator  stubValidator
		wantStatus int
	}{
		{name: "token válido", header: "Bearer abc", validator: stubValidator{id: wantID}, wantStatus: http.StatusOK},
		{name: "sin header", header: "", validator: stubValidator{id: wantID}, wantStatus: http.StatusUnauthorized},
		{name: "sin prefijo Bearer", header: "abc", validator: stubValidator{id: wantID}, wantStatus: http.StatusUnauthorized},
		{name: "token inválido", header: "Bearer abc", validator: stubValidator{err: errors.New("bad")}, wantStatus: http.StatusUnauthorized},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			app.Use(AuthMiddleware(tc.validator))
			app.Get("/private", func(c *fiber.Ctx) error {
				if got := UserIDFrom(c); got != wantID {
					t.Errorf("UserIDFrom() = %v, want %v", got, wantID)
				}
				return c.SendStatus(http.StatusOK)
			})

			req := httptest.NewRequest(http.MethodGet, "/private", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}

			resp, err := app.Test(req)
			if err != nil {
				t.Fatalf("app.Test: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tc.wantStatus {
				t.Errorf("status = %d, want %d", resp.StatusCode, tc.wantStatus)
			}
		})
	}
}
```

> **`c.Locals` es Fiber.** Es un detalle del framework. Por eso queda encerrado en `middleware.go` + `UserIDFrom`. Ningún controller escribe `c.Locals("userID")` a mano: llaman a `UserIDFrom(c)`. **Si mañana migrás a Echo, cambiás dos funciones.**

### Checkpoint

```bash
go test ./... && git commit -am "feat(auth): login interactor + auth middleware"
```

---

## Slice 3 — Universe: invariantes reales y cuándo NO usar una transacción

> *Como escritor, quiero crear un universo con nombre, género y formato, y listar los míos.*

Lo nuevo: **una entidad con invariantes de verdad** (no un `if x == ""`), **una regla de propiedad** (nadie ve los universos de otro), y una lección sobre transacciones que va a contramano de lo que te enseñan.

### La entidad, con sus reglas adentro

📦 `internal/entity/universe.go`

```go
package entity

import (
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Estos conjuntos SON la regla de negocio. No son configuración.
// Viven en la capa 1 porque valdrían aunque Quill fuera una app de escritorio.
var allowedGenres = map[string]struct{}{
	"sci-fi": {}, "fantasy": {}, "mystery": {}, "romance": {}, "horror": {},
	"non-fiction": {}, "thriller": {}, "historical": {}, "adventure": {},
	"comedy": {}, "drama": {},
}

var allowedFormats = map[string]struct{}{
	"novel": {}, "short-story": {}, "screenplay": {},
	"poetry": {}, "essay": {}, "article": {}, "graphic-novel": {},
}

type Universe struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	Name        string
	Description string
	Genre       string
	Format      string
	CreatedAt   time.Time
}

// NewUniverse: si te devuelve un *Universe, es válido. Punto.
// No existe forma de tener un Universe con género "reggaeton" en este sistema.
func NewUniverse(userID uuid.UUID, name, description, genre, format string) (*Universe, error) {
	name = strings.TrimSpace(name)

	if name == "" {
		return nil, ErrUniverseNameRequired
	}
	if format == "" {
		return nil, ErrUniverseFormatRequired
	}
	if _, ok := allowedFormats[format]; !ok {
		// %w envuelve el sentinel: errors.Is(err, ErrInvalidFormat) sigue funcionando,
		// y además el mensaje dice cuál fue el valor malo.
		return nil, fmt.Errorf("%w: %q", ErrInvalidFormat, format)
	}
	if genre != "" {
		if _, ok := allowedGenres[genre]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrInvalidGenre, genre)
		}
	}

	return &Universe{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        name,
		Description: strings.TrimSpace(description),
		Genre:       genre,
		Format:      format,
	}, nil
}

// OwnedBy es una regla de negocio, no un `if` suelto en un handler.
func (u *Universe) OwnedBy(userID uuid.UUID) bool { return u.UserID == userID }
```

Agregá a `internal/entity/errors.go`:

```go
var (
	ErrUniverseNameRequired   = errors.New("universe name is required")
	ErrUniverseFormatRequired = errors.New("universe format is required")
	ErrInvalidGenre           = errors.New("invalid genre")
	ErrInvalidFormat          = errors.New("invalid format")
)
```

📦 `internal/entity/universe_test.go`

```go
package entity

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewUniverse(t *testing.T) {
	uid := uuid.New()

	cases := []struct {
		name    string
		gName   string
		genre   string
		format  string
		wantErr error
	}{
		{name: "válido con género", gName: "Middle-earth", genre: "fantasy", format: "novel"},
		{name: "género vacío es válido", gName: "Sin género", genre: "", format: "novel"},
		{name: "nombre vacío", gName: "   ", genre: "fantasy", format: "novel", wantErr: ErrUniverseNameRequired},
		{name: "formato vacío", gName: "X", genre: "fantasy", format: "", wantErr: ErrUniverseFormatRequired},
		{name: "género inventado", gName: "X", genre: "reggaeton", format: "novel", wantErr: ErrInvalidGenre},
		{name: "formato inventado", gName: "X", genre: "fantasy", format: "tiktok", wantErr: ErrInvalidFormat},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewUniverse(uid, tc.gName, "", tc.genre, tc.format)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("NewUniverse() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewUniverse() unexpected error: %v", err)
			}
			if !got.OwnedBy(uid) {
				t.Error("OwnedBy(creator) = false, want true")
			}
		})
	}
}
```

> **Cero infraestructura, seis reglas de negocio verificadas.** Este es el test que más valor por línea te da en todo el proyecto, y corre en microsegundos.

### El interactor: la regla de propiedad

📦 `internal/usecase/universe/universe.go` *(un archivo, es chico)*

```go
package universe

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/you/quill/internal/entity"
)

var ErrForbidden = errors.New("forbidden")

// ── Models ───────────────────────────────────────────────────────────────

type CreateInput struct {
	UserID      uuid.UUID
	Name        string
	Description string
	Genre       string
	Format      string
}

type CreateOutput struct{ Universe *entity.Universe }

type ListInput struct {
	UserID uuid.UUID
	Page   int
	Limit  int
}

type ListOutput struct {
	Universes []entity.Universe
	Total     int
}

// ── Ports ────────────────────────────────────────────────────────────────

type CreateInputPort interface {
	Execute(ctx context.Context, in CreateInput, out CreateOutputPort)
}
type CreateOutputPort interface {
	PresentCreated(out CreateOutput)
	PresentError(err error)
}

type ListInputPort interface {
	Execute(ctx context.Context, in ListInput, out ListOutputPort)
}
type ListOutputPort interface {
	PresentList(out ListOutput)
	PresentError(err error)
}

// ── Gateway (definido acá, implementado en adapter/gateway/postgres) ──────

type Repository interface {
	Save(ctx context.Context, u *entity.Universe) error
	ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]entity.Universe, int, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Universe, error)
}

// GraphStore provisiona el grafo del universo. Best-effort: si falla,
// el universo igual existe. Es una decisión de negocio, no técnica.
type GraphStore interface {
	CreateGraph(ctx context.Context, universeID uuid.UUID) error
}

type Logger interface{ Warn(msg string, args ...any) }
```

📦 `internal/usecase/universe/create_interactor.go`

```go
package universe

import (
	"context"

	"github.com/you/quill/internal/entity"
)

type CreateInteractor struct {
	repo  Repository
	graph GraphStore
	log   Logger
}

func NewCreateInteractor(repo Repository, graph GraphStore, log Logger) *CreateInteractor {
	return &CreateInteractor{repo: repo, graph: graph, log: log}
}

var _ CreateInputPort = (*CreateInteractor)(nil)

func (i *CreateInteractor) Execute(ctx context.Context, in CreateInput, out CreateOutputPort) {
	u, err := entity.NewUniverse(in.UserID, in.Name, in.Description, in.Genre, in.Format)
	if err != nil {
		out.PresentError(err)
		return
	}

	// Un solo INSERT. NO necesita transacción (ver la nota de abajo).
	if err := i.repo.Save(ctx, u); err != nil {
		out.PresentError(err)
		return
	}

	// Best-effort: el grafo se puede crear después. No aborta la operación.
	if err := i.graph.CreateGraph(ctx, u.ID); err != nil {
		i.log.Warn("create graph failed", "universe", u.ID, "err", err)
	}

	out.PresentCreated(CreateOutput{Universe: u})
}
```

### ⚠️ La lección que va a contramano: cuándo NO usar una transacción

El Quill original hace esto para crear un universo:

```go
tx, _ := s.pool.Begin(ctx)          // ← abre transacción
defer tx.Rollback(ctx)
s.universeRepo.Create(ctx, tx, u)   // ← UN SOLO INSERT
tx.Commit(ctx)                      // ← comitea
```

**Eso es ceremonia.** Un `INSERT` suelto en Postgres **ya es atómico**: o entra entero o no entra. Envolverlo en `BEGIN/COMMIT` no agrega ninguna garantía; agrega un round-trip y ruido.

Peor: obliga a que el repositorio exponga `pgx.Tx` en su firma pública, y de golpe **el caso de uso conoce el driver de la base**.

> **La regla real:** una transacción sirve para que **varias** escrituras sean atómicas entre sí. Si tu caso de uso escribe **un solo agregado**, dejá que el gateway maneje lo suyo y seguí de largo.
>
> Vas a necesitar una transacción de verdad en el **Slice 6** (Entity), donde una operación escribe la entidad, su historial, un nodo del grafo y un embedding. **Ahí** vamos a construir el `UnitOfWork` — y vas a entender para qué sirve porque lo vas a haber extrañado.

Aprender un patrón es fácil. Aprender **cuándo no usarlo** es lo que separa al que copia del que diseña.

### El gateway y el controller

📦 `internal/adapter/gateway/postgres/universe_gateway.go`

```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/you/quill/internal/entity"
	"github.com/you/quill/internal/usecase/universe"
)

type UniverseGateway struct{ pool *pgxpool.Pool }

func NewUniverseGateway(pool *pgxpool.Pool) *UniverseGateway { return &UniverseGateway{pool: pool} }

var _ universe.Repository = (*UniverseGateway)(nil)

func (g *UniverseGateway) Save(ctx context.Context, u *entity.Universe) error {
	_, err := g.pool.Exec(ctx,
		`INSERT INTO universes (id, user_id, name, description, genre, format)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		u.ID, u.UserID, u.Name, u.Description, u.Genre, u.Format,
	)
	if err != nil {
		return fmt.Errorf("save universe: %w", err)
	}
	return nil
}

func (g *UniverseGateway) ListByUser(ctx context.Context, userID uuid.UUID, page, limit int) ([]entity.Universe, int, error) {
	offset := (page - 1) * limit

	rows, err := g.pool.Query(ctx,
		`SELECT id, user_id, name, description, genre, format, created_at
		   FROM universes WHERE user_id = $1
		  ORDER BY created_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("list universes: %w", err)
	}
	defer rows.Close()

	var out []entity.Universe
	for rows.Next() {
		var u entity.Universe
		if err := rows.Scan(&u.ID, &u.UserID, &u.Name, &u.Description, &u.Genre, &u.Format, &u.CreatedAt); err != nil {
			return nil, 0, fmt.Errorf("scan universe: %w", err)
		}
		out = append(out, u)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate universes: %w", err)
	}

	var total int
	if err := g.pool.QueryRow(ctx, `SELECT COUNT(*) FROM universes WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count universes: %w", err)
	}
	return out, total, nil
}

func (g *UniverseGateway) FindByID(ctx context.Context, id uuid.UUID) (*entity.Universe, error) {
	var u entity.Universe
	err := g.pool.QueryRow(ctx,
		`SELECT id, user_id, name, description, genre, format, created_at FROM universes WHERE id = $1`, id,
	).Scan(&u.ID, &u.UserID, &u.Name, &u.Description, &u.Genre, &u.Format, &u.CreatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, universe.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find universe: %w", err)
	}
	return &u, nil
}
```

*(Agregá `var ErrNotFound = errors.New("universe not found")` en `usecase/universe`.)*

📦 `internal/adapter/controller/universe_controller.go` *(fragmento del método `Create`)*

```go
func (c *UniverseController) Create(ctx *fiber.Ctx) error {
	var req createUniverseRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(http.StatusBadRequest).JSON(presenter.ErrorViewModel{Error: "invalid request body"})
	}

	p := &presenter.UniversePresenter{}

	c.create.Execute(ctx.Context(), universe.CreateInput{
		UserID:      UserIDFrom(ctx), // ← lo dejó el middleware del Slice 2
		Name:        req.Name,
		Description: req.Description,
		Genre:       req.Genre,
		Format:      req.Format,
	}, p)

	return ctx.Status(p.Status()).JSON(p.Body())
}
```

### Probalo

```bash
$ TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login \
    -H 'Content-Type: application/json' \
    -d '{"email":"ana@quill.dev","password":"supersecret"}' | jq -r .token)

$ curl -s -X POST localhost:8080/api/v1/universes \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
    -d '{"name":"Middle-earth","genre":"fantasy","format":"novel"}' | jq

$ # y ahora el género inventado:
$ curl -s -X POST localhost:8080/api/v1/universes \
    -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
    -d '{"name":"X","genre":"reggaeton","format":"novel"}'
{"error":"invalid genre: \"reggaeton\""}   # 400, y la regla nunca salió de entity/
```

```bash
go test ./... && git commit -am "feat(universe): entity invariants, ownership rule, create+list"
```

---

## Slice 4 — ✍️ Ejercicio: Work + Chapter

**Ahora te toca a vos.** Ya viste el molde tres veces. `Work` y `Chapter` no traen ni un concepto nuevo: son el Slice 3 con otros campos.

**Te doy los tests. Esos son la especificación.** Escribí el código hasta que pasen. La solución de referencia está en el **Apéndice A**.

### Lo que tenés que construir

```
internal/entity/work.go                            entity.NewWork(...)
internal/entity/chapter.go                         entity.NewChapter(...) + (*Chapter).UpdateContent(...)
internal/usecase/work/…                            Create + ListByUniverse
internal/usecase/chapter/…                         Create + Update
internal/adapter/gateway/postgres/work_gateway.go
internal/adapter/gateway/postgres/chapter_gateway.go
internal/adapter/presenter/work_presenter.go
internal/adapter/controller/work_controller.go
```

### La especificación (los tests)

📦 `internal/entity/work_test.go`

```go
package entity

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewWork(t *testing.T) {
	uid := uuid.New()

	cases := []struct {
		name     string
		title    string
		workType string
		wantErr  error
	}{
		{name: "válido", title: "The Fellowship", workType: "novel"},
		{name: "título vacío", title: "  ", workType: "novel", wantErr: ErrWorkTitleRequired},
		{name: "tipo inventado", title: "X", workType: "podcast", wantErr: ErrInvalidWorkType},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NewWork(uid, tc.title, tc.workType, "")
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("NewWork() error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("NewWork() unexpected error: %v", err)
			}
			if got.Status != "in_progress" {
				t.Errorf("Status = %q, want %q (default)", got.Status, "in_progress")
			}
			if got.OrderIndex != 1 {
				t.Errorf("OrderIndex = %d, want 1 (default)", got.OrderIndex)
			}
		})
	}
}
```

📦 `internal/entity/chapter_test.go`

```go
package entity

import (
	"testing"

	"github.com/google/uuid"
)

func TestChapter_UpdateContent_RecalculatesWordCount(t *testing.T) {
	ch, err := NewChapter(uuid.New(), "Chapter One")
	if err != nil {
		t.Fatalf("NewChapter: %v", err)
	}
	if ch.WordCount != 0 {
		t.Errorf("WordCount = %d, want 0 for a new chapter", ch.WordCount)
	}

	ch.UpdateContent("<p>hola mundo cruel</p>", "hola mundo cruel")

	if ch.WordCount != 3 {
		t.Errorf("WordCount = %d, want 3", ch.WordCount)
	}
	if ch.RawText != "hola mundo cruel" {
		t.Errorf("RawText = %q, want %q", ch.RawText, "hola mundo cruel")
	}
	if ch.Status != "draft" {
		t.Errorf("Status = %q, want %q", ch.Status, "draft")
	}
}

func TestChapter_UpdateContent_EmptyTextIsZeroWords(t *testing.T) {
	ch, _ := NewChapter(uuid.New(), "Chapter One")
	ch.UpdateContent("", "   ")

	if ch.WordCount != 0 {
		t.Errorf("WordCount = %d, want 0", ch.WordCount)
	}
}
```

### Pistas (leelas solo si te trabás)

1. `WordCount` sale de `len(strings.Fields(rawText))`. **Es la misma función `WordCount` que escribiste en la Parte 2.** ¿Dónde debería vivir ahora? Pista: es una regla de negocio sobre capítulos.
2. `allowedWorkTypes` es un `map[string]struct{}` igual que `allowedGenres`. Mirá el schema (`migrations/003_create_works.up.sql`) para saber los valores.
3. El interactor de `Create` para Work necesita verificar que **el universo existe y es tuyo** antes de crear. Eso significa que `work.Repository` necesita… pensalo. El test te lo va a decir.
4. Si tu interactor de Chapter no puede ser testeado sin Postgres, **inyectaste mal algo**. Volvé al Slice 1.

### Auto-evaluación

Cuando termines, chequeá:

- [ ] `go test ./internal/entity/` pasa **sin** `TEST_DATABASE_URL`.
- [ ] `go test ./internal/usecase/work/` pasa **sin** `TEST_DATABASE_URL`.
- [ ] `internal/entity/work.go` **no importa** nada de `internal/`.
- [ ] Ningún interactor importa `pgx`, `fiber` ni `bcrypt`.
- [ ] Tu `WorkRepository` está definido en `usecase/work/`, **no** en un paquete `ports/`.

Solución completa: **Apéndice A**.

---

## Slice 5 — Los tres gateways externos: Qwen, pgvector y AGE

Este slice **no tiene interactor**. Construimos las tres piezas de infraestructura que los slices 6 a 9 van a usar: el cliente del LLM, el buscador vectorial y el grafo.

> **¿Por qué ahora y no dentro de cada slice?** Porque los comparten cuatro slices. Si los dejás para después, los vas a re-escribir cuatro veces. **El trabajo compartido se hace una vez, antes de sus consumidores.**

### 🔑 Un truco de Go que hace esto posible

Fijate en algo raro: vamos a escribir los **adapters** antes que las **interfaces** que implementan.

En Java o C# eso no compilaría. En Go **sí**, porque el tipado es **estructural**: un tipo satisface una interfaz si tiene los métodos, sin declararlo. El adapter no importa la interfaz. **La interfaz la va a declarar después el caso de uso que lo consuma** — que es exactamente donde Clean dice que debe estar.

```bash
go get github.com/pgvector/pgvector-go@v0.2.3
```

---

### 5.a — El gateway de Qwen (TDD de un cliente HTTP)

**La pregunta del principiante:** *"¿cómo testeo código que llama a una API que cuesta plata?"*

**La respuesta:** levantás un servidor HTTP falso en el mismo test. `httptest.NewServer` te da uno de verdad, en `localhost`, en un puerto random, que muere cuando termina el test.

🔴 **RED.**

📦 `internal/adapter/gateway/qwen/client_test.go`

```go
package qwen

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Embed_ParsesResponseAndSendsAuth(t *testing.T) {
	var gotPath, gotAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{{"embedding": []float32{0.1, 0.2, 0.3}}},
		})
	}))
	defer server.Close()

	c := NewClient(server.URL, "test-key", "text-embedding-v3", "qwen-max")

	got, err := c.Embed(context.Background(), "hola mundo")
	if err != nil {
		t.Fatalf("Embed() unexpected error: %v", err)
	}

	if gotPath != "/embeddings" {
		t.Errorf("path = %q, want %q", gotPath, "/embeddings")
	}
	if gotAuth != "Bearer test-key" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer test-key")
	}
	if len(got) != 3 || got[0] != 0.1 {
		t.Errorf("Embed() = %v, want [0.1 0.2 0.3]", got)
	}
}

func TestClient_Embed_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "k", "m", "m").Embed(context.Background(), "x")
	if err == nil {
		t.Fatal("Embed() error = nil, want an error on HTTP 500")
	}
}

func TestClient_Embed_EmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer server.Close()

	_, err := NewClient(server.URL, "k", "m", "m").Embed(context.Background(), "x")
	if err == nil {
		t.Fatal("Embed() error = nil, want an error when the API returns no embeddings")
	}
}
```

🟢 **GREEN.**

📦 `internal/adapter/gateway/qwen/client.go`

```go
package qwen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	http           *http.Client
	baseURL        string
	apiKey         string
	embeddingModel string
	chatModel      string
}

func NewClient(baseURL, apiKey, embeddingModel, chatModel string) *Client {
	return &Client{
		http:           &http.Client{Timeout: 60 * time.Second},
		baseURL:        baseURL,
		apiKey:         apiKey,
		embeddingModel: embeddingModel,
		chatModel:      chatModel,
	}
}

type embeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// Embed convierte texto en un vector. Este método satisface (sin saberlo)
// la interfaz `Embedder` que el Slice 6 va a declarar en su caso de uso.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	var resp embeddingResponse
	err := c.post(ctx, "/embeddings", embeddingRequest{
		Model: c.embeddingModel,
		Input: []string{text},
	}, &resp)
	if err != nil {
		return nil, err
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("qwen: empty embedding response")
	}
	return resp.Data[0].Embedding, nil
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("call qwen %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("qwen %s: unexpected status %d", path, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode qwen response: %w", err)
	}
	return nil
}
```

```bash
$ go test ./internal/adapter/gateway/qwen/
ok  github.com/you/quill/internal/adapter/gateway/qwen  0.008s
```

> **Cero llamadas reales. Cero tokens gastados. Cero API key.** Y probaste el path, el header de auth, el parseo, el error 500 y la respuesta vacía. **Así se testea cualquier integración externa.**

---

### 5.b — El gateway vectorial (pgvector)

📦 `internal/adapter/gateway/postgres/vector_gateway.go`

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pgvector/pgvector-go"
)

type VectorGateway struct{ pool *pgxpool.Pool }

func NewVectorGateway(pool *pgxpool.Pool) *VectorGateway { return &VectorGateway{pool: pool} }

// SaveEntityEmbedding: el port habla en []float32. La conversión a
// pgvector.Vector ocurre ACÁ adentro. pgvector NO cruza la frontera.
func (g *VectorGateway) SaveEntityEmbedding(ctx context.Context, entityID uuid.UUID, emb []float32) error {
	_, err := g.pool.Exec(ctx,
		`INSERT INTO entity_embeddings (id, entity_id, description_embedding)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (entity_id) DO UPDATE SET description_embedding = EXCLUDED.description_embedding,
		                                       updated_at = NOW()`,
		uuid.New(), entityID, pgvector.NewVector(emb),
	)
	if err != nil {
		return fmt.Errorf("save entity embedding: %w", err)
	}
	return nil
}

// FindSimilarEntity: `<=>` es la distancia coseno de pgvector (0 = idéntico).
// Devolvemos el ID más parecido si la distancia está por debajo del umbral.
func (g *VectorGateway) FindSimilarEntity(
	ctx context.Context, universeID uuid.UUID, emb []float32, maxDistance float64,
) (*uuid.UUID, float64, error) {
	var id uuid.UUID
	var distance float64

	err := g.pool.QueryRow(ctx,
		`SELECT e.id, ee.description_embedding <=> $2 AS distance
		   FROM entity_embeddings ee
		   JOIN entities e ON e.id = ee.entity_id
		  WHERE e.universe_id = $1
		  ORDER BY distance ASC
		  LIMIT 1`,
		universeID, pgvector.NewVector(emb),
	).Scan(&id, &distance)

	if err != nil {
		return nil, 0, nil // sin candidatos: no es un error, simplemente no hay match
	}
	if distance > maxDistance {
		return nil, distance, nil
	}
	return &id, distance, nil
}
```

> ⚠️ El `ON CONFLICT (entity_id)` **necesita** una constraint UNIQUE en esa columna. Está en `migrations/016`. Sin ella, Postgres tira el error `42P10`. (Fue un bug real del proyecto original.)

**El pool necesita registrar los tipos de pgvector** al abrir cada conexión:

📦 `internal/infrastructure/database/pool.go`

```go
package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	pgxvector "github.com/pgvector/pgvector-go/pgx"
)

func NewPool(ctx context.Context, url string, maxConns int32) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = maxConns

	// Cada conexión nueva del pool necesita conocer el tipo `vector`.
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		return pgxvector.RegisterTypes(ctx, conn)
	}

	return pgxpool.NewWithConfig(ctx, cfg)
}
```

> **Nota importante:** el Quill original también hace `LOAD 'age'` y `SET search_path` en el `AfterConnect`. **Nosotros no.** Y por una razón muy buena — mirá el punto siguiente.

---

### 5.c — El gateway del grafo (Apache AGE): el arista más filosa

AGE es una extensión de Postgres que corre Cypher. Tiene **tres trampas**:

1. **No acepta queries parametrizadas** dentro del bloque `$$ … $$`. Todo va interpolado. Hola, inyección.
2. Necesita `SET search_path = ag_catalog, …` para resolver la función `cypher()`.
3. **`ag_catalog` tapa a `public`.** Tiene sus propias tablas `entities`, `works`, etc. Si dejás el `search_path` puesto y devolvés esa conexión al pool, **la próxima query escribe en las tablas equivocadas.**

> **Esto pasó de verdad en el Quill original.** Una conexión envenenada del pool escribió tablas sombra dentro de `ag_catalog`. Está en el historial de git.

#### La decisión de arquitectura: ¿qué es dominio y qué es adapter?

| Pieza | ¿Dónde vive? | Por qué |
|---|---|---|
| Nombre del grafo (`universe_<uuid>`) | Adapter | Un UUID no puede inyectar nada. Seguro por construcción. |
| `escapeCypherString` | Adapter | Sintaxis de Cypher. Puro detalle. |
| `LOAD 'age'` + `search_path` + restaurarlo | Adapter | Mecánica de la extensión. |
| **"Un tipo de entidad debe ser `[A-Za-z_][A-Za-z0-9_]*`"** | **Entity** (¡y el adapter!) | Es una **regla de modelado**, no de Cypher. Además los labels salen del LLM. |

Esa última fila es la sutil. Los labels vienen del output de un modelo de lenguaje, o sea **de un desconocido**. La regla vive en `entity` como invariante, y el adapter **vuelve a verificarla** antes de interpolar. Defensa en profundidad: **nunca confíes en que el de arriba validó, cuando abajo estás construyendo Cypher a mano.**

📦 `internal/adapter/gateway/postgres/graph_gateway.go`

```go
package postgres

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/you/quill/internal/entity"
)

type GraphGateway struct{ pool *pgxpool.Pool }

func NewGraphGateway(pool *pgxpool.Pool) *GraphGateway { return &GraphGateway{pool: pool} }

// El nombre del grafo deriva de un UUID: no hay superficie de inyección.
func graphName(universeID uuid.UUID) string { return "universe_" + universeID.String() }

var identifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func escapeCypherString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, `'`, `\'`)
}

// withAgeConn es LA función que hace que AGE sea seguro de usar.
//
// Toma una conexión del pool, carga AGE, pone el search_path... y SIEMPRE
// lo restaura antes de devolver la conexión. Si no restauraras, la conexión
// volvería al pool con `ag_catalog` adelante de `public`, y la próxima query
// de CUALQUIER parte del sistema escribiría en las tablas equivocadas.
//
// Toda operación de grafo pasa por acá. Sin excepciones.
func (g *GraphGateway) withAgeConn(ctx context.Context, fn func(conn *pgxpool.Conn) error) error {
	conn, err := g.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire conn: %w", err)
	}
	defer conn.Release()

	var prior string
	if err := conn.QueryRow(ctx, "SHOW search_path").Scan(&prior); err != nil {
		return fmt.Errorf("read search_path: %w", err)
	}

	if _, err := conn.Exec(ctx, "LOAD 'age'"); err != nil {
		return fmt.Errorf("load age: %w", err)
	}
	if _, err := conn.Exec(ctx, `SET search_path = ag_catalog, "$user", public`); err != nil {
		return fmt.Errorf("set search_path: %w", err)
	}

	// El defer corre pase lo que pase: error, panic, return temprano.
	defer func() {
		// set_config SÍ acepta parámetros, a diferencia del bloque cypher.
		_, _ = conn.Exec(ctx, `SELECT set_config('search_path', $1, false)`, prior)
	}()

	return fn(conn)
}

func (g *GraphGateway) CreateGraph(ctx context.Context, universeID uuid.UUID) error {
	return g.withAgeConn(ctx, func(conn *pgxpool.Conn) error {
		var exists bool
		err := conn.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM ag_catalog.ag_graph WHERE name = $1)`, graphName(universeID),
		).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check graph exists: %w", err)
		}
		if exists {
			return nil // idempotente
		}
		_, err = conn.Exec(ctx, `SELECT create_graph($1)`, graphName(universeID))
		return err
	})
}

// CreateNode interpola `label` en Cypher. Por eso lo valida ANTES.
func (g *GraphGateway) CreateNode(ctx context.Context, universeID uuid.UUID, label string, props map[string]string) error {
	if !identifierRe.MatchString(label) {
		return fmt.Errorf("%w: %q", entity.ErrInvalidIdentifier, label)
	}

	pairs := make([]string, 0, len(props))
	for k, v := range props {
		if !identifierRe.MatchString(k) {
			return fmt.Errorf("%w: property key %q", entity.ErrInvalidIdentifier, k)
		}
		pairs = append(pairs, fmt.Sprintf("%s: '%s'", k, escapeCypherString(v)))
	}

	return g.withAgeConn(ctx, func(conn *pgxpool.Conn) error {
		q := fmt.Sprintf(
			`SELECT * FROM cypher('%s', $$ CREATE (:%s {%s}) $$) AS (v agtype)`,
			graphName(universeID), label, strings.Join(pairs, ", "),
		)
		_, err := conn.Exec(ctx, q)
		if err != nil {
			return fmt.Errorf("create node: %w", err)
		}
		return nil
	})
}
```

*(Agregá `ErrInvalidIdentifier = errors.New("invalid identifier")` a `internal/entity/errors.go`.)*

📦 `internal/adapter/gateway/postgres/graph_gateway_test.go` *(lo más importante que vas a testear en todo el proyecto)*

```go
package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/you/quill/internal/entity"
	"github.com/you/quill/internal/testutil"
)

func TestGraphGateway_CreateNode_RejectsInvalidLabels(t *testing.T) {
	g := NewGraphGateway(nil) // ni siquiera necesita DB: valida antes de conectarse

	bad := []string{
		"",
		"1Character",
		"Character; DROP TABLE users",
		"Char-acter",
		"Character'})--",
	}

	for _, label := range bad {
		t.Run(label, func(t *testing.T) {
			err := g.CreateNode(context.Background(), uuid.New(), label, nil)
			if !errors.Is(err, entity.ErrInvalidIdentifier) {
				t.Errorf("CreateNode(%q) error = %v, want ErrInvalidIdentifier", label, err)
			}
		})
	}
}

// EL test que te salva la vida: la conexión vuelve al pool LIMPIA.
func TestGraphGateway_DoesNotPoisonSearchPath(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	ctx := context.Background()
	universeID := uuid.New()

	var before string
	if err := pool.QueryRow(ctx, "SHOW search_path").Scan(&before); err != nil {
		t.Fatalf("read search_path: %v", err)
	}

	if err := NewGraphGateway(pool).CreateGraph(ctx, universeID); err != nil {
		t.Fatalf("CreateGraph: %v", err)
	}

	var after string
	if err := pool.QueryRow(ctx, "SHOW search_path").Scan(&after); err != nil {
		t.Fatalf("read search_path: %v", err)
	}

	if before != after {
		t.Errorf("search_path leaked: before = %q, after = %q", before, after)
	}
}
```

> **El "por qué" del senior:** "Mirá el primer test: `NewGraphGateway(nil)`. Le pasás un pool **nulo** y el test pasa, porque la validación ocurre **antes** de tocar la base. Eso no es casualidad: es que validar input hostil es lo primero que hace la función. Si tuvieras que levantar Postgres para probar que rechazás `"Character; DROP TABLE users"`, nadie escribiría ese test.
>
> Y el segundo test es el que va a evitar que pierdas un fin de semana. **Un bug de estado compartido en un pool de conexiones no se reproduce en desarrollo: se reproduce en producción, con carga, un martes.** Escribir ese test cuesta 15 líneas."

```bash
go test ./... && git commit -am "feat(gateways): qwen client, pgvector store, AGE graph with search_path guard"
```

---

## Slice 6 — Entity: un interactor que orquesta cuatro gateways

> *Cuando el sistema detecta "Aragorn" en un párrafo, quiero que reconozca si ya existe (por nombre, por alias, o por parecido semántico) y fusione la información nueva; y si no existe, que lo cree.*

Este es **el corazón funcional de Quill**. Y trae dos cosas nuevas:
1. **Una entidad con comportamiento de verdad**: `Entity.Merge`.
2. **El Unit of Work** — la transacción que en el Slice 3 te dije que no necesitabas. **Ahora sí.**

### La entidad: `Merge` es una regla de negocio, no un helper

📦 `internal/entity/entity.go`

```go
package entity

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

// InitialRelevanceScore: una entidad recién descubierta arranca "caliente".
// Es una regla de negocio con nombre, no un 0.8 mágico perdido en un service.
const InitialRelevanceScore = 0.8

// identifierRe: un tipo de entidad tiene que ser un identificador válido.
//
// ¿Por qué es una regla de DOMINIO y no de Cypher? Porque los tipos vienen
// del output de un LLM, o sea de un desconocido. "Un tipo de entidad se
// escribe como un identificador" es una regla de modelado que valdría
// aunque no existiera Apache AGE.
var identifierRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func ValidIdentifier(s string) bool { return identifierRe.MatchString(s) }

// ExtractedEntity es lo que el LLM sacó del texto. Es un concepto del NEGOCIO
// ("una mención extraída"), no un tipo del repositorio.
type ExtractedEntity struct {
	Type        string
	Name        string
	Aliases     []string
	Description string
	Status      string
}

type Entity struct {
	ID             uuid.UUID
	UniverseID     uuid.UUID
	Type           string
	Name           string
	Aliases        []string
	Description    string
	Status         string
	RelevanceScore float64
}

func NewEntity(universeID uuid.UUID, in ExtractedEntity) (*Entity, error) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, ErrEntityNameRequired
	}
	if !ValidIdentifier(in.Type) {
		return nil, fmt.Errorf("%w: entity type %q", ErrInvalidIdentifier, in.Type)
	}

	status := in.Status
	if status == "" {
		status = "active"
	}

	return &Entity{
		ID:             uuid.New(),
		UniverseID:     universeID,
		Type:           in.Type,
		Name:           name,
		Aliases:        in.Aliases,
		Description:    in.Description,
		Status:         status,
		RelevanceScore: InitialRelevanceScore,
	}, nil
}

// Merge absorbe información de una mención nueva.
//
// Las tres reglas: los alias se UNEN (nunca se pierden), la descripción MÁS
// LARGA gana (asumimos que más texto = más información), y el status nuevo
// pisa al viejo si viene informado (un personaje puede morir).
func (e *Entity) Merge(in ExtractedEntity) {
	for _, alias := range in.Aliases {
		if !e.HasAlias(alias) {
			e.Aliases = append(e.Aliases, alias)
		}
	}
	if len(in.Description) > len(e.Description) {
		e.Description = in.Description
	}
	if in.Status != "" {
		e.Status = in.Status
	}
}

func (e *Entity) HasAlias(alias string) bool {
	for _, a := range e.Aliases {
		if strings.EqualFold(a, alias) {
			return true
		}
	}
	return false
}
```

📦 `internal/entity/entity_test.go`

```go
package entity

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewEntity_RejectsInvalidType(t *testing.T) {
	// Los tipos vienen del LLM. Un LLM puede alucinar cualquier cosa.
	bad := []string{"", "1Character", "Char-acter", "Character; DROP TABLE users"}

	for _, typ := range bad {
		t.Run(typ, func(t *testing.T) {
			_, err := NewEntity(uuid.New(), ExtractedEntity{Name: "Aragorn", Type: typ})
			if !errors.Is(err, ErrInvalidIdentifier) {
				t.Errorf("NewEntity(type=%q) error = %v, want ErrInvalidIdentifier", typ, err)
			}
		})
	}
}

func TestNewEntity_Defaults(t *testing.T) {
	e, err := NewEntity(uuid.New(), ExtractedEntity{Name: "Aragorn", Type: "Character"})
	if err != nil {
		t.Fatalf("NewEntity: %v", err)
	}
	if e.Status != "active" {
		t.Errorf("Status = %q, want %q", e.Status, "active")
	}
	if e.RelevanceScore != InitialRelevanceScore {
		t.Errorf("RelevanceScore = %v, want %v", e.RelevanceScore, InitialRelevanceScore)
	}
}

func TestEntity_Merge(t *testing.T) {
	base := &Entity{
		Name:        "Aragorn",
		Aliases:     []string{"Strider"},
		Description: "A ranger.",
		Status:      "alive",
	}

	base.Merge(ExtractedEntity{
		Aliases:     []string{"Strider", "Elessar"}, // "Strider" ya está: no se duplica
		Description: "A ranger of the North, heir of Isildur.", // más larga: gana
		Status:      "king",
	})

	if len(base.Aliases) != 2 {
		t.Errorf("Aliases = %v, want 2 unique aliases", base.Aliases)
	}
	if !base.HasAlias("Elessar") {
		t.Error("Merge() lost the new alias")
	}
	if base.Description != "A ranger of the North, heir of Isildur." {
		t.Errorf("Description = %q, want the longer one", base.Description)
	}
	if base.Status != "king" {
		t.Errorf("Status = %q, want %q", base.Status, "king")
	}
}

func TestEntity_Merge_ShorterDescriptionDoesNotWin(t *testing.T) {
	base := &Entity{Description: "A ranger of the North, heir of Isildur."}
	base.Merge(ExtractedEntity{Description: "A guy."})

	if base.Description == "A guy." {
		t.Error("Merge() replaced a longer description with a shorter one")
	}
}
```

*(Agregá `ErrEntityNameRequired` a `errors.go`.)*

🔵 **REFACTOR entre slices.** Ahora que la regla del identificador vive en `entity`, el `GraphGateway` del Slice 5 puede dejar de tener su propio regex:

```go
// internal/adapter/gateway/postgres/graph_gateway.go — antes:
if !identifierRe.MatchString(label) { … }

// después:
if !entity.ValidIdentifier(label) { … }
```

**Sigue siendo defensa en profundidad** (el adapter valida de nuevo, en otro momento del tiempo), pero la *regla* está definida una sola vez. Corré los tests: siguen verdes. **Eso es refactorizar.**

### 🔑 El Unit of Work: ahora sí lo necesitás

Crear una entidad nueva escribe en **dos tablas** (`entities` y `entity_relevance_history`). Si la segunda falla, la primera **no puede quedar**. Eso es una transacción de verdad.

Pero no queremos que el interactor sepa qué es `pgx.Tx`. Entonces el caso de uso declara **qué necesita**, en su propio vocabulario:

📦 `internal/usecase/entityres/gateway.go`

```go
package entityres

import (
	"context"

	"github.com/google/uuid"

	"github.com/you/quill/internal/entity"
)

// UnitOfWork: "corré esto de forma atómica". Ni una palabra sobre Postgres.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

type Repository interface {
	FindByName(ctx context.Context, universeID uuid.UUID, name string) (*entity.Entity, error)
	FindByAlias(ctx context.Context, universeID uuid.UUID, alias string) (*entity.Entity, error)
	FindByID(ctx context.Context, id uuid.UUID) (*entity.Entity, error)
	Save(ctx context.Context, e *entity.Entity) error
}

type HistoryRepository interface {
	Append(ctx context.Context, e *entity.Entity) error
}

type VectorStore interface {
	FindSimilarEntity(ctx context.Context, universeID uuid.UUID, emb []float32, maxDistance float64) (*uuid.UUID, float64, error)
	SaveEntityEmbedding(ctx context.Context, entityID uuid.UUID, emb []float32) error
}

type GraphStore interface {
	CreateNode(ctx context.Context, universeID uuid.UUID, label string, props map[string]string) error
}

type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}

type Logger interface{ Warn(msg string, args ...any) }
```

> **Ahí está `qwen.Client` satisfaciendo `Embedder` sin haberlo sabido nunca.** Escribimos el adapter en el Slice 5 y la interfaz recién ahora, en el paquete que la consume. **Eso es Clean, y es Go.**

#### La implementación: la transacción viaja en el `context`

📦 `internal/adapter/gateway/postgres/unit_of_work.go`

```go
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Querier es el subconjunto de métodos que comparten *pgxpool.Pool y pgx.Tx.
// Gracias a esto, un gateway no necesita saber si está dentro de una transacción.
type Querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// txKey es un tipo privado: nadie fuera de este paquete puede leer ni escribir
// la transacción del context. Encapsulación por el sistema de tipos.
type txKey struct{}

type UnitOfWork struct{ pool *pgxpool.Pool }

func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork { return &UnitOfWork{pool: pool} }

func (u *UnitOfWork) Do(ctx context.Context, fn func(context.Context) error) error {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	// Si fn falla o entra en panic, esto revierte. Si ya se comiteó, es no-op.
	defer tx.Rollback(ctx)

	if err := fn(context.WithValue(ctx, txKey{}, tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// db devuelve la transacción activa si el context la trae; si no, el pool.
// TODOS los gateways de este paquete la usan en lugar de `g.pool` directo.
func db(ctx context.Context, pool *pgxpool.Pool) Querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}
```

Y ahora los gateways cambian **una línea**:

```go
func (g *EntityGateway) Save(ctx context.Context, e *entity.Entity) error {
	_, err := db(ctx, g.pool).Exec(ctx, `INSERT INTO entities (...) VALUES (...)
	                                     ON CONFLICT (id) DO UPDATE SET ...`, /* ... */)
	return err
}
```

📦 `internal/adapter/gateway/postgres/unit_of_work_test.go`

```go
package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/you/quill/internal/entity"
	"github.com/you/quill/internal/testutil"
)

func TestUnitOfWork_RollsBackOnError(t *testing.T) {
	pool := testutil.SetupTestDB(t)
	testutil.Truncate(t, pool, "users")

	uow := NewUnitOfWork(pool)
	users := NewUserGateway(pool)
	ctx := context.Background()

	u, _ := entity.NewUser("ana@quill.dev", "Ana")
	boom := errors.New("boom")

	err := uow.Do(ctx, func(ctx context.Context) error {
		if err := users.Save(ctx, u, "hash"); err != nil {
			return err
		}
		return boom // algo falla DESPUÉS de escribir
	})

	if !errors.Is(err, boom) {
		t.Fatalf("Do() error = %v, want boom", err)
	}

	// La escritura tiene que haber desaparecido.
	exists, err := users.ExistsByEmail(ctx, "ana@quill.dev")
	if err != nil {
		t.Fatalf("ExistsByEmail: %v", err)
	}
	if exists {
		t.Error("el usuario sobrevivió al rollback: la transacción no funcionó")
	}
}
```

> ⚠️ Para que ese test pase, `UserGateway.Save` tiene que usar `db(ctx, g.pool)` en vez de `g.pool`. **Ese es el punto entero del Unit of Work**: los gateways no saben si están en una transacción, pero la respetan.

### El interactor: el funnel de cuatro pasos

📦 `internal/usecase/entityres/resolve_interactor.go`

```go
package entityres

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/you/quill/internal/entity"
)

// SimilarityThreshold: por debajo de esta distancia coseno consideramos que
// dos nombres son la misma entidad. Regla de negocio, ajustable.
const SimilarityThreshold = 0.15

var ErrEntityNotFound = errors.New("entity not found")

type ResolveInput struct {
	UniverseID uuid.UUID
	Data       entity.ExtractedEntity
}

type ResolveOutput struct {
	Entity         *entity.Entity
	PreviousStatus string // "" si es nueva
	IsNew          bool
}

type ResolveInputPort interface {
	Execute(ctx context.Context, in ResolveInput, out ResolveOutputPort)
}

type ResolveOutputPort interface {
	PresentResolved(out ResolveOutput)
	PresentError(err error)
}

type ResolveInteractor struct {
	entities Repository
	history  HistoryRepository
	vectors  VectorStore
	graph    GraphStore
	embedder Embedder
	uow      UnitOfWork
	log      Logger
}

func NewResolveInteractor(
	entities Repository, history HistoryRepository, vectors VectorStore,
	graph GraphStore, embedder Embedder, uow UnitOfWork, log Logger,
) *ResolveInteractor {
	return &ResolveInteractor{entities, history, vectors, graph, embedder, uow, log}
}

var _ ResolveInputPort = (*ResolveInteractor)(nil)

func (i *ResolveInteractor) Execute(ctx context.Context, in ResolveInput, out ResolveOutputPort) {
	var result *entity.Entity
	var prevStatus string
	var isNew bool

	// Todo lo relacional, atómico. Sin una línea de pgx.
	err := i.uow.Do(ctx, func(ctx context.Context) error {
		existing, err := i.findExisting(ctx, in.UniverseID, in.Data)
		if err != nil {
			return err
		}

		if existing != nil {
			prevStatus = existing.Status // OJO: antes del Merge, que lo pisa
			existing.Merge(in.Data)      // ← el DOMINIO hace el trabajo
			result = existing
			return i.entities.Save(ctx, existing)
		}

		created, err := entity.NewEntity(in.UniverseID, in.Data)
		if err != nil {
			return err
		}
		result, isNew = created, true

		if err := i.entities.Save(ctx, created); err != nil {
			return err
		}
		// Segunda escritura: por esto existe la transacción.
		return i.history.Append(ctx, created)
	})
	if err != nil {
		out.PresentError(err)
		return
	}

	if isNew {
		i.indexNewEntity(ctx, result) // best-effort, fuera de la transacción
	}

	out.PresentResolved(ResolveOutput{Entity: result, PreviousStatus: prevStatus, IsNew: isNew})
}

// findExisting: el funnel. Nombre exacto → alias → parecido semántico.
func (i *ResolveInteractor) findExisting(ctx context.Context, universeID uuid.UUID, data entity.ExtractedEntity) (*entity.Entity, error) {
	// 1. Nombre exacto
	found, err := i.entities.FindByName(ctx, universeID, data.Name)
	if err == nil {
		return found, nil
	}
	if !errors.Is(err, ErrEntityNotFound) {
		return nil, err
	}

	// 2. Alias
	for _, alias := range data.Aliases {
		found, err := i.entities.FindByAlias(ctx, universeID, alias)
		if err == nil {
			return found, nil
		}
		if !errors.Is(err, ErrEntityNotFound) {
			return nil, err
		}
	}

	// 3. Parecido semántico. Si el LLM no responde, tratamos la entidad como nueva:
	//    perder una fusión es mejor que abortar la escritura del capítulo.
	emb, err := i.embedder.Embed(ctx, data.Name)
	if err != nil {
		i.log.Warn("embed failed, skipping semantic match", "name", data.Name, "err", err)
		return nil, nil
	}
	similarID, _, err := i.vectors.FindSimilarEntity(ctx, universeID, emb, SimilarityThreshold)
	if err != nil || similarID == nil {
		return nil, nil
	}
	return i.entities.FindByID(ctx, *similarID)
}

// indexNewEntity: efectos externos. Si fallan, la entidad ya existe igual.
func (i *ResolveInteractor) indexNewEntity(ctx context.Context, e *entity.Entity) {
	props := map[string]string{
		"entity_id": e.ID.String(),
		"name":      e.Name,
		"status":    e.Status,
	}
	if err := i.graph.CreateNode(ctx, e.UniverseID, e.Type, props); err != nil {
		i.log.Warn("create graph node failed", "entity", e.Name, "err", err)
	}

	emb, err := i.embedder.Embed(ctx, e.Name)
	if err != nil {
		i.log.Warn("embed new entity failed", "entity", e.Name, "err", err)
		return
	}
	if err := i.vectors.SaveEntityEmbedding(ctx, e.ID, emb); err != nil {
		i.log.Warn("save embedding failed", "entity", e.Name, "err", err)
	}
}
```

### El test: cinco fakes, cero infraestructura

📦 `internal/usecase/entityres/resolve_interactor_test.go` *(extracto)*

```go
package entityres

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/you/quill/internal/entity"
)

// ¡EL FAKE MÁS IMPORTANTE DE LA GUÍA! Una línea.
// El interactor no distingue entre "una transacción de verdad" y
// "correr la función y listo". Esa indiferencia ES el desacople.
type fakeUoW struct{}

func (fakeUoW) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

type fakeEntities struct {
	byName map[string]*entity.Entity
	saved  *entity.Entity
}

func (f *fakeEntities) FindByName(_ context.Context, _ uuid.UUID, name string) (*entity.Entity, error) {
	if e, ok := f.byName[name]; ok {
		return e, nil
	}
	return nil, ErrEntityNotFound
}
func (f *fakeEntities) FindByAlias(context.Context, uuid.UUID, string) (*entity.Entity, error) {
	return nil, ErrEntityNotFound
}
func (f *fakeEntities) FindByID(context.Context, uuid.UUID) (*entity.Entity, error) {
	return nil, ErrEntityNotFound
}
func (f *fakeEntities) Save(_ context.Context, e *entity.Entity) error { f.saved = e; return nil }

type spyHistory struct{ appended int }

func (s *spyHistory) Append(context.Context, *entity.Entity) error { s.appended++; return nil }

type stubEmbedder struct{}

func (stubEmbedder) Embed(context.Context, string) ([]float32, error) { return []float32{0.1}, nil }

type stubVectors struct{}

func (stubVectors) FindSimilarEntity(context.Context, uuid.UUID, []float32, float64) (*uuid.UUID, float64, error) {
	return nil, 0, nil // nunca hay parecidos
}
func (stubVectors) SaveEntityEmbedding(context.Context, uuid.UUID, []float32) error { return nil }

type spyGraph struct{ nodes int }

func (s *spyGraph) CreateNode(context.Context, uuid.UUID, string, map[string]string) error {
	s.nodes++
	return nil
}

type nopLogger struct{}

func (nopLogger) Warn(string, ...any) {}

type spyPresenter struct {
	out *ResolveOutput
	err error
}

func (s *spyPresenter) PresentResolved(out ResolveOutput) { s.out = &out }
func (s *spyPresenter) PresentError(err error)            { s.err = err }

func newInteractor(e *fakeEntities, h *spyHistory, g *spyGraph) *ResolveInteractor {
	return NewResolveInteractor(e, h, stubVectors{}, g, stubEmbedder{}, fakeUoW{}, nopLogger{})
}

// ── Tests ────────────────────────────────────────────────────────────────

func TestResolve_CreatesNewEntity(t *testing.T) {
	entities := &fakeEntities{byName: map[string]*entity.Entity{}}
	history, graph, spy := &spyHistory{}, &spyGraph{}, &spyPresenter{}

	newInteractor(entities, history, graph).Execute(context.Background(), ResolveInput{
		UniverseID: uuid.New(),
		Data:       entity.ExtractedEntity{Name: "Aragorn", Type: "Character"},
	}, spy)

	if spy.err != nil {
		t.Fatalf("unexpected error: %v", spy.err)
	}
	if !spy.out.IsNew {
		t.Error("IsNew = false, want true")
	}
	if spy.out.PreviousStatus != "" {
		t.Errorf("PreviousStatus = %q, want empty for a new entity", spy.out.PreviousStatus)
	}
	if history.appended != 1 {
		t.Errorf("history.Append called %d times, want 1", history.appended)
	}
	if graph.nodes != 1 {
		t.Errorf("graph.CreateNode called %d times, want 1", graph.nodes)
	}
}

func TestResolve_MergesExistingByExactName(t *testing.T) {
	existing := &entity.Entity{
		ID: uuid.New(), Name: "Aragorn", Status: "alive",
		Aliases: []string{"Strider"}, Description: "A ranger.",
	}
	entities := &fakeEntities{byName: map[string]*entity.Entity{"Aragorn": existing}}
	history, graph, spy := &spyHistory{}, &spyGraph{}, &spyPresenter{}

	newInteractor(entities, history, graph).Execute(context.Background(), ResolveInput{
		UniverseID: uuid.New(),
		Data: entity.ExtractedEntity{
			Name: "Aragorn", Type: "Character",
			Aliases: []string{"Elessar"}, Status: "king",
		},
	}, spy)

	if spy.out.IsNew {
		t.Error("IsNew = true, want false (la entidad ya existía)")
	}
	// ⚠️ previousStatus se captura ANTES del Merge. Si no, siempre daría "king".
	if spy.out.PreviousStatus != "alive" {
		t.Errorf("PreviousStatus = %q, want %q", spy.out.PreviousStatus, "alive")
	}
	if spy.out.Entity.Status != "king" {
		t.Errorf("Status = %q, want %q after merge", spy.out.Entity.Status, "king")
	}
	if !spy.out.Entity.HasAlias("Elessar") {
		t.Error("el merge perdió el alias nuevo")
	}
	if history.appended != 0 {
		t.Error("no se debe escribir historial al fusionar una entidad existente")
	}
}
```

> **Ese `PreviousStatus` capturado antes del `Merge` no es un detalle.** Es la información que el Slice 7 necesita para detectar contradicciones del tipo *"este personaje estaba muerto en el capítulo 3 y habla en el 5"*. Si lo leyeras después del merge, siempre te devolvería el estado nuevo y **nunca detectarías nada**. Un test lo clava para siempre.

```bash
go test ./... && git commit -am "feat(entity): rich Merge, resolve funnel, unit of work"
```

### El "por qué" del senior

> "Contá las dependencias del `ResolveInteractor`: **siete**. Base de datos, historial, vectores, grafo, LLM, transacciones y logs. Es el método más conectado que escribiste hasta ahora.
>
> Y sin embargo su test **no levanta nada**. Siete fakes, ninguno de más de cinco líneas. El más importante tiene **una**: `func (fakeUoW) Do(ctx, fn) error { return fn(ctx) }`.
>
> Ese fake de una línea es la prueba de que hiciste bien las cosas. **El interactor no sabe si hay una transacción.** Le dijiste 'hacé esto de forma atómica' y él confía. Si mañana esa atomicidad la da Postgres, o una cola, o nada, **el caso de uso no cambia**. Eso es lo que compraste con toda la ceremonia."

---

## Slice 7 — Contradiction: el agente ReAct como caso de uso

> *Cuando aparece un párrafo nuevo, quiero detectar contradicciones con lo ya escrito: las obvias por regla (un muerto que habla), y las sutiles preguntándole a un modelo que puede consultar la memoria del universo.*

Dos caminos, dos lecciones:

1. **Determinístico**: una regla pura de dominio. Sin LLM, sin red, sin costo.
2. **Semántico**: un **agente ReAct** — el modelo piensa, pide herramientas, vos las ejecutás, le devolvés el resultado, repite.

### 🔑 La decisión de arquitectura del slice

> **El agente no es Qwen. El agente es un patrón:** pensar → actuar → observar → repetir.
>
> - El **loop** (cuántas vueltas, cuándo parar, qué hacer si una tool falla) es **política de aplicación** → capa 2.
> - El **formato de function-calling de OpenAI** (cómo se serializan los `tool_calls` en el JSON) es **detalle de transporte** → capa 3, adapter.
>
> Mezclarlos es el error que comete el 90% de la gente que "integra un LLM". El día que cambies de modelo, el loop **no se toca**.

### Lo determinístico: una regla pura

📦 `internal/entity/contradiction.go`

```go
package entity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type Contradiction struct {
	ID          uuid.UUID
	UniverseID  uuid.UUID
	EntityID    *uuid.UUID
	Severity    string
	Description string
	EvidenceA   string
	EvidenceB   string
	Fingerprint string
	Status      string
}

// NewContradiction: una contradicción SIN fingerprint no existe.
// Al calcularlo en el constructor, es imposible olvidarse.
func NewContradiction(universeID uuid.UUID, entityID *uuid.UUID, severity, description, evidenceA, evidenceB string) *Contradiction {
	return &Contradiction{
		ID:          uuid.New(),
		UniverseID:  universeID,
		EntityID:    entityID,
		Severity:    severity,
		Description: description,
		EvidenceA:   evidenceA,
		EvidenceB:   evidenceB,
		Fingerprint: fingerprint(evidenceA, evidenceB, description),
		Status:      "open",
	}
}

// fingerprint identifica una contradicción por su CONTENIDO, no por su ID.
// Dos detecciones de lo mismo colisionan → dedup gratis.
func fingerprint(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func (c *Contradiction) SameAs(other *Contradiction) bool {
	return c.Fingerprint == other.Fingerprint
}

// ── La regla determinística ──────────────────────────────────────────────

var deadStatuses = map[string]struct{}{"deceased": {}, "dead": {}}

// CheckRevival: si una entidad estaba muerta y vuelve a estar activa,
// es una contradicción. Función PURA: sin ctx, sin gateways, sin I/O.
// Devuelve nil si no hay contradicción.
func CheckRevival(e *Entity, previousStatus string) *Contradiction {
	if previousStatus == "" {
		return nil // entidad nueva: no hay pasado con qué contradecirse
	}
	if _, wasDead := deadStatuses[strings.ToLower(previousStatus)]; !wasDead {
		return nil
	}
	if strings.EqualFold(e.Status, previousStatus) {
		return nil // sigue muerta, todo bien
	}

	id := e.ID
	return NewContradiction(e.UniverseID, &id, "high",
		fmt.Sprintf("%s was previously %s but now appears as %s", e.Name, previousStatus, e.Status),
		fmt.Sprintf("%s: %s", e.Name, previousStatus),
		fmt.Sprintf("%s: %s", e.Name, e.Status),
	)
}
```

📦 `internal/entity/contradiction_test.go`

```go
package entity

import (
	"testing"

	"github.com/google/uuid"
)

func TestCheckRevival(t *testing.T) {
	cases := []struct {
		name           string
		previousStatus string
		currentStatus  string
		wantDetection  bool
	}{
		{name: "muerto que revive", previousStatus: "deceased", currentStatus: "alive", wantDetection: true},
		{name: "dead → king", previousStatus: "dead", currentStatus: "king", wantDetection: true},
		{name: "sigue muerto", previousStatus: "deceased", currentStatus: "deceased"},
		{name: "vivo que muere (normal)", previousStatus: "alive", currentStatus: "deceased"},
		{name: "entidad nueva", previousStatus: "", currentStatus: "alive"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := &Entity{ID: uuid.New(), UniverseID: uuid.New(), Name: "Boromir", Status: tc.currentStatus}

			got := CheckRevival(e, tc.previousStatus)

			if tc.wantDetection && got == nil {
				t.Error("CheckRevival() = nil, want a contradiction")
			}
			if !tc.wantDetection && got != nil {
				t.Errorf("CheckRevival() = %+v, want nil", got)
			}
		})
	}
}

func TestContradiction_FingerprintIsContentBased(t *testing.T) {
	uid := uuid.New()
	a := NewContradiction(uid, nil, "high", "same desc", "evidence A", "evidence B")
	b := NewContradiction(uid, nil, "high", "same desc", "evidence A", "evidence B")

	if a.ID == b.ID {
		t.Fatal("los IDs deberían ser distintos")
	}
	if !a.SameAs(b) {
		t.Error("SameAs() = false; mismo contenido debe dar el mismo fingerprint (dedup)")
	}

	c := NewContradiction(uid, nil, "high", "otra desc", "evidence A", "evidence B")
	if a.SameAs(c) {
		t.Error("SameAs() = true; contenido distinto debe dar fingerprint distinto")
	}
}
```

> **`CheckRevival` no tiene `ctx`, no tiene gateways, no tiene errores.** Es la regla de negocio más valiosa de Quill y se testea con cinco líneas. **Cuando encuentres una función así, protegela**: es lo que hace que el producto sea el producto.

### El agente: capa 2, sin una línea de HTTP

📦 `internal/usecase/agent/agent.go`

```go
package agent

import (
	"context"
	"errors"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role
	Content    string
	ToolCallID string // solo cuando Role == RoleTool
}

// ToolCall: lo que el modelo PIDIÓ ejecutar. Ya parseado — nada de JSON crudo.
type ToolCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

type ToolSpec struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type Request struct {
	Messages []Message
	Tools    []ToolSpec
}

type Reply struct {
	Content   string
	ToolCalls []ToolCall
}

// LLM: el port. Lo implementa adapter/gateway/qwen.
// Fijate que NO habla de HTTP, ni de OpenAI, ni de `tool_calls`.
type LLM interface {
	Chat(ctx context.Context, req Request) (Reply, error)
}

// ToolExecutor: quien sabe ejecutar las herramientas del dominio.
type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall) (result string, err error)
}

var ErrMaxDepth = errors.New("agent: max reasoning depth reached")

// RunLoop es el patrón ReAct: razonar → actuar → observar → repetir.
//
// ESTA función es el corazón del agente, y no importa ni `net/http`
// ni `encoding/json`. Cambiá Qwen por cualquier otro modelo con
// function calling: esta función no se toca.
func RunLoop(ctx context.Context, llm LLM, exec ToolExecutor, req Request, maxDepth int) (Reply, error) {
	for depth := 0; depth < maxDepth; depth++ {
		reply, err := llm.Chat(ctx, req)
		if err != nil {
			return Reply{}, err
		}

		// El modelo dejó de pedir herramientas: terminó de razonar.
		if len(reply.ToolCalls) == 0 {
			return reply, nil
		}

		req.Messages = append(req.Messages, Message{Role: RoleAssistant, Content: reply.Content})

		for _, call := range reply.ToolCalls {
			result, err := exec.Execute(ctx, call)
			if err != nil {
				// Una tool que falla NO mata al agente: se le informa y sigue.
				// El modelo puede corregir el rumbo. Esto es resiliencia, no pereza.
				result = "error: " + err.Error()
			}
			req.Messages = append(req.Messages, Message{
				Role:       RoleTool,
				ToolCallID: call.ID,
				Content:    result,
			})
		}
	}
	return Reply{}, ErrMaxDepth
}
```

📦 `internal/usecase/agent/agent_test.go`

```go
package agent

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// scriptedLLM: un STUB con guion. Le decís qué contestar en cada turno.
type scriptedLLM struct {
	replies  []Reply
	turn     int
	lastReq  Request
}

func (s *scriptedLLM) Chat(_ context.Context, req Request) (Reply, error) {
	s.lastReq = req
	if s.turn >= len(s.replies) {
		return Reply{}, errors.New("scriptedLLM: se quedó sin respuestas")
	}
	r := s.replies[s.turn]
	s.turn++
	return r, nil
}

type spyExecutor struct {
	calls  []ToolCall
	result string
	err    error
}

func (s *spyExecutor) Execute(_ context.Context, call ToolCall) (string, error) {
	s.calls = append(s.calls, call)
	return s.result, s.err
}

func TestRunLoop_NoToolCalls_ReturnsImmediately(t *testing.T) {
	llm := &scriptedLLM{replies: []Reply{{Content: "no contradictions found"}}}
	exec := &spyExecutor{}

	got, err := RunLoop(context.Background(), llm, exec, Request{}, 5)
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if got.Content != "no contradictions found" {
		t.Errorf("Content = %q, want %q", got.Content, "no contradictions found")
	}
	if llm.turn != 1 {
		t.Errorf("llamó al LLM %d veces, want 1", llm.turn)
	}
	if len(exec.calls) != 0 {
		t.Errorf("ejecutó %d tools, want 0", len(exec.calls))
	}
}

func TestRunLoop_ExecutesToolAndFeedsResultBack(t *testing.T) {
	llm := &scriptedLLM{replies: []Reply{
		{ToolCalls: []ToolCall{{ID: "call_1", Name: "search_vector_memory", Arguments: map[string]any{"query": "Boromir"}}}},
		{Content: "Boromir died in chapter 3"},
	}}
	exec := &spyExecutor{result: "chapter 3: Boromir falls"}

	got, err := RunLoop(context.Background(), llm, exec, Request{}, 5)
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if got.Content != "Boromir died in chapter 3" {
		t.Errorf("Content = %q, want the final answer", got.Content)
	}
	if len(exec.calls) != 1 || exec.calls[0].Name != "search_vector_memory" {
		t.Fatalf("tools ejecutadas = %+v, want search_vector_memory", exec.calls)
	}

	// El resultado de la tool tiene que haber vuelto al modelo como mensaje `tool`.
	var fedBack bool
	for _, m := range llm.lastReq.Messages {
		if m.Role == RoleTool && m.ToolCallID == "call_1" && m.Content == "chapter 3: Boromir falls" {
			fedBack = true
		}
	}
	if !fedBack {
		t.Errorf("el resultado de la tool nunca volvió al modelo; mensajes = %+v", llm.lastReq.Messages)
	}
}

func TestRunLoop_ToolErrorIsReportedNotFatal(t *testing.T) {
	llm := &scriptedLLM{replies: []Reply{
		{ToolCalls: []ToolCall{{ID: "c1", Name: "query_entity_graph"}}},
		{Content: "recovered"},
	}}
	exec := &spyExecutor{err: errors.New("graph unavailable")}

	got, err := RunLoop(context.Background(), llm, exec, Request{}, 5)
	if err != nil {
		t.Fatalf("RunLoop should not fail when a tool fails: %v", err)
	}
	if got.Content != "recovered" {
		t.Errorf("Content = %q, want %q", got.Content, "recovered")
	}

	var sawError bool
	for _, m := range llm.lastReq.Messages {
		if m.Role == RoleTool && strings.HasPrefix(m.Content, "error:") {
			sawError = true
		}
	}
	if !sawError {
		t.Error("el error de la tool nunca se le informó al modelo")
	}
}

func TestRunLoop_StopsAtMaxDepth(t *testing.T) {
	// Un modelo que pide tools para siempre.
	forever := &scriptedLLM{replies: make([]Reply, 10)}
	for i := range forever.replies {
		forever.replies[i] = Reply{ToolCalls: []ToolCall{{ID: "c", Name: "t"}}}
	}

	_, err := RunLoop(context.Background(), forever, &spyExecutor{}, Request{}, 3)

	if !errors.Is(err, ErrMaxDepth) {
		t.Errorf("err = %v, want ErrMaxDepth", err)
	}
	if forever.turn != 3 {
		t.Errorf("llamó al LLM %d veces, want 3 (maxDepth)", forever.turn)
	}
}
```

> **Cuatro tests. Cero tokens gastados. Cero API key.** Probaste que el agente para cuando debe, que le devuelve los resultados al modelo, que sobrevive a una tool rota y que no entra en loop infinito. **Esas cuatro cosas son todo lo que puede salir mal en un agente**, y las verificás en 8 milisegundos.

### El adapter: acá sí vive el JSON de Qwen

📦 `internal/adapter/gateway/qwen/chat.go`

```go
package qwen

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/you/quill/internal/usecase/agent"
)

// ── El formato de cable de OpenAI/Qwen. Vive SOLO en este archivo. ────────

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []chatTool    `json:"tools,omitempty"`
}

type chatMessage struct {
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
}

type chatTool struct {
	Type     string `json:"type"` // siempre "function"
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"` // ¡JSON dentro de un string!
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

// Chat implementa agent.LLM. Traduce en ambas direcciones:
// tipos del dominio → JSON de Qwen → tipos del dominio.
func (c *Client) Chat(ctx context.Context, req agent.Request) (agent.Reply, error) {
	var resp chatResponse
	if err := c.post(ctx, "/chat/completions", toWire(c.chatModel, req), &resp); err != nil {
		return agent.Reply{}, err
	}
	if len(resp.Choices) == 0 {
		return agent.Reply{}, fmt.Errorf("qwen: empty chat response")
	}

	msg := resp.Choices[0].Message
	reply := agent.Reply{Content: msg.Content}

	for _, tc := range msg.ToolCalls {
		// Qwen manda los argumentos como un STRING que contiene JSON.
		// El agente no debería tener que saber eso. Lo desarmamos acá.
		args := map[string]any{}
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return agent.Reply{}, fmt.Errorf("decode tool arguments for %q: %w", tc.Function.Name, err)
			}
		}
		reply.ToolCalls = append(reply.ToolCalls, agent.ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}
	return reply, nil
}

func toWire(model string, req agent.Request) chatRequest { /* mapeo mecánico */ }

var _ agent.LLM = (*Client)(nil)
```

> **Mirá `Arguments string` en la respuesta.** Qwen te manda los argumentos de la función como **un string que adentro tiene JSON**. Es una fealdad del protocolo de OpenAI. Y el agente **jamás se entera**, porque el adapter lo desarma y le entrega un `map[string]any` limpio. **Eso es lo que hace un adapter: absorber la fealdad del mundo.**

Su test usa `httptest.NewServer` devolviendo un `tool_calls` real, y verifica que salga un `agent.ToolCall` con los argumentos ya parseados.

### Las herramientas: casos de uso, no funciones sueltas

📦 `internal/usecase/contradiction/tools.go`

```go
package contradiction

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/you/quill/internal/usecase/agent"
)

type VectorStore interface {
	FindSimilarParagraphs(ctx context.Context, universeID uuid.UUID, emb []float32, k int) ([]string, error)
}
type GraphStore interface {
	GetNeighbors(ctx context.Context, universeID uuid.UUID, entityID uuid.UUID) ([]string, error)
}
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
type EntityFinder interface {
	FindByName(ctx context.Context, universeID uuid.UUID, name string) (uuid.UUID, error)
}

// Executor implementa agent.ToolExecutor. Cada tool es una mini-orquestación
// sobre gateways. Nada de HTTP, nada de SQL.
type Executor struct {
	universeID uuid.UUID
	vectors    VectorStore
	graph      GraphStore
	entities   EntityFinder
	embedder   Embedder
}

var _ agent.ToolExecutor = (*Executor)(nil)

func (e *Executor) Execute(ctx context.Context, call agent.ToolCall) (string, error) {
	switch call.Name {
	case "search_vector_memory":
		query, _ := call.Arguments["query"].(string)
		if query == "" {
			return "", fmt.Errorf("search_vector_memory: missing 'query'")
		}
		emb, err := e.embedder.Embed(ctx, query)
		if err != nil {
			return "", err
		}
		paragraphs, err := e.vectors.FindSimilarParagraphs(ctx, e.universeID, emb, 5)
		if err != nil {
			return "", err
		}
		return strings.Join(paragraphs, "\n---\n"), nil

	case "query_entity_graph":
		name, _ := call.Arguments["entity_name"].(string)
		id, err := e.entities.FindByName(ctx, e.universeID, name)
		if err != nil {
			return "", fmt.Errorf("query_entity_graph: %w", err)
		}
		neighbors, err := e.graph.GetNeighbors(ctx, e.universeID, id)
		if err != nil {
			return "", err
		}
		return strings.Join(neighbors, "\n"), nil
	}

	return "", fmt.Errorf("unknown tool: %q", call.Name)
}
```

```bash
go test ./... && git commit -am "feat(contradiction): domain rule + fingerprint, ReAct agent loop, tool executor"
```

---

## Slice 8 — Memory / RRF: la función pura

> *Cuando el escritor pide contexto, quiero combinar cinco búsquedas distintas (vectorial, grafo, recencia, palabras clave, memorias consolidadas) en un solo ranking.*

Este slice tiene el **fan-out más grande** del sistema… y adentro esconde la joya: **Reciprocal Rank Fusion (RRF)**, que es **matemática pura**.

### La joya: `FuseRRF`

La idea de RRF: cada resultado suma `1 / (k + posición)` por cada lista donde aparece. Un ítem que aparece en varias listas gana, aunque no sea el primero de ninguna.

📦 `internal/usecase/memory/fusion.go`

```go
package memory

import "sort"

// rrfK amortigua la ventaja de los primeros puestos. 60 es el valor del paper original.
const rrfK = 60

// RankedList: el resultado ordenado de UNA estrategia de búsqueda.
type RankedList struct {
	Source string   // "vector", "graph", "keyword", "recency", "consolidated"
	IDs    []string // ordenados: índice 0 = mejor
}

type Fused struct {
	ID    string
	Score float64
}

// FuseRRF combina listas rankeadas.
//
// Mirá la firma: sin ctx, sin gateways, sin error. Es una FUNCIÓN PURA.
// Mismas entradas → mismas salidas, siempre. Es el algoritmo de recuperación
// de Quill, y no necesita nada del mundo para funcionar.
func FuseRRF(lists []RankedList) []Fused {
	scores := make(map[string]float64)

	for _, list := range lists {
		for rank, id := range list.IDs {
			scores[id] += 1.0 / float64(rrfK+rank+1)
		}
	}

	out := make([]Fused, 0, len(scores))
	for id, score := range scores {
		out = append(out, Fused{ID: id, Score: score})
	}

	// Orden estable: por score desc; ante empate, por ID, para que el
	// resultado sea determinístico (los maps de Go no tienen orden).
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	return out
}
```

📦 `internal/usecase/memory/fusion_test.go`

```go
package memory

import "testing"

func TestFuseRRF_SingleListPreservesOrder(t *testing.T) {
	got := FuseRRF([]RankedList{{Source: "vector", IDs: []string{"a", "b", "c"}}})

	want := []string{"a", "b", "c"}
	for i, w := range want {
		if got[i].ID != w {
			t.Errorf("posición %d = %q, want %q", i, got[i].ID, w)
		}
	}
}

func TestFuseRRF_ScoreFormula(t *testing.T) {
	got := FuseRRF([]RankedList{{Source: "vector", IDs: []string{"a"}}})

	want := 1.0 / float64(rrfK+1) // rank 0 → 1/(60+0+1)
	if got[0].Score != want {
		t.Errorf("Score = %v, want %v", got[0].Score, want)
	}
}

// LA propiedad que justifica usar RRF: el consenso vence a la posición.
func TestFuseRRF_ConsensusBeatsRank(t *testing.T) {
	got := FuseRRF([]RankedList{
		{Source: "vector", IDs: []string{"a", "b"}},
		{Source: "graph", IDs: []string{"b", "c"}},
	})

	// "a" es primero en una lista. "b" es segundo en una y primero en la otra.
	// b: 1/61 + 1/62 ≈ 0.03252   |   a: 1/61 ≈ 0.01639
	if got[0].ID != "b" {
		t.Errorf("ganador = %q, want %q (aparece en las dos listas)", got[0].ID, "b")
	}
}

func TestFuseRRF_IsDeterministic(t *testing.T) {
	lists := []RankedList{
		{Source: "vector", IDs: []string{"a", "b"}},
		{Source: "graph", IDs: []string{"b", "a"}}, // empate perfecto
	}

	first := FuseRRF(lists)
	for i := 0; i < 50; i++ {
		if got := FuseRRF(lists); got[0].ID != first[0].ID {
			t.Fatalf("resultado no determinístico: %q vs %q", got[0].ID, first[0].ID)
		}
	}
}

func TestFuseRRF_Empty(t *testing.T) {
	if got := FuseRRF(nil); len(got) != 0 {
		t.Errorf("FuseRRF(nil) = %v, want empty", got)
	}
}
```

```bash
$ go test ./internal/usecase/memory/ -run TestFuseRRF -v
=== RUN   TestFuseRRF_SingleListPreservesOrder
=== RUN   TestFuseRRF_ScoreFormula
=== RUN   TestFuseRRF_ConsensusBeatsRank
=== RUN   TestFuseRRF_IsDeterministic
=== RUN   TestFuseRRF_Empty
PASS
ok  github.com/you/quill/internal/usecase/memory  0.002s
```

> **El "por qué" del senior:** "Cinco tests. Dos milisegundos. Sin Docker, sin API key, sin `context.Background()`.
>
> Y sin embargo acabás de verificar el algoritmo que decide **qué recuerda Quill** — la feature que vende el producto. Fijate el test de determinismo: los `map` de Go **no tienen orden garantizado**. Si no hubieras desempatado por ID, ese test fallaría uno de cada tantos runs, y lo ibas a descubrir en producción, un martes.
>
> **Clean Architecture no crea esa pureza: la revela.** Empujás el I/O hacia los bordes hasta que en el centro queda solo la lógica. Cuando encontrás una función sin `ctx` ni `error`, encontraste el corazón de tu sistema."

### El interactor: los cinco pipelines

El caso de uso corre las cinco búsquedas **en paralelo** y las fusiona. La concurrencia es **orquestación de aplicación** → vive en la capa 2, no baja al gateway.

📦 `internal/usecase/memory/recall_interactor.go` *(el núcleo)*

```go
func (i *RecallInteractor) Execute(ctx context.Context, in RecallInput, out RecallOutputPort) {
	emb, err := i.embedder.Embed(ctx, in.Query)
	if err != nil {
		out.PresentError(err)
		return
	}

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		lists []RankedList
	)

	run := func(source string, fn func() ([]string, error)) {
		defer wg.Done()
		ids, err := fn()
		if err != nil {
			i.log.Warn("pipeline failed", "source", source, "err", err)
			return // un pipeline caído NO tumba el recall: degradamos
		}
		mu.Lock()
		lists = append(lists, RankedList{Source: source, IDs: ids})
		mu.Unlock()
	}

	wg.Add(4)
	go run("vector", func() ([]string, error) { return i.vectors.SimilarParagraphIDs(ctx, in.UniverseID, emb, in.K) })
	go run("keyword", func() ([]string, error) { return i.vectors.KeywordSearch(ctx, in.UniverseID, in.Query, in.K) })
	go run("recency", func() ([]string, error) { return i.entities.MostRelevantIDs(ctx, in.UniverseID, in.K) })
	go run("consolidated", func() ([]string, error) { return i.consolidated.SimilarIDs(ctx, in.UniverseID, emb, in.K) })
	wg.Wait()

	fused := FuseRRF(lists) // ← la función pura, testeada aparte

	out.PresentRecall(RecallOutput{Items: fused[:min(len(fused), in.K)]})
}
```

> **Fijate en `run`: si un pipeline falla, se loguea y se sigue.** Buscar en cinco lugares y que uno esté caído no debería dejar al escritor sin memoria. Eso se llama **degradación elegante**, y es una decisión de negocio que vive en el interactor — no un `try/catch` casual.

```bash
go test ./... && git commit -am "feat(memory): pure RRF fusion + parallel recall pipelines"
```

---

## Slice 9 — Analysis + WebSocket: el capstone

> *Cuando el escritor termina un párrafo, quiero analizarlo en segundo plano (extraer entidades, detectar contradicciones) y empujarle los resultados al editor en vivo, sin que tenga que refrescar.*

Acá convergen **todos** los slices anteriores. Y trae la lección más importante de toda la guía.

### El problema: una dependencia circular

Pensalo antes de leer la solución:

- El **hub de WebSocket** recibe el párrafo del navegador → necesita llamar al **análisis**.
- El **análisis** produce resultados → necesita empujarlos por el **hub**.

```
   ws.Hub  ⇄  AnalysisService
```

Go **prohíbe los ciclos de import entre paquetes**. No compila. El Quill original lo "resuelve" así:

```go
hub := ws.NewHub(authSvc, nil, memorySvc, qwenSvc)   // ← submitter = nil (!)
analysisSvc := services.NewAnalysisService(..., hub, ...)
hub.SetSubmitter(analysisSvc)                        // ← back-patch
```

El hub **nace roto** (con un `nil` adentro) y alguien tiene que acordarse de repararlo después. Funciona. Pero es un síntoma.

### 🔑 La lección: la dependencia circular casi nunca es real

> **Un ciclo es casi siempre una responsabilidad mal repartida.**

Preguntate: **¿el Hub hace una cosa o dos?**

Hace **dos**:
1. Es un **registro de conexiones**: "dado un userID, mandale este mensaje". → Es un **destino de salida**.
2. Es un **controller**: "llegó un mensaje del navegador, ejecutá el caso de uso". → Es una **entrada**.

**Son dos responsabilidades distintas metidas en un struct.** Separalas y el ciclo se evapora solo. Sin `nil`. Sin setters.

```
ANTES (ciclo, parcheado con nil + SetSubmitter):

    ws.Hub  ⇄  AnalysisService


DESPUÉS (dos flechas, las dos hacia adentro):

    ws.Controller ──────► analysis.SubmitInputPort      (entrada: afuera → adentro ✅)

    analysis ──► analysis.EventPublisher ◄────── ws.Hub  (salida: el hub implementa
                  (interfaz, capa 2)              (capa 3)  el port de adentro ✅)
```

`ws` importa `analysis`. **`analysis` NO importa `ws`.** El ciclo de compilación desapareció de verdad, no lo escondimos.

### Capa 2: el caso de uso define su Output Port

📦 `internal/usecase/analysis/analysis.go`

```go
package analysis

import (
	"context"

	"github.com/google/uuid"

	"github.com/you/quill/internal/entity"
)

type EventKind string

const (
	EventProgress           EventKind = "analysis_progress"
	EventEntityDiscovered   EventKind = "entity_discovered"
	EventContradictionFound EventKind = "contradiction_found"
	EventDone               EventKind = "analysis_done"
)

// Event es vocabulario del NEGOCIO. No sabe qué es un frame de WebSocket,
// ni un `type`/`payload`, ni JSON. Si mañana los eventos salen por
// Server-Sent Events o por una cola, este struct no cambia.
type Event struct {
	Kind          EventKind
	ChapterID     uuid.UUID
	Stage         string
	Entity        *entity.Entity
	Contradiction *entity.Contradiction
}

// EventPublisher es el OUTPUT PORT. Lo implementa el hub (capa 3).
type EventPublisher interface {
	Publish(ctx context.Context, userID uuid.UUID, ev Event) error
}

// SubmitInputPort es el INPUT PORT. Lo llama el controller de WS (capa 3).
type SubmitInputPort interface {
	Submit(ctx context.Context, in SubmitInput) error
}

type SubmitInput struct {
	WorkID     uuid.UUID
	ChapterID  uuid.UUID
	UniverseID uuid.UUID
	UserID     uuid.UUID
	Text       string
}
```

### El interactor: la cola secuencial por obra

📦 `internal/usecase/analysis/interactor.go`

```go
package analysis

import (
	"context"
	"sync"

	"github.com/google/uuid"

	"github.com/you/quill/internal/entity"
	"github.com/you/quill/internal/usecase/entityres"
)

type Extractor interface {
	ExtractEntities(ctx context.Context, text string) ([]entity.ExtractedEntity, error)
}

type Interactor struct {
	extractor Extractor
	resolve   entityres.ResolveInputPort
	events    EventPublisher
	log       Logger

	mu     sync.Mutex
	queues map[uuid.UUID]chan SubmitInput // una cola por WorkID
}

func New(extractor Extractor, resolve entityres.ResolveInputPort, events EventPublisher, log Logger) *Interactor {
	return &Interactor{
		extractor: extractor,
		resolve:   resolve,
		events:    events,
		log:       log,
		queues:    make(map[uuid.UUID]chan SubmitInput),
	}
}

var _ SubmitInputPort = (*Interactor)(nil)

// Submit encola. NO analiza: vuelve enseguida para que el editor no se trabe.
//
// Una goroutine por obra, con cola secuencial. ¿Por qué secuencial?
// Porque los párrafos de una misma obra deben analizarse EN ORDEN:
// si el capítulo 5 se procesa antes que el 3, la detección de
// contradicciones ve el pasado al revés. Es una regla de negocio.
func (i *Interactor) Submit(ctx context.Context, in SubmitInput) error {
	i.mu.Lock()
	queue, exists := i.queues[in.WorkID]
	if !exists {
		queue = make(chan SubmitInput, 100)
		i.queues[in.WorkID] = queue
		go i.worker(queue) // un worker nuevo para esta obra
	}
	i.mu.Unlock()

	select {
	case queue <- in:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (i *Interactor) worker(queue <-chan SubmitInput) {
	for job := range queue {
		i.process(context.Background(), job)
	}
}

func (i *Interactor) process(ctx context.Context, job SubmitInput) {
	i.publish(ctx, job, Event{Kind: EventProgress, Stage: "extracting_entities", ChapterID: job.ChapterID})

	extracted, err := i.extractor.ExtractEntities(ctx, job.Text)
	if err != nil {
		i.log.Warn("extract entities failed", "chapter", job.ChapterID, "err", err)
		return
	}

	for _, data := range extracted {
		c := &resolveCollector{}
		i.resolve.Execute(ctx, entityres.ResolveInput{UniverseID: job.UniverseID, Data: data}, c)
		if c.err != nil {
			i.log.Warn("resolve entity failed", "name", data.Name, "err", c.err)
			continue
		}

		if c.out.IsNew {
			i.publish(ctx, job, Event{Kind: EventEntityDiscovered, ChapterID: job.ChapterID, Entity: c.out.Entity})
		}

		// La regla pura del Slice 7, aplicada acá. Sin LLM, sin red.
		if contra := entity.CheckRevival(c.out.Entity, c.out.PreviousStatus); contra != nil {
			i.publish(ctx, job, Event{Kind: EventContradictionFound, ChapterID: job.ChapterID, Contradiction: contra})
		}
	}

	i.publish(ctx, job, Event{Kind: EventDone, ChapterID: job.ChapterID})
}

func (i *Interactor) publish(ctx context.Context, job SubmitInput, ev Event) {
	if err := i.events.Publish(ctx, job.UserID, ev); err != nil {
		i.log.Warn("publish event failed", "kind", ev.Kind, "err", err)
	}
}

// resolveCollector implementa entityres.ResolveOutputPort.
//
// ⚠️ Cuando un caso de uso llama a OTRO caso de uso, también recibe el
// resultado por el Output Port. El "presenter" no es solo para HTTP:
// es la forma UNIVERSAL en que un resultado sale de un interactor.
type resolveCollector struct {
	out *entityres.ResolveOutput
	err error
}

func (c *resolveCollector) PresentResolved(out entityres.ResolveOutput) { c.out = &out }
func (c *resolveCollector) PresentError(err error)                      { c.err = err }

type Logger interface{ Warn(msg string, args ...any) }
```

### El test: un spy publisher lo prueba todo

📦 `internal/usecase/analysis/interactor_test.go`

```go
package analysis

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/you/quill/internal/entity"
	"github.com/you/quill/internal/usecase/entityres"
)

type spyPublisher struct {
	mu     sync.Mutex
	events []Event
	done   chan struct{}
}

func (s *spyPublisher) Publish(_ context.Context, _ uuid.UUID, ev Event) error {
	s.mu.Lock()
	s.events = append(s.events, ev)
	s.mu.Unlock()
	if ev.Kind == EventDone {
		close(s.done)
	}
	return nil
}

func (s *spyPublisher) kinds() []EventKind {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]EventKind, len(s.events))
	for i, e := range s.events {
		out[i] = e.Kind
	}
	return out
}

type stubExtractor struct{ entities []entity.ExtractedEntity }

func (s stubExtractor) ExtractEntities(context.Context, string) ([]entity.ExtractedEntity, error) {
	return s.entities, nil
}

// stubResolve simula el caso de uso del Slice 6.
type stubResolve struct {
	result entityres.ResolveOutput
}

func (s stubResolve) Execute(_ context.Context, _ entityres.ResolveInput, out entityres.ResolveOutputPort) {
	out.PresentResolved(s.result)
}

type nopLogger struct{}

func (nopLogger) Warn(string, ...any) {}

func TestAnalysis_PublishesDiscoveryAndContradiction(t *testing.T) {
	revived := &entity.Entity{ID: uuid.New(), UniverseID: uuid.New(), Name: "Boromir", Status: "alive"}

	pub := &spyPublisher{done: make(chan struct{})}
	interactor := New(
		stubExtractor{entities: []entity.ExtractedEntity{{Name: "Boromir", Type: "Character"}}},
		stubResolve{result: entityres.ResolveOutput{
			Entity: revived, PreviousStatus: "deceased", IsNew: true,
		}},
		pub, nopLogger{},
	)

	if err := interactor.Submit(context.Background(), SubmitInput{
		WorkID: uuid.New(), ChapterID: uuid.New(), UniverseID: uuid.New(), UserID: uuid.New(),
		Text: "Boromir stood up and spoke.",
	}); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	// El análisis es asíncrono: esperamos el evento final (con timeout).
	select {
	case <-pub.done:
	case <-time.After(2 * time.Second):
		t.Fatal("el análisis nunca terminó")
	}

	want := []EventKind{EventProgress, EventEntityDiscovered, EventContradictionFound, EventDone}
	got := pub.kinds()

	if len(got) != len(want) {
		t.Fatalf("eventos = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("evento[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
```

> **Ese test verifica todo el pipeline** —extracción, resolución, detección de contradicción, publicación de cuatro eventos en orden— **sin WebSocket, sin LLM, sin Postgres.** El `spyPublisher` te deja assertar *qué eventos emitió el sistema*. Es exactamente lo que le importa al usuario.
>
> Fijate también el `select` con `time.After`: en tests asíncronos **nunca** uses `time.Sleep`. Esperás la señal, con timeout. Si el código se cuelga, el test falla en 2 segundos en vez de bloquear el CI para siempre.

### Capa 3: el Hub (registro) y el Controller (entrada)

📦 `internal/adapter/ws/hub.go` — **solo maneja conexiones**

```go
package ws

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"

	"github.com/you/quill/internal/usecase/analysis"
)

// wireMessage es el formato del cable. Detalle de transporte, vive acá.
type wireMessage struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type Hub struct {
	mu    sync.RWMutex
	conns map[uuid.UUID]*websocket.Conn
}

func NewHub() *Hub { return &Hub{conns: make(map[uuid.UUID]*websocket.Conn)} }

// El hub NO conoce el caso de uso. Solo implementa su Output Port.
var _ analysis.EventPublisher = (*Hub)(nil)

func (h *Hub) Register(userID uuid.UUID, c *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[userID] = c
}

func (h *Hub) Unregister(userID uuid.UUID) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.conns, userID)
}

// Publish traduce un Event del dominio a un mensaje de cable.
// Esta traducción es EXACTAMENTE lo que hace un Presenter.
func (h *Hub) Publish(_ context.Context, userID uuid.UUID, ev analysis.Event) error {
	h.mu.RLock()
	conn, ok := h.conns[userID]
	h.mu.RUnlock()
	if !ok {
		return nil // el usuario cerró el editor: no es un error
	}

	msg := wireMessage{Type: string(ev.Kind), Payload: toPayload(ev)}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}
```

📦 `internal/adapter/ws/controller.go` — **solo recibe mensajes**

```go
package ws

import (
	"context"

	"github.com/gofiber/contrib/websocket"
	"github.com/google/uuid"

	"github.com/you/quill/internal/usecase/analysis"
)

// Controller es un controller igual que uno de HTTP: traduce lo que llega
// del cable a un Request Model y llama al Input Port.
type Controller struct {
	hub    *Hub
	submit analysis.SubmitInputPort
}

func NewController(hub *Hub, submit analysis.SubmitInputPort) *Controller {
	return &Controller{hub: hub, submit: submit}
}

func (c *Controller) Handle(conn *websocket.Conn) {
	userID, ok := conn.Locals("userID").(uuid.UUID)
	if !ok {
		return
	}

	c.hub.Register(userID, conn)
	defer c.hub.Unregister(userID)

	for {
		var msg struct {
			Type    string `json:"type"`
			Payload struct {
				WorkID     uuid.UUID `json:"work_id"`
				ChapterID  uuid.UUID `json:"chapter_id"`
				UniverseID uuid.UUID `json:"universe_id"`
				Text       string    `json:"text"`
			} `json:"payload"`
		}
		if err := conn.ReadJSON(&msg); err != nil {
			return // conexión cerrada
		}

		if msg.Type == "paragraph_submit" {
			_ = c.submit.Submit(context.Background(), analysis.SubmitInput{
				WorkID:     msg.Payload.WorkID,
				ChapterID:  msg.Payload.ChapterID,
				UniverseID: msg.Payload.UniverseID,
				UserID:     userID,
				Text:       msg.Payload.Text,
			})
		}
	}
}
```

### El cableado: sin `nil`, sin setters

📦 `cmd/api/main.go` *(fragmento)*

```go
// 1. El hub no conoce a nadie. Solo registra conexiones.
hub := ws.NewHub()

// 2. El análisis recibe el hub COMO EventPublisher (una interfaz que él definió).
analysisInteractor := analysis.New(qwenClient, resolveInteractor, hub, logger)

// 3. El controller conoce a los dos. Es su trabajo.
wsController := ws.NewController(hub, analysisInteractor)

app.Get("/api/v1/ws", websocket.New(wsController.Handle))
```

Compará con el original:

```go
hub := ws.NewHub(authSvc, nil, memorySvc, qwenSvc)   // ❌ nace roto
analysisSvc := services.NewAnalysisService(..., hub, ...)
hub.SetSubmitter(analysisSvc)                        // ❌ hay que acordarse
```

> **El "por qué" del senior:** "El `nil` + `SetSubmitter` no era un bug: era un **síntoma**. El código te estaba gritando que el Hub tenía dos trabajos.
>
> Cuando veas un `SetX` mutable puesto 'para romper un ciclo de inicialización', **frená**. No busques un truco. Preguntate qué responsabilidad está de más. Casi siempre hay un objeto haciendo dos cosas, y cuando lo partís en dos, **el ciclo se evapora porque nunca existió**.
>
> Esa es la última cosa que Clean Architecture te da, y la más difícil de ver: **te muestra los problemas de diseño obligándote a dibujar las flechas.** Si no podés dibujarlas hacia adentro, no es la arquitectura la que está mal. Es tu modelo."

```bash
go test ./... && git commit -am "feat(analysis): per-work queue, event publisher port, dissolve ws cycle"
```

---

## Cierre — Lo que construiste, y dónde Go pelea con Clean

### El árbol final

```
quill-remake/
├── cmd/api/main.go                      # ÚNICO archivo que importa pgx, fiber, qwen y ws
└── internal/
    ├── entity/                          # ── Capa 1: cero imports del proyecto
    │   user.go universe.go work.go chapter.go entity.go contradiction.go errors.go
    │
    ├── usecase/                         # ── Capa 2: interactors + los ports que definen
    │   ├── auth/       model port gateway errors register_interactor login_interactor
    │   ├── universe/   create_interactor list_interactor
    │   ├── work/ chapter/
    │   ├── entityres/  resolve_interactor  (UnitOfWork, VectorStore, GraphStore, Embedder)
    │   ├── agent/      agent.go            (RunLoop, LLM, ToolExecutor)
    │   ├── contradiction/ tools.go
    │   ├── memory/     fusion.go           (FuseRRF — función pura)
    │   └── analysis/   interactor.go       (EventPublisher, SubmitInputPort)
    │
    ├── adapter/                         # ── Capa 3
    │   ├── controller/ auth universe work middleware
    │   ├── presenter/  auth universe       (ViewModels + mapeo de errores → HTTP)
    │   ├── gateway/
    │   │   ├── postgres/ user universe entity vector graph unit_of_work
    │   │   ├── qwen/     client.go chat.go
    │   │   └── crypto/ token/
    │   └── ws/         hub.go controller.go
    │
    ├── infrastructure/                  # ── Capa 4
    │   ├── config/ database/ clock/
    └── testutil/db.go
```

### Contá las flechas

| Desde | Hacia | ¿Legal? |
|---|---|---|
| `entity` | *nada del proyecto* | ✅ el centro no depende de nadie |
| `usecase/*` | `entity` | ✅ hacia adentro |
| `adapter/*` | `usecase/*`, `entity` | ✅ hacia adentro |
| `infrastructure/*` | *nada del dominio* | ✅ |
| `cmd/api` | todo | ✅ es el borde externo |
| `usecase/analysis` | `adapter/ws` | ❌ **nunca ocurre** |

**Ni una flecha sale del centro.** La Regla de Dependencia se sostiene, y el compilador de Go te la hace cumplir.

### Los números que importan

De todos los tests que escribiste:

| Necesitan infraestructura | No necesitan nada |
|---|---|
| Los gateways de Postgres (SQL) | Entidades, interactors, presenters, controllers, el agente, `FuseRRF` |
| ~6 tests | ~40 tests |

`go test ./...` sin `TEST_DATABASE_URL` corre **entero, verde, en menos de un segundo**. Eso es la recompensa.

### 🔬 Clean vs Hexagonal, ahora que lo construiste

Volvé a la Parte 1 y releé la tabla. Ahora vas a ver lo que antes era abstracto:

- **Escribiste `Interactor`, `Input Port`, `Output Port`, `Presenter`, `ViewModel`, `Request Model`, `Response Model`.** Ninguno de esos artefactos existe en Hexagonal.
- **Tus interactors nunca dicen `return output, nil`.** Dicen `out.PresentRegistered(...)`. Esa sola firma es la línea divisoria.
- **Separaste `entity` de `usecase`.** Hexagonal no te lo pide: para Cockburn, "la aplicación" es un bloque.

Y ahora podés detectar el fraude: si un repo dice "Clean Architecture in Go", andá a un caso de uso y mirá su firma. Si retorna, es Hexagonal. **Puede estar perfecto** — pero llamalo por su nombre.

### Dónde Go pelea con Clean canónico (la parte honesta)

Te lo debo, porque nadie te lo dice:

**1. El Presenter es incómodo en Go.** Go quiere `return`. El push a un Output Port obliga a un presenter con estado, un `Status()`/`Body()`, y un controller que lo lee después. Es ceremonia real.

**2. Se nota más cuando un caso de uso llama a otro.** En el Slice 9 tuviste que escribir un `resolveCollector` solo para leer el resultado de `entityres`. Un `return` habría sido una línea.

**3. Qué haría yo en producción.** Colapsaría el Presenter: el interactor devuelve el Response Model, el controller lo mapea a ViewModel y a status HTTP. Se pierde la posibilidad de tener presenters intercambiables (que en la práctica casi nadie usa), y se gana legibilidad.

**Pero fijate lo que NO colapsaría, jamás:**

- La separación `entity` / `usecase`.
- Las interfaces de gateway **definidas en el paquete del caso de uso**.
- Que el interactor no importe `pgx`, `fiber` ni `qwen`.
- Que las reglas de negocio (`Merge`, `CheckRevival`, `FuseRRF`, `NewUniverse`) sean funciones puras.

**Esas cuatro cosas son el 95% del valor y el 20% del costo.** El Presenter canónico es el 5% restante.

> **Y por eso hicimos el remake canónico.** No para que lo uses siempre. **Para que sepas exactamente qué estás tirando cuando lo tires.** Colapsar un patrón que entendiste es ingeniería. Colapsar uno que nunca construiste es adivinar.

### Checklist final

- [ ] `go test ./...` verde sin `TEST_DATABASE_URL`.
- [ ] `grep -r "jackc/pgx" internal/usecase/` → **cero resultados**.
- [ ] `grep -r "gofiber" internal/usecase/ internal/entity/` → **cero resultados**.
- [ ] `grep -r "quill/internal" internal/entity/` → **cero resultados**.
- [ ] Ningún interactor tiene `return out, nil`; todos empujan al Output Port.
- [ ] Ninguna interfaz de gateway vive en un paquete `ports/` compartido.
- [ ] Podés explicarle a alguien, sin mirar, por qué el ciclo `ws ⇄ analysis` no existía.

---

## Apéndice A — Solución del ejercicio (Slice 4)

📦 `internal/entity/work.go`

```go
package entity

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

var allowedWorkTypes = map[string]struct{}{
	"novel": {}, "short-story": {}, "screenplay": {},
	"poetry": {}, "essay": {}, "article": {}, "graphic-novel": {},
}

type Work struct {
	ID         uuid.UUID
	UniverseID uuid.UUID
	Title      string
	Type       string
	OrderIndex int
	Synopsis   string
	Status     string
}

func NewWork(universeID uuid.UUID, title, workType, synopsis string) (*Work, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrWorkTitleRequired
	}
	if _, ok := allowedWorkTypes[workType]; !ok {
		return nil, fmt.Errorf("%w: %q", ErrInvalidWorkType, workType)
	}

	return &Work{
		ID:         uuid.New(),
		UniverseID: universeID,
		Title:      title,
		Type:       workType,
		OrderIndex: 1,             // default del schema
		Synopsis:   strings.TrimSpace(synopsis),
		Status:     "in_progress", // default del schema
	}, nil
}
```

📦 `internal/entity/chapter.go`

```go
package entity

import (
	"strings"

	"github.com/google/uuid"
)

type Chapter struct {
	ID         uuid.UUID
	WorkID     uuid.UUID
	Title      string
	OrderIndex int
	Content    string // HTML del editor
	RawText    string // texto plano, para analizar
	WordCount  int
	Status     string
}

func NewChapter(workID uuid.UUID, title string) (*Chapter, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return nil, ErrChapterTitleRequired
	}
	return &Chapter{
		ID:         uuid.New(),
		WorkID:     workID,
		Title:      title,
		OrderIndex: 1,
		Status:     "draft",
	}, nil
}

// UpdateContent: el conteo de palabras SIEMPRE queda consistente con el texto.
// No existe forma de setear RawText sin recalcular WordCount. Esa es la invariante.
func (c *Chapter) UpdateContent(content, rawText string) {
	c.Content = content
	c.RawText = rawText
	c.WordCount = WordCount(rawText)
	c.Status = "draft"
}

// WordCount: ¿te acordás de la Parte 2? Ahora ya sabés dónde vivía.
// Es una regla de negocio sobre capítulos, no una utilidad de strings.
func WordCount(s string) int { return len(strings.Fields(s)) }
```

Agregá a `errors.go`:

```go
var (
	ErrWorkTitleRequired    = errors.New("work title is required")
	ErrInvalidWorkType      = errors.New("invalid work type")
	ErrChapterTitleRequired = errors.New("chapter title is required")
)
```

**Sobre la pista 3** (el interactor de `Create` para Work): sí, `work.Repository` necesita poder verificar que el universo existe y es del usuario. Pero **no le pidas el universo entero a `universe.Repository`** — eso acopla dos casos de uso. Definí en `usecase/work` una interfaz mínima:

```go
// usecase/work/gateway.go
type UniverseChecker interface {
	ExistsAndBelongsTo(ctx context.Context, universeID, userID uuid.UUID) (bool, error)
}
```

Un gateway de Postgres la implementa con un solo `SELECT EXISTS(... WHERE id = $1 AND user_id = $2)`. **La interfaz la define quien la consume, y pide lo mínimo.** Eso se llama *Interface Segregation*, la "I" de SOLID, y es la misma idea que venimos usando todo el tiempo.

---

## Y ahora qué

Construiste un backend con Clean Architecture canónico, guiado por tests, sin escribir una línea de código antes de un test que la exigiera.

Lo que te queda, si querés seguir:

1. **Los slices que no hicimos**: Timeline, PlotHole, Ingestion, Demo, Relevance/Consolidation. Ya tenés el molde. Son el Slice 3 con otros campos.
2. **El frontend**, con la misma disciplina.
3. **Colapsar el Presenter** en una rama aparte y comparar los dos árboles. Ahí vas a *sentir* el tradeoff en vez de leerlo.

> **La última del senior:** "Vas a querer aplicar esto a todo. No lo hagas. Clean Architecture cuesta archivos, indirección y un impuesto de '¿dónde va esto?'. Se paga cuando el proyecto vive años, cuando el equipo crece, cuando hay varios adapters, cuando el testing es crítico. Un script de 200 líneas no la necesita, y meterla ahí es tan tonto como no usarla nunca.
>
> **Aprendiste a construirla para aprender a decidir cuándo.** Eso es lo que hace un arquitecto: no aplicar patrones, sino saber cuál, cuándo, y sobre todo **por qué no**. Nos vemos del otro lado."
