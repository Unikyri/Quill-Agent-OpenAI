# Quill — Product Requirements Document (PRD)

> **Versión:** 1.0  
> **Última actualización:** 2026-06-28  
> **Track:** MemoryAgent | **Hackathon:** Global AI Hackathon Series with Qwen Cloud  
> **Deadline de submission:** Jul 9, 2026 (2:00 PM PT)

---

## 1. Visión del Producto

### 1.1 Declaración de Visión

**Quill** es un IDE para escritores creativos con memoria persistente impulsado por IA. Funciona como un "co-piloto narrativo" que **recuerda cada detalle de tu universo ficticio** — personajes, lugares, eventos, reglas y tramas — para que el escritor nunca tenga que volver a releer su propia obra ni mantener wikis manuales.

> *"Quill recuerda tu historia mejor que tú."*

### 1.2 Propuesta de Valor Única

Los editores de texto actuales (Word, Google Docs, Scrivener) son **agnósticos al contenido**: no entienden que "Aragorn" es un personaje, que "Mordor" es un lugar, ni que en el capítulo 5 ese personaje murió. Quill es el primer editor que **comprende la narrativa** y construye automáticamente un Knowledge Graph vivo de todo el universo del escritor.

### 1.3 Elevator Pitch

> Para escritores de ficción y guionistas que luchan por mantener la consistencia de sus historias a lo largo de múltiples libros, temporadas o volúmenes, **Quill** es un editor de escritura inteligente que automáticamente extrae, recuerda y relaciona cada personaje, lugar, evento y regla de tu universo, alertándote en tiempo real cuando algo contradice lo que ya escribiste. A diferencia de Scrivener o Google Docs, Quill construye un Knowledge Graph vivo de tu historia y se vuelve más útil con cada capítulo que escribes.

---

## 2. Contexto del Problema

### 2.1 El Problema

Los escritores de ficción larga (sagas, series de TV, mangas) enfrentan un desafío creciente: **a medida que su obra crece, su capacidad de recordar todos los detalles disminuye**. Esto provoca:

- **Contradicciones narrativas** que los lectores notan (personajes resucitados, ciudades que cambian de ubicación)
- **Tramas abandonadas** que nunca se resolvieron
- **Inconsistencias temporales** que rompen la inmersión
- **Horas perdidas** releyendo capítulos anteriores antes de poder escribir

### 2.2 Soluciones Actuales y sus Limitaciones

| Solución Actual | Limitación |
|---|---|
| Wikis manuales (Notion, Fandom) | Se desactualizan rápido, requieren mantenimiento constante, no están integradas al editor |
| Hojas de cálculo (Excel, Sheets) | No entienden relaciones entre entidades, no alertan sobre contradicciones |
| Scrivener (corkboard, fichas) | Fichas estáticas que el autor debe mantener manualmente, sin AI |
| Releer la obra antes de escribir | No escala — releer 5 libros cada vez que empiezas el 6to es impráctico |
| Memoria del propio autor | Falible — imposible recordar miles de detalles tras meses/años de escritura |

### 2.3 Oportunidad de Mercado

- **311 millones** de libros de ficción auto-publicados anualmente (fuentes: Bowker, ISBN reports)
- **Scrivener** tiene ~500K usuarios activos con un producto que no ha innovado significativamente en años
- **Crecimiento del manga independiente** (webtoons, tapas) con miles de creadores manteniendo arcos de +100 capítulos
- **Industria de guiones de series** donde la continuidad entre temporadas es un trabajo de equipo completo (script supervisors, show bibles)

---

## 3. User Personas

### 3.1 Persona Principal: Elena — Escritora Independiente de Fantasía

| Atributo | Detalle |
|---|---|
| **Nombre** | Elena Velasco |
| **Edad** | 32 años |
| **Ocupación** | Escritora self-published en Amazon KDP |
| **Género literario** | Fantasía épica, sagas de 5+ libros |
| **Herramientas actuales** | Google Docs + Notion (wiki de personajes) + hojas de Excel |
| **Experiencia técnica** | Media — usa apps pero no programa |
| **Frecuencia de escritura** | 4-5 días/semana, sesiones de 2-3 horas |

