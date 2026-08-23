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

## Current C1 constraints

The pinned reference API accepts a signed 32-bit block number. The candidate
boundary therefore rejects heights above `2,147,483,647`; removing that limit
requires a reviewed source/API decision before any engine integration.
The direct native bridge applies the same guard before converting Go's `int`
to the C API's `int`, so callers cannot bypass it.

Core-Geth's existing `2^256 / difficulty` convention produces a 257-bit target
at difficulty 1, while the C1 verifier accepts a 256-bit boundary. The boundary
rejects that case rather than silently changing the rule. A future protocol
specification must define the minimum difficulty and exact target encoding.
