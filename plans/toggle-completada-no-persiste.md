# Bug: marcar una tarea como completada no persiste tras recargar

## Encargo y resultado esperado

Encargo recibido: "al marcar una tarea como completada y recargar la página no se queda como completada".

Resultado esperado:

- Al pulsar el botón de completar/reabrir de una tarea, el cambio se guarda en la base de datos.
- Tras recargar la página, la tarea conserva su estado (completada o pendiente) y su indicador visual.
- Si la petición falla, la interfaz no debe mostrar un estado que el servidor no ha guardado.

## Evidencia y causa raíz

### Hechos observados

1. **El servidor sí alterna el estado.** La ruta `PUT /tasks/{task_id}/toggle` está registrada en [actions/app.go](../actions/app.go) y `TasksToggle` en [actions/tasks.go](../actions/tasks.go) invierte `task.Completed` y llama a `tx.Update`. El test `Test_TasksToggle_CompletesAndReopens` en [actions/tasks_test.go](../actions/tasks_test.go) lo verifica con una petición `PUT` directa.

2. **El formulario del botón es un `POST` con `_method=PUT`.** En [templates/tasks/index.plush.html](../templates/tasks/index.plush.html) el formulario `[data-task-toggle]` tiene `method="POST"`, un `authenticity_token` oculto y `_method=PUT`. Buffalo aplica la sobrescritura de método desde el campo `_method` del formulario (`method_override.go` en `buffalo@v1.1.4`).

3. **El JavaScript intercepta ese envío y no hace la petición correcta.** En [public/assets/app.js](../public/assets/app.js), el listener de `submit` hace `event.preventDefault()` para `[data-task-toggle]`, cambia la UI y termina con:

   ```js
   fetch(form.action);
   ```

   Esa llamada es un `GET` a `/tasks/{id}/toggle`, sin `_method`, sin `authenticity_token` y sin comprobar la respuesta. No existe ninguna ruta `GET` para esa URL en `actions/app.go`, así que el servidor no ejecuta `TasksToggle`.

4. **El estado visual cambia antes de confirmar nada.** La clase `task-completed` y el texto "Completada" se aplican de forma optimista. Al recargar, el servidor devuelve el estado guardado, que nunca cambió.

5. **La CSRF del servidor exige el token.** El middleware de CSRF busca `authenticity_token` en el formulario o en la cabecera `X-CSRF-Token` (`middleware/csrf@v1.0.0`). Una petición sin token no puede pasar en producción, aunque llegara a una ruta válida.

6. **Los tests no cubren el camino del navegador.** La suite de `actions/` sustituye `csrf.New` por un paso directo en `SetupTest`, y los tests de toggle llaman al servidor con `PUT` directamente. Ningún test ejecuta `app.js`.

7. **Caché del JavaScript.** [templates/application.plush.html](../templates/application.plush.html) carga `/assets/app.js?v=2`. Si el arreglo cambia `app.js`, hay que subir la versión para que los navegadores no sigan usando el fichero antiguo.

### Inferencias (no verificadas ejecutando)

- La causa raíz más probable es el `fetch(form.action)` de `app.js`: al ser un `GET`, el servidor no tiene ruta y el cambio nunca llega a la base de datos. Lo confirma la lectura del código, pero no lo he visto en un navegador.
- Buffalo responde con 404 o 405 a un `GET` sobre esa URL. Ninguna de las dos opciones guarda nada.

### Reproducción mínima (pendiente de ejecutar)

1. Levantar PostgreSQL y la aplicación en modo desarrollo (ver "Tests y validación").
2. Abrir `/`, crear una tarea (o usar la del seed en desarrollo).
3. Pulsar el botón de completar. La tarea se marca en pantalla.
4. Recargar la página. Resultado observado esperado: la tarea vuelve a estar pendiente.
5. Comprobar en la consola del navegador o en la pestaña Network que la petición de `app.js` es un `GET` a `/tasks/<id>/toggle`.

### Incertidumbres

- No he comprobado en un navegador real la respuesta exacta de Buffalo al `GET` (404 o 405); no cambia la conclusión.
- No he comprobado si hay otros scripts que vuelvan a alternar el estado al cargar la página. Por el código leído, no los hay.

## Alcance

### Incluido

- Corregir el envío que hace `app.js` al alternar una tarea, para que use el mismo método, cuerpo y token que el formulario.
- Gestionar un fallo de la petición de forma que la interfaz refleje el estado del servidor (recargar la página).
- Subir la versión de `app.js` en `templates/application.plush.html` para invalidar la caché.
- Añadir un test de regresión que documente el contrato del servidor usado por el formulario (`POST` con `_method=PUT`).
- Añadir una comprobación de navegador para el flujo completo (marcar, recargar, verificar).

### Excluido

- Cambiar el modelo, la validación o el esquema de la tabla `tasks`.
- Eliminar la actualización optimista. Se mantiene; solo se corrige cómo se envía y qué pasa si falla.
- Añadir autenticación, sincronización en tiempo real o nuevos endpoints.
- Cambiar el formulario de edición, el borrado o la creación.
- Tocar `TasksToggle` o `app.go`: el servidor ya es correcto.

