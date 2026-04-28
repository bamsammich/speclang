model Shipment {
  status: string
  tracking: string when status == "shipped"
}

model ShipmentResult {
  ok: bool
}

scope state_dependent_test {
  contract StateDependentContract -> ShipmentResult {
    status: string
    tracking: string when status == "shipped"

    action {
      return { ok: true }
    }

    scenario shipped_status {
      given {
        status: "shipped"
        tracking: "1Z999AA10123456784"
      }
      then {
        output.ok == true
      }
    }

    scenario pending_status {
      given {
        status: "pending"
      }
      then {
        output.ok == true
      }
    }
  }
}
