---
name: adw
description: Lleva una tarea de principio a fin - clasifica, planifica, implementa, prueba, hace commit, push y abre la pull request
allowed-tools: Bash, Read, Glob, Grep, Write, Edit, Skill
---

# Desarrolla una tarea de principio a fin

Recibe en `$ARGUMENTS` los detalles de la tarea: una descripción en texto libre o la URL de una issue de GitHub. Opcionalmente admite `--remote <nombre>` (por defecto `origin`), `--base <rama>` (por defecto `main`) y `--label <etiqueta>` (por defecto `adw`, la que usa el trigger).

Esta skill coordina `/bug`, `/chore`, `/feature` e `/implement`, de `.claude/skills/`, y completa después las pruebas, el commit, el push y la pull request. Debe poder ejecutarse sin intervención, por ejemplo desde el [trigger de issues](../../../adws/README.md) con `claude -p`. No hagas preguntas: si algo impide continuar con seguridad, detente y explica el bloqueo en el reporte final.

## 0. Reclamar la issue

Si la tarea viene de una issue, antes de nada quítale la etiqueta con `gh issue edit <url> --remove-label <etiqueta>`. Así el trigger no la vuelve a lanzar mientras trabajas ni después: cada issue se procesa una sola vez, termine en pull request o en bloqueo. Para reintentarla basta con volver a ponerle la etiqueta.

Desde ese momento, el reporte final debe publicarse también como comentario en la issue (ver **Reporte final**), también si el flujo se detiene en cualquier paso.

## 1. Comprobaciones previas

1. Ejecuta `git status --short --branch`. Si hay cambios en archivos con seguimiento, detente: no mezcles trabajo ajeno. Los archivos sin seguimiento que ya existían se dejan fuera de todo el flujo; anota su lista para excluirlos después.
2. Comprueba que el remoto existe con `git remote get-url <remote>` y que `gh auth status` responde. Si falla, detente.
3. Ejecuta `git fetch <remote> <base>`.

## 2. Detalles de la tarea

- Si `$ARGUMENTS` contiene una URL de issue, obtén sus datos con `gh issue view <url> --json number,title,body,labels,url`. Usa título, cuerpo y etiquetas como descripción de la tarea y conserva el número para la pull request.
- En otro caso, la descripción es el texto de `$ARGUMENTS` sin las opciones.

Si la descripción no permite saber qué hay que conseguir, detente antes de crear nada.

## 3. Clasificación

Decide un único tipo y anota el motivo en una frase:

- **bug**: un comportamiento existente no hace lo que debe (error, dato que no se guarda, fallo visible). Prefijo `fix`.
- **feature**: comportamiento observable nuevo para el usuario. Prefijo `feat`.
- **chore**: mantenimiento sin cambio funcional (dependencias, versiones, configuración, refactor, limpieza, documentación, tooling). Prefijo `chore`, o `refactor` en el commit si es un refactor.

Si la issue tiene una etiqueta `bug`, `enhancement`/`feature` o `chore`, úsala salvo que el contenido la contradiga claramente. En caso de duda entre bug y feature, elige bug si la descripción cita un comportamiento actual que falla.

## 4. Rama

1. Deriva un `<slug>` kebab-case corto y descriptivo de la tarea, en español como las ramas existentes.
2. Crea la rama `<prefijo>/<slug>` desde la base remota: `git switch -c <prefijo>/<slug> <remote>/<base>` y después `git branch --unset-upstream` para que no siga a la base.
3. Si la rama ya existe en local o en el remoto, detente: no reutilices ni sobrescribas trabajo anterior.

## 5. Plan

Invoca con la herramienta `Skill` la skill del tipo elegido (`bug`, `chore` o `feature`) con los argumentos `plans/<slug>.md — <descripción completa de la tarea>`.

Al terminar, comprueba que `plans/<slug>.md` existe y contiene las secciones que exige esa skill. Lanzar esta skill equivale a aprobar el plan resultante: no esperes otra confirmación. Si la skill de planificación informa de incertidumbres que impiden implementar con seguridad, detente aquí.

## 6. Implementación

Invoca la skill `implement` con los argumentos `plans/<slug>.md`. Sigue sus reglas: no rediseñes ni amplíes el alcance.

Si `/implement` se detiene por un bloqueo, no sigas con el commit: ve al reporte final.