## Archivos relevantes

| Ruta | Responsabilidad |
|---|---|
| [public/assets/app.js](../public/assets/app.js) | Listener de `submit` para `[data-task-toggle]`. Aquí está el `fetch(form.action)` incorrecto. |
| [templates/tasks/index.plush.html](../templates/tasks/index.plush.html) | Formulario de toggle con `POST`, `_method=PUT` y `authenticity_token`. Referencia del contrato. |
| [templates/application.plush.html](../templates/application.plush.html) | Carga de `app.js?v=2`. Hay que subir la versión. |
| [actions/app.go](../actions/app.go) | Define `PUT /tasks/{task_id}/toggle`. Sin cambios. |
| [actions/tasks.go](../actions/tasks.go) | `TasksToggle`. Sin cambios. |
| [actions/tasks_test.go](../actions/tasks_test.go) | Tests de toggle. Aquí se añade el de regresión. |

## Implementación paso a paso

1. **Añadir el test de regresión del servidor** en [actions/tasks_test.go](../actions/tasks_test.go), junto a `Test_TasksToggle_CompletesAndReopens`. Debe enviar un `POST` con `_method=PUT` y los campos del formulario, y comprobar que la tarea cambia de estado:

   ```go
   func (as *ActionSuite) Test_TasksToggle_PostWithMethodOverride() {
       task := as.taskForEditing(false)
       res := as.HTML("/tasks/%s/toggle", task.ID).Post(url.Values{"_method": {"PUT"}})
       as.Equal(303, res.Code)
       as.NoError(as.DB.Reload(task))
       as.True(task.Completed)
   }
   ```

   Este test debería pasar ya con el servidor actual; sirve para fijar el contrato que el JavaScript debe cumplir. Antes del arreglo, el fallo lo muestra solo el paso 4 (comprobación de navegador).

2. **Corregir el envío en `app.js`.** Sustituir `fetch(form.action)` por una petición que coincida con el formulario:

   ```js
   fetch(form.action, {
     method: "POST",
     body: new FormData(form),
     credentials: "same-origin",
   }).then(function (response) {
     if (!response.ok) {
       window.location.reload();
     }
   }).catch(function () {
     window.location.reload();
   });
   ```

   - `FormData(form)` incluye `authenticity_token` y `_method=PUT`, así Buffalo aplica la sobrescritura y la CSRF pasa.
   - Un fallo de red o una respuesta de error recarga la página, que muestra el estado real del servidor.
   - Mantener el resto del listener (cambio optimista de clases, `aria-pressed`, `aria-label`, `title`, texto "Completada") sin cambios.

3. **Subir la versión de `app.js`** en [templates/application.plush.html](../templates/application.plush.html): de `/assets/app.js?v=2` a `/assets/app.js?v=3`.

4. **Comprobar en el navegador.** Con PostgreSQL y la app en marcha, marcar una tarea, recargar y verificar que sigue completada. Repetir para reabrirla. Se puede usar la skill `playwright-cli` o una comprobación manual; el resultado debe anotarse en el informe de implementación.

5. **No cambiar** `TasksToggle`, las rutas ni el modelo.

## Tests y validación

Comandos de la validación habitual (de [.github/workflows/ci.yaml](../.github/workflows/ci.yaml)):

```bash
gofmt -l .
go vet ./...
go test -p 1 ./...
go build ./...
```

Los tests necesitan PostgreSQL con la base `todo_test` y las migraciones aplicadas (`soda migrate -e test`), según [database.yml](../database.yml). En esta máquina no hay `soda`, `docker` ni `psql`; antes de ejecutar hay que prepararlos o usar un Postgres accesible con `DB_HOST`.

Comprobaciones específicas del cambio:

- `go test -p 1 ./actions -run Test_TasksToggle` debe pasar, incluido el test nuevo.
- Comprobación de navegador (paso 4 del plan): marcar, recargar y verificar, para completar y para reabrir. Antes del arreglo debe fallar; después debe pasar.
- Comprobar en la pestaña Network que la petición es `POST /tasks/<id>/toggle` con `_method=PUT` y que responde 303.
- Comprobar que `app.js` se sirve con `?v=3` y que el navegador no usa la versión anterior.

## Riesgos y recuperación

- **Caché del navegador:** sin subir `?v=3`, algunos usuarios seguirán con el `app.js` roto. Por eso el paso 3 es obligatorio.
- **Pérdida de tests:** el test de regresión del servidor no detecta el bug en el JavaScript, porque el fallo está en el cliente. Por eso la comprobación de navegador es el criterio real de aceptación.
- **Recarga en fallo:** `window.location.reload()` puede perder cambios no guardados en el formulario de creación, si la tarea se cambió mientras se escribía. Es un efecto aceptable para un fallo poco probable y evita estados incoherentes.
- **Recuperación:** el cambio afecta a un solo fichero JavaScript, a una línea de plantilla y a un test. Revertir el commit restablece el comportamiento anterior sin migraciones ni cambios de datos.
- **Datos:** el bug no ha corrompido datos: el servidor nunca recibió el cambio. Las tareas que quedaron "completadas" solo en pantalla no necesitan limpieza.
