# Feature: fecha límite opcional en las tareas

Origen: issue #2 — https://github.com/criera-mgpsa/desarrollo-agentico-todo-03/issues/2 («Quiero añadir fecha límite a las tareas. Vale con que sea solo el día, sin hora.»). Plan escrito en ejecución desatendida desde `/adw`: las decisiones que la issue no fija se marcan como **inferencia**.

## Encargo y criterios de aceptación

Requisitos explícitos:

1. Una tarea puede tener una fecha límite.
2. La fecha límite es solo un día, sin hora.

Inferencias (decisiones tomadas sin poder preguntar):

3. La fecha límite es **opcional**: las tareas existentes y las nuevas sin fecha siguen siendo válidas.
4. Se puede indicar al **crear** la tarea (formulario de la lista) y **cambiar o quitar** al **editarla** (formulario de edición). Dejar el campo vacío significa «sin fecha límite».
5. La lista muestra la fecha límite de cada tarea que la tenga, en formato `dd/mm/aaaa`, con un elemento `<time datetime="aaaa-mm-dd">`.
6. Se admiten fechas pasadas (no se valida que sea futura) y la fecha no afecta al orden de la lista ni al estado completada/pendiente.
7. Un valor que no sea una fecha `aaaa-mm-dd` válida se rechaza con 422 y el mensaje «La fecha límite no es válida.», sin guardar nada.
8. Completar/reabrir una tarea conserva su fecha límite.

Criterios de aceptación:

- CA1: crear una tarea con `due_date=2026-11-15` la guarda con esa fecha y la lista muestra `15/11/2026`.
- CA2: crear una tarea sin `due_date` (o vacío) la guarda sin fecha y la lista no muestra ninguna fecha para ella.
- CA3: crear con `due_date` inválido (`31/12/2026`, `2026-02-30`, `mañana`) devuelve 422, muestra «La fecha límite no es válida.» y no crea la tarea.
- CA4: el formulario de edición muestra la fecha actual en un `<input type="date" name="due_date" value="aaaa-mm-dd">`.
- CA5: editar con otra fecha la cambia; editar con el campo vacío la quita; el título y el estado completada se siguen conservando como hasta ahora.
- CA6: editar con fecha inválida devuelve 422 con el mensaje y no cambia ni el título ni la fecha guardados.
- CA7: alternar completada conserva la fecha límite.
- CA8: el día guardado es exactamente el introducido (sin desplazamientos por zona horaria).

## Comportamiento actual y propuesta

Actual: `tasks` tiene `id`, `title`, `completed`, `created_at`, `updated_at` (`migrations/20260901000000_create_tasks.up.fizz`). El formulario de creación (`templates/tasks/index.plush.html`) solo pide el título y `TasksCreate` hace `c.Bind(task)`. `TasksUpdate` (`actions/tasks.go`) solo actualiza el título a partir de un struct de entrada propio. No existe ningún concepto de fecha límite.

Propuesta: nueva columna nullable `due_date` de tipo `date`; campo `DueDate *time.Time` en `models.Task`; un campo `<input type="date" name="due_date">` en los formularios de creación y edición; las acciones interpretan el valor con una función del modelo; la lista muestra la fecha bajo el título.

## Alcance

Incluido:

- Migración Fizz que añade `due_date` (nullable, `date`).
- Modelo: campo, parseo y formateo de la fecha.
- Acciones `TasksCreate` y `TasksUpdate`.
- Plantillas de lista y edición, y estilos mínimos en `public/assets/app.css`.
- Tests de modelo y de acciones.

Excluido:

- Resaltar tareas vencidas, ordenar o filtrar por fecha, recordatorios, notificaciones.
- Horas o zonas horarias.
- Cambiar la semilla `migrations/20260901000001_seed_tasks.up.sql` o `fixtures/tasks.toml` (las tareas sin fecha siguen siendo válidas).
- JavaScript (`public/assets/app.js`) y nuevas dependencias (`go.mod` no cambia).
- Despliegues.

Repos y módulos: un único repo (`todo`), paquetes `migrations`, `models`, `actions`, `templates`, `public`.

## Diseño por capas

### Persistencia y migraciones

