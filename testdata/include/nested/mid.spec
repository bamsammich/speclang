# Transitive include: pulls in leaf.spec, then defines its own model.
include "leaf.spec"

model Container {
  count: int
}
