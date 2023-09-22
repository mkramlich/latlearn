latlearn (aka latlearn.go, LatencyLearner, or LL)
    by Mike Kramlich

https://github.com/mkramlich/latlearn

This is a simple instrumentation API and library for Golang software. For measuring and reporting the latency performance of code. Across arbitrary spans. Spans that you name with simple strings.

To see an ultra short (7 second!) video of what latlearn can do, here's a link to a screencast clip on YouTube, of a report file `watch` session:
    [https://youtu.be/H5EojV3vlYc]

For each span you instrument, latlearn will determine the minimum latency ever observed for it, and the maximum, the mean, and it will remember the last value observed as well. It can report all these PLUS the "weight" of that mean (essentially, the number of completed before()/after() pairs), as well as the "time fraction" spent in/under that span, since latlearn was initialized. All latencies are measured and reported, explicitly, in nanoseconds.

It supports both "expected" spans -- ones mainly where you care about having a stable ordering of them when printed in the report. As well as "ad hoc" spans. You can also choose to do either an "eager init" of latlearn, or a lazy init. A lazy init happens if you call llB() before having previously made an explicit call to init latlearn. In that case, latlearn will init itself (prepare it's internal state as needed) for you, under the hood.

Latlearn also tracks the cost of its own measurements and reporting. And includes a few built-in benchmarked tasks, to help the user quickly make an apples-to-apples comparison, in their mind, when trying to interpret the meaning of the numbers they are seeing in their own latency reports. This also helps when comparing results ran on different machines with possibly wildly different hardware capabilities or external dependencies (network stacks, env conditions, persistance backends, etc.) All of latlearn's built-in measurements have a "LL." as the prefix of the span name.

The act of taking a measurement (and stuffing it somewhere) has a cost. In compute and therefore in latency. Though this codebase is young (and relatively NOT optimized, so far) we DO make an attempt to write reasonably efficient code. So as to impose the minimum "overhead" cost from the act of measuring itself. On an older, low-end Apple laptop (by the standards of 2023), the author has consistently seen an overhead cost of around 76 nanoseconds. Per application span (per completed pair of before() and after()) being measured. When using latlearn. We make a best faith effort to devise a reasonable number for this. Currently, we use the "LL.no-op" span's metrics for it. And it's minimum observed value, in any given run session. Obviously the actual overhead cost, in time, will vary by your host hardware, etc.

Since latlearn makes an effort to deduce it's own measurement overhead cost it goes one step further to deliver an extra feature. Though it is purely optional. If you toggle the latlearn variable named "latlearn_should_subtract_overhead" to true then in all subsequent reports it will automatically subtract out the believed overhead cost, from all the reported latencies (to be clear: for every span's min, last, max and mean.) It does exempt, however, ONE span and stat permutation. It exempts LL.no-op's "min" field value from this auto-compensation logic. The reason why is so we *always* let LL.no-op min's value pass through, unchanged, into the report. So you can better judge the impact it had, especially when automatic overhead subtraction is happening for all the other values shown. If LL.no-op's "last" appears higher than it's "min" then that is why!

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

There is MUCH more to come! There is more LL-related code to extract from Slartboz (and clean up, of course.) And there's a long list of in-house ideas for further enhancement.

Consulting

The author specializes in the performance and scalability of software. And has long had a passion for it, going back to his days as a starry-eyed, would-be Physics major back in his college days. He can be available to help out with your system and legacy codebases, ideally as a remote contractor.

Performance upgrades. Troubleshooting. Improving the scalability of your architecture. Identifying and removing bottlenecks. Tuning pass follow-up to your own internal, biz-speciic or feature-oriented efforts. Shrinking latencies can yield a better UX and more happy users. And a higher (better) SEO ranking in the eyes of Google, for example. By shrinking latencies it tends to also boost the total "payload" throughput. And reduce the compute load burden on your servers -- and *that* in turn can reduce your total *billing* spend each month. Spend a little money (on an expert consultant) to SAVE much more money going forward. And the benefits of a performance tuning effort yield even bigger profit boosts (or net savings) the *earlier* they are performed, and the more thorough.

Golang codebases (on Linux or Mac) are ideal, but also can work with C, Python and Java.

Contact the author for a free, initial conversation.

Slartboz

For more info on Slartboz (a new sci-fi, post-apoc, real-time Rogue-like, in Golang):
    https://github.com/mkramlich/slartboz-pub

Latlearn Contribution Policy

New feature or fix suggestions are welcome! Praise feedback, especially! Or small tips, or blatant bribes. However, note that we are NOT taking any PRs or direct code contributions, at this time.

To contact the author, email him at:
    groglogic@gmail.com

thanks!

Mike
2023 September 21

