# Bonsai
Bonsai is a massive vegetation evolution simulator. Things below are just plans and not implemented.

Bonsai simulates plants at individual cell level, and has well-defined physics for lighting, sofy-body, etc..
All physics is local and energy / mass is carefully conserved.

This project is basically (huge) revamp of [old bonsai](http://www.xanxys.net/bonsai), which was limited because it's written in pure javascript.

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

![Deployment](https://docs.google.com/drawings/d/1cgvuM5Y_7A9-c1va3ftDwIgyta5Pe9MayIU3aiRTt9E/pub?w=851&amp;h=492)
[link](https://docs.google.com/drawings/d/1cgvuM5Y_7A9-c1va3ftDwIgyta5Pe9MayIU3aiRTt9E/edit)

![Data flow](https://docs.google.com/drawings/d/1sQcEftxncmAuRojqud-RQ05DrXb53Im6t8XrnxmwQD4/pub?w=851&amp;h=492)
[link](https://docs.google.com/drawings/d/1sQcEftxncmAuRojqud-RQ05DrXb53Im6t8XrnxmwQD4/edit)

## About Performance
At 6d177626e0dddd23fbd9039e7945aa899e51eea1
(300 water particles & 300 soil particles, https://gyazo.com/9e414fc8ba2fecb76a08528d013c24c5),

Measured time for simulating 1200 steps.

Average of 3 measurements:
* javascript (`client/biosphere.js`): 12.3sec
* go (`chunk/service.go`): 1.9sec

I did confirm these two results in visually identical simulation.

### Micro optimization in go
* math.Pow -> custom int pow: very effective
* map -> array: very effective
* pre-allocation of slice: moderately effective if cap is known
* Mutable pointer-based Vec3f (like three.js): slower than value-passing, even with some hand-optimization of equations to make them in-place

### Go Performance memo
* chan int message latency: 1us
* chan int message throughput: 5M messages / sec (no buffer), 20M messages / sec (w/ buffer len=100)
* float <-> int conversion is 100x slower than serializing to proto

### GCE / gRPC performance memo
* same-zone empty RPC RTT w/ connection reuse: 0.6ms-1ms
* same-zone empty RPC RTT w/ new connection:  0.8ms-1ms (connect) + 0.6ms-3ms (RPC)