**Contexto:**
Elena está escribiendo el 4to libro de su saga de fantasía "Los Reinos de Aether". Tiene +80 personajes con nombre, 15 ciudades, un sistema de magia con reglas específicas, y 3 líneas temporales paralelas. Mantiene una wiki en Notion con fichas de personajes, pero dejó de actualizarla en el libro 2.

**Dolores (Pain Points):**
1. "No recuerdo si Lord Vareth era aliado o enemigo del Reino de Solaria en el libro 2"
2. "Un lector me dijo que un personaje que maté en el libro 1 apareció vivo en el libro 3. Me dio mucha vergüenza"
3. "Tengo un subplot sobre una profecía que introduje en el libro 1 y creo que nunca la resolví"
4. "Cada vez que empiezo un nuevo capítulo, pierdo 45 minutos releyendo mis notas y capítulos anteriores"
5. "Mi wiki de Notion tiene información del libro 1 y 2, pero nunca actualicé los libros 3 y 4"

**Objetivo:**
Quiere escribir más rápido y con más confianza, sabiendo que alguien (o algo) está vigilando la consistencia de su universo.

**Quote:**
> *"Necesito un asistente que conozca mi mundo tan bien como yo lo imaginé, pero que nunca olvide nada."*

---

### 3.2 Persona Secundaria: Marco — Guionista de Series

| Atributo | Detalle |
|---|---|
| **Nombre** | Marco Ruiz |
| **Edad** | 38 años |
| **Ocupación** | Guionista freelance para plataformas de streaming |
| **Género** | Thriller, drama, sci-fi para TV |
| **Herramientas actuales** | Final Draft + Google Sheets (character bible) |
| **Experiencia técnica** | Baja-media |
| **Trabajo** | En equipo (room de guionistas) pero escribe episodios solo |

**Contexto:**
Marco trabaja en la temporada 3 de un thriller de ciencia ficción. Hay 25+ personajes recurrentes, múltiples locaciones, y el showrunner le pide que mantenga absoluta continuidad con las temporadas anteriores. El "show bible" es un documento de 60 páginas que nadie mantiene actualizado.

**Dolores (Pain Points):**
1. "El showrunner me dijo que el personaje X no puede estar en esa ciudad porque en la temporada 1 se estableció que está prófugo de ahí"
2. "Escribí una escena donde un personaje usa un celular en una época donde aún no existían"
3. "Necesito saber exactamente en qué episodio se mencionó por primera vez la organización secreta para hacer un callback"
4. "El show bible tiene 60 páginas pero está desactualizado desde la temporada 1"

**Objetivo:**
Quiere evitar notas del showrunner que dicen "esto contradice lo que establecimos" y poder entregar guiones con continuidad impecable.

---

## 4. Alcance del Producto (Scope)

### 4.1 En Scope (MVP para Hackathon)

| # | Funcionalidad | Prioridad | Justificación |
|---|---|---|---|
| F1 | Editor de texto rico integrado | **P0 - Crítica** | Es la interfaz principal donde el escritor trabaja |
| F2 | Extracción automática de entidades | **P0 - Crítica** | Core del sistema de memoria — sin esto no hay Knowledge Graph |
| F3 | Knowledge Graph visual e interactivo | **P0 - Crítica** | El diferenciador visual más impactante para la demo |
| F4 | Detección de contradicciones en tiempo real | **P0 - Crítica** | La funcionalidad #1 que justifica el producto |
| F5 | Recall contextual en panel lateral | **P0 - Crítica** | Demuestra "recalling critical memories within limited context" |
| F6 | Sistema de decaimiento de relevancia | **P0 - Crítica** | Demuestra "timely forgetting" — requisito del track |
| F7 | Línea temporal inteligente | **P1 - Alta** | Detecta inconsistencias temporales, añade profundidad |
| F8 | Fichas de personajes autogeneradas | **P1 - Alta** | Muy visual en demo, elimina trabajo manual |
| F9 | Detector de huecos narrativos | **P1 - Alta** | Alto valor para el escritor, demuestra razonamiento complejo |
| F10 | Ingesta de documentos (PDF/DOCX/MD) | **P1 - Alta** | Permite importar obras existentes, esencial para onboarding |
| F11 | Jerarquía Universo > Obra > Capítulo | **P1 - Alta** | Organización del contenido |
| F12 | Autenticación simplificada | **P2 - Media** | Skip auth con usuario demo@quill.ai auto-logueado para la demo |
| F13 | Mapa de relaciones visual | **P1 - Alta** | Subconjunto del Knowledge Graph enfocado en relaciones |

