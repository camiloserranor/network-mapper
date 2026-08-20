# network-mapper — deprecated & moved

> [!IMPORTANT]
> **This repository is deprecated as of August 2026 and is preserved for historical
> reference only.** Active development has moved to an internal Microsoft repository:
> **`microsoft/network-telemetry-mapper`** (access is limited to Microsoft organization
> members).
>
> No further commits, issues, or pull requests will be accepted here.

## What was this?

A proof-of-concept that discovered and displayed the physical network topology of an
Azure Local deployment by querying Top-of-Rack (TOR) switches over gNMI — LLDP-based
topology discovery, an interactive web UI, and early telemetry export.

## Where did it go?

The project graduated from a topology-visualization PoC into a production-oriented
physical-fabric **telemetry and metrics** collector. That work — persistent gNMI
subscriptions, a Geneva/ETW telemetry pipeline, a numeric metrics catalog, and
multi-vendor (SONiC / Cisco NX-OS) support — now lives in
`microsoft/network-telemetry-mapper`.

## History

The full commit history and all branches of this PoC are preserved here in read-only
form.
