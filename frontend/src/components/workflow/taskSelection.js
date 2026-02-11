export function computeSelectionDetails(tasks = [], selectedTaskIds = []) {
  const explicit = new Set((selectedTaskIds || []).filter(Boolean));

  const taskById = new Map();
  for (const t of tasks || []) {
    if (t?.id) taskById.set(t.id, t);
  }

  const effective = new Set();
  const stack = Array.from(explicit);
  while (stack.length > 0) {
    const id = stack.pop();
    if (!id || effective.has(id)) continue;
    effective.add(id);
    const t = taskById.get(id);
    const deps = Array.isArray(t?.dependencies) ? t.dependencies : [];
    for (const dep of deps) {
      if (!effective.has(dep)) stack.push(dep);
    }
  }

  const required = new Set();
  for (const id of effective) {
    if (!explicit.has(id)) required.add(id);
  }

  return { explicit, effective, required };
}

