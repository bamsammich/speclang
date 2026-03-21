# Test fixture: duplicate model names across includes (should error).
spec Dup {
  include "models_a.spec"
  include "models_b.spec"
}
