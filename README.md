# Bonsai
Bonsai is a massive vegetation evolution simulator. Things below are just plans and not implemented.

Bonsai simulates plants at individual cell level, and has well-defined physics for lighting, sofy-body, etc..
All physics is local and energy / mass is carefully conserved.

## Simulated Aspects
* soft-body (particle-based)
* light (ray-traced)
* fake chemistry
* cell cycle & division
* DNA - protein relationship

All of these affects each other, and all interactions are localized and completely well-defined like real world.
This is super important property since evolving things tend to *cheat* by exploiting undefined edge cases
(e.g. infinite efficiency by creating infinitely thin cell).

## Infra
World is divided to re-sizable chunks and they're simulated in parallel on google cloud platform.
Simulation is bit-wise reproducible when run in any chunk size and/or in presence of server failures.
