# Self-verification: speclang verifying its own runtime behavior.
spec Speclang {
  description: "Black-box verification of the specrun CLI: parsing, generation, and end-to-end verify"

  target {
    services {
      transfer_server {
        build: "../examples/server"
        port: 8080
      }
      broken_server {
        build: "../testdata/self/broken_server"
        port: 8081
      }
      http_test_server {
        build: "../testdata/self/http_server"
        port: 8082
      }
    }
    command: env(SPECRUN_BIN, "./specrun")
  }

  include "parse.spec"
  include "generate.spec"
  include "verify.spec"
  include "verify_fail.spec"
  include "types.spec"
  include "generate_types.spec"
  include "cli_flags.spec"
  include "adapters.spec"
  include "enum.spec"
  include "exists.spec"
  include "error_assertions.spec"
  include "shrinking.spec"
  include "import.spec"
}
