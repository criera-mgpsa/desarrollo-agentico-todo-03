package actions

import (
	"errors"
	"net/http"
	"strings"

	"todo/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// TasksIndex muestra la lista de tareas y el formulario de creación.
func TasksIndex(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	tasks := models.Tasks{}
	if err := tx.Order("position asc, created_at asc, id asc").All(&tasks); err != nil {
		return err
	}

	c.Set("tasks", tasks)
	c.Set("task", &models.Task{})
	c.Set("errors", validate.NewErrors())
	return c.Render(http.StatusOK, r.HTML("tasks/index.plush.html"))
}

// TasksCreate crea una tarea nueva a partir del formulario.
func TasksCreate(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := c.Bind(task); err != nil {
		return err
	}
	task.Title = strings.TrimSpace(task.Title)

	verrs, err := tx.ValidateAndCreate(task)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		tasks := models.Tasks{}
		if err := tx.Order("position asc, created_at asc, id asc").All(&tasks); err != nil {
			return err
		}

		c.Set("tasks", tasks)
		c.Set("task", task)
		c.Set("errors", verrs)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("tasks/index.plush.html"))
	}

	c.Flash().Add("success", "Tarea creada.")
	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksEdit muestra el formulario de edición de una tarea existente.
func TasksEdit(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("task", task)
	c.Set("errors", validate.NewErrors())
	return c.Render(http.StatusOK, r.HTML("tasks/edit.plush.html"))
}

// TasksUpdate actualiza solo el título, conservando los demás datos de la tarea.
func TasksUpdate(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	input := struct {
		Title string `form:"title" json:"title"`
	}{}
	if err := c.Bind(&input); err != nil {
		return err
	}
	task.Title = strings.TrimSpace(input.Title)

	verrs, err := tx.ValidateAndUpdate(task)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		c.Set("task", task)
		c.Set("errors", verrs)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("tasks/edit.plush.html"))
	}

	c.Flash().Add("success", "Tarea actualizada.")
	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksToggle alterna una tarea entre completada y pendiente.
func TasksToggle(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	task.Completed = !task.Completed
	if err := tx.Update(task); err != nil {
		return err
	}

	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksReorder guarda el orden de la lista a partir de todos los ids en su nuevo orden.
func TasksReorder(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	if err := c.Request().ParseForm(); err != nil {
		return c.Error(http.StatusUnprocessableEntity, err)
	}
	errInvalidOrder := errors.New("El orden recibido no coincide con las tareas actuales.")

	received := c.Request().Form["task_ids"]
	ids := make([]uuid.UUID, 0, len(received))
	seen := make(map[uuid.UUID]bool, len(received))
	for _, raw := range received {
		id, err := uuid.FromString(raw)
		if err != nil || seen[id] {
			return c.Error(http.StatusUnprocessableEntity, errInvalidOrder)
		}
		seen[id] = true
		ids = append(ids, id)
	}

	tasks := models.Tasks{}
	if err := tx.Select("id").All(&tasks); err != nil {
		return err
	}
	if len(tasks) != len(ids) {
		return c.Error(http.StatusUnprocessableEntity, errInvalidOrder)
	}
	for _, t := range tasks {
		if !seen[t.ID] {
			return c.Error(http.StatusUnprocessableEntity, errInvalidOrder)
		}
	}

	if len(ids) > 0 {
		values := make([]string, len(ids))
		args := make([]interface{}, 0, len(ids)*2)
		for i, id := range ids {
			values[i] = "(?::uuid, ?::integer)"
			args = append(args, id, i+1)
		}
		query := "UPDATE tasks SET position = v.position FROM (VALUES " + strings.Join(values, ", ") +
			") AS v(id, position) WHERE tasks.id = v.id"
		if err := tx.RawQuery(query, args...).Exec(); err != nil {
			return err
		}
	}

	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksDestroy elimina una tarea.
func TasksDestroy(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(task); err != nil {
		return err
	}

	c.Flash().Add("success", "Tarea eliminada.")
	return c.Redirect(http.StatusSeeOther, "/")
}
