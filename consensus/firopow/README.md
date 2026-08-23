# FiroPoW Candidate Boundary (C2)

This package is an **experimental Phase 2A boundary** in the AIChain
Core-Geth fork. It uses the MIT-licensed `firoorg/firo` source, pinned at
`adba4310a1b118f879cb16013c669ea8b7dae01f`, solely to verify its official
FiroPoW vectors on CPU.

It does **not** implement `consensus.Engine`, choose an algorithm, alter
Core-Geth engine selection, add a miner, change genesis, or activate a devnet
rule. The nested source is an upstream pin, not an AIChain algorithm adoption.

The pinned FiroPoW API takes a signed `int` block number. The boundary rejects
values outside `0..2,147,483,647` before native conversion. Any future engine
proposal must separately define consensus rules, target/header mapping,
performance limits, the launch block interval, and a pre-limit upgrade plan.