## 7. Pruebas

Ejecuta todas las validaciones del apartado `## Tests y validación` del plan y, como mínimo, las del CI (`.github/workflows/ci.yaml`) que apliquen al cambio:

- `gofmt -l .` (no debe listar archivos)
- `go vet ./...`
- `go test -p 1 ./...`
- `go build ./...`

Los tests de aplicación necesitan PostgreSQL con la base `todo_test` migrada (`database.yml`). Si no responde, intenta `docker compose up -d db` y espera a que esté saludable; las migraciones de test se aplican con `soda migrate -e test`, como en el CI. Si aun así no hay base de datos, no inventes el resultado: anota esas pruebas como **no ejecutadas** y el motivo.

Guarda, por cada comando, el comando exacto, si pasó o falló y un extracto breve de la salida relevante.

Si una prueba falla por el cambio, puedes corregirlo solo si la causa es un detalle mecánico dentro del alcance del plan; vuelve a ejecutar las pruebas. Si la corrección exigiría rediseñar, o la prueba sigue fallando, no hagas commit: ve al reporte final. Nunca desactives ni borres tests para obtener una ejecución verde.

## 8. Commit

Sigue las reglas de `.claude/commands/git/commit.md`:

1. Revisa `git status --short`, `git diff` y `git diff --check`.
2. Añade con `git add -- <archivos>` una lista explícita: los archivos que cambió `/implement` y el plan `plans/<slug>.md`. Excluye los archivos sin seguimiento previos, secretos, `.env`, binarios y artefactos generados. No uses `git add .` ni `git add -A`.
3. Revisa `git diff --cached`.
4. Escribe el mensaje en Conventional Commits, en español como el historial: `<tipo>(<scope>): <descripción>` en imperativo, minúsculas, sin punto final y de 50 caracteres o menos. El cuerpo explica qué fallaba o qué se añade, qué se cambia y cómo se ha comprobado. Si la tarea viene de una issue, añade `Refs #<número>`.
5. Ejecuta `git commit` y anota el hash.

## 9. Push

Ejecuta `git push -u <remote> <prefijo>/<slug>`. Nunca hagas push a `<base>` ni uses `--force`.

## 10. Pull request

Escribe la descripción en un archivo temporal fuera del repositorio y crea la PR con `gh pr create --base <base> --head <prefijo>/<slug> --title "<primera línea del commit>" --body-file <archivo>`. Si el repositorio tiene una plantilla en `.github/pull_request_template.md`, respeta sus secciones.

La descripción debe incluir:

- `## Resumen`: qué problema resuelve o qué añade, en dos o tres frases.
- `## Tarea`: origen (enlace a la issue o la descripción recibida) y tipo elegido con el motivo de la clasificación.
- `## Plan`: ruta de `plans/<slug>.md` y sus decisiones principales.
- `## Cambios`: cada archivo modificado con lo que cambia.
- `## Pruebas`: una tabla `Comando | Resultado | Detalle` con cada validación ejecutada, y las no ejecutadas con su motivo. Añade las pruebas manuales que se hayan hecho.
- `## Riesgos y pendientes`: desviaciones respecto al plan, riesgos residuales y lo que quede por verificar.
- `Closes #<número>` si la tarea viene de una issue.

No actives el auto-merge ni hagas merge.

## Reporte final

Informa de:

- tipo elegido y motivo;
- rama, ruta del plan, hash del commit y URL de la pull request;
- archivos cambiados y tests añadidos o actualizados;
- cada comando de prueba con su resultado;
- desviaciones, riesgos residuales y, si el flujo se detuvo, en qué paso, por qué y qué queda en el árbol de trabajo (sin commit, sin push o sin PR).

Si el flujo se detuvo después de crear la rama y quedan cambios sin commit en archivos con seguimiento, guárdalos con `git stash push -m "adw #<número>: <motivo>" -- <archivos>` e indica el stash en el reporte. El siguiente encargo del trigger empieza en este mismo checkout y necesita el árbol limpio.

Si la tarea viene de una issue, escribe el reporte en un archivo temporal fuera del repositorio y publícalo con `gh issue comment <url> --body-file <archivo>`. Empieza el comentario con la URL de la pull request o, si no la hay, con `Bloqueado en el paso <n>: <motivo>` y cómo reintentarlo (volver a poner la etiqueta).