Siguiendo la convención existente (Fizz, prefijo de timestamp, último `20260901000001`):

- `migrations/20260901000002_add_due_date_to_tasks.up.fizz`:
  ```
  add_column("tasks", "due_date", "date", {"null": true})
  ```
- `migrations/20260901000002_add_due_date_to_tasks.down.fizz`:
  ```
  drop_column("tasks", "due_date")
  ```

`migrations/embed.go` ya incluye `*.fizz`, no cambia.

### Modelo (`models/task.go`)

- Campo nuevo, tras `Completed`:
  ```go
  DueDate *time.Time `json:"due_date" db:"due_date" form:"-"`
  ```
  `form:"-"` para que `c.Bind` no lo toque: lo asigna la acción tras parsearlo. Se usa `*time.Time` (nil = sin fecha) en lugar de `nulls.Time` para no convertir `github.com/gobuffalo/nulls` en dependencia directa.
- Constantes:
  ```go
  // DueDateLayout es el formato de la fecha límite en formularios (input type="date").
  const DueDateLayout = "2006-01-02"

  // DueDateInvalidMessage es el error que se muestra si la fecha límite no se puede interpretar.
  const DueDateInvalidMessage = "La fecha límite no es válida."
  ```
- `func ParseDueDate(value string) (*time.Time, error)`: recorta espacios; vacío → `nil, nil`; si no, `time.Parse(DueDateLayout, value)` (devuelve medianoche UTC; `time.Parse` ya rechaza `2026-02-30`) y devuelve el puntero, o el error.
- Métodos con receptor por valor (para poder llamarlos desde Plush sobre los `Task` del bucle y sobre `*Task`):
  - `func (t Task) DueDateValue() string`: `""` si `DueDate == nil`; si no, `t.DueDate.Format(DueDateLayout)`.
  - `func (t Task) DueDateLabel() string`: `""` si nil; si no, `t.DueDate.Format("02/01/2006")`.
- `Validate` no cambia (la fecha ya llega parseada o se rechaza en la acción).

### Acciones (`actions/tasks.go`)

Añadir un helper privado que reúna el patrón «parsear fecha + validar + guardar»:

```go
// saveTask asigna la fecha límite recibida y guarda la tarea con save; si la
// fecha no es válida, devuelve los errores de validación sin guardar nada.
func saveTask(tx *pop.Connection, task *models.Task, dueDate string, save func(interface{}, ...string) (*validate.Errors, error)) (*validate.Errors, error) {
	parsed, parseErr := models.ParseDueDate(dueDate)
	if parseErr != nil {
		verrs, err := task.Validate(tx)
		if err != nil {
			return nil, err
		}
		verrs.Add("due_date", models.DueDateInvalidMessage)
		return verrs, nil
	}
	task.DueDate = parsed
	return save(task)
}
```

- `TasksCreate`: tras el `Bind` y el `TrimSpace`, sustituir `tx.ValidateAndCreate(task)` por `saveTask(tx, task, c.Param("due_date"), tx.ValidateAndCreate)`. El resto (recargar lista y 422 con `errors`) igual.
- `TasksUpdate`: añadir `DueDate string \`form:"due_date" json:"due_date"\`` al struct `input` y sustituir `tx.ValidateAndUpdate(task)` por `saveTask(tx, task, input.DueDate, tx.ValidateAndUpdate)`. Actualizar el comentario: «actualiza el título y la fecha límite, conservando los demás datos de la tarea».
- En 422 con fecha inválida, `task.DueDate` conserva el valor previo (nil al crear, el guardado al editar), así que el formulario vuelve a mostrar ese valor. Aceptable: con `<input type="date">` el navegador solo envía `aaaa-mm-dd` o vacío.
- `TasksToggle` usa `tx.Update(task)` sobre la tarea cargada, así que conserva la fecha sin cambios.

Nota: comprobar al implementar que `c.Param("due_date")` devuelve el valor del formulario en `TasksCreate` (en Buffalo `Param` incluye los valores del formulario). Si no, añadir un struct `input` análogo al de `TasksUpdate` o usar `c.Request().FormValue("due_date")`; es un detalle mecánico.

