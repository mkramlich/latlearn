latlearn (aka latlearn.go, LatencyLearner, or LL)
    by Mike Kramlich

https://github.com/mkramlich/latlearn

This is a simple instrumentation API and library for Golang software. For measuring and reporting the latency performance of code. Across arbitrary spans. Spans that you name with simple strings.

For each span you instrument it will determine the minimum latency ever observed for it, and the maximum, the mean, and the remember the last value observed as well. It can report all these plus the "weight" of that mean (essentially, the number of completed before()/after() pairs), as well as the "time fraction" spent in/under that span, since latlearn_init was called. All latencies are measured and reported in nanoseconds.

It supports both "expected" spans -- ones mainly where you care about having a stable ordering of them when printed in the report. As well as "ad hoc" spans. You can also choose to do either an "eager init" of latlearn, or a lazy init. A lazy init happens if you call llB() before having previously made an explicit call to init latlearn. In that case, latlearn will init itself (prepare it's internal state as needed) for you, under the hood.

Latlearn also tracks the cost of its own measurements and reporting. And includes a few built-in benchmarked tasks, to help the user quick make apples-to-apples comparisons when trying to interpret the meaning of the numbers they are seeing in their own reports. This also helps when comparing results ran on different machines with possibly wildly different hardware capabilities or external dependencies (network stacks, persistance backends, etc.) All of latlearn's built-in measurements have a "LL." as the prefix of the span name.

Simplest Example:
```
ll := llB( "foo")
foo()
ll.A()
latlearn_report()
// it just wrote a report (on latency stats) into a file at "./latlearn-report.txt"
```

Dependencies

None. Latlearn has no dependencies beyond Golang and its standard library/runtime. Therefore, in terms of host or target compatibility it should work fine wherever Golang itself can go.

Transparency

Latlearn is distributed only as open source, 100%. You can see every line of code. Therefore, no surprises or secrets. Important because its instrumentation must be wired directly into your code -- it becomes a part of your code, essentially -- and so you must be able to trust it, completely. You are also free to inspect it for quality, and run your own tests/scans on it, and so forth.

There are no opaque binaries or external service dependencies. There is no "phoning home" to The Cloud. Your data and metrics do not go anywhere else. They exist only inside the process of your app's runtime -- ephemerally -- and, at most, in whatever static report files you choose to generate. This gives you full control. And the report files are plain text, therefore are friendly for version control. Yet *structured* enough to help with further "value-add" processing downstream, in-house, if desired.

The combination of Total Transparency, Zero Dependencies, and Zero Price may be attractive for some devs or teams looking for an alternative to big/heavy/murky commercial packages or cloud services in this problem space. I won't name names but if I coughed and my cough sounded to you like "AtaOgd" or "Ew Relicked" or "Plunks" I'd say you were *fantastically* good at spelling the sounds of a cough! And then I would look at you funny, and slowly back away, all casual like.

The Full Story

For more complex examples, features and permutations see `./example-app1.go` and use `./buildrun.sh` to run it.

Extracted from it's author's "slartboz.go" file, originally, on 2023 Sep 10, from the private/closed-source Slartboz game's source tree. It was homegrown there in order to meet that game's early goals/needs for:

    * in-game UX monitoring & dynamic adjustment of task strategies, to maintain QoS
    * benchmark regression tests (for basic QA automation & CI/CD pipelines)
    * engine performance tuning & optimization refactors

To help understand what latlearn can do, there is a (very simple) sample of a latency report file included in this repo. It is at `./latency-report.txt` But it is also recommended that you run `./buildrun.sh` and poke around.

The latlearn code is NOT intended to meet everyone's needs. It scratched an itch, in-house. And it has the benefit of being well-understood by its creator, with no surprises. And it is easy to enhance or augment where desired.

There is MUCH more to come! There is more LL-related code to extract from Slartboz (and clean up, of course.) And there's a lost list of in-house ideas for further enhancement.

For more info on Slartboz (a new sci-fi, post-apoc, real-time Rogue-like, in Golang):
    https://github.com/mkramlich/slartboz-pub

To contact the author, email him at:
    groglogic@gmail.com

New feature or fix suggestions are welcome! Praise feedback, especially! Or small tips, or blatant bribes. However, note that we are NOT taking any PRs or direct code contributions, at this time.

thanks!

Mike
2023 September 21

