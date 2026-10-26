package models

import (
	"strings"
	"time"
)

func (ms *ModelSuite) Test_Task_Create_Valid() {
	task := &Task{Title: "Comprar café"}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	ms.NotZero(task.ID)
	ms.False(task.Completed)

	count, err := ms.DB.Count(&Task{})
	ms.NoError(err)
	ms.Equal(1, count)
}

func (ms *ModelSuite) Test_Task_Title_Required() {
	task := &Task{Title: ""}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.True(verrs.HasAny())
	ms.Contains(verrs.Get("title"), "El título es obligatorio.")

	count, err := ms.DB.Count(&Task{})
	ms.NoError(err)
	ms.Equal(0, count)
}

func (ms *ModelSuite) Test_Task_Title_OnlySpaces_Invalid() {
	task := &Task{Title: "   \t  "}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.True(verrs.HasAny())
	ms.Contains(verrs.Get("title"), "El título es obligatorio.")
}

func (ms *ModelSuite) Test_Task_Title_MaxLength() {
	task := &Task{Title: strings.Repeat("a", TitleMaxLength)}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())
}

func (ms *ModelSuite) Test_Task_Title_TooLong_Invalid() {
	task := &Task{Title: strings.Repeat("a", TitleMaxLength+1)}

	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.True(verrs.HasAny())
	ms.NotEmpty(verrs.Get("title"))
}

func (ms *ModelSuite) Test_Task_Toggle_Persists() {
	task := &Task{Title: "Revisar el correo"}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	task.Completed = true
	ms.NoError(ms.DB.Update(task))

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.True(reloaded.Completed)
}

func (ms *ModelSuite) Test_ParseDueDate() {
	for _, value := range []string{"", "   "} {
		date, err := ParseDueDate(value)
		ms.NoError(err)
		ms.Nil(date)
	}

	date, err := ParseDueDate(" 2026-11-15 ")
	ms.NoError(err)
	ms.Equal(time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC), *date)

	for _, value := range []string{"31/12/2026", "2026-02-30", "mañana"} {
		date, err := ParseDueDate(value)
		ms.Error(err, value)
		ms.Nil(date, value)
	}
}

func (ms *ModelSuite) Test_Task_DueDateLabel() {
	task := Task{Title: "Sin fecha"}
	ms.Equal("", task.DueDateValue())
	ms.Equal("", task.DueDateLabel())

	date := time.Date(2026, time.November, 15, 0, 0, 0, 0, time.UTC)
	task.DueDate = &date
	ms.Equal("2026-11-15", task.DueDateValue())
	ms.Equal("15/11/2026", task.DueDateLabel())
}

func (ms *ModelSuite) Test_Task_DueDate_Persists() {
	date, err := ParseDueDate("2026-11-15")
	ms.NoError(err)
	task := &Task{Title: "Entregar la declaración", DueDate: date}
	verrs, err := ms.DB.ValidateAndCreate(task)
	ms.NoError(err)
	ms.False(verrs.HasAny())

	reloaded := &Task{}
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.Equal("2026-11-15", reloaded.DueDateValue())

	reloaded.DueDate = nil
	ms.NoError(ms.DB.Update(reloaded))
	ms.NoError(ms.DB.Find(reloaded, task.ID))
	ms.Nil(reloaded.DueDate)
}