### 4.2 Fuera de Scope (Post-Hackathon)

| Funcionalidad | Razón de exclusión |
|---|---|
| Colaboración en tiempo real (multi-autor) | Complejidad de sincronización demasiado alta para hackathon |
| Exportación a formatos de publicación (ePub, PDF estilizado) | Nice-to-have, no aporta al track MemoryAgent |
| Sugerencias de trama / continuación generativa | Se aleja del foco de memoria, puede parecer genérico |
| Soporte offline | Requiere sincronización compleja |
| Apps móviles nativas | Solo web para hackathon |
| Internacionalización (i18n) | La UI será en inglés para los jueces |
| Integración con otros editores (VS Code, Scrivener plugins) | Post-MVP |

---

## 5. User Stories con Criterios de Aceptación

### 5.1 Épica: Gestión de Universos y Obras

#### US-001: Crear un Universo
**Como** escritor independiente,  
**Quiero** crear un nuevo universo para mi saga,  
**Para** tener un espacio donde toda mi obra comparta un Knowledge Graph unificado.

**Criterios de Aceptación:**
```gherkin
Given que estoy autenticado y en el Dashboard
When hago clic en "New Universe" y relleno nombre, descripción, género y formato
Then se crea un universo vacío con un Knowledge Graph vacío
And aparece en mi lista de universos
And puedo acceder a él para crear obras dentro

Given que intento crear un universo sin nombre
When hago clic en "Create"
Then se muestra un error de validación "Universe name is required"
And el universo NO se crea
```

#### US-002: Crear una Obra dentro de un Universo
**Como** escritor independiente,  
**Quiero** crear un nuevo libro/guión dentro de mi universo,  
**Para** organizar mi saga en obras individuales que comparten el mismo mundo.

**Criterios de Aceptación:**
```gherkin
Given que estoy dentro de un universo
When creo una nueva obra con título, tipo (book/screenplay/manga_volume) y sinopsis
Then la obra aparece en la lista ordenada por order_index
And hereda el Knowledge Graph del universo

Given que creo una segunda obra en el mismo universo
When escribo sobre un personaje que apareció en la primera obra
Then el agente reconoce al personaje del Knowledge Graph compartido
```

#### US-003: Crear y Gestionar Capítulos
**Como** escritor independiente,  
**Quiero** crear capítulos dentro de mis obras,  
**Para** organizar mi escritura en secciones manejables.

**Criterios de Aceptación:**
```gherkin
Given que estoy dentro de una obra
When creo un nuevo capítulo con título opcional
Then se crea con order_index = (último capítulo + 1)
And se abre el editor de texto rico

Given que tengo múltiples capítulos
When reordeno un capítulo (drag & drop)
Then los order_index se actualizan correctamente
And el análisis de timeline se ajusta
```

---

### 5.2 Épica: Editor de Escritura

#### US-004: Escribir en el Editor Rico
**Como** escritor independiente,  
**Quiero** escribir en un editor WYSIWYG completo,  
**Para** tener una experiencia de escritura cómoda sin salir de Quill.

**Criterios de Aceptación:**
```gherkin
Given que estoy en un capítulo
When escribo texto en el editor
Then puedo usar formatting: bold, italic, headings (h1-h3), lists, block quotes
And el contenido se auto-guarda cada 30 segundos
And el word count se actualiza en la barra inferior

Given que escribo un párrafo y dejo de escribir por 5 segundos
When el debounce se activa
Then el párrafo se envía al backend vía WebSocket para análisis
And aparece un indicador sutil de "analizando..." en el panel lateral
```

#### US-005: Highlighting de Entidades en el Editor
**Como** escritor independiente,  
**Quiero** ver las entidades reconocidas resaltadas en mi texto,  
**Para** saber qué ha detectado el agente y poder hacer clic para ver más info.

