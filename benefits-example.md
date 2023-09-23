Example of Real World Benefits

Here's an example snippet taken from latlearn's report files (well, grepped & merged
some) that illustrates how latlearn's custom instrumentation and reporting features
can be helpful when attempting to do a performance refactor, and ensure no regressions.

Why a performance refactor? To improve the UX impact of a latency-sensitive code path,
like in a tight loop in a game engine, for example.

The measured span below ("is-mv-bl") was a function that was called very frequently
-- anytime that any moving sprite (eg. a creature or gun projectile) was moving and
needed to check whether it was physically possible to move into a certain specific new
location. Or whether it would be blocked or denied for any reason. Since the game
features sometimes very high numbers of moving sprites you can imagine this function
would need to be called quite a lot, and could easily become part of a "hot path"
deserving of extra scrutiny.

Eventually, this one did.

```
span                     :        min (ns) |       last (ns) |        max (ns) |       mean (ns) | weight (B&As) |   time frac | span 
is-mv-bl (orig)          :             181 |           8,809 |      14,192,814 |          13,963 | w      12,532 | tf 0.017637 | is-mv-bl
is-mv-bl (perf refactor) :               8 |             242 |         167,009 |             350 | w      13,440 | tf 0.000358 | is-mv-bl
```

(carried out during a Slartboz dev session, sometime in 2023 September)