### Renderizado (Plush)

`templates/tasks/index.plush.html`:

- En la lista, envolver el título en un contenedor y añadir la fecha:
  ```html
  <div class="task-body">
    <span class="task-title"><%= t.Title %></span>
    <%= if (t.DueDateValue() != "") { %>
      <span class="task-due">Fecha límite: <time datetime="<%= t.DueDateValue() %>"><%= t.DueDateLabel() %></time></span>
    <% } %>
  </div>
  ```
  (el `.task` es un grid de 3 columnas; el contenedor ocupa la columna del título).
- En el formulario de creación, dentro de `.new-task-row` entre el input del título y el botón:
  ```html
  <label for="task-due-date" class="sr-only">Fecha límite (opcional)</label>
  <input id="task-due-date" type="date" name="due_date" value="<%= task.DueDateValue() %>"
    <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } %>>
  ```
- Los atributos `autofocus aria-invalid aria-describedby="title-error"` del título pasan a depender de `len(errors.Get("title")) > 0` en vez de `errors.HasAny()`, y el bloque de error se divide en dos párrafos condicionales: `#title-error` (errores de `title`) y `#due-date-error` (errores de `due_date`), ambos con `class="field-error" role="alert"`.

`templates/tasks/edit.plush.html`:

- Mismo cambio de condición para el título (`len(errors.Get("title")) > 0`, si no `aria-describedby="title-help"`) y para su párrafo de error.
- Cambiar el hint del título a: «Hasta 200 caracteres.».
- Tras el error del título, añadir:
  ```html
  <label for="task-due-date">Fecha límite</label>
  <input id="task-due-date" type="date" name="due_date" value="<%= task.DueDateValue() %>"
    <%= if (len(errors.Get("due_date")) > 0) { %>aria-invalid="true" aria-describedby="due-date-error"<% } else { %>aria-describedby="due-date-help"<% } %>>
  <p id="due-date-help" class="field-hint">Opcional. Déjala vacía para quitarla. El estado de la tarea no cambia.</p>
  <%= if (len(errors.Get("due_date")) > 0) { %>
    <p id="due-date-error" class="field-error" role="alert"><%= for (msg) in errors.Get("due_date") { %><%= msg %><% } %></p>
  <% } %>
  ```

Plush escapa la salida por defecto; la fecha solo contiene dígitos y separadores.

### CSS (`public/assets/app.css`)

- `input[type="text"]` → `input[type="text"], input[type="date"]` (mismo aspecto).
- `.task-body { align-self: center; min-width: 0; padding: 0.65rem 0; }` y `.task-body .task-title { display: block; padding: 0; }` (el padding pasa del título al contenedor).
- `.task-due { display: block; margin-top: 0.15rem; color: var(--color-muted); font-size: 0.8rem; }`.
- `.new-task-row input[type="date"] { flex: 0 0 auto; width: auto; }` para que no compita con el título.
- `.edit-task input[type="date"] { width: auto; }` y `.edit-task label[for="task-due-date"] { margin-top: 1.25rem; }` para separarlo del bloque del título.
- En `@media (max-width: 380px)`, `.new-task-row input[type="date"] { flex-basis: calc(100% - 44px); margin-left: 44px; }` para que baje de línea alineado.

## Archivos relevantes

| Ruta | Estado | Responsabilidad |
| --- | --- | --- |
| `migrations/20260901000002_add_due_date_to_tasks.up.fizz` | nuevo | Añade la columna `due_date` |
| `migrations/20260901000002_add_due_date_to_tasks.down.fizz` | nuevo | La elimina |
| `models/task.go` | existente | Campo `DueDate`, `ParseDueDate`, `DueDateValue`, `DueDateLabel` |
| `models/task_test.go` | existente | Tests de modelo |
| `actions/tasks.go` | existente | `saveTask`, `TasksCreate`, `TasksUpdate` |
| `actions/tasks_test.go` | existente | Tests de acciones |
| `templates/tasks/index.plush.html` | existente | Lista y formulario de creación |
| `templates/tasks/edit.plush.html` | existente | Formulario de edición |
| `public/assets/app.css` | existente | Estilos del campo y de la fecha en la lista |
| `migrations/embed.go`, `actions/app.go` | sin cambios | Embebido de migraciones y rutas (no hay rutas nuevas) |