**Criterios de Aceptación:**
```gherkin
Given que el agente ha analizado un párrafo y detectó entidades
When las entidades son reconocidas en el texto
Then los nombres de personajes se resaltan con color púrpura sutil
And los nombres de lugares se resaltan con color esmeralda sutil
And los eventos se resaltan con color dorado sutil
And el highlight es no-intrusivo (no distrae la escritura)

Given que hago clic en una entidad resaltada
When se abre un tooltip/popover
Then muestra: nombre, tipo, estado actual, y un link a la ficha completa
```

---

### 5.3 Épica: Sistema de Memoria (Core del Track MemoryAgent)

#### US-006: Extracción Automática de Entidades
**Como** escritor independiente,  
**Quiero** que Quill extraiga automáticamente personajes, lugares, eventos y demás entidades de mi texto,  
**Para** no tener que crear fichas manualmente.

**Criterios de Aceptación:**
```gherkin
Given que termino de escribir un párrafo (debounce de 5s)
When el backend procesa el texto en una cola secuencial con cancelación por contexto (context.WithCancel) usando Qwen-Turbo
Then si se envía otro párrafo antes de terminar, se cancela el análisis anterior
And extrae entidades con sus tipos: Character, Place, Event, Faction, WorldRule, PlotArc
And si la entidad ya existe en el Knowledge Graph, la VINCULA (no crea duplicado)
And si la entidad es nueva, la CREA con la información disponible
And si la entidad tiene alias conocido, lo resuelve (ej: "el Elegido" = "Aragorn")
And envía notificación vía WebSocket: "entity_discovered" o "entity_updated"

Given que el agente extrae una entidad que podría ser nueva o existente
When no está seguro (ej: "el rey" podría ser 3 personajes)
Then crea la entidad con confianza baja y la marca para revisión del autor
```

#### US-007: Recall Contextual en Tiempo Real
**Como** escritor independiente,  
**Quiero** ver información relevante sobre las entidades que estoy mencionando en mi texto actual,  
**Para** no tener que buscar manualmente qué establecí sobre ese personaje o lugar.

**Criterios de Aceptación:**
```gherkin
Given que estoy escribiendo un párrafo que menciona a "Lord Vareth"
When el agente detecta la entidad en el párrafo analizado
Then el panel lateral muestra en la sección "Recuerdos Relevantes":
  - Estado actual: "Vivo, exiliado del Reino de Solaria"
  - Última ubicación conocida: "Bosque de Thendar (Capítulo 12, Libro 2)"
  - Relaciones clave: "Enemigo de Rey Aldric, Mentor de Lyra"
  - Último evento relevante: "Fue traicionado por su lugarteniente (Cap. 8, Libro 3)"
  - Reglas del mundo que le afectan: "Los exiliados no pueden usar magia en territorio de Solaria"
And la información se ordena por relevance_score (más relevante primero)
And cada recuerdo tiene un indicador visual de "frescura" (barra de color que indica qué tan reciente es)

Given que menciono una entidad con relevance_score < 0.15 (archivada)
When el agente la detecta
Then la reactiva (score sube a 0.8)
And muestra un mensaje: "📌 Lord Vareth no aparecía desde el Capítulo 3 del Libro 1. Aquí está lo que sabemos..."
```

#### US-008: Detección de Contradicciones
**Como** escritor independiente,  
**Quiero** que Quill me alerte cuando lo que escribo contradice algo que establecí anteriormente,  
**Para** evitar errores de continuidad que arruinen la experiencia del lector.

**Criterios de Aceptación:**
```gherkin
Given que escribo "Lord Vareth entró al salón del trono de Solaria"
When el agente compara esto con el Knowledge Graph
And encuentra que Lord Vareth tiene status="exiliado de Solaria"
Then muestra una alerta en el panel lateral:
  ⚠️ CONTRADICCIÓN (Severidad: Warning)
  "Lord Vareth fue exiliado del Reino de Solaria en el Capítulo 5, Libro 2. 
  ¿Puede entrar al salón del trono?"
  Evidencia: [link al párrafo del Cap. 5, Libro 2]
  Sugerencia: "Considera explicar cómo pudo regresar (¿indulto? ¿infiltración?)"
And la alerta se registra en la tabla contradictions con status='open'

Given que escribo "Lyra lanzó un hechizo de fuego en la Catedral de Hielo"
When el agente compara con la regla del mundo: "La magia de fuego es imposible dentro de la Catedral de Hielo"
Then muestra una alerta:
  🔴 CONTRADICCIÓN (Severidad: Critical)
  "La regla del mundo 'Inmunidad al Fuego de la Catedral' (Capítulo 1, Libro 1) 
  establece que la magia de fuego no funciona dentro de la Catedral de Hielo."
  Evidencia: [link al párrafo original]

Given que una contradicción es intencional (el autor quiere que suceda)
When el autor hace clic en "Dismiss" en la alerta
Then la contradicción se marca como status='dismissed'
And no vuelve a aparecer para esa combinación de hechos (identificada por fingerprint: hash de entity_id + capítulo evidencia A + capítulo evidencia B)
And si el párrafo se re-analiza, el sistema verifica el fingerprint antes de crear una nueva contradicción
```

