latlearn (aka latlearn.go, LatencyLearner, or LL)
    by Mike Kramlich

https://github.com/mkramlich/latlearn

This is a simple instrumentation API and library for Golang software. For measuring and reporting the latency performance of code. Across arbitrary spans. Spans that you name with simple strings.

Simple Example:
```
latlearn_init( []string { "foo", "bar"})
ll := llB( "foo")
foo()
ll.A()
latency_report_gen( []string { "ver=1.2", "commit=whatever", "N=2", "cores=2"})
// it just wrote a report (on latency stats) into a file at "./latency-report.txt"
```

Extracted from slartboz.go on 2023 Sep 10 around 10:30am local time,
from the private/closed-source Slartboz game's source tree -- where
it was homegrown by its creator to meet that game's early needs for:

    * in-game UX monitoring
    * benchmark regression tests (for basic QA automation & CI/CD pipelines)
    * engine performance tuning & optimization refactors

To help understand what latlearn can do, there is a screenshot included of a metrics dashboard window. One run from a terminal, and based on a 'watch' of a report file. This file is generated only when an app calls the fn in this module named latency_report_gen. The screenshot is:
![](./report-watch-screenshot.png)

It is not intended to meet everyone's needs. It scratched an itch, in-house. And it has the benefit of being well-understood by its creator, with no surprises. And it is easy to enhance or augment where desired.
n
More to come! There is more LL-related code to extract from Slartboz (and clean up, of course.) And there's an in-house list of ideas for further enhancement.

For more info on Slartboz (a new sci-fi, post-apoc, real-time Rogue-like, in Golang):
    https://github.com/mkramlich/slartboz-pub

To contact the author, email him at:
    groglogic@gmail.com

thanks

Mike
2023 September 11

