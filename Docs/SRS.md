# Quill — Software Requirements Specification (SRS)

> **Versión:** 1.0  
> **Última actualización:** 2026-06-28  
> **Documento complementario a:** [PRD.md](file:///c:/Workspace/Hackathon-QwenCloude/Docs/PRD.md)  
> **Plan de implementación:** [implementation_plan.md](file:///C:/Users/Daikyri/.gemini/antigravity-ide/brain/fdff4545-3ac9-4bf4-b8ec-ef62a7d36419/implementation_plan.md)

---

## 1. Introducción

### 1.1 Propósito

Este documento especifica los requisitos de software de **Quill**, un IDE para escritores con memoria persistente. Está diseñado para que cualquier desarrollador pueda entender exactamente qué construir — las interfaces, los algoritmos, las validaciones y los flujos de datos — y solo tenga que traducir las especificaciones a código Go/React.

### 1.2 Alcance del Sistema

Quill es una aplicación web compuesta por:

- **Frontend:** SPA (Single Page Application) en React + Vite
- **Backend:** API REST + WebSocket en Go (Fiber v2.52.x)
- **Base de datos:** PostgreSQL 16 (vía Apache AGE) con extensiones pgvector (búsqueda vectorial) y Apache AGE (grafos)
- **AI:** Qwen Cloud API (multi-modelo: Qwen-Max, Qwen-Turbo, Qwen-Embedding)

### 1.3 Convenciones del Documento

- `[REQ-XXX]` — Requisito funcional numerado
- `[NF-XXX]` — Requisito no funcional numerado
- Pseudocódigo en bloques con sintaxis tipo Python para legibilidad
- JSON Schemas usan el estándar JSON Schema Draft 7
- Diagramas en Mermaid

### 1.4 Referencias

| Documento | Ubicación |
|---|---|
| PRD | [Docs/PRD.md](file:///c:/Workspace/Hackathon-QwenCloude/Docs/PRD.md) |
| Plan de Implementación | [implementation_plan.md](file:///C:/Users/Daikyri/.gemini/antigravity-ide/brain/fdff4545-3ac9-4bf4-b8ec-ef62a7d36419/implementation_plan.md) |
| Qwen Cloud API Docs | https://www.qwencloud.com/docs |
| Apache AGE Docs | https://age.apache.org/docs |
| pgvector Docs | https://github.com/pgvector/pgvector |

---

## 2. Arquitectura General

### 2.1 Diagrama de Componentes

```mermaid
graph TB
    subgraph "Client Layer"
        Browser["Browser (React SPA)"]
    end

    subgraph "API Layer (Go Fiber)"
        Router["Fiber Router"]
        AuthMW["Auth Middleware (JWT)"]
        CORSMW["CORS Middleware"]
        
        subgraph "Handlers"
            AuthH["AuthHandler"]
            UnivH["UniverseHandler"]
            WorkH["WorkHandler"]
            ChapH["ChapterHandler"]
            EntH["EntityHandler"]
            MemH["MemoryHandler"]
            IngH["IngestionHandler"]
            WSH["WebSocketHandler"]
        end
    end

    subgraph "Service Layer"
        AuthS["AuthService"]
        UnivS["UniverseService"]
        WorkS["WorkService"]
        ChapS["ChapterService"]
        AnalysisS["AnalysisService"]
        EntityS["EntityService"]
        ContradS["ContradictionService"]
        RelevS["RelevanceService"]
        TimelineS["TimelineService"]
        PlotHoleS["PlotHoleService"]
        IngS["IngestionService"]
        QwenS["QwenService"]
        MemS["MemoryService"]
    end

    subgraph "Repository Layer"
        UserR["UserRepo"]
        UnivR["UniverseRepo"]
        WorkR["WorkRepo"]
        ChapR["ChapterRepo"]
        EntityR["EntityRepo"]
        VectorR["VectorRepo"]
        GraphR["GraphRepo"]
        MemR["MemoryRepo"]
    end

    subgraph "Data Layer"
        PG["PostgreSQL 16"]
        PGV["pgvector"]
        AGE["Apache AGE"]
    end

    subgraph "External"
        QwenAPI["Qwen Cloud API"]
    end

    Browser -->|HTTP/WS| Router
    Router --> AuthMW --> CORSMW
    CORSMW --> AuthH & UnivH & WorkH & ChapH & EntH & MemH & IngH & WSH

    AuthH --> AuthS
    UnivH --> UnivS
    WorkH --> WorkS
    ChapH --> ChapS
    EntH --> EntityS
    MemH --> MemS
    IngH --> IngS
    WSH --> AnalysisS

    AnalysisS --> EntityS & ContradS & RelevS & TimelineS & PlotHoleS
    EntityS --> QwenS
    ContradS --> QwenS
    IngS --> QwenS

    AuthS --> UserR
    UnivS --> UnivR
    WorkS --> WorkR
    ChapS --> ChapR
    EntityS --> EntityR & VectorR & GraphR
    ContradS --> EntityR & VectorR & GraphR
    RelevS --> EntityR & GraphR
    MemS --> EntityR & VectorR & GraphR

    UserR & UnivR & WorkR & ChapR & EntityR --> PG
    VectorR --> PGV
    GraphR --> AGE
    QwenS --> QwenAPI
```

### 2.2 Patrones Arquitectónicos

| Patrón | Aplicación |
|---|---|
| **Repository Pattern** | Aislamiento de la capa de datos. Cada repo encapsula queries SQL/Cypher |
| **Service Layer** | Toda la lógica de negocio en servicios. Handlers solo validan input y llaman servicios |
| **Unit-of-Work (pgx.Tx)** | Las escrituras que tocan entities + grafo AGE + embeddings pgvector se envuelven en una sola transacción `pgx.Tx`. El servicio abre la Tx y la pasa a los repos. Si falla cualquier escritura, rollback atómico. Posible porque AGE y pgvector son extensiones de la misma instancia PostgreSQL. |
| **Dependency Injection** | Los servicios reciben sus dependencias (repos, otros servicios) por constructor |
| **Event-Driven (WebSocket)** | Los resultados de análisis se publican como eventos WS, el frontend reacciona |
| **Pipeline Pattern** | La ingesta de documentos es un pipeline de pasos secuenciales con worker pool concurrente |

---

## 3. Modelo de Datos

### 3.1 Diagrama Entidad-Relación

```mermaid
erDiagram
    USERS ||--o{ UNIVERSES : "creates"
    UNIVERSES ||--o{ WORKS : "contains"
    UNIVERSES ||--o{ ENTITIES : "contains"
    UNIVERSES ||--o{ CONTRADICTIONS : "has"
    UNIVERSES ||--o{ TIMELINE_EVENTS : "has"
    UNIVERSES ||--o{ PLOT_HOLES : "has"
    WORKS ||--o{ CHAPTERS : "contains"
    WORKS ||--o{ INGESTION_JOBS : "receives"
    CHAPTERS ||--o{ ENTITY_MENTIONS : "contains"
    CHAPTERS ||--o{ PARAGRAPH_EMBEDDINGS : "contains"
    ENTITIES ||--o{ ENTITY_MENTIONS : "mentioned_in"
    ENTITIES ||--o{ ENTITY_EMBEDDINGS : "has"
    ENTITIES ||--o{ CONTRADICTIONS : "involved_in"
    
    USERS {
        uuid id PK
        varchar email UK
        varchar password_hash
        varchar display_name
        timestamp created_at
        timestamp updated_at
    }
    
    UNIVERSES {
        uuid id PK
        uuid user_id FK
        varchar name
        text description
        varchar genre
        varchar format
        varchar session_id
        boolean is_demo_template
        timestamp created_at
        timestamp updated_at
    }
    
    WORKS {
        uuid id PK
        uuid universe_id FK
        varchar title
        varchar type
        int order_index
        text synopsis
        varchar status
        timestamp created_at
        timestamp updated_at
    }
    
    CHAPTERS {
        uuid id PK
        uuid work_id FK
        varchar title
        int order_index
        text content
        text raw_text
        int word_count
        varchar status
        timestamp analyzed_at
        timestamp created_at
        timestamp updated_at
    }
    
    ENTITIES {
        uuid id PK
        uuid universe_id FK
        varchar type
        varchar name
        text_array aliases
        text description
        jsonb properties
        varchar status
        float relevance_score
        uuid last_mentioned_chapter_id
        timestamp last_mentioned_at
        timestamp created_at
        timestamp updated_at
    }
    
    ENTITY_MENTIONS {
        uuid id PK
        uuid entity_id FK
        uuid chapter_id FK
        int paragraph_index
        varchar paragraph_node_id
        text context_snippet
        varchar mention_type
        timestamp created_at
    }
    
    CONTRADICTIONS {
        uuid id PK
        uuid universe_id FK
        uuid entity_id FK
        varchar severity
        text description
        text suggestion
        text evidence_a
        uuid evidence_a_chapter_id FK
        text evidence_b
        uuid evidence_b_chapter_id FK
        varchar fingerprint UK
        varchar status
        timestamp resolved_at
        timestamp created_at
    }
    
    TIMELINE_EVENTS {
        uuid id PK
        uuid universe_id FK
        uuid event_entity_id FK
        varchar title
        text description
        float timeline_position
        varchar timeline_label
        uuid chapter_id FK
        uuid_array participants
        timestamp created_at
    }
    
    PLOT_HOLES {
        uuid id PK
        uuid universe_id FK
        varchar title
        text description
        uuid_array related_entity_ids
        uuid first_mentioned_chapter_id FK
        varchar status
        timestamp created_at
    }
    
    PARAGRAPH_EMBEDDINGS {
        uuid id PK
        uuid chapter_id FK
        int paragraph_index
        varchar paragraph_node_id
        text content
        vector embedding
        timestamp created_at
    }
    
    ENTITY_EMBEDDINGS {
        uuid id PK
        uuid entity_id FK
        vector description_embedding
        timestamp created_at
        timestamp updated_at
    }
    
    INGESTION_JOBS {
        uuid id PK
        uuid universe_id FK
        uuid work_id FK
        varchar filename
        varchar file_type
        varchar status
        int total_chapters_detected
        int chapters_processed
        int entities_extracted
        text error_message
        timestamp started_at
        timestamp completed_at
        timestamp created_at
    }
```

### 3.2 Knowledge Graph Schema (Apache AGE)

Cada universo tiene su propio grafo nombrado `universe_{uuid_sin_guiones}` (los guiones del UUID se reemplazan por underscores para compatibilidad con AGE, ej: `universe_550e8400e29b41d4a716446655440000`).

> [!IMPORTANT]
> **Configuración de pool `pgx`:** Cada conexión del pool debe ejecutar `LOAD 'age'; SET search_path = ag_catalog, "$user", public;` al crearse. Esto se hace via el hook `AfterConnect` de `pgxpool.Config`.
>
> **IDs del grafo:** Los nodos de AGE tienen un `id` numérico interno asignado por el motor. La aplicación **nunca debe usar este ID** para buscar o referenciar nodos. En su lugar, se usa la propiedad `entity_id` (UUID de la tabla `entities`) como identificador estable.

**Tipos de Nodos:**

| Tipo de Nodo | Propiedades | Descripción |
|---|---|---|
| `Character` | `entity_id`, `name`, `status`, `current_location`, `role`, `relevance_score` | Personaje de la historia |
| `Place` | `entity_id`, `name`, `type`, `relevance_score` | Ubicación geográfica o lugar |
| `Event` | `entity_id`, `name`, `timeline_position`, `relevance_score` | Evento narrativo |
| `Faction` | `entity_id`, `name`, `ideology`, `relevance_score` | Organización o grupo |
| `WorldRule` | `entity_id`, `name`, `category`, `scope` | Regla del universo (física, mágica, social) |
| `PlotArc` | `entity_id`, `name`, `status` | Hilo argumental |

**Tipos de Relaciones (Edges):**

| Relación | Desde → Hasta | Propiedades | Descripción |
|---|---|---|---|
| `ALLY_OF` | Character → Character | `since_chapter` | Alianza entre personajes |
| `ENEMY_OF` | Character → Character | `since_chapter`, `reason` | Enemistad |
| `PARENT_OF` | Character → Character | — | Relación parental |
| `CHILD_OF` | Character → Character | — | Inversa de PARENT_OF |
| `LOVER_OF` | Character → Character | `status` (active/past) | Relación romántica |
| `MENTOR_OF` | Character → Character | — | Mentoría |
| `MEMBER_OF` | Character → Faction | `role`, `since_chapter` | Pertenencia a facción |
| `LOCATED_AT` | Character → Place | `since_chapter` | Ubicación actual |
| `PARTICIPATED_IN` | Character → Event | `role_in_event` | Participación en evento |
| `OCCURRED_AT` | Event → Place | — | Dónde ocurrió un evento |
| `CAUSED` | Event → Event | — | Causalidad entre eventos |
| `INVOLVES` | Event → PlotArc | — | Evento avanza un arco |
| `CONTROLS` | Faction → Place | `since_chapter` | Control territorial |
| `ALLIED_WITH` | Faction → Faction | `since_chapter` | Alianza entre facciones |
| `AT_WAR_WITH` | Faction → Faction | `since_chapter`, `reason` | Conflicto |
| `APPLIES_TO` | WorldRule → Place | — | Regla aplica en lugar |
| `AFFECTS` | WorldRule → Character | — | Regla afecta a personaje |

### 3.3 Propiedades JSONB por Tipo de Entidad

```json
// CHARACTER properties
{
  "appearance": "Tall, silver hair, scar across left eye",
  "personality": "Stoic, loyal, secretive",
  "abilities": ["Swordsmanship", "Fire magic"],
  "current_location": "Thendar Forest",
  "age": "35",
  "role": "protagonist",
  "cause_of_death": null,
  "last_known_action": "Escaped from Thorn Prison in Chapter 14"
}

// PLACE properties
{
  "geography": "Mountain fortress surrounded by frozen lakes",
  "climate": "Arctic, permanent winter",
  "special_rules": ["Fire magic is nullified within the walls"],
  "population": "~5,000",
  "controlled_by": "Order of the Frost"
}

// EVENT properties
{
  "timeline_position": 145.0,
  "duration": "3 days",
  "consequences": ["Lord Vareth exiled", "Treaty of Solaria broken"],
  "participants": ["Lord Vareth", "King Aldric", "Lyra"]
}

// FACTION properties
{
  "hierarchy": "Council of Five → Wardens → Initiates",
  "territory": "Northern Reaches",
  "ideology": "Preservation of ancient magic",
  "members": ["Elder Mora", "Warden Thane", "Initiate Lyra"]
}

// WORLD_RULE properties
{
  "category": "magic",
  "scope": "global",
  "exceptions": ["The Chosen One is immune", "During solar eclipses, the rule is suspended"]
}

// PLOT_ARC properties
{
  "arc_status": "open",
  "chapters_involved": ["ch-uuid-1", "ch-uuid-5", "ch-uuid-12"],
  "resolution": null
}
```

---

## 4. Especificación de API REST

### 4.1 Convenciones Generales

- **Base URL:** `/api/v1`
- **Formato:** JSON
- **Autenticación:** Bearer token JWT en header `Authorization`
- **Paginación:** `?page=1&limit=20` (por defecto page=1, limit=20)
- **Errores:** Formato estándar (ver sección 4.2)
- **IDs:** UUID v4

### 4.2 Formato de Error Estándar

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Universe name is required",
    "details": [
      {
        "field": "name",
        "message": "must not be empty"
      }
    ]
  }
}
```

**Códigos de error:**

| HTTP Status | Error Code | Cuándo se usa |
|---|---|---|
| 400 | `VALIDATION_ERROR` | Input inválido, campos faltantes |
| 401 | `UNAUTHORIZED` | JWT faltante o expirado |
| 403 | `FORBIDDEN` | Intento de acceder a recurso de otro usuario |
| 404 | `NOT_FOUND` | Recurso no encontrado |
| 409 | `CONFLICT` | Duplicado (ej: email ya registrado, entidad duplicada) |
| 413 | `FILE_TOO_LARGE` | Archivo supera 10 MB |
| 415 | `UNSUPPORTED_FORMAT` | Formato de archivo no soportado |
| 422 | `PROCESSING_ERROR` | Error durante análisis AI |
| 429 | `RATE_LIMITED` | Demasiadas requests a Qwen API |
| 500 | `INTERNAL_ERROR` | Error interno del servidor |

---

### 4.3 Auth Endpoints (Simplificado para Demo/Hackathon)

> [!NOTE]
> Para la demo del hackathon, se omite el flujo complejo de registro y login. La aplicación realiza un "Skip Auth" autologueando al usuario hardcoded `demo@quill.ai` con un JWT predeterminado al cargar la interfaz. Las llamadas API validarán este token de sesión predeterminado.

#### `POST /api/v1/auth/register`

**Descripción:** Registra un nuevo usuario.

**Request Body:**
```json
{
  "email": "elena@example.com",
  "password": "SecurePass123!",
  "display_name": "Elena Velasco"
}
```

**Validaciones:**
- `email`: requerido, formato email válido, único en la DB
- `password`: requerido, mínimo 8 caracteres
- `display_name`: requerido, 2-100 caracteres

**Response 201:**
```json
{
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "elena@example.com",
    "display_name": "Elena Velasco",
    "created_at": "2026-06-28T12:00:00Z"
  },
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Errores posibles:** `VALIDATION_ERROR`, `CONFLICT` (email duplicado)

---

#### `POST /api/v1/auth/login`

**Request Body:**
```json
{
  "email": "elena@example.com",
  "password": "SecurePass123!"
}
```

**Response 200:**
```json
{
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "elena@example.com",
    "display_name": "Elena Velasco"
  },
  "token": "eyJhbGciOiJIUzI1NiIs..."
}
```

**Errores posibles:** `UNAUTHORIZED` (credenciales inválidas)

---

#### `GET /api/v1/auth/me`

**Headers:** `Authorization: Bearer <token>`

**Response 200:**
```json
{
  "user": {
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "email": "elena@example.com",
    "display_name": "Elena Velasco",
    "created_at": "2026-06-28T12:00:00Z"
  }
}
```

---

### 4.4 Universe Endpoints

#### `POST /api/v1/universes`

**Request Body:**
```json
{
  "name": "Los Reinos de Aether",
  "description": "Una saga de fantasía épica sobre la lucha por el control de la magia ancestral",
  "genre": "fantasy",
  "format": "novel"
}
```

**Validaciones:**
- `name`: requerido, 1-255 caracteres
- `description`: opcional, máx 5000 caracteres
- `genre`: opcional, valores permitidos: `fantasy`, `sci-fi`, `mystery`, `thriller`, `romance`, `horror`, `drama`, `action`, `comedy`, `other`
- `format`: requerido, valores permitidos: `novel`, `screenplay`, `manga`, `mixed`

**Response 201:**
```json
{
  "universe": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "user_id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "Los Reinos de Aether",
    "description": "Una saga de fantasía épica...",
    "genre": "fantasy",
    "format": "novel",
    "stats": {
      "works_count": 0,
      "entities_count": 0,
      "chapters_count": 0
    },
    "created_at": "2026-06-28T12:00:00Z",
    "updated_at": "2026-06-28T12:00:00Z"
  }
}
```

**Efecto secundario:** Se crea un grafo vacío en Apache AGE: `SELECT create_graph('universe_660e8400...')`

---

#### `GET /api/v1/universes`

**Query params:** `?page=1&limit=20`

**Response 200:**
```json
{
  "universes": [
    {
      "id": "660e8400-...",
      "name": "Los Reinos de Aether",
      "genre": "fantasy",
      "format": "novel",
      "stats": {
        "works_count": 3,
        "entities_count": 85,
        "chapters_count": 42
      },
      "created_at": "2026-06-28T12:00:00Z",
      "updated_at": "2026-07-01T15:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 1,
    "total_pages": 1
  }
}
```

---

#### `GET /api/v1/universes/:id`

**Response 200:** Igual que el objeto universe en POST pero con stats actualizados.

#### `PUT /api/v1/universes/:id`

**Request Body:** Mismos campos que POST, todos opcionales (PATCH semántico).

#### `DELETE /api/v1/universes/:id`

**Response 204:** No content. Elimina universo, todas sus obras, capítulos, entidades y el grafo de AGE.

---

### 4.5 Work Endpoints

#### `POST /api/v1/universes/:universe_id/works`

**Request Body:**
```json
{
  "title": "El Despertar de Aether",
  "type": "book",
  "synopsis": "Lyra descubre que tiene el poder de controlar la magia ancestral..."
}
```

**Validaciones:**
- `title`: requerido, 1-255 caracteres
- `type`: requerido, valores: `book`, `screenplay`, `manga_volume`
- `synopsis`: opcional, máx 5000 caracteres
- `order_index`: auto-calculado (max(order_index) + 1 del universo)

**Response 201:**
```json
{
  "work": {
    "id": "770e8400-...",
    "universe_id": "660e8400-...",
    "title": "El Despertar de Aether",
    "type": "book",
    "order_index": 1,
    "synopsis": "Lyra descubre...",
    "status": "in_progress",
    "stats": {
      "chapters_count": 0,
      "word_count": 0
    },
    "created_at": "2026-06-28T12:00:00Z"
  }
}
```

---

#### `GET /api/v1/universes/:universe_id/works`

**Response 200:** Lista de obras ordenadas por `order_index`.

#### `GET /api/v1/works/:id` | `PUT /api/v1/works/:id` | `DELETE /api/v1/works/:id`

Endpoints CRUD estándar con la misma estructura.

---

### 4.6 Chapter Endpoints

#### `POST /api/v1/works/:work_id/chapters`

**Request Body:**
```json
{
  "title": "Chapter 1: The Awakening"
}
```

**Validaciones:**
- `title`: opcional, 0-255 caracteres
- `order_index`: auto-calculado

**Response 201:**
```json
{
  "chapter": {
    "id": "880e8400-...",
    "work_id": "770e8400-...",
    "title": "Chapter 1: The Awakening",
    "order_index": 1,
    "content": "",
    "word_count": 0,
    "status": "draft",
    "analyzed_at": null,
    "created_at": "2026-06-28T12:00:00Z"
  }
}
```

---

#### `PUT /api/v1/chapters/:id`

**Request Body:**
```json
{
  "title": "Chapter 1: The Awakening (Revised)",
  "content": "<p>Lyra opened her eyes to find the world had changed...</p>",
  "raw_text": "Lyra opened her eyes to find the world had changed..."
}
```

> [!NOTE]
> `content` contiene el HTML/JSON del editor TipTap (para renderizar). `raw_text` contiene el texto plano (para análisis AI). El frontend debe enviar ambos.

**Response 200:** Capítulo actualizado con `word_count` recalculado.

---

### 4.7 Entity Endpoints

#### `GET /api/v1/universes/:universe_id/entities`

**Query params:**
- `?type=character` — filtrar por tipo
- `?status=active` — filtrar por estado
- `?min_relevance=0.3` — filtrar por relevancia mínima
- `?search=Lyra` — búsqueda por nombre/alias
- `?page=1&limit=50`

**Response 200:**
```json
{
  "entities": [
    {
      "id": "990e8400-...",
      "type": "character",
      "name": "Lyra Ashveil",
      "aliases": ["The Chosen", "Ashveil"],
      "description": "Young mage who discovered her connection to ancestral magic",
      "status": "active",
      "relevance_score": 0.95,
      "properties": {
        "appearance": "Dark hair, violet eyes, slender build",
        "abilities": ["Ancestral magic", "Empathy"],
        "current_location": "Aether Academy",
        "role": "protagonist"
      },
      "last_mentioned_at": "2026-07-01T15:30:00Z",
      "mentions_count": 47,
      "created_at": "2026-06-28T12:00:00Z"
    }
  ],
  "pagination": { "page": 1, "limit": 50, "total": 85 }
}
```

---

#### `GET /api/v1/entities/:id`

**Response 200:**
```json
{
  "entity": {
    "id": "990e8400-...",
    "type": "character",
    "name": "Lyra Ashveil",
    "aliases": ["The Chosen", "Ashveil"],
    "description": "Young mage who discovered...",
    "status": "active",
    "relevance_score": 0.95,
    "properties": { "..." },
    "relationships": [
      {
        "type": "MENTOR_OF",
        "direction": "incoming",
        "target_entity": {
          "id": "aa0e8400-...",
          "name": "Elder Mora",
          "type": "character"
        },
        "properties": { "since_chapter": "Chapter 3" }
      },
      {
        "type": "MEMBER_OF",
        "direction": "outgoing",
        "target_entity": {
          "id": "bb0e8400-...",
          "name": "Order of the Dawn",
          "type": "faction"
        },
        "properties": { "role": "Initiate" }
      }
    ],
    "mentions": [
      {
        "chapter_id": "880e8400-...",
        "chapter_title": "Chapter 1: The Awakening",
        "work_title": "El Despertar de Aether",
        "paragraph_index": 3,
        "context_snippet": "...Lyra opened her eyes to find the world had changed...",
        "mention_type": "direct"
      }
    ],
    "mentions_count": 47
  }
}
```

---

#### `PUT /api/v1/entities/:id`

**Request Body (edición manual por el autor):**
```json
{
  "name": "Lyra Ashveil",
  "aliases": ["The Chosen", "Ashveil", "The Last Mage"],
  "description": "Young mage who discovered her connection to ancestral magic. Daughter of the late King Ashveil.",
  "status": "active",
  "properties": {
    "appearance": "Dark hair, violet eyes, scar on right palm",
    "abilities": ["Ancestral magic", "Empathy", "Dream walking"],
    "current_location": "Thendar Forest",
    "role": "protagonist"
  }
}
```

---

### 4.8 Knowledge Graph Endpoints

#### `GET /api/v1/universes/:universe_id/graph`

**Query params:**
- `?types=character,place,faction` — filtrar tipos de nodo
- `?min_relevance=0.1` — excluir nodos de baja relevancia
- `?center_entity_id=990e8400-...` — centrar vista en una entidad
- `?depth=2` — profundidad de vecinos desde center

**Response 200:**
```json
{
  "graph": {
    "nodes": [
      {
        "id": "990e8400-...",
        "type": "character",
        "label": "Lyra Ashveil",
        "properties": {
          "status": "active",
          "relevance_score": 0.95,
          "role": "protagonist"
        }
      },
      {
        "id": "aa0e8400-...",
        "type": "place",
        "label": "Aether Academy",
        "properties": {
          "relevance_score": 0.82
        }
      }
    ],
    "edges": [
      {
        "id": "edge-001",
        "source": "990e8400-...",
        "target": "aa0e8400-...",
        "type": "LOCATED_AT",
        "properties": {
          "since_chapter": "Chapter 5"
        }
      }
    ],
    "stats": {
      "total_nodes": 85,
      "total_edges": 142,
      "nodes_returned": 85,
      "edges_returned": 142
    }
  }
}
```

---

### 4.9 Timeline Endpoint

#### `GET /api/v1/universes/:universe_id/timeline`

**Query params:**
- `?from_position=0&to_position=1000` — rango de la cronología
- `?work_id=770e8400-...` — filtrar por obra

**Response 200:**
```json
{
  "timeline": {
    "events": [
      {
        "id": "te-001",
        "title": "The Awakening of Lyra",
        "description": "Lyra discovers her connection to ancestral magic",
        "timeline_position": 1.0,
        "timeline_label": "Year 1, Day 1",
        "chapter": {
          "id": "880e8400-...",
          "title": "Chapter 1: The Awakening",
          "work_title": "El Despertar de Aether"
        },
        "participants": [
          { "id": "990e8400-...", "name": "Lyra Ashveil", "type": "character" }
        ],
        "has_inconsistency": false
      },
      {
        "id": "te-002",
        "title": "Battle of Solaria Gate",
        "description": "Lord Vareth attacks Solaria...",
        "timeline_position": 45.0,
        "timeline_label": "Year 1, Day 45",
        "has_inconsistency": true,
        "inconsistency_detail": "This event references characters arriving from Thendar, but the journey from Thendar takes 30 days according to Chapter 3"
      }
    ]
  }
}
```

---

### 4.10 Contradiction Endpoints

#### `GET /api/v1/universes/:universe_id/contradictions`

**Query params:** `?status=open&severity=critical`

**Response 200:**
```json
{
  "contradictions": [
    {
      "id": "ct-001",
      "severity": "critical",
      "description": "Lyra uses fire magic inside the Ice Cathedral, but the world rule 'Ice Cathedral Immunity' (Chapter 1, Book 1) states fire magic is nullified there.",
      "entity": {
        "id": "990e8400-...",
        "name": "Lyra Ashveil",
        "type": "character"
      },
      "evidence_a": {
        "text": "Within the ancient walls of the Ice Cathedral, no flame could ever burn...",
        "chapter_id": "880e8400-...",
        "chapter_title": "Chapter 1",
        "work_title": "Book 1"
      },
      "evidence_b": {
        "text": "Lyra raised her hand and a burst of fire erupted, melting the ice columns...",
        "chapter_id": "881e8400-...",
        "chapter_title": "Chapter 15",
        "work_title": "Book 3"
      },
      "suggestion": "Consider having Lyra use a different type of magic, or explain how she overcame the Cathedral's protection.",
      "status": "open",
      "created_at": "2026-07-01T15:30:00Z"
    }
  ]
}
```

#### `PUT /api/v1/contradictions/:id/resolve`

**Request Body:**
```json
{
  "resolution_note": "The Eclipse of Aether temporarily suspended all world rules, including the Cathedral's immunity."
}
```

#### `PUT /api/v1/contradictions/:id/dismiss`

**Request Body:**
```json
{
  "reason": "Intentional — Lyra has a unique power that bypasses this rule, revealed later."
}
```

---

### 4.11 Ingestion Endpoint

#### `POST /api/v1/universes/:universe_id/works/:work_id/ingest`

**Content-Type:** `multipart/form-data`

**Form Fields:**
- `file`: archivo (PDF, DOCX, MD) — máximo 10 MB

**Validaciones:**
- Archivo requerido
- Extensiones permitidas: `.pdf`, `.docx`, `.md`
- Tamaño máximo: 10 MB
- La obra no debe tener un job de ingesta activo (`status = 'processing'`)

**Response 202 (Accepted):**
```json
{
  "ingestion_job": {
    "id": "ij-001",
    "universe_id": "660e8400-...",
    "work_id": "770e8400-...",
    "filename": "book1_the_awakening.pdf",
    "file_type": "pdf",
    "status": "pending",
    "created_at": "2026-07-01T15:30:00Z"
  },
  "message": "Ingestion job created. Progress will be sent via WebSocket."
}
```

**El progreso se envía por WebSocket (ver sección 5).**

#### `GET /api/v1/ingestion/:job_id/status`

**Response 200:**
```json
{
  "ingestion_job": {
    "id": "ij-001",
    "status": "processing",
    "total_chapters_detected": 12,
    "chapters_processed": 5,
    "entities_extracted": 23,
    "progress_percentage": 42,
    "started_at": "2026-07-01T15:30:05Z"
  }
}
```

---

### 4.12 Plot Holes Endpoints

#### `GET /api/v1/universes/:universe_id/plotholes`

**Response 200:**
```json
{
  "plot_holes": [
    {
      "id": "ph-001",
      "title": "The Oracle's Prophecy",
      "description": "A prophecy about a 'chosen one' was introduced in Chapter 3, Book 1, but has not been developed or resolved in 15 subsequent chapters.",
      "related_entities": [
        { "id": "...", "name": "The Oracle", "type": "character" },
        { "id": "...", "name": "Lyra Ashveil", "type": "character" }
      ],
      "first_mentioned": {
        "chapter_title": "Chapter 3",
        "work_title": "Book 1"
      },
      "chapters_since_mention": 15,
      "status": "open"
    }
  ]
}
```

---

### 4.13 Demo & Session Endpoints

#### `POST /api/v1/demo/clone`

**Descripción:** Crea un clon aislado de la saga demo para la sesión del visitante actual. Copia de forma rápida en base de datos el universo marcado como `is_demo_template=TRUE`, incluyendo todas sus obras, capítulos, entidades, menciones y el grafo de AGE asociado a un nuevo UUID.

**Request Headers:**
- `X-Session-ID`: UUID generado por el cliente (localStorage) que identifica la sesión del visitante.

**Response 200:**
```json
{
  "status": "success",
  "universe_id": "990e8400-e29b-41d4-a716-446655440000",
  "message": "Demo universe cloned and isolated for session successfully."
}
```

---

#### `POST /api/v1/demo/reset`

**Descripción:** Borra todas las tablas relacionales y el grafo de AGE correspondientes al universo de la sesión actual (`X-Session-ID`) y vuelve a clonar la plantilla demo limpia. Usado para restaurar el estado inicial de la demo.

**Request Headers:**
- `X-Session-ID`: UUID de sesión del cliente.

**Response 200:**
```json
{
  "status": "success",
  "universe_id": "new-universe-uuid",
  "message": "Demo data reset successfully."
}
```


## 5. Especificación WebSocket

### 5.1 Conexión y Autenticación

**URL:** `ws://<host>/ws`

Una vez abierta la conexión, el cliente **debe** enviar inmediatamente un mensaje de tipo `auth_init` con el token JWT dentro de los primeros 5 segundos. Si el mensaje no llega o el token es inválido, el servidor cierra la conexión cerrando el WebSocket con código `4001` (Unauthorized).

#### Mensaje Client → Server: `auth_init`
```json
{
  "type": "auth_init",
  "payload": {
    "token": "eyJhbGciOiJIUzI1NiIs..."
  },
  "timestamp": "2026-07-01T15:30:00Z"
}
```

### 5.2 Formato de Mensaje

Todos los mensajes WebSocket siguen esta estructura:

```json
{
  "type": "<message_type>",
  "payload": { },
  "timestamp": "2026-07-01T15:30:00Z"
}
```

### 5.3 Mensajes Client → Server

#### `paragraph_submit` — Enviar párrafo para análisis

```json
{
  "type": "paragraph_submit",
  "payload": {
    "universe_id": "660e8400-...",
    "chapter_id": "880e8400-...",
    "paragraph_index": 5,
    "paragraph_node_id": "p_a1b2c3d4",
    "text": "Lord Vareth drew the Sword of Dawn and stepped through the gates of Solaria...",
    "full_chapter_text": "..."
  }
}
```


**Trigger:** Se envía cuando el escritor deja de escribir por 5 segundos (debounce en el frontend).

**Flujo de procesamiento en el backend:**
1. Recibir mensaje
2. Cancelar cualquier análisis pendiente/activo para este `chapter_id` usando cancelación por contexto (`context.WithCancel`).
3. Encolar secuencialmente y ejecutar `AnalysisService.AnalyzeParagraph()` con el nuevo contexto.
4. Enviar resultados como múltiples mensajes Server → Client si no fue cancelado en el camino.

---

#### `entity_query` — Consultar info de una entidad

```json
{
  "type": "entity_query",
  "payload": {
    "entity_id": "990e8400-..."
  }
}
```

**Respuesta:** `contextual_recall` con info de la entidad.

---

#### `context_request` — Pedir contexto del cursor actual

```json
{
  "type": "context_request",
  "payload": {
    "universe_id": "660e8400-...",
    "chapter_id": "880e8400-...",
    "cursor_paragraph_index": 5,
    "surrounding_text": "... Lyra walked into the chamber where..."
  }
}
```

---

### 5.4 Mensajes Server → Client

#### `analysis_result` — Resultado completo del análisis

```json
{
  "type": "analysis_result",
  "payload": {
    "chapter_id": "880e8400-...",
    "paragraph_index": 5,
    "entities_found": [
      {
        "id": "990e8400-...",
        "name": "Lord Vareth",
        "type": "character",
        "is_new": false,
        "status": "active",
        "relevance_score": 0.78
      },
      {
        "id": "bb1e8400-...",
        "name": "Sword of Dawn",
        "type": "object",
        "is_new": true,
        "relevance_score": 1.0
      }
    ],
    "analysis_duration_ms": 2450
  }
}
```

---

#### `contradiction_alert` — Contradicción detectada

```json
{
  "type": "contradiction_alert",
  "payload": {
    "id": "ct-002",
    "severity": "warning",
    "entity_name": "Lord Vareth",
    "description": "Lord Vareth was exiled from Solaria in Chapter 5, Book 2. He should not be able to enter the gates of Solaria.",
    "evidence_a": {
      "text": "By decree of King Aldric, Lord Vareth was banished from all lands of Solaria, never to return...",
      "chapter_title": "Chapter 5",
      "work_title": "Book 2"
    },
    "evidence_b": {
      "text": "Lord Vareth drew the Sword of Dawn and stepped through the gates of Solaria...",
      "chapter_title": "Chapter 15",
      "work_title": "Book 3"
    },
    "suggestion": "Consider explaining how Lord Vareth gained entry despite his exile (e.g., disguise, pardon, or forceful entry)."
  }
}
```

---

#### `contextual_recall` — Recuerdos relevantes para el contexto

```json
{
  "type": "contextual_recall",
  "payload": {
    "memories": [
      {
        "entity_id": "990e8400-...",
        "entity_name": "Lord Vareth",
        "entity_type": "character",
        "relevance_score": 0.78,
        "recall_points": [
          {
            "fact": "Status: Active, exiled from Solaria",
            "source_chapter": "Chapter 5, Book 2",
            "importance": "high"
          },
          {
            "fact": "Last location: Thendar Forest",
            "source_chapter": "Chapter 12, Book 2",
            "importance": "medium"
          },
          {
            "fact": "Relationship: Enemy of King Aldric, Mentor of Lyra",
            "source_chapter": "Multiple",
            "importance": "high"
          },
          {
            "fact": "Was betrayed by his lieutenant Kael",
            "source_chapter": "Chapter 8, Book 3",
            "importance": "medium"
          }
        ],
        "freshness": 0.78,
        "was_reactivated": false
      }
    ]
  }
}
```

---

#### `entity_discovered` — Nueva entidad extraída

```json
{
  "type": "entity_discovered",
  "payload": {
    "entity": {
      "id": "new-uuid-...",
      "type": "character",
      "name": "Sword of Dawn",
      "description": "A legendary sword drawn by Lord Vareth",
      "properties": {},
      "relevance_score": 1.0
    },
    "source_chapter": "Chapter 15, Book 3",
    "source_paragraph": 5
  }
}
```

---

#### `graph_updated` — Knowledge Graph cambió

```json
{
  "type": "graph_updated",
  "payload": {
    "changes": {
      "nodes_added": [{ "id": "...", "type": "character", "label": "Sword of Dawn" }],
      "nodes_updated": [{ "id": "...", "field": "relevance_score", "old_value": 0.65, "new_value": 0.78 }],
      "edges_added": [{ "source": "...", "target": "...", "type": "LOCATED_AT" }],
      "edges_removed": []
    }
  }
}
```

---

#### `ingestion_progress` — Progreso de ingesta

```json
{
  "type": "ingestion_progress",
  "payload": {
    "job_id": "ij-001",
    "phase": "extracting_entities",
    "phase_label": "Extracting entities from Chapter 5/12...",
    "progress_percentage": 42,
    "chapters_processed": 5,
    "total_chapters": 12,
    "entities_found_so_far": 23,
    "latest_entity": {
      "name": "Elder Mora",
      "type": "character"
    }
  }
}
```

---

#### `plot_hole_detected` — Hueco narrativo detectado

```json
{
  "type": "plot_hole_detected",
  "payload": {
    "id": "ph-002",
    "title": "The Missing Amulet",
    "description": "In Chapter 7, Lyra found a mysterious amulet that 'held the key to everything', but it has not been mentioned in the following 8 chapters.",
    "chapters_since_mention": 8,
    "first_mentioned": "Chapter 7, Book 2"
  }
}
```

---

#### `timeline_inconsistency` — Inconsistencia temporal

```json
{
  "type": "timeline_inconsistency",
  "payload": {
    "description": "The journey from Thendar Forest to Solaria was described as taking '30 days on horseback' in Chapter 3, but Lord Vareth appears to have made the trip in '2 days' based on the events in Chapters 14-15.",
    "event_a": {
      "title": "Thendar-Solaria travel time established",
      "chapter": "Chapter 3, Book 1"
    },
    "event_b": {
      "title": "Lord Vareth arrives at Solaria gates",
      "chapter": "Chapter 15, Book 3"
    }
  }
}
```

---

## 6. Algoritmos Core (Pseudocódigo)

### 6.1 Algoritmo: Análisis de Párrafo (AnalysisService)

Este es el orquestador principal. Se invoca cada vez que el frontend envía un párrafo por WebSocket.

```python
def analyze_paragraph(universe_id, chapter_id, paragraph_index, paragraph_node_id, text, ctx):
    """
    Orquesta el análisis completo de un párrafo.
    Se ejecuta de forma asíncrona dentro de una cola secuencial cancelable por capítulo.
    La variable `ctx` representa el context de Go, y se evalúa periódicamente (`ctx.Done()`).
    
    Separado en dos fases:
    - FASE A (Transacción Núcleo): Pasos 1-4. Atómica (pgx.Tx). No cancelable una vez empezada.
    - FASE B (Cola Enriquecedora): Pasos 5-10. Cancelable en cualquier checkpoint.
    """
    
    # ═══════════════════════════════════════════════════════════════
    # FASE A: TRANSACCIÓN NÚCLEO (atómica, no cancelable una vez empezada)
    # Si ctx se cancela ANTES de esta fase, salimos. Una vez dentro, siempre se completa.
    # ═══════════════════════════════════════════════════════════════
    if ctx.is_cancelled(): return
    
    # PASO 1: Extraer entidades del párrafo (Qwen-Turbo — única llamada externa en fase A)
    extracted = qwen_turbo.extract_entities(text, get_universe_context(universe_id))
    
    # PASO 2-4: Dentro de una transacción pgx.Tx
    tx = db.begin()
    try:
        resolved_entities = []
        
        # PASO 2: Resolver cada entidad contra el Knowledge Graph
        for entity_data in flatten(extracted):
            existing = find_existing_entity_tx(tx, universe_id, entity_data.name, entity_data.aliases)
            if existing:
                merged = merge_entity_info(existing, entity_data)
                update_entity_tx(tx, merged)
                update_graph_node_tx(tx, universe_id, merged)  # AGE en misma Tx
                resolved_entities.append(merged)
            else:
                new_entity = create_entity_tx(tx, universe_id, entity_data)
                create_graph_node_tx(tx, universe_id, new_entity)  # AGE en misma Tx
                # Generar y guardar embedding (pgvector en misma Tx)
                embedding = qwen_embedding.generate(entity_data.description)
                save_entity_embedding_tx(tx, new_entity.id, embedding)
                resolved_entities.append(new_entity)
                ws_send("entity_discovered", new_entity)
        
        # PASO 3: Registrar menciones
        for entity in resolved_entities:
            create_mention_tx(tx, entity.id, chapter_id, paragraph_index, paragraph_node_id, text)
        
        # PASO 4: Actualizar relevancia (Touch)
        for entity in resolved_entities:
            touch_tx(tx, entity.id, chapter_id)
        
        tx.commit()  # Las 3 capas (entities + AGE + pgvector) se comprometen juntas
    except Exception as e:
        tx.rollback()  # Rollback atómico de las 3 capas
        raise e
    
    # ═══════════════════════════════════════════════════════════════
    # FASE B: COLA ENRIQUECEDORA (cancelable en cada checkpoint)
    # Si se cancela aquí, la entidad ya existe correctamente en la DB y el grafo.
    # La "enrichment" (relaciones, recall, contradicciones) llegará con el próximo análisis.
    # ═══════════════════════════════════════════════════════════════
    
    if ctx.is_cancelled(): return
    # PASO 5: Extraer y crear relaciones entre entidades mencionadas
    if len(resolved_entities) > 1:
        relationships = qwen_turbo.analyze_relationships(text, resolved_entities)
        for rel in relationships:
            create_or_update_graph_edge(universe_id, rel)
        ws_send("graph_updated", get_graph_changes())
    
    if ctx.is_cancelled(): return
    # PASO 6: Recall contextual — buscar recuerdos relevantes
    memories = memory_service.recall_for_context(universe_id, text, resolved_entities)
    ws_send("contextual_recall", memories)
    
    if ctx.is_cancelled(): return
    # PASO 7: Detección de contradicciones (batched — ver algoritmo 6.4)
    all_contradictions = contradiction_service.check_batch(text, resolved_entities, universe_id, chapter_id)
    for c in all_contradictions:
        save_contradiction(c)  # Verifica fingerprint antes de guardar
        ws_send("contradiction_alert", c)
    
    if ctx.is_cancelled(): return
    # PASO 8: Validación de línea temporal
    timeline_issues = timeline_service.validate(text, resolved_entities, universe_id)
    for issue in timeline_issues:
        ws_send("timeline_inconsistency", issue)
    
    if ctx.is_cancelled(): return
    # PASO 9: Escaneo de huecos narrativos
    plot_holes = plothole_service.scan(universe_id, resolved_entities)
    for ph in plot_holes:
        ws_send("plot_hole_detected", ph)
    
    if ctx.is_cancelled(): return
    # PASO 10: Enviar resultado completo
    ws_send("analysis_result", {
        "chapter_id": chapter_id,
        "paragraph_index": paragraph_index,
        "paragraph_node_id": paragraph_node_id,
        "entities_found": resolved_entities,
        "analysis_duration_ms": elapsed()
    })
```

---

### 6.2 Algoritmo: Resolución de Entidades (Entity Resolution)

```python
def find_existing_entity(universe_id, name, aliases):
    """
    Busca si una entidad ya existe en el universo por nombre o alias.
    Usa matching exacto + fuzzy para manejar variaciones.
    """
    
    # Paso 1: Búsqueda exacta por nombre
    exact = db.query("""
        SELECT * FROM entities 
        WHERE universe_id = ? AND (
            LOWER(name) = LOWER(?) 
            OR LOWER(?) = ANY(SELECT LOWER(unnest(aliases)))
        )
    """, universe_id, name, name)
    
    if exact:
        return exact[0]
    
    # Paso 2: Búsqueda por alias proporcionados
    for alias in aliases:
        found = db.query("""
            SELECT * FROM entities 
            WHERE universe_id = ? AND (
                LOWER(name) = LOWER(?)
                OR LOWER(?) = ANY(SELECT LOWER(unnest(aliases)))
            )
        """, universe_id, alias, alias)
        if found:
            return found[0]
    
    # Paso 3: Búsqueda semántica con embeddings (para resolver "el Rey" → "King Aldric")
    embedding = qwen_embedding.generate(name + " " + " ".join(aliases))
    similar = db.query("""
        SELECT e.*, ee.description_embedding <=> ? AS distance
        FROM entities e
        JOIN entity_embeddings ee ON e.id = ee.entity_id
        WHERE e.universe_id = ?
        ORDER BY distance ASC
        LIMIT 1
    """, embedding, universe_id)
    
    if similar and similar[0].distance < 0.15:  # Umbral de similitud
        return similar[0]
    
    # No encontrada → es nueva
    return None


def merge_entity_info(existing, new_data):
    """
    Fusiona nueva información con la entidad existente.
    Solo actualiza campos que tienen nueva información; no sobrescribe con vacíos.
    """
    merged = copy(existing)
    
    # Añadir aliases nuevos sin duplicados
    new_aliases = set(existing.aliases or [])
    new_aliases.update(new_data.aliases or [])
    merged.aliases = list(new_aliases)
    
    # Actualizar propiedades JSONB — merge profundo
    for key, value in new_data.properties.items():
        if value and (key not in existing.properties or not existing.properties[key]):
            merged.properties[key] = value
        elif key == "current_location" and value:
            # La ubicación SIEMPRE se actualiza (el más reciente gana)
            merged.properties[key] = value
    
    # Actualizar estado si cambió
    if new_data.status and new_data.status != existing.status:
        merged.status = new_data.status
    
    # Actualizar descripción si la nueva es más detallada
    if new_data.description and len(new_data.description) > len(existing.description or ""):
        merged.description = new_data.description
    
    merged.updated_at = now()
    return merged
```

---

### 6.3 Algoritmo: Decaimiento de Relevancia (Timely Forgetting)

```python
import math

# Constantes del algoritmo
DECAY_LAMBDA = 0.1       # Tasa de decaimiento (0.1 = suave)
ARCHIVE_THRESHOLD = 0.15  # Por debajo de esto, se archiva
REACTIVATION_SCORE = 0.8  # Score al reactivar una entidad archivada
BASE_IMPORTANCE = {
    "protagonist": 1.0,
    "major": 0.8,
    "supporting": 0.6,
    "minor": 0.4,
    "background": 0.2
}


def calculate_relevance_score(entity, current_chapter_order, max_mentions_in_universe):
    """
    Calcula el relevance_score de una entidad.
    
    Fórmula:
    relevance_score = base_importance × recency_factor × mention_frequency_factor
    
    Donde:
    - base_importance: importancia inherente del personaje (protagonista > figurante)
    - recency_factor: decaimiento exponencial basado en capítulos desde última mención
    - mention_frequency_factor: normalización logarítmica de frecuencia de menciones
    """
    
    # Factor 1: Importancia base
    role = entity.properties.get("role", "minor")
    base = BASE_IMPORTANCE.get(role, 0.4)
    
    # Factor 2: Recencia (decaimiento exponencial)
    chapters_since_mention = current_chapter_order - get_chapter_order(entity.last_mentioned_chapter_id)
    recency = math.exp(-DECAY_LAMBDA * chapters_since_mention)
    # Ejemplo: 0 capítulos → 1.0, 5 capítulos → 0.61, 10 → 0.37, 20 → 0.14, 30 → 0.05
    
    # Factor 3: Frecuencia de menciones (logarítmica para no dominar)
    total_mentions = count_mentions(entity.id)
    if max_mentions_in_universe > 0:
        frequency = math.log(1 + total_mentions) / math.log(1 + max_mentions_in_universe)
    else:
        frequency = 1.0
    
    # Score final
    score = base * recency * frequency
    
    return max(0.0, min(1.0, score))  # Clamp entre 0 y 1


def touch(entity_id, chapter_id):
    """
    "Toca" una entidad cuando es mencionada, actualizando su relevancia.
    Si estaba archivada (score < ARCHIVE_THRESHOLD), la reactiva.
    """
    entity = get_entity(entity_id)
    chapter_order = get_chapter_order(chapter_id)
    
    was_archived = entity.relevance_score < ARCHIVE_THRESHOLD
    
    # Actualizar última mención
    entity.last_mentioned_chapter_id = chapter_id
    entity.last_mentioned_at = now()
    
    # Recalcular score
    max_mentions = get_max_mentions_in_universe(entity.universe_id)
    entity.relevance_score = calculate_relevance_score(entity, chapter_order, max_mentions)
    
    # Si estaba archivada, reactivar con score mínimo de 0.8
    if was_archived:
        entity.relevance_score = max(entity.relevance_score, REACTIVATION_SCORE)
    
    save_entity(entity)
    
    return was_archived  # Para notificar al frontend si fue reactivada


def decay_all(universe_id, current_chapter_order):
    """
    Ejecuta el decaimiento en TODAS las entidades del universo.
    Se invoca cuando el escritor avanza a un nuevo capítulo o periódicamente.
    """
    max_mentions = get_max_mentions_in_universe(universe_id)
    entities = get_all_entities(universe_id)
    
    for entity in entities:
        old_score = entity.relevance_score
        new_score = calculate_relevance_score(entity, current_chapter_order, max_mentions)
        
        if new_score != old_score:
            entity.relevance_score = new_score
            save_entity(entity)
            
            # Actualizar también el nodo en el grafo de AGE
            update_graph_node_relevance(universe_id, entity.id, new_score)
    
    return len(entities)
```

---

### 6.4 Algoritmo: Detección de Contradicciones

```python
MAX_CONTRADICTION_CANDIDATES = 3  # Límite duro de candidatos por párrafo

def check_contradictions_batch(paragraph_text, entities, universe_id, chapter_id):
    """
    Detecta contradicciones entre el párrafo nuevo y hechos establecidos.
    Versión ALTAMENTE OPTIMIZADA:
    - Embedding del párrafo cacheado (generado UNA sola vez).
    - Concurrencia real (paralelismo por entidad con errgroup + semáforo con límite de 3).
    - Priorización del Top 3 (determinista > reglas > semántica) aplicada a los candidatos.
    - Evita re-disparar descartados (fingerprint checking).
    """
    
    # ──────────────────────────────────────────
    # PRE-PROCESO: Generar embedding una sola vez
    # ──────────────────────────────────────────
    paragraph_embedding = qwen_embedding.generate(paragraph_text)
    
    # ──────────────────────────────────────────
    # FASE 1: Recolectar candidatos en paralelo
    # ──────────────────────────────────────────
    candidates = []  # Lista compartida (protegida por mutex)
    candidates_mutex = Mutex()
    
    # Usamos errgroup y un semáforo de concurrencia = 3
    g = new_errgroup()
    sem = new_semaphore(3)
    
    for entity in entities:
        # Capturar variable en el closure
        entity_ref = entity
        
        def run_check():
            sem.acquire()
            defer(sem.release)
            
            entity_candidates = []
            graph_facts = get_entity_graph_facts(universe_id, entity_ref.id)
            
            # 1. Chequeo Determinista (Character status)
            if entity_ref.type == "character" and entity_ref.status == "deceased":
                if paragraph_mentions_as_alive(paragraph_text, entity_ref.name):
                    fp = sha256(f"{entity_ref.id}:deceased_alive:{chapter_id}")
                    if not contradiction_already_processed(fp):
                        entity_candidates.append(("deterministic", entity_ref, {
                            "severity": "critical",
                            "description": f"{entity_ref.name} is deceased but appears alive/active.",
                            "evidence_a": get_death_mention(entity_ref.id),
                            "evidence_b": paragraph_text,
                            "fingerprint": fp
                        }))
            
            # 2. Chequeo de Reglas del Mundo
            applicable_rules = get_applicable_world_rules(universe_id, entity_ref.id)
            for rule in applicable_rules:
                fp = sha256(f"{entity_ref.id}:rule:{rule.id}:{chapter_id}")
                if not contradiction_already_processed(fp):
                    entity_candidates.append(("rule", entity_ref, {
                        "rule": rule,
                        "graph_facts": graph_facts,
                        "fingerprint": fp
                    }))
            
            # 3. Chequeo Semántico (pgvector utilizando el embedding pre-generado)
            similar_paragraphs = find_similar_paragraphs(
                paragraph_embedding, universe_id, chapter_id, limit=3
            )
            for similar in similar_paragraphs:
                fp = sha256(f"{entity_ref.id}:{similar.chapter_id}:{chapter_id}")
                if not contradiction_already_processed(fp):
                    entity_candidates.append(("semantic", entity_ref, {
                        "similar": similar,
                        "graph_facts": graph_facts,
                        "fingerprint": fp
                    }))
            
            # Guardar resultados con exclusión mutua
            candidates_mutex.lock()
            candidates.extend(entity_candidates)
            candidates_mutex.unlock()
            
            return nil
            
        g.go(run_check)
        
    g.wait()  # Esperar a que terminen todos los hilos
    
    # ──────────────────────────────────────────
    # FASE 2: Priorizar y aplicar límite duro (Top 3)
    # ──────────────────────────────────────────
    # Prioridad: deterministic (0) > rules (1) > semantic (2)
    # Segundo criterio: relevancia del personaje (de mayor a menor)
    candidates.sort(key=lambda c: (
        {"deterministic": 0, "rule": 1, "semantic": 2}[c[0]],
        -c[1].relevance_score
    ))
    
    # Tomamos un máximo de 3 candidatos totales para mitigar fan-out y latencia
    candidates = candidates[:MAX_CONTRADICTION_CANDIDATES]
    
    if not candidates:
        return []
    
    # ──────────────────────────────────────────
    # FASE 3: Resolver contradicciones deterministas (sin LLM)
    # ──────────────────────────────────────────
    contradictions = []
    llm_candidates = []
    
    for ctype, entity, data in candidates:
        if ctype == "deterministic":
            contradictions.append(Contradiction(
                severity=data["severity"],
                description=data["description"],
                evidence_a=data["evidence_a"],
                evidence_b=data["evidence_b"],
                entity_id=entity.id,
                fingerprint=data["fingerprint"]
            ))
        else:
            llm_candidates.append((ctype, entity, data))
    
    # ──────────────────────────────────────────
    # FASE 4: Batched LLM check con Qwen-Max
    # ──────────────────────────────────────────
    if llm_candidates:
        evidence_pairs = []
        for ctype, entity, data in llm_candidates:
            if ctype == "rule":
                evidence_pairs.append({
                    "entity": entity.name,
                    "type": "rule_violation",
                    "fact_a": f"World Rule '{data['rule'].name}': {data['rule'].description}",
                    "fact_b": paragraph_text,
                    "fingerprint": data["fingerprint"]
                })
            elif ctype == "semantic":
                evidence_pairs.append({
                    "entity": entity.name,
                    "type": "semantic_contradiction",
                    "fact_a": data["similar"].content,
                    "fact_a_chapter": data["similar"].chapter_title,
                    "fact_b": paragraph_text,
                    "fingerprint": data["fingerprint"]
                })
        
        # Una sola llamada batched
        results = qwen_max.detect_contradictions_batch(evidence_pairs)
        
        for result in results:
            if result.is_contradiction:
                pair = evidence_pairs[result.pair_index]
                contradictions.append(Contradiction(
                    severity=result.severity,
                    description=result.explanation,
                    suggestion=result.suggestion,
                    evidence_a=pair["fact_a"],
                    evidence_b=pair["fact_b"],
                    entity_id=find_entity_by_name(entities, pair["entity"]).id,
                    fingerprint=pair["fingerprint"]
                ))
    
    return contradictions


def contradiction_already_processed(fingerprint):
    """Verifica si un fingerprint ya existe con status 'resolved' o 'dismissed'."""
    return db.exists(
        "SELECT 1 FROM contradictions WHERE fingerprint = ? AND status IN ('resolved', 'dismissed')",
        fingerprint
    )
```


---

### 6.5 Algoritmo: Recall Contextual

```python
def recall_for_context(universe_id, text, mentioned_entities):
    """
    Genera recuerdos relevantes para el contexto actual del escritor.
    Combina 3 señales: relevance_score + vector_similarity + graph_proximity.
    
    Returns: list[MemoryRecall]
    """
    memories = []
    
    for entity in mentioned_entities:
        # Obtener hechos del grafo
        graph_facts = get_entity_graph_facts(universe_id, entity.id)
        
        # Obtener últimas menciones relevantes
        recent_mentions = db.query("""
            SELECT em.context_snippet, c.title, w.title as work_title, c.order_index
            FROM entity_mentions em
            JOIN chapters c ON em.chapter_id = c.id
            JOIN works w ON c.work_id = w.id
            WHERE em.entity_id = ?
            ORDER BY c.order_index DESC
            LIMIT 5
        """, entity.id)
        
        # Construir recall points priorizados
        recall_points = []
        
        # 1. Estado actual (siempre incluir)
        if entity.status:
            recall_points.append({
                "fact": f"Status: {entity.status}",
                "source": "Current state",
                "importance": "high"
            })
        
        # 2. Ubicación actual (alta prioridad para personajes)
        if entity.type == "character" and entity.properties.get("current_location"):
            recall_points.append({
                "fact": f"Last known location: {entity.properties['current_location']}",
                "source": get_location_source(entity.id),
                "importance": "high"
            })
        
        # 3. Relaciones directas (desde el grafo)
        relationships = get_relationships(universe_id, entity.id)
        if relationships:
            rel_summary = ", ".join([
                f"{r.type.replace('_', ' ').title()} {r.target_name}" 
                for r in relationships[:5]
            ])
            recall_points.append({
                "fact": f"Relationships: {rel_summary}",
                "source": "Knowledge Graph",
                "importance": "high"
            })
        
        # 4. Reglas del mundo que afectan a esta entidad
        rules = get_applicable_world_rules(universe_id, entity.id)
        for rule in rules:
            recall_points.append({
                "fact": f"World rule: {rule.name} — {rule.description}",
                "source": get_rule_source(rule.id),
                "importance": "medium"
            })
        
        # 5. Eventos recientes en los que participó
        events = get_recent_events(universe_id, entity.id, limit=3)
        for event in events:
            recall_points.append({
                "fact": f"Event: {event.title} — {event.description}",
                "source": f"{event.chapter_title}, {event.work_title}",
                "importance": "medium"
            })
        
        # 6. Usar Qwen-Turbo para generar un resumen inteligente si hay mucha info
        if len(recall_points) > 8:
            summary = qwen_turbo.summarize_recall(
                entity_name=entity.name,
                facts=recall_points,
                current_context=text,
                max_points=5
            )
            recall_points = summary
        
        memories.append(MemoryRecall(
            entity_id=entity.id,
            entity_name=entity.name,
            entity_type=entity.type,
            relevance_score=entity.relevance_score,
            recall_points=recall_points,
            freshness=entity.relevance_score,
            was_reactivated=entity.relevance_score < ARCHIVE_THRESHOLD
        ))
    
    # Ordenar por relevance_score descendente
    memories.sort(key=lambda m: m.relevance_score, reverse=True)
    
    return memories
```

---

### 6.6 Algoritmo: Pipeline de Ingesta de Documentos Concurrente

```python
def process_document(job_id, file_path, file_type, universe_id, work_id):
    """
    Pipeline optimizado de ingesta de un documento existente:
    - Worker pool con 5 goroutines en paralelo para procesamiento de párrafos.
    - Generación de embeddings agrupados (batching de 10 párrafos por llamada).
    - Reportes de progreso granulares via WebSocket.
    """
    try:
        update_job_status(job_id, "processing")
        
        # ═══════════════════════════════════════════
        # FASE 1: Parseo del documento
        # ═══════════════════════════════════════════
        ws_send("ingestion_progress", {
            "job_id": job_id, "phase": "parsing", 
            "phase_label": "Parsing document...", "progress_percentage": 5
        })
        
        if file_type == "pdf":
            raw_text = pdf_parser.extract_text(file_path)
        elif file_type == "docx":
            raw_text = docx_parser.extract_text(file_path)
        elif file_type == "md":
            raw_text = read_file(file_path)
        
        # ═══════════════════════════════════════════
        # FASE 2: Detección de capítulos
        # ═══════════════════════════════════════════
        ws_send("ingestion_progress", {
            "job_id": job_id, "phase": "splitting",
            "phase_label": "Detecting chapters...", "progress_percentage": 15
        })
        
        chapters = chapter_splitter.split(raw_text)
        update_job_field(job_id, "total_chapters_detected", len(chapters))
        
        # ═══════════════════════════════════════════
        # FASE 3: Procesar capítulos en paralelo
        # ═══════════════════════════════════════════
        total_entities = 0
        entities_mutex = Mutex()
        
        # Procesar los capítulos uno a uno para mantener el orden, pero procesar
        # los párrafos del capítulo de forma paralela (concurrencia de 5 workers)
        for i, chapter_data in enumerate(chapters):
            progress = 20 + int((i / len(chapters)) * 60)  # Progreso 20% a 80%
            ws_send("ingestion_progress", {
                "job_id": job_id, "phase": "extracting_entities",
                "phase_label": f"Processing Chapter {i+1}/{len(chapters)}...",
                "progress_percentage": progress,
                "chapters_processed": i + 1, "total_chapters": len(chapters),
                "entities_found_so_far": total_entities
            })
            
            # 3a. Crear capítulo en DB
            chapter = create_chapter(work_id, chapter_data.title, chapter_data.content, i + 1)
            paragraphs = split_into_paragraphs(chapter_data.content)
            
            # 3b. Generar embeddings en batches de 10 párrafos para optimizar red/costo
            batch_size = 10
            for start_idx in range(0, len(paragraphs), batch_size):
                end_idx = min(start_idx + batch_size, len(paragraphs))
                batch_paragraphs = paragraphs[start_idx:end_idx]
                
                # Una sola llamada API para generar los 10 embeddings juntos
                embeddings = qwen_embedding.generate_batch(batch_paragraphs)
                
                for idx, emb in enumerate(embeddings):
                    p_idx = start_idx + idx
                    save_paragraph_embedding(chapter.id, p_idx, batch_paragraphs[idx].paragraph_node_id, batch_paragraphs[idx].text, emb)
            
            # 3c. Procesar párrafos con Worker Pool concurrente (5 workers)
            # NOTA DE CONCURRENCIA: La llamada a Qwen-Turbo (la parte cara) es
            # paralela, pero la resolución de entidades (find-or-create) se
            # serializa con entity_resolution_mutex para evitar duplicación
            # cuando el mismo personaje aparece en múltiples párrafos.
            # Red de seguridad adicional: UNIQUE INDEX en (universe_id, lower(name))
            # con manejo de conflicto "ON CONFLICT DO NOTHING + re-read".
            g = new_errgroup()
            sem = new_semaphore(5)
            entity_resolution_mutex = Mutex()  # Serializa find-or-create por capítulo
            
            for p_idx, paragraph in enumerate(paragraphs):
                p_ref_idx = p_idx
                paragraph_ref = paragraph
                
                def process_paragraph_worker():
                    sem.acquire()
                    defer(sem.release)
                    
                    # PASO PARALELO: Llamada a Qwen-Turbo (no toca la DB)
                    extracted = qwen_turbo.extract_entities(paragraph_ref.text, get_universe_context(universe_id))
                    
                    # PASO SERIALIZADO: Resolver/crear entidades bajo mutex
                    entity_resolution_mutex.lock()
                    tx = db.begin_transaction()
                    try:
                        for entity_data in flatten(extracted):
                            existing = find_existing_entity_tx(tx, universe_id, entity_data.name, entity_data.aliases)
                            if existing:
                                merged = merge_entity_info(existing, entity_data)
                                update_entity_tx(tx, merged)
                                create_mention_tx(tx, existing.id, chapter.id, p_ref_idx, paragraph_ref.paragraph_node_id, paragraph_ref.text)
                                relevance_service.touch_tx(tx, existing.id, chapter.id)
                            else:
                                new_entity = create_entity_tx(tx, universe_id, entity_data)
                                create_graph_node_tx(tx, universe_id, new_entity)
                                create_mention_tx(tx, new_entity.id, chapter.id, p_ref_idx, paragraph_ref.paragraph_node_id, paragraph_ref.text)
                                relevance_service.touch_tx(tx, new_entity.id, chapter.id)
                                
                                entities_mutex.lock()
                                total_entities += 1
                                entities_mutex.unlock()
                                ws_send("entity_discovered", new_entity)
                        
                        tx.commit()
                    except Exception as tx_err:
                        tx.rollback()
                        return tx_err
                    finally:
                        entity_resolution_mutex.unlock()
                    
                    # PASO PARALELO: Relaciones (lectura, no crea entidades nuevas)
                    paragraph_entities = get_entities_mentioned_in_paragraph(paragraph_ref.text)
                    if len(paragraph_entities) > 1:
                        relationships = qwen_turbo.analyze_relationships(paragraph_ref.text, paragraph_entities)
                        for rel in relationships:
                            create_or_update_graph_edge(universe_id, rel)
                    
                    return nil
                
                g.go(process_paragraph_worker)
                
            g.wait()  # Esperar a que terminen los 5 workers para este capítulo
            
            update_job_field(job_id, "chapters_processed", i + 1)
            update_job_field(job_id, "entities_extracted", total_entities)
        
        # ═══════════════════════════════════════════
        # FASE 4: Relaciones globales y timeline
        # ═══════════════════════════════════════════
        ws_send("ingestion_progress", {
            "job_id": job_id, "phase": "building_graph",
            "phase_label": "Building knowledge graph relationships...",
            "progress_percentage": 85
        })
        build_timeline(universe_id)
        
        # ═══════════════════════════════════════════
        # FASE 5: Contradicciones y Plot Holes
        # ═══════════════════════════════════════════
        ws_send("ingestion_progress", {
            "job_id": job_id, "phase": "analysis",
            "phase_label": "Scanning for contradictions and plot holes...",
            "progress_percentage": 92
        })
        scan_all_contradictions(universe_id)
        scan_all_plot_holes(universe_id)
        
        update_job_status(job_id, "completed")
        ws_send("ingestion_complete", {
            "job_id": job_id,
            "total_chapters": len(chapters),
            "total_entities": total_entities,
            "contradictions_found": count_contradictions(universe_id),
            "plot_holes_found": count_plot_holes(universe_id)
        })
    
    except Exception as e:
        update_job_status(job_id, "failed", error_message=str(e))
        ws_send("ingestion_progress", {
            "job_id": job_id, "phase": "error",
            "phase_label": f"Error: {str(e)}", "progress_percentage": -1
        })
```

---

### 6.7 Algoritmo: Clonación de Universo Demo (Aislamiento de Sesiones)

```python
def clone_universe_demo(session_id):
    """
    Clona la plantilla de la saga demo (marcada como is_demo_template = TRUE)
    creando un universo nuevo e independiente para el session_id provisto.
    La clonación se realiza a nivel PostgreSQL con SQL masivo y recrea el
    grafo de AGE correspondiente. Esto evita que los jueces se pisen datos.
    """
    tx = db.begin_transaction()
    try:
        # 1. Obtener el universo plantilla
        template = tx.query_one("SELECT * FROM universes WHERE is_demo_template = TRUE LIMIT 1")
        if not template:
            raise Exception("No demo template universe found. Please run seed.sql first.")
            
        new_universe_id = uuid.new()
        new_graph_name = f"universe_{strings.replace(new_universe_id, '-', '')}"
        
        # 2. Clonar registro de universo para el session_id
        tx.execute("""
            INSERT INTO universes (id, user_id, name, description, genre, format, session_id, is_demo_template, created_at, updated_at)
            VALUES (?, ?, ?, ?, ?, ?, ?, FALSE, NOW(), NOW())
        """, new_universe_id, template.user_id, template.name, template.description, template.genre, template.format, session_id)
        
        # 3. Crear el nuevo grafo en Apache AGE
        tx.execute("SELECT create_graph(?)", new_graph_name)
        
        # 4. Clonar obras (Works)
        works_mapping = {}  # old_work_id -> new_work_id
        template_works = tx.query("SELECT * FROM works WHERE universe_id = ?", template.id)
        for w in template_works:
            new_work_id = uuid.new()
            works_mapping[w.id] = new_work_id
            tx.execute("""
                INSERT INTO works (id, universe_id, title, type, order_index, synopsis, status, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
            """, new_work_id, new_universe_id, w.title, w.type, w.order_index, w.synopsis, w.status)
            
        # 5. Clonar capítulos (Chapters)
        chapters_mapping = {}  # old_chapter_id -> new_chapter_id
        for old_work_id, new_work_id in works_mapping.items():
            template_chapters = tx.query("SELECT * FROM chapters WHERE work_id = ?", old_work_id)
            for c in template_chapters:
                new_chapter_id = uuid.new()
                chapters_mapping[c.id] = new_chapter_id
                tx.execute("""
                    INSERT INTO chapters (id, work_id, title, order_index, content, raw_text, word_count, status, analyzed_at, created_at, updated_at)
                    VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
                """, new_chapter_id, new_work_id, c.title, c.order_index, c.content, c.raw_text, c.word_count, c.status, c.analyzed_at)
                
        # 6. Clonar entidades (Entities)
        entities_mapping = {}  # old_entity_id -> new_entity_id
        template_entities = tx.query("SELECT * FROM entities WHERE universe_id = ?", template.id)
        for e in template_entities:
            new_entity_id = uuid.new()
            entities_mapping[e.id] = new_entity_id
            tx.execute("""
                INSERT INTO entities (id, universe_id, name, type, description, aliases, properties, status, relevance_score, last_mentioned_chapter_id, last_mentioned_at, created_at, updated_at)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())
            """, new_entity_id, new_universe_id, e.name, e.type, e.description, e.aliases, e.properties, e.status, e.relevance_score, chapters_mapping.get(e.last_mentioned_chapter_id), e.last_mentioned_at)
            
            # 6b. Clonar sus embeddings (pgvector)
            emb = tx.query_one("SELECT * FROM entity_embeddings WHERE entity_id = ?", e.id)
            if emb:
                tx.execute("""
                    INSERT INTO entity_embeddings (id, entity_id, description_embedding, created_at, updated_at)
                    VALUES (?, ?, ?, NOW(), NOW())
                """, uuid.new(), new_entity_id, emb.description_embedding)

        # 7. Clonar embeddings de párrafos (PARAGRAPH_EMBEDDINGS)
        for old_ch_id, new_ch_id in chapters_mapping.items():
            ch_embeddings = tx.query("SELECT * FROM paragraph_embeddings WHERE chapter_id = ?", old_ch_id)
            for pe in ch_embeddings:
                tx.execute("""
                    INSERT INTO paragraph_embeddings (id, chapter_id, paragraph_index, paragraph_node_id, content, embedding, created_at)
                    VALUES (?, ?, ?, ?, ?, ?, NOW())
                """, uuid.new(), new_ch_id, pe.paragraph_index, pe.paragraph_node_id, pe.content, pe.embedding)
                
        # 8. Clonar menciones (ENTITY_MENTIONS)
        for old_ent_id, new_ent_id in entities_mapping.items():
            mentions = tx.query("SELECT * FROM entity_mentions WHERE entity_id = ?", old_ent_id)
            for m in mentions:
                tx.execute("""
                    INSERT INTO entity_mentions (id, entity_id, chapter_id, paragraph_index, paragraph_node_id, context_snippet, mention_type, created_at)
                    VALUES (?, ?, ?, ?, ?, ?, ?, NOW())
                """, uuid.new(), new_ent_id, chapters_mapping[m.chapter_id], m.paragraph_index, m.paragraph_node_id, m.context_snippet, m.mention_type)

        # 9. Clonar el grafo de Apache AGE
        # Copiar todos los nodos (Character, Place, etc.) y relaciones (ALLY_OF, etc.)
        # traduciendo el id de entidad original al nuevo id clonado
        old_graph_name = f"universe_{strings.replace(template.id, '-', '')}"
        
        # 9a. Copiar nodos
        # IMPORTANTE: Todos los valores de texto se escapan con agtype.escape_cypher()
        # para evitar inyección y roturas por apóstrofes en nombres de entidades
        # (e.g. "D'Angelo", "Vareth's Blade"). Esta función duplica comillas simples
        # y escapa backslashes. Ver pkg/agtype/escape.go.
        nodos_vertices = tx.query("SELECT * FROM cypher(?, $$ MATCH (v) RETURN v $$) AS (v agtype)", old_graph_name)
        for row in nodos_vertices:
            v = agtype_parser.parse_vertex(row.v)
            old_eid = v.properties.get("entity_id")
            new_eid = entities_mapping.get(old_eid)
            if new_eid:
                escaped_name = agtype.escape_cypher(v.properties.get("name", ""))
                escaped_status = agtype.escape_cypher(v.properties.get("status", "active"))
                score = v.properties.get("relevance_score", 0.8)
                cypher_query = f"CREATE (n:{v.label} {{entity_id: '{new_eid}', name: '{escaped_name}', status: '{escaped_status}', relevance_score: {score}}})"
                tx.execute("SELECT * FROM cypher(?, $$ " + cypher_query + " $$) AS (a agtype)", new_graph_name)

        # 9b. Copiar aristas (Edges)
        edges = tx.query("SELECT * FROM cypher(?, $$ MATCH (a)-[e]->(b) RETURN a.entity_id, type(e), e, b.entity_id $$) AS (a_id agtype, rel_type agtype, edge agtype, b_id agtype)", old_graph_name)
        for row in edges:
            old_a = agtype_parser.parse_scalar(row.a_id)
            old_b = agtype_parser.parse_scalar(row.b_id)
            new_a = entities_mapping.get(old_a)
            new_b = entities_mapping.get(old_b)
            r_type = agtype_parser.parse_scalar(row.rel_type)
            
            if new_a and new_b:
                # entity_id es UUID (sin apóstrofes), r_type es un enum validado
                cypher_query = f"MATCH (x {{entity_id: '{new_a}'}}), (y {{entity_id: '{new_b}'}}) CREATE (x)-[:{r_type}]->(y)"
                tx.execute("SELECT * FROM cypher(?, $$ " + cypher_query + " $$) AS (r agtype)", new_graph_name)

        tx.commit()
        return new_universe_id
    except Exception as e:
        tx.rollback()
        raise e
```

---

### 6.8 Algoritmo: Detección de Huecos Narrativos (Plot Holes)

```python
def scan_plot_holes(universe_id, recently_mentioned_entities=None):
    """
    Escanea arcos narrativos abiertos que no han tenido desarrollo reciente.
    Se ejecuta:
    - Después de cada análisis de párrafo (solo para entidades mencionadas)
    - Después de una ingesta completa (scan global)
    """
    
    # Obtener todos los arcos narrativos abiertos
    open_arcs = db.query("""
        SELECT e.*, 
               (SELECT MAX(c.order_index) FROM entity_mentions em 
                JOIN chapters c ON em.chapter_id = c.id
                WHERE em.entity_id = e.id) as last_mention_order,
               (SELECT MAX(c.order_index) FROM chapters c
                JOIN works w ON c.work_id = w.id
                WHERE w.universe_id = e.universe_id) as current_max_order
        FROM entities e
        WHERE e.universe_id = ?
        AND e.type = 'plot_arc'
        AND e.properties->>'arc_status' = 'open'
    """, universe_id)
    
    plot_holes = []
    
    STALE_THRESHOLD = 8  # Capítulos sin mención para considerar "olvidado"
    
    for arc in open_arcs:
        chapters_since = arc.current_max_order - arc.last_mention_order
        
        if chapters_since >= STALE_THRESHOLD:
            # Este arco no ha sido mencionado en STALE_THRESHOLD capítulos
            
            # Verificar si ya existe un plot_hole registrado para este arco
            existing_ph = db.query("""
                SELECT * FROM plot_holes 
                WHERE universe_id = ? AND ? = ANY(related_entity_ids) AND status = 'open'
            """, universe_id, arc.id)
            
            if not existing_ph:
                ph = PlotHole(
                    universe_id=universe_id,
                    title=arc.name,
                    description=f"'{arc.name}' was introduced {chapters_since} chapters ago but has not been developed or resolved since.",
                    related_entity_ids=[arc.id],
                    first_mentioned_chapter_id=get_first_mention_chapter(arc.id),
                    status="open"
                )
                save_plot_hole(ph)
                plot_holes.append(ph)
    
    return plot_holes
```

---

### 6.9 Algoritmo: Detección de Capítulos en Texto

```python
def split_into_chapters(text):
    """
    Divide texto largo en capítulos usando heurísticas + fallback a LLM.
    """
    
    # Patrones comunes de inicio de capítulo
    CHAPTER_PATTERNS = [
        r"^(?:Chapter|CHAPTER|Capítulo|CAPÍTULO)\s+\d+",          # "Chapter 1", "Capítulo 3"
        r"^(?:Chapter|CHAPTER|Capítulo|CAPÍTULO)\s+\w+",          # "Chapter One"
        r"^(?:Ch\.|Cap\.)\s*\d+",                                  # "Ch. 1", "Cap. 3"
        r"^#{1,2}\s+",                                              # Markdown headings
        r"^PART\s+\d+",                                             # "PART 1"
        r"^ACT\s+\d+",                                              # "ACT 1" (guiones)
        r"^INT\.\s|^EXT\.\s",                                      # Formato de guión cinematográfico
        r"^SCENE\s+\d+",                                            # "SCENE 1"
        r"^Episode\s+\d+",                                          # "Episode 1"
    ]
    
    lines = text.split("\n")
    chapter_boundaries = []
    
    # Paso 1: Buscar con patrones regex
    for i, line in enumerate(lines):
        stripped = line.strip()
        for pattern in CHAPTER_PATTERNS:
            if re.match(pattern, stripped, re.IGNORECASE):
                chapter_boundaries.append({
                    "line": i,
                    "title": stripped,
                    "method": "regex"
                })
                break
    
    # Paso 2: Si no se encontraron capítulos, usar LLM como fallback
    if len(chapter_boundaries) < 2:
        # Enviar muestra del texto a Qwen-Turbo para detectar boundaries
        sample = text[:10000]  # Primeros 10K chars como muestra
        llm_boundaries = qwen_turbo.detect_chapter_boundaries(sample)
        chapter_boundaries = llm_boundaries
    
    # Paso 3: Dividir el texto en los boundaries detectados
    chapters = []
    for i, boundary in enumerate(chapter_boundaries):
        start = boundary["line"]
        end = chapter_boundaries[i + 1]["line"] if i + 1 < len(chapter_boundaries) else len(lines)
        
        content = "\n".join(lines[start:end]).strip()
        chapters.append({
            "title": boundary["title"],
            "content": content,
            "order": i + 1
        })
    
    # Si no se detectó ningún boundary, tratar todo como un solo capítulo
    if not chapters:
        chapters = [{"title": "Chapter 1", "content": text, "order": 1}]
    
    return chapters
```

---

## 7. Qwen Cloud API — Especificación de Uso

### 7.1 Modelos Utilizados

| Modelo | Uso | Justificación |
|---|---|---|
| `qwen-max` | Detección de contradicciones, análisis de reglas del mundo, razonamiento complejo | Máxima capacidad de razonamiento para tareas que requieren comprensión profunda |
| `qwen-turbo` | Extracción de entidades, análisis de relaciones, detección de capítulos, resúmenes | Rápido y económico para tareas de extracción/clasificación |
| `text-embedding-v3` | Generación de embeddings para búsqueda semántica | Embeddings de alta calidad para pgvector |

### 7.2 Configuración del Cliente

```go
// Base URL para Qwen Cloud (compatible con OpenAI API)
QWEN_BASE_URL = "https://dashscope-intl.aliyuncs.com/compatible-mode/v1"

// Headers requeridos
Authorization: Bearer <QWEN_API_KEY>
Content-Type: application/json
```

### 7.3 Prompts del Sistema

#### Prompt: Extracción de Entidades (Qwen-Turbo)

```
System: You are a narrative analysis AI specializing in fiction. Your job is to extract 
ALL named entities and narrative elements from the given paragraph of a creative work.

You MUST respond with ONLY valid JSON, no markdown, no explanation.

User: 
Analyze the following paragraph from the universe "{universe_name}" ({genre}, {format}).

Context about this universe:
{universe_summary_or_recent_entities}

Paragraph:
"{paragraph_text}"

Extract entities in this exact JSON format:
{
  "characters": [
    {
      "name": "exact name as written",
      "aliases": ["any other names used"],
      "status": "active|deceased|unknown",
      "current_location": "if mentioned or implied, else null",
      "description": "brief description based on this paragraph",
      "role": "protagonist|major|supporting|minor|background",
      "properties": {
        "appearance": "if mentioned",
        "personality": "if mentioned",
        "abilities": ["if mentioned"]
      }
    }
  ],
  "places": [
    {
      "name": "exact name",
      "description": "brief description",
      "special_rules": ["any rules mentioned about this place"]
    }
  ],
  "events": [
    {
      "name": "short event title",
      "description": "what happened",
      "participants": ["character names involved"],
      "timeline_hint": "any temporal reference (e.g., 'three days later', 'during the eclipse')"
    }
  ],
  "factions": [
    {
      "name": "organization name",
      "description": "brief description",
      "members_mentioned": ["character names"]
    }
  ],
  "world_rules": [
    {
      "name": "rule name",
      "description": "what the rule establishes",
      "category": "magic|physics|social|political|biological",
      "exceptions": ["any exceptions mentioned"]
    }
  ],
  "plot_developments": [
    {
      "arc_name": "name of the plot thread",
      "description": "what developed",
      "status": "open|progressing|resolved"
    }
  ]
}

If no entities of a type are found, return an empty array for that type.
Only extract what is explicitly stated or strongly implied. Do NOT invent information.
```

#### Prompt: Detección de Contradicciones (Qwen-Max)

```
System: You are an expert continuity checker for fiction writing. Your job is to determine 
if a NEW piece of text contradicts ESTABLISHED facts from earlier in the story.

Be precise: only flag TRUE contradictions, not stylistic choices or intentional ambiguity.

User:
Universe: "{universe_name}"

ESTABLISHED FACT (from {chapter_title}, {work_title}):
"{evidence_a}"

Entity state in knowledge graph:
- Name: {entity_name}
- Type: {entity_type}  
- Status: {entity_status}
- Current location: {entity_location}
- Key relationships: {entity_relationships}
- Applicable world rules: {applicable_rules}

NEW TEXT (being written now, {current_chapter_title}):
"{evidence_b}"

Analyze for contradictions. Respond in this exact JSON format:
{
  "is_contradiction": true/false,
  "severity": "critical|warning|info",
  "explanation": "Clear explanation of why this is or is not a contradiction",
  "suggestion": "If it IS a contradiction, suggest how the author could resolve it. If not, null.",
  "affected_facts": ["list of established facts that are contradicted"]
}

Severity guide:
- critical: Direct logical impossibility (dead character alive, destroyed place intact)
- warning: Strong inconsistency that breaks immersion (wrong location, wrong relationship)
- info: Minor detail mismatch (eye color, minor timeline imprecision)
```

#### Prompt: Análisis de Relaciones (Qwen-Turbo)

```
System: You are a narrative relationship analyzer. Given a paragraph and a list of entities 
mentioned in it, identify the relationships between them.

User:
Paragraph: "{paragraph_text}"

Entities mentioned:
{entity_list_with_types}

Known relationships from previous text:
{existing_relationships}

Identify NEW or CHANGED relationships. Respond in JSON:
{
  "relationships": [
    {
      "source": "entity name",
      "target": "entity name", 
      "type": "ALLY_OF|ENEMY_OF|PARENT_OF|CHILD_OF|LOVER_OF|MENTOR_OF|MEMBER_OF|LOCATED_AT|PARTICIPATED_IN|CONTROLS|ALLIED_WITH|AT_WAR_WITH",
      "is_new": true/false,
      "has_changed": true/false,
      "previous_type": "if changed, what was it before",
      "properties": {
        "since_chapter": "{current_chapter_title}",
        "reason": "brief reason if applicable"
      }
    }
  ]
}

Only include relationships that are explicitly stated or strongly implied in the paragraph.
```

---

## 8. Requisitos No Funcionales Detallados

### [NF-001] Rendimiento de Análisis de Párrafo

| Métrica | Objetivo | Cómo medir |
|---|---|---|
| Tiempo total de análisis | < 5 segundos (p95) | Timestamp inicio → último mensaje WS enviado |
| Extracción de entidades | < 1.5 segundos | Llamada a Qwen-Turbo |
| Búsqueda vectorial | < 200ms | Query a pgvector |
| Búsqueda en grafo | < 300ms | Query Cypher a AGE |
| Detección de contradicciones | < 3 segundos | Llamada a Qwen-Max |

### [NF-002] Rendimiento de Ingesta

| Tamaño del documento | Objetivo |
|---|---|
| 10,000 palabras (~40 páginas) | < 2 minutos |
| 50,000 palabras (~200 páginas) | < 5 minutos |
| 100,000 palabras (~400 páginas) | < 12 minutos |

### [NF-003] Concurrencia

| Métrica | Objetivo |
|---|---|
| Usuarios simultáneos | Hasta 20 (suficiente para demo) |
| WebSocket connections | Hasta 50 |
| Análisis de párrafo concurrentes | Hasta 10 (queue con goroutines) |

### [NF-004] Disponibilidad

| Métrica | Objetivo |
|---|---|
| Uptime durante judging period | 99% (Jul 10-31) |
| Recovery time de crash | < 5 minutos (Docker restart) |

### [NF-005] Seguridad

| Requisito | Implementación |
|---|---|
| [NF-005a] Password hashing | bcrypt con cost=12 |
| [NF-005b] JWT expiration | 24 horas |
| [NF-005c] JWT signing | HMAC-SHA256 |
| [NF-005d] Data isolation | Middleware verifica user_id en cada request a recursos |
| [NF-005e] API key protection | QWEN_API_KEY solo en env vars del backend, nunca expuesta |
| [NF-005f] Input sanitization | Validar y sanitizar todo input del usuario antes de guardar |
| [NF-005g] SQL injection | Prepared statements / parameterized queries siempre |
| [NF-005h] XSS | TipTap sanitiza HTML internamente; no renderizar raw HTML del usuario |
| [NF-005i] Secrets en repo público | `.env` en `.gitignore`, `.env.example` sin valores reales, verificar pre-push que no se filtre `QWEN_API_KEY` |

### [NF-006] Health & Monitoring

| Requisito | Implementación |
|---|---|
| [NF-006a] Health endpoint | `GET /api/v1/health` retorna status de DB, AGE, pgvector, Qwen API, disco libre, uptime |
| [NF-006b] Monitoreo externo | UptimeRobot (gratuito) monitorea el health endpoint cada 5 minutos y envía alerta por email si cae |
| [NF-006c] Health response | JSON con `status`, `db`, `age`, `pgvector`, `qwen_api`, `disk_free_mb`, `uptime_seconds` |

**Health Endpoint Response:**
```json
{
  "status": "healthy",
  "db": "connected",
  "age": "loaded",
  "pgvector": "available",
  "qwen_api": "reachable",
  "disk_free_mb": 4200,
  "uptime_seconds": 345600
}
```

### [NF-007] Resiliencia de WebSocket

| Requisito | Implementación |
|---|---|
| [NF-007a] Reconexión automática | Backoff exponencial: 1s, 2s, 4s, 8s, máx 30s |
| [NF-007b] Re-fetch al reconectar | Al reconectar, el frontend hace re-fetch vía REST del estado actual (entidades, contradicciones abiertas) |
| [NF-007c] Indicador visual | 🟢 Conectado / 🟡 Reconectando... / 🔴 Desconectado (clic para reintentar) |
| [NF-007d] Tab en background | Reconexión automática al volver a la pestaña si el WS se cerró |

### [NF-008] Rate Limiting de Qwen API

| Requisito | Implementación |
|---|---|
| [NF-008a] Semaphore global Qwen-Max | Máximo 3 llamadas concurrentes globalmente (semaphore/channel) |
| [NF-008b] Semaphore global Qwen-Turbo | Máximo 5 llamadas concurrentes globalmente |
| [NF-008c] Retry con backoff | Si la API retorna 429, retry automático con backoff exponencial (1s, 2s, 4s) hasta 3 intentos |
| [NF-008d] Métricas de uso | Log de tokens consumidos y latencia por llamada para monitorear costos |

### [NF-009] Degradación Graceful

| Requisito | Implementación |
|---|---|
| [NF-009a] Qwen API caída | Frontend muestra banner "AI analysis temporarily unavailable". El editor sigue funcionando normalmente. |
| [NF-009b] AGE extensión no disponible | El backend captura errores del grafo para evitar caídas. Si AGE no responde, se desactiva el Knowledge Graph visual y las relaciones en el panel lateral, pero el editor, el CRUD de entidades y las búsquedas semánticas siguen funcionando normalmente. |
| [NF-009c] WebSocket caído | El frontend usa polling REST como fallback para operaciones críticas. |

---


## 9. Configuración y Variables de Entorno

```bash
# .env.example

# Database
DATABASE_URL=postgres://quill:password@localhost:5432/quill?sslmode=disable
DB_MAX_CONNECTIONS=25
DB_MAX_IDLE_CONNECTIONS=5

# Qwen Cloud
QWEN_API_KEY=sk-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
QWEN_BASE_URL=https://dashscope-intl.aliyuncs.com/compatible-mode/v1
QWEN_MAX_MODEL=qwen-max-latest
QWEN_TURBO_MODEL=qwen-turbo-latest
QWEN_EMBEDDING_MODEL=text-embedding-v3
QWEN_EMBEDDING_DIMENSIONS=1024

# Auth
JWT_SECRET=your-256-bit-secret-here
JWT_EXPIRATION_HOURS=24
BCRYPT_COST=12

# Server
PORT=8080
FRONTEND_URL=http://localhost:3000
ALLOWED_ORIGINS=http://localhost:3000

# File Upload
MAX_UPLOAD_SIZE_MB=10
UPLOAD_DIR=./uploads

# Analysis
DEBOUNCE_SECONDS=5
RELEVANCE_DECAY_LAMBDA=0.1
ARCHIVE_THRESHOLD=0.15
REACTIVATION_SCORE=0.8
PLOT_HOLE_STALE_THRESHOLD=8
MAX_CONTRADICTION_CANDIDATES=3

# Qwen Rate Limiting
QWEN_MAX_CONCURRENCY=3
QWEN_TURBO_CONCURRENCY=5
QWEN_RETRY_MAX_ATTEMPTS=3

# Migrations (golang-migrate)
MIGRATIONS_DIR=./migrations
```


---

## 10. Estructura de Carpetas del Frontend (Detallada)

### 10.1 Estado Global (Zustand Stores)

```javascript
// authStore.js
{
  user: { id, email, display_name } | null,
  token: string | null,
  isAuthenticated: boolean,
  login(email, password): Promise<void>,
  register(email, password, displayName): Promise<void>,
  logout(): void
}

// universeStore.js
{
  universes: Universe[],
  currentUniverse: Universe | null,
  currentWork: Work | null,
  currentChapter: Chapter | null,
  fetchUniverses(): Promise<void>,
  createUniverse(data): Promise<Universe>,
  selectUniverse(id): void,
  selectWork(id): void,
  selectChapter(id): void,
}

// editorStore.js
{
  content: string,           // HTML content del editor
  rawText: string,           // Texto plano
  wordCount: number,
  isSaving: boolean,
  lastSavedAt: Date | null,
  isAnalyzing: boolean,
  setContent(content, rawText): void,
  saveContent(): Promise<void>,
}

// memoryStore.js
{
  contextualMemories: MemoryRecall[],
  contradictions: Contradiction[],
  plotHoles: PlotHole[],
  discoveredEntities: Entity[],     // Entidades descubiertas en esta sesión
  graphUpdates: GraphUpdate[],      // Actualizaciones pendientes del grafo
  timelineIssues: TimelineIssue[],
  
  // Acciones disparadas por WebSocket
  addContextualMemory(memory): void,
  addContradiction(contradiction): void,
  addPlotHole(plotHole): void,
  addEntity(entity): void,
  updateGraph(update): void,
  addTimelineIssue(issue): void,
  
  // Acciones del usuario
  dismissContradiction(id): Promise<void>,
  resolveContradiction(id, note): Promise<void>,
  resolvePlotHole(id): Promise<void>,
  clearSessionData(): void,
}
```

### 10.2 Rutas del Frontend

```javascript
// React Router v6 routes
/login                     → LoginPage
/register                  → RegisterPage (si se separa)
/dashboard                 → DashboardPage (lista de universos)
/universe/:universeId      → UniversePage (obras + stats del universo)
/editor/:chapterId         → EditorPage (editor + panel lateral)
/graph/:universeId         → KnowledgeGraphPage (grafo fullscreen)
/timeline/:universeId      → TimelinePage (línea temporal fullscreen)
```

---

## Apéndice A: Queries Cypher para Apache AGE

```sql
-- Crear un nodo Character
SELECT * FROM cypher('universe_<uuid>', $$
    CREATE (c:Character {
        id: '<entity-uuid>',
        name: 'Lord Vareth',
        status: 'active',
        current_location: 'Thendar Forest',
        role: 'major',
        relevance_score: 0.78
    })
    RETURN c
$$) AS (result agtype);

-- Crear una relación
SELECT * FROM cypher('universe_<uuid>', $$
    MATCH (a:Character {id: '<id-1>'}), (b:Character {id: '<id-2>'})
    CREATE (a)-[:ENEMY_OF {since_chapter: 'Chapter 5', reason: 'Betrayal'}]->(b)
$$) AS (result agtype);

-- Obtener todos los vecinos de una entidad (1 nivel)
SELECT * FROM cypher('universe_<uuid>', $$
    MATCH (n {id: '<entity-uuid>'})-[r]-(m)
    RETURN n, type(r) AS rel_type, properties(r) AS rel_props, m
$$) AS (n agtype, rel_type agtype, rel_props agtype, m agtype);

-- Obtener el grafo completo
SELECT * FROM cypher('universe_<uuid>', $$
    MATCH (n)
    OPTIONAL MATCH (n)-[r]->(m)
    RETURN n, r, m
$$) AS (n agtype, r agtype, m agtype);

-- Actualizar relevance_score de un nodo
SELECT * FROM cypher('universe_<uuid>', $$
    MATCH (n {id: '<entity-uuid>'})
    SET n.relevance_score = 0.45
    RETURN n
$$) AS (result agtype);

-- Buscar reglas del mundo que aplican a un personaje
SELECT * FROM cypher('universe_<uuid>', $$
    MATCH (r:WorldRule)-[:AFFECTS]->(c:Character {id: '<char-uuid>'})
    RETURN r
$$) AS (result agtype);

-- Buscar reglas del mundo que aplican en un lugar
SELECT * FROM cypher('universe_<uuid>', $$
    MATCH (r:WorldRule)-[:APPLIES_TO]->(p:Place {id: '<place-uuid>'})
    RETURN r
$$) AS (result agtype);

-- Obtener path entre dos personajes (máximo 3 saltos)
SELECT * FROM cypher('universe_<uuid>', $$
    MATCH path = shortestPath(
        (a:Character {id: '<id-1>'})-[*..3]-(b:Character {id: '<id-2>'})
    )
    RETURN path
$$) AS (result agtype);
```

---

## Apéndice B: Queries pgvector

```sql
-- Buscar párrafos semánticamente similares
SELECT pe.content, pe.chapter_id, c.title, w.title as work_title,
       pe.embedding <=> $1 AS distance
FROM paragraph_embeddings pe
JOIN chapters c ON pe.chapter_id = c.id
JOIN works w ON c.work_id = w.id
WHERE w.universe_id = $2
ORDER BY pe.embedding <=> $1 ASC
LIMIT 5;

-- Buscar entidades por descripción similar
SELECT e.*, ee.description_embedding <=> $1 AS distance
FROM entities e
JOIN entity_embeddings ee ON e.id = ee.entity_id
WHERE e.universe_id = $2
  AND ee.description_embedding <=> $1 < 0.3
ORDER BY distance ASC
LIMIT 10;

-- Buscar párrafos que contradigan potencialmente un hecho
-- (alta similitud semántica pero de capítulos diferentes)
SELECT pe.content, pe.chapter_id, c.title,
       pe.embedding <=> $1 AS distance
FROM paragraph_embeddings pe
JOIN chapters c ON pe.chapter_id = c.id
WHERE c.work_id IN (SELECT id FROM works WHERE universe_id = $2)
  AND pe.chapter_id != $3
  AND pe.embedding <=> $1 < 0.3
ORDER BY distance ASC
LIMIT 5;
```

---

## Apéndice C: Docker Compose Detallado

```yaml
version: "3.9"

services:
  # ════════════════════════════════════════════
  # PostgreSQL con pgvector + Apache AGE
  # ════════════════════════════════════════════
  postgres:
    build:
      context: ./docker/postgres
      dockerfile: Dockerfile
    # Custom Dockerfile que instala pgvector y AGE sobre PostgreSQL 16
    environment:
      POSTGRES_DB: quill
      POSTGRES_USER: quill
      POSTGRES_PASSWORD: ${DB_PASSWORD:-quill_dev_password}
    ports:
      - "5432:5432"
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./backend/migrations:/docker-entrypoint-initdb.d
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U quill"]
      interval: 10s
      timeout: 5s
      retries: 5
    restart: unless-stopped

  # ════════════════════════════════════════════
  # Go Backend API
  # ════════════════════════════════════════════
  backend:
    build:
      context: ./backend
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    environment:
      DATABASE_URL: postgres://quill:${DB_PASSWORD:-quill_dev_password}@postgres:5432/quill?sslmode=disable
      QWEN_API_KEY: ${QWEN_API_KEY}
      QWEN_BASE_URL: https://dashscope-intl.aliyuncs.com/compatible-mode/v1
      QWEN_MAX_MODEL: qwen-max-latest
      QWEN_TURBO_MODEL: qwen-turbo-latest
      QWEN_EMBEDDING_MODEL: text-embedding-v3
      QWEN_EMBEDDING_DIMENSIONS: "1024"
      JWT_SECRET: ${JWT_SECRET:-dev-secret-change-in-production}
      PORT: "8080"
      FRONTEND_URL: ${FRONTEND_URL:-http://localhost:3000}
      ALLOWED_ORIGINS: ${FRONTEND_URL:-http://localhost:3000}
      DEBOUNCE_SECONDS: "5"
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - uploads:/app/uploads
    restart: unless-stopped

  # ════════════════════════════════════════════
  # React Frontend (Nginx)
  # ════════════════════════════════════════════
  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
      args:
        VITE_API_URL: ${API_URL:-http://localhost:8080}
        VITE_WS_URL: ${WS_URL:-ws://localhost:8080}
    ports:
      - "3000:80"
    depends_on:
      - backend
    restart: unless-stopped

volumes:
  pgdata:
  uploads:
```

### Dockerfile para PostgreSQL Custom

```dockerfile
# docker/postgres/Dockerfile
FROM apache/age:release_PG16_1.6.0

# Instalar pgvector sobre la imagen oficial de Apache AGE (PostgreSQL 16)
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential \
        postgresql-server-dev-16 \
        git \
        ca-certificates && \
    git clone --branch v0.8.2 https://github.com/pgvector/pgvector.git /tmp/pgvector && \
    cd /tmp/pgvector && \
    make && \
    make install && \
    cd / && rm -rf /tmp/pgvector && \
    apt-get remove -y build-essential git && \
    apt-get autoremove -y && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Apache AGE ya viene pre-cargado y pre-configurado en la imagen base.
```

---

## Apéndice D: Estimación de Consumo de Qwen Cloud API

### Cálculos Estimados (con $40 de créditos)

| Operación | Modelo | Tokens/llamada (aprox) | Costo/llamada (aprox) |
|---|---|---|---|
| Extracción de entidades | Qwen-Turbo | ~800 input + ~400 output | ~$0.0005 |
| Detección de contradicciones | Qwen-Max | ~1500 input + ~300 output | ~$0.005 |
| Análisis de relaciones | Qwen-Turbo | ~600 input + ~300 output | ~$0.0004 |
| Generación de embedding | text-embedding-v3 | ~200 input | ~$0.0001 |
| Detección de capítulos | Qwen-Turbo | ~2000 input + ~200 output | ~$0.001 |

### Escenario: Ingesta de un libro de 50,000 palabras (~15 capítulos)

| Operación | Cantidad | Costo estimado |
|---|---|---|
| Extracción de entidades (por párrafo ~300 párrafos) | 300 | ~$0.15 |
| Embeddings (por párrafo) | 300 | ~$0.03 |
| Análisis de relaciones (~100 párrafos con múltiples entidades) | 100 | ~$0.04 |
| Detección de contradicciones (~50 pares potenciales) | 50 | ~$0.25 |
| **Total por libro** | | **~$0.47** |

### Escenario: Sesión de escritura (2 horas, ~20 párrafos)

| Operación | Cantidad | Costo estimado |
|---|---|---|
| Extracción de entidades | 20 | ~$0.01 |
| Embeddings | 20 | ~$0.002 |
| Contradicciones | 10 | ~$0.05 |
| Recall contextual (resúmenes) | 20 | ~$0.01 |
| **Total por sesión** | | **~$0.07** |

### Presupuesto Total Estimado

| Fase | Uso | Costo |
|---|---|---|
| Desarrollo + testing | ~100 sesiones de prueba | ~$7 |
| Demo saga fantasía (ingesta 3 capítulos) | 1 ingesta + 5 sesiones | ~$1 |
| Demo en vivo (video) | 2-3 intentos | ~$0.50 |
| Buffer para judging (que los jueces prueben) | ~20 sesiones | ~$1.50 |
| **Total estimado** | | **~$10** |
| **Créditos disponibles** | | **$40** |
| **Margen de seguridad** | | **$30 (75%)** |

> [!TIP]
> Con $40 de créditos hay más que suficiente margen. El diseño multi-modelo (Turbo para tareas rápidas, Max solo para contradicciones) optimiza el costo significativamente.