#### US-009: Sistema de Decaimiento de Relevancia (Timely Forgetting)
**Como** escritor independiente,  
**Quiero** que el agente priorice la información reciente y relevante sobre información antigua y poco mencionada,  
**Para** que el panel lateral no me abrume con cientos de detalles irrelevantes del capítulo 1 cuando estoy en el capítulo 30.

**Criterios de Aceptación:**
```gherkin
Given que un personaje secundario ("Tabernero de Rivenwold") fue mencionado solo en el Capítulo 2 del Libro 1
When estoy escribiendo el Capítulo 15 del Libro 3
Then su relevance_score ha decaído significativamente (< 0.15)
And aparece translúcido, con borde gris punteado y un icono de archivado en el Knowledge Graph
And NO aparece en el panel de recall contextual activo y se mueve a una sección colapsable de "Archivo" en el panel lateral
 
Given que menciono al "Tabernero de Rivenwold" en el Capítulo 15
When el agente detecta la mención
Then su relevance_score sube a 0.8 (reactivación)
And aparece en el panel lateral con un badge "📌 Reactivado"
And en el Knowledge Graph se activa un efecto visual de animación tipo pulso/glow y vuelve a ser opaco con borde normal

Given que el protagonista ("Lyra") es mencionado constantemente
When calculo su relevance_score
Then su score se mantiene alto (~0.95) gracias a:
  - base_importance alta (protagonista = 1.0)
  - recency_factor alto (mencionada recientemente)
  - mention_frequency_factor alto (mencionada muchas veces)
```

---

### 5.4 Épica: Knowledge Graph y Visualización

#### US-010: Ver el Knowledge Graph Interactivo
**Como** escritor independiente,  
**Quiero** ver un grafo visual de todas las entidades y relaciones de mi universo,  
**Para** entender de un vistazo la estructura de mi mundo y encontrar conexiones que no había notado.

**Criterios de Aceptación:**
```gherkin
Given que navego a la vista de Knowledge Graph de mi universo
When se carga el grafo
Then veo nodos coloreados por tipo:
  - Personajes: púrpura (#6c5ce7)
  - Lugares: esmeralda (#00b894)
  - Eventos: dorado (#f0c040)
  - Facciones: cyan (#00cec9)
  - Reglas del Mundo: rosa (#fd79a8)
  - Arcos Narrativos: lavanda (#a29bfe)
And los nodos tienen tamaño proporcional a su relevance_score
And las entidades archivadas (score < 0.15) aparecen translúcidas, con borde gris punteado y con un icono de archivado
And las edges muestran el tipo de relación como label

Given que hago clic en un nodo
When se abre la ficha detallada de la entidad
Then puedo ver y editar sus propiedades
And ver en qué capítulos se menciona
And ver sus relaciones directas

Given que quiero filtrar el grafo
When selecciono filtros por tipo de entidad
Then solo se muestran los nodos del tipo seleccionado y sus relaciones
```

#### US-011: Fichas de Personajes Autogeneradas
**Como** escritor independiente,  
**Quiero** que Quill genere y actualice automáticamente fichas de mis personajes,  
**Para** tener siempre una referencia actualizada sin trabajo manual.

**Criterios de Aceptación:**
```gherkin
Given que el agente ha extraído un personaje nuevo
When se crea la ficha
Then incluye (según la info disponible):
  - Nombre y aliases
  - Apariencia física (si se mencionó)
  - Personalidad/rasgos (si se mencionó)
  - Habilidades/poderes (si se mencionó)
  - Estado actual (vivo/muerto/desconocido)
  - Ubicación actual
  - Facciones a las que pertenece
  - Relaciones con otros personajes
  - Primera mención (capítulo + extracto)
  - Última mención (capítulo + extracto)
  - Indicador de relevancia

Given que escribo nueva información sobre un personaje existente
When el agente analiza el párrafo
Then la ficha se actualiza automáticamente con la nueva información
And se registra el cambio (ej: "ubicación cambió de X a Y en Cap. 15")
```

