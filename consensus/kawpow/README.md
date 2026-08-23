# KawPoW Candidate Boundary (C1)

This package is an **experimental Phase 2A boundary** in the AIChain Core-Geth fork.

It derives a deterministic verifier input from a Core-Geth header, embeds the
pinned Apache-2.0 `cpp-kawpow` reference source as a nested submodule, and tests
a known ProgPoW vector plus the candidate pre-seal hash against the current
Ethash implementation. It does **not**:

- select KawPoW as AIChain's final mining algorithm;
- implement `consensus.Engine` or alter Core-Geth engine selection;
- provide mining support;
- activate a new rule on any network, including the development network.

The parent AIChain repository retains the broader C1 experiment, benchmarks, and
conformance history at `spikes/c1-kawpow-verifier`. Any activation must first
record the mining-algorithm decision, full consensus rules, test vectors,
hardware/DoS measurements, genesis activation point, and migration policy.