## Implementación paso a paso

1. Crear las dos migraciones Fizz.
2. Modelo: campo, constantes, `ParseDueDate`, `DueDateValue`, `DueDateLabel`.
3. Acciones: `saveTask`, cambios en `TasksCreate` y `TasksUpdate` y comentario.
4. Plantillas: lista, formulario de creación y formulario de edición.
5. CSS.
6. Tests (ver abajo).
7. `gofmt -w` sobre los `.go` tocados.

## Tests y validación

Comandos (los del CI que aplican):

- `gofmt -l .` (vacío)
- `go vet ./...`
- `soda migrate -e test` (con PostgreSQL de `compose.yaml`: `docker compose up -d db`; base `todo_test`)
- `go test -p 1 ./...` con `GO_ENV=test`
- `go build ./...`

Tests de modelo (`models/task_test.go`):

- `Test_ParseDueDate`: vacío y espacios → nil sin error; `2026-11-15` → 15/11/2026; `31/12/2026`, `2026-02-30`, `mañana` → error.
- `Test_Task_DueDate_Persists` (CA8): crear con `DueDate` 2026-11-15, recargar con `Find` y comprobar `DueDateValue() == "2026-11-15"`; ponerla a nil, `Update`, recargar y comprobar `DueDate == nil`.
- `Test_Task_DueDateLabel`: `""` sin fecha; `15/11/2026` con fecha.

Tests de acciones (`actions/tasks_test.go`, con `url.Values` como en los existentes):

- `Test_TasksCreate_WithDueDate` (CA1): POST `title` + `due_date=2026-11-15` → 303; la tarea tiene `DueDateValue() == "2026-11-15"`; GET `/` contiene `15/11/2026` y `datetime="2026-11-15"`.
- `Test_TasksCreate_WithoutDueDate` (CA2): POST con `due_date` vacío → 303, `DueDate == nil`; GET `/` no contiene `Fecha límite:`.
- `Test_TasksCreate_InvalidDueDate` (CA3): POST con `due_date=31/12/2026` → 422, contiene `La fecha límite no es válida.` y `aria-describedby="due-date-error"`; 0 tareas.
- `Test_TasksEdit_PrefillsDueDate` (CA4): tarea con fecha → la página de edición contiene `name="due_date"` y `value="2026-11-15"`.
- `Test_TasksUpdate_ChangesAndClearsDueDate` (CA5): PUT con otra fecha → cambia; PUT con `due_date` vacío → nil; `Completed` se conserva.
- `Test_TasksUpdate_InvalidDueDate` (CA6): PUT `title=Otro` + `due_date=mañana` → 422 con el mensaje; título y fecha guardados no cambian.
- `Test_TasksToggle_KeepsDueDate` (CA7): tarea con fecha, toggle → fecha igual.
- Los tests existentes de `TasksUpdate` deben seguir pasando; `Test_TasksUpdate_EmptyTitle` sigue esperando `aria-describedby="title-error"`.

Prueba manual opcional: `docker compose up` y crear/editar una tarea con el selector de fecha.

## Riesgos y recuperación

- **Zona horaria al guardar `date`** (CA8): pop usa pgx; con sentencias preparadas pgx conoce el tipo `date` y codifica el día del `time.Time`. El test de persistencia lo cubre; si fallara, normalizar en `ParseDueDate` a `time.Date(y, m, d, 0, 0, 0, 0, time.UTC)` (ya lo es) y revisar la conversión antes de cambiar el diseño.
- **`*time.Time` en Plush**: por eso la plantilla solo usa `DueDateValue()`/`DueDateLabel()`, nunca el puntero.
- **`c.Param` en creación**: alternativa mecánica descrita en la capa de acciones.
- **Bases ya migradas**: la migración solo añade una columna nullable; las filas existentes quedan sin fecha. Recuperación: `soda migrate down` ejecuta el `.down.fizz`.
- **Cambio de hint en edición**: el texto «Los demás datos de la tarea no cambian» deja de ser cierto; ningún test existente lo comprueba.