#### US-012: Línea Temporal Inteligente
**Como** escritor independiente,  
**Quiero** ver una cronología visual de los eventos de mi universo,  
**Para** verificar que la secuencia temporal tiene sentido.

**Criterios de Aceptación:**
```gherkin
Given que navego a la vista de Timeline
When se carga la línea temporal
Then veo eventos posicionados cronológicamente en un eje horizontal
And cada evento muestra: título, participantes, ubicación
And los eventos están coloreados por obra/libro
And puedo hacer zoom para ver más o menos detalle

Given que dos eventos tienen inconsistencia temporal
When el agente los detecta (ej: "la batalla de X fue ANTES de la muerte de Y, pero el texto dice lo contrario")
Then se muestra un indicador de warning en la línea temporal
And una alerta en el panel de contradicciones
```

---

### 5.5 Épica: Ingesta de Documentos

#### US-013: Subir una Obra Existente
**Como** escritor independiente que ya lleva 3 libros escritos,  
**Quiero** subir mis archivos PDF/DOCX/MD al sistema,  
**Para** que el agente tenga el contexto completo de toda mi obra anterior.

**Criterios de Aceptación:**
```gherkin
Given que estoy en la vista de una obra
When hago clic en "Import Document" y subo un archivo (.pdf, .docx, .md)
Then se inicia un job de ingesta asíncrono
And veo una barra de progreso con las fases:
  1. "Parsing document..." (10%)
  2. "Detecting chapters..." (20%)
  3. "Extracting entities from Chapter 1/N..." (20-80%)
  4. "Building knowledge graph..." (80-90%)
  5. "Generating embeddings..." (90-100%)
And puedo ver en tiempo real cómo se van descubriendo entidades (counter)
And el Knowledge Graph se va llenando progresivamente (animado)

Given que el documento tiene 50,000 palabras
When la ingesta se completa
Then todos los capítulos detectados están creados en la obra
And todas las entidades están en el Knowledge Graph
And las contradicciones internas (si existen) están listadas
And puedo ver la línea temporal de eventos extraídos
```

---

### 5.6 Épica: Detección de Huecos Narrativos

#### US-014: Detectar Tramas No Resueltas
**Como** escritor independiente,  
**Quiero** que Quill identifique subplots y misterios que introduje pero nunca resolví,  
**Para** no dejar hilos sueltos que frustren a mis lectores.

**Criterios de Aceptación:**
```gherkin
Given que en el Capítulo 3 escribí "La profecía del Oráculo anunció que un elegido destruiría el Cristal de las Eras"
When el agente analiza esta referencia
Then crea un PlotArc: "La Profecía del Oráculo" con status='open'
And lo registra en la línea temporal

Given que llevo 10 capítulos sin resolver o avanzar esa trama
When el agente escanea arcos narrativos abiertos
Then muestra en el panel:
  📖 TRAMA ABIERTA (10 capítulos sin avance)
  "La Profecía del Oráculo — introducida en Cap. 3, Libro 1. 
  No ha habido desarrollo desde entonces."
  Sugerencia: "Considera avanzar o resolver esta trama pronto."

Given que resuelvo la profecía escribiendo sobre ella
When el agente detecta el avance/resolución
Then actualiza el PlotArc a status='resolved'
And desaparece de la lista de tramas abiertas
```

---

## 6. Requisitos No Funcionales

### 6.1 Rendimiento

| Requisito | Métrica | Objetivo |
|---|---|---|
| Latencia de recall contextual | Tiempo desde fin del párrafo hasta mostrar recuerdos | < 3 segundos |
| Latencia de detección de contradicciones | Tiempo desde fin del párrafo hasta mostrar alertas | < 5 segundos |
| Tiempo de ingesta de documento | Tiempo total para un documento de 50K palabras | < 5 minutos |
| Auto-save del editor | Intervalo de guardado automático | Cada 30 segundos |
| Carga del Knowledge Graph | Tiempo para renderizar grafo con 200 nodos | < 2 segundos |

