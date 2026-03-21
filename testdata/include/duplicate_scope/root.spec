# Test fixture: duplicate scope names across includes (should error).
spec DupScope {
  include "scope_a.spec"
  include "scope_b.spec"
}
