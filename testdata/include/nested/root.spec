# Test fixture: nested include resolution (root -> mid -> leaf).
use http

spec Nested {
  include "mid.spec"
}