### 6.2 Escalabilidad

| Requisito | Límite MVP |
|---|---|
| Entidades por universo | Hasta 1,000 |
| Capítulos por obra | Hasta 100 |
| Obras por universo | Hasta 20 |
| Palabras por capítulo | Hasta 20,000 |
| Tamaño máximo de archivo para ingesta | 10 MB |
| Formatos de ingesta soportados | PDF, DOCX, MD |

### 6.3 Seguridad

| Requisito | Implementación |
|---|---|
| Autenticación | JWT con expiración de 24h |
| Passwords | Hash con bcrypt (cost=12) |
| Aislamiento de datos | Cada usuario solo ve sus propios universos |
| API key de Qwen | Almacenada en variables de entorno, nunca expuesta al frontend |

### 6.4 Usabilidad

| Requisito | Detalle |
|---|---|
| Responsive | Desktop-first, funcional en tablets (≥768px) |
| Dark mode | Único tema (dark), optimizado para escritura prolongada |
| Onboarding | Tutorial interactivo de primera vez (tooltip guide) |
| Idioma de la UI | Inglés (para los jueces del hackathon) |

---

## 7. Métricas de Éxito (KPIs)

### 7.1 Métricas de Efectividad de Memoria

| KPI | Cómo se mide | Objetivo Demo |
|---|---|---|
| **Contradicciones detectadas por capítulo** | Contador de alertas de contradicción generadas | ≥ 2 en la demo |
| **Entidades extraídas automáticamente** | Total de entidades creadas sin intervención manual | ≥ 30 en la saga de demo |
| **Precisión del recall** | % de recuerdos mostrados que son relevantes para el contexto | ≥ 80% |
| **Latencia de recall** | Tiempo promedio desde fin de párrafo hasta mostrar recuerdos | < 3 seg |
| **Entidades en Knowledge Graph** | Riqueza del modelo de mundo | ≥ 50 nodos, ≥ 80 relaciones |

### 7.2 Métricas de Demostración (para los Jueces)

| KPI | Objetivo |
|---|---|
| Contradicción detectada en vivo durante la demo | Sí — al menos 1 |
| Recall contextual preciso al mencionar un personaje | Sí — al menos 3 entidades |
| Decaimiento visible (entidad archivada vs. activa) | Sí — mostrar diferencia visual |
| Knowledge Graph con ≥ 30 nodos interconectados | Sí |
| Ingesta de documento con progreso visual | Sí — ≥ 1 documento |

---

## 8. Experiencia de Usuario (Flujos Principales)

### 8.1 Flujo: Primera Vez del Usuario (Onboarding de Demo Aislado)

```
1. Landing page → "Get Started / Try Demo"
2. El sistema detecta si el visitante tiene una sesión activa (X-Session-ID en localStorage).
3. Si es un visitante nuevo, genera un session_id y clona automáticamente la saga demo "Echoes of Eternity" (plantilla is_demo_template) en un universo aislado para evitar colisiones con otros jueces.
4. Redirección automática al Dashboard con la saga ya precargada, lista para escribir.
5. Si el juez lo desea, puede hacer clic en "Reset Demo" en el editor para restaurar el estado inicial limpio de la saga.
```

### 8.2 Flujo: Sesión de Escritura Típica

```
1. Auto-login (usuario demo@quill.ai) → Dashboard → seleccionar universo → seleccionar obra → seleccionar capítulo
2. Editor se abre con el contenido guardado
3. El escritor escribe un párrafo...
4. Pausa de 5 segundos (debounce)
5. El backend analiza el párrafo en cola secuencial cancelable:
   a. Extrae entidades → actualiza Knowledge Graph
   b. Busca recuerdos relevantes → actualiza panel lateral
   c. Compara con hechos conocidos → ¿contradicciones?
   d. Valida timeline → ¿inconsistencias temporales?
6. El panel lateral se actualiza en ~3-5 segundos:
   - Recuerdos relevantes aparecen con fade-in
   - Si hay contradicción: alerta con shake animation
7. El escritor continúa escribiendo... (ciclo se repite)
8. Periódicamente puede abrir el Knowledge Graph o la Timeline
```

