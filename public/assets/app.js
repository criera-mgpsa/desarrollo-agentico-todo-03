// Pide confirmación antes de enviar formularios marcados con data-confirm.
document.addEventListener("submit", function (event) {
  var form = event.target;
  if (form.dataset.confirm && !window.confirm(form.dataset.confirm)) {
    event.preventDefault();
  }

  if (!form.matches("[data-task-toggle]")) {
    return;
  }

  event.preventDefault();

  var task = form.closest(".task");
  var completed = task.classList.toggle("task-completed");
  var button = form.querySelector("button");
  var status = task.querySelector(".task-status");

  var label = completed ? "Reabrir" : "Completar";
  button.querySelector(".toggle-label").textContent = label;
  button.setAttribute("aria-pressed", String(completed));
  button.setAttribute("aria-label", label + " tarea: " + task.querySelector(".task-title").textContent);
  button.setAttribute("title", label + " tarea");

  if (completed && !status) {
    status = document.createElement("span");
    status.className = "task-status";
    status.textContent = "Completada";
    task.insertBefore(status, task.querySelector(".task-actions"));
  } else if (!completed && status) {
    status.remove();
  }

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
});

// Ordenación de la lista: arrastre con puntero (ratón, táctil, lápiz) y flechas en el asa.
function taskOrder(list) {
  return Array.prototype.map.call(list.querySelectorAll("li[data-task-id]"), function (task) {
    return task.dataset.taskId;
  });
}

// Envía el orden completo; si falla, recarga para mostrar el que tiene el servidor.
function saveOrder(list) {
  var form = document.querySelector("[data-task-reorder]");
  var body = new FormData(form);
  taskOrder(list).forEach(function (id) {
    body.append("task_ids", id);
  });

  fetch(form.action, {
    method: "POST",
    body: body,
    credentials: "same-origin",
  }).then(function (response) {
    if (!response.ok) {
      window.location.reload();
    }
  }).catch(function () {
    window.location.reload();
  });
}

// Agrupa las pulsaciones seguidas de teclado en una sola petición.
var saveTimer = null;
function scheduleSave(list) {
  window.clearTimeout(saveTimer);
  saveTimer = window.setTimeout(function () {
    saveOrder(list);
  }, 300);
}

function announce(task) {
  var status = document.querySelector("[data-reorder-status]");
  var tasks = Array.prototype.slice.call(task.parentNode.querySelectorAll("li[data-task-id]"));
  status.textContent = "Tarea movida a la posición " + (tasks.indexOf(task) + 1) + " de " + tasks.length + ".";
}

var drag = null;

document.addEventListener("pointerdown", function (event) {
  var handle = event.target.closest("[data-task-drag]");
  if (!handle || event.button !== 0) {
    return;
  }

  event.preventDefault();
  var task = handle.closest(".task");
  drag = { handle: handle, task: task, list: task.parentNode, order: taskOrder(task.parentNode).join(",") };
  task.classList.add("task-dragging");
  handle.setPointerCapture(event.pointerId);
});

document.addEventListener("pointermove", function (event) {
  if (!drag) {
    return;
  }

  var siblings = drag.list.querySelectorAll("li[data-task-id]");
  for (var i = 0; i < siblings.length; i++) {
    var other = siblings[i];
    if (other === drag.task) {
      continue;
    }
    var rect = other.getBoundingClientRect();
    var middle = rect.top + rect.height / 2;
    var before = other.compareDocumentPosition(drag.task) & Node.DOCUMENT_POSITION_PRECEDING;
    if (before && event.clientY > middle) {
      drag.list.insertBefore(drag.task, other.nextSibling);
    } else if (!before && event.clientY < middle) {
      drag.list.insertBefore(drag.task, other);
      break;
    }
  }
});

function endDrag() {
  if (!drag) {
    return;
  }

  var current = drag;
  drag = null;
  current.task.classList.remove("task-dragging");
  if (taskOrder(current.list).join(",") !== current.order) {
    saveOrder(current.list);
    announce(current.task);
  }
}

document.addEventListener("pointerup", endDrag);
document.addEventListener("pointercancel", endDrag);

document.addEventListener("keydown", function (event) {
  var handle = event.target.closest && event.target.closest("[data-task-drag]");
  if (!handle || (event.key !== "ArrowUp" && event.key !== "ArrowDown")) {
    return;
  }

  event.preventDefault();
  var task = handle.closest(".task");
  var list = task.parentNode;
  if (event.key === "ArrowUp") {
    var previous = task.previousElementSibling;
    if (!previous) {
      return;
    }
    list.insertBefore(task, previous);
  } else {
    var next = task.nextElementSibling;
    if (!next) {
      return;
    }
    list.insertBefore(next, task);
  }

  handle.focus();
  scheduleSave(list);
  announce(task);
});
