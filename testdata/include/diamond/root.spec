include "b.spec"
include "c.spec"
include "shared.spec"

contract Foo() -> Result {
  action { return "x" }
}