### 8.3 Flujo: Detección de Contradicción

```
1. Escritor escribe: "Kael desenvainó la Espada del Alba"
2. Backend detecta: entidad "Kael" + entidad "Espada del Alba"
3. Backend busca en el grafo: 
   - Kael: status=active, location=Prisión de Thorn
   - Espada del Alba: owner=Lord Vareth, location=Cámara del Consejo
4. Dos contradicciones:
   a. Kael está en prisión → ¿cómo desenvaina una espada?
   b. La Espada del Alba la tiene Lord Vareth → ¿cómo la tiene Kael?
5. Panel lateral muestra alertas con evidencia y sugerencias
6. Escritor puede:
   a. Corregir el texto
   b. Dismiss la alerta (era intencional)
   c. Resolver (explicar cómo escapó / obtuvo la espada)
```

---

## 9. Requisitos del Hackathon (Checklist)

| Requisito | Cómo lo cumplimos | Estado |
|---|---|---|
| ✅ Usar Qwen models en Qwen Cloud | Multi-modelo: Qwen-Max + Qwen-Turbo + Embeddings | Planeado |
| ✅ Track MemoryAgent | Persistent memory, cross-session, timely forgetting | Core del producto |
| ✅ Código open source en repositorio público | GitHub repo con licencia MIT | Pendiente |
| ✅ Video demo ≤ 3 minutos en YouTube | Saga de fantasía pre-cargada + escritura en vivo | Pendiente |
| ✅ Proof of Alibaba Cloud Deployment | Docker Compose en ECS + link a código de Qwen API | Pendiente |
| ✅ Architecture Diagram | Mermaid diagram en README + imagen | En PRD/SRS |
| ✅ Descripción de funcionalidades | PRD + README | En progreso |
| ✅ URL de demo funcional | Deploy en ECS con URL pública | Pendiente |
| ⭐ Blog post (bonus) | Blog sobre el journey de construir Quill | Pendiente |

---

## 10. Cronograma Tentativo

> [!IMPORTANT]
> **Días restantes:** ~11 días (Jun 28 → Jul 9, 2026)

| Fase | Días | Entregables |
|---|---|---|
| **Fase 0: Setup** | Día 1 (Jun 28) | Repo, Docker Compose, DB migrations, proyecto Go + React inicializados |
| **Fase 1: Core Backend** | Días 2-4 (Jun 29 - Jul 1) | Auth, CRUD universos/obras/capítulos, QwenService, EntityService |
| **Fase 2: Memory Engine** | Días 5-7 (Jul 2-4) | ContradictionService, RelevanceService, TimelineService, PlotHoleService, WebSocket |
| **Fase 3: Frontend Core** | Días 5-7 (Jul 2-4) | Editor TipTap, Panel lateral, Dashboard, Auth pages (paralelo) |
| **Fase 4: Visualización** | Días 8-9 (Jul 5-6) | Knowledge Graph (React Flow), Timeline, Fichas de personajes |
| **Fase 5: Ingesta** | Día 8 (Jul 5) | Pipeline de ingesta PDF/DOCX/MD |
| **Fase 6: Deploy + Demo** | Días 10-11 (Jul 7-8) | Deploy ECS, saga de fantasía, video, blog post, README |
| **Buffer** | Día 12 (Jul 9 AM) | Últimos fixes antes del deadline (2 PM PT) |

---

## Apéndice A: Competidores y Diferenciación

| Producto | Qué hace | Qué le falta (que Quill tiene) |
|---|---|---|
| **Scrivener** | Organización de manuscritos, fichas de personajes manuales | Sin AI, sin detección de contradicciones, sin Knowledge Graph |
| **Campfire** | World-building tools, mapas, timelines | Sin editor integrado, sin análisis automático, sin memoria |
| **World Anvil** | Wiki para world-building | Sin editor de escritura, sin AI, wiki manual |
| **Sudowrite** | AI writing assistant | Se enfoca en generar texto, no en recordar/analizar consistencia |
| **NovelAI** | AI generativa para historias | Genera texto, no analiza consistencia ni construye Knowledge Graph |

**Diferenciador único de Quill:** Es el **único** producto que combina un editor de escritura con un sistema de memoria persistente que construye automáticamente un Knowledge Graph y detecta contradicciones en tiempo real.
