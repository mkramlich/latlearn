latlearn (aka latlearn.go, LatencyLearner, or LL)
    by Mike Kramlich

https://github.com/mkramlich/latlearn

This is a simple instrumentation API and library for Golang software. For measuring and reporting the latency performance of code. Across arbitrary spans. Spans that you name with simple strings.

Extracted from slartboz.go on 2023 Sep 10 around 10:30am local time,
from the private/closed-source Slartboz game's source tree -- where
it was homegrown by its creator to meet that game's early needs for:

    * in-game UX monitoring
    * benchmark regression tests (for basic QA automation & CI/CD pipelines)
    * engine performance tuning & optimization refactors

It is not intended to meet everyone's needs. It scratched an itch, in-house. And it has the benefit of being well-understood by its creator, with no surprises. And it is easy to enhance or augment where desired.

More to come! There is more LL-related code to extract from Slartboz (and clean up, of course), And there's an in-house list of ideas for further enhancement.

For more info on Slartboz (a new sci-fi, post-apoc, real-time Rogue-like, in Golang):
    https://github.com/mkramlich/slartboz-pub

To contact the author, email him at:
    groglogic@gmail.com

thanks

Mike
2023 September 11

