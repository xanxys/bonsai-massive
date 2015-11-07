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

## About Performance
At 5912e8fa61eaa8b0e4a6af0788a659535113975a
(300 water particles & 300 soil particles, https://gyazo.com/9e414fc8ba2fecb76a08528d013c24c5),

Measured time for simulating 1200 steps.

Average of 3 measurements:
* javascript (`client/biosphere.js`): 12.3sec
* go (`chunk/service.go`): 1.72sec

Note that initial go version (naitve port) was initially 4 second, and 1.72 second is achived after optimizing math.Pow).

Also note that I didn't confirm the result are idential in both simulations.

Go profiling (`go tool pprof`) top 20 bottlenecks for 1.72sec version.
```
Showing top 20 nodes out of 95 (cum >= 40ms)
      flat  flat%   sum%        cum   cum%
     210ms  9.46%  9.46%      320ms 14.41%  runtime.mapaccess2
     150ms  6.76% 16.22%      150ms  6.76%  runtime.mapiternext
     130ms  5.86% 22.07%      240ms 10.81%  runtime.mapassign1
     110ms  4.95% 27.03%      590ms 26.58%  main.(*GrainWorld).IndexNeighbors
     100ms  4.50% 31.53%     1670ms 75.23%  main.(*GrainWorld).Step
     100ms  4.50% 36.04%      230ms 10.36%  runtime.mallocgc
      90ms  4.05% 40.09%      100ms  4.50%  runtime.scanblock
      80ms  3.60% 43.69%       80ms  3.60%  main.(*Vec3f).Length
      80ms  3.60% 47.30%       80ms  3.60%  runtime.aeshashbody
      80ms  3.60% 50.90%       80ms  3.60%  runtime.heapBitsSetType
      80ms  3.60% 54.50%       80ms  3.60%  runtime.mSpan_Sweep.func1
      80ms  3.60% 58.11%      120ms  5.41%  runtime.scanobject
      70ms  3.15% 61.26%      720ms 32.43%  main.(*GrainWorld).ConstraintsFor
      70ms  3.15% 64.41%      190ms  8.56%  runtime.growslice
      50ms  2.25% 66.67%      150ms  6.76%  main.(*GrainWorld).ConstraintsFor.func2
      40ms  1.80% 68.47%       40ms  1.80%  runtime.futex
      40ms  1.80% 70.27%       90ms  4.05%  runtime.makemap
      40ms  1.80% 72.07%      150ms  6.76%  runtime.mapiterinit
      40ms  1.80% 73.87%       40ms  1.80%  runtime.memmove
      30ms  1.35% 75.23%       40ms  1.80%  main.SphKernel
```

Observe that computation are fairly evenly distributed, and further optimization would require
special memory handling (e.g. replacing map with contant length vector).
