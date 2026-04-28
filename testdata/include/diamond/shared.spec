# Shared file included by b.spec, c.spec, and root.spec directly.
# The diamond pattern means this file is reachable via three chains:
#   root -> b -> shared
#   root -> c -> shared
#   root -> shared (direct)
# It must be spliced exactly once.
model Shared { id: string }
model Result { value: string }
