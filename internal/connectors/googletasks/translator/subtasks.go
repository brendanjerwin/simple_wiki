package translator

// HasSubtasks reports whether any task in the slice has a non-empty
// `parent` field. Used by sync at the bind ceremony to refuse-to-
// subscribe a Tasks list that already contains a hierarchy (per plan
// §3, "Subtasks (parent): refuse-to-subscribe initially; tolerant
// flatten if subtasks appear post-subscribe").
func HasSubtasks(tasks []Task) bool {
	for _, t := range tasks {
		if t.Parent != "" {
			return true
		}
	}
	return false
}

// FlattenSubtasks returns a copy of the input slice with `Parent`
// cleared on every task. Pure transformation — no I/O, no mutation
// of the input. Used by the inbound sync path when a previously-flat
// list grows a subtask post-subscribe; per the plan, the wiki
// flattens silently and surfaces the event via observability rather
// than failing.
func FlattenSubtasks(tasks []Task) []Task {
	out := make([]Task, len(tasks))
	for i, t := range tasks {
		t.Parent = ""
		out[i] = t
	}
	return out
}
