LatLearn (aka LL, LatencyLearner or latlearn.go)
    by Mike Kramlich

https://github.com/mkramlich/LatLearn

This is a simple instrumentation API and library for Golang software. For measuring and reporting the latency performance of code. Across arbitrary spans. Spans that you name with simple strings.

To see an ultra short (7 second!) video of what LatLearn can do, here's a link to a screencast clip on YouTube, of a report file `watch` session:
    https://youtu.be/H5EojV3vlYc

For each span you instrument, LatLearn will determine the minimum latency ever observed for it, and the maximum, the mean, and it will remember the last value observed as well. It can report all these PLUS the "weight" of that mean (essentially, the number of completed before()/after() pairs), as well as the "time fraction" spent in/under that span, since LatLearn was initialized. All latencies are measured and reported, explicitly, in nanoseconds.

It supports both "expected" spans -- ones mainly where you care about having a stable ordering of them when printed in the report. As well as "ad hoc" spans. You can specify the expected span names during init. You *must* explicitly make a call to initialize LatLearn *before* your app begins exercising its instrumentation, or requesting any reports, etc.

LatLearn also tracks the cost of its own measurements and reporting. And includes a few built-in benchmarked tasks, to help the user quickly make an apples-to-apples comparison, in their mind, when trying to interpret the meaning of the numbers they are seeing in their own latency reports. This also helps when comparing results ran on different machines with possibly wildly different hardware capabilities or external dependencies (network stacks, env conditions, persistance backends, etc.) All of LatLearn's built-in measurements have a "LL." as the prefix of the span name.

The act of taking a measurement (and stuffing it somewhere) has a cost. In compute and therefore in latency. Though this codebase is young (and relatively NOT optimized, so far) we DO make an attempt to write reasonably efficient code. So as to impose the minimum "overhead" cost from the act of measuring itself. On an older, low-end Apple laptop (by the standards of 2023), the author has consistently seen an overhead cost of around 76 nanoseconds. Per application span (per completed pair of before() and after()) being measured. When using LatLearn. We make a best faith effort to devise a reasonable number for this. Currently, we use the "LL.no-op" span's metrics for it. And it's minimum observed value, in any given run session. Obviously the actual overhead cost, in time, will vary by your host hardware, etc.

Since LatLearn makes an effort to deduce it's own measurement overhead cost it goes one step further to deliver an extra feature. Though it is purely optional. If you toggle the LatLearn variable named ```Should_subtract_overhead``` to true then in all subsequent reports it will automatically subtract out the believed overhead cost, from all the reported latencies (to be clear: for every span's min, last, max and mean.) It does exempt, however, ONE span and stat permutation. It exempts LL.no-op's "min" field value from this auto-compensation logic. The reason why is so we *always* let LL.no-op min's value pass through, unchanged, into the report. So you can better judge the impact it had, especially when automatic overhead subtraction is happening for all the other values shown. In your report, if LL.no-op's "last" value (or max) sometimes appears LOWER than it's "min" (thus, an apparent paradox) then that is why!

Simplest Example:
```
ll := latlearn.B( "foo")
foo()
ll.A()
latlearn.Report()
// by here, has wrote a latency report into a file on your host at "./latlearn-report.txt"
```

Dependencies

None. Latlearn has no dependencies beyond Golang and its standard library/runtime. Therefore, in terms of host or target compatibility it should work fine wherever Golang itself can go.

Transparency

Latlearn is distributed only as open source, 100%. You can see every line of code. Therefore, no surprises or secrets. Important because its instrumentation must be wired directly into your code -- it becomes a part of your code, essentially -- and so you must be able to trust it, completely. You are also free to inspect it for quality, and run your own tests/scans on it, and so forth.

There are no opaque binaries or external service dependencies. There is no "phoning home" to The Cloud. Your data and metrics do not go anywhere else. They exist only inside the process of your app's runtime -- ephemerally -- and, at most, in whatever static report files you choose to generate. This gives you full control. And the report files are plain text, therefore are friendly for version control. Yet *structured* enough to help with further "value-add" processing downstream, in-house, if desired.

The combination of No Dependencies, Total Transparency and Zero Price may be attractive for some devs or teams looking for an alternative to big/heavy/murky commercial packages or cloud services in this problem space. I won't name names but if I coughed and my cough sounded to you like "AtaOgd" or "Ew Relicked" or "Plunks" I'd say you were *fantastically* good at spelling the sounds of a cough! And then I would look at you funny, and slowly back away, all casual like.

Span Variants

Every programmer knows that "at runtime, it's the Wild West!" Meaning that *many* possible variants on the code flow path can play out, at runtime, and especially in prod. ("Hey, it worked on my box!") And the more of these variant permutations there are, the greater the total complexity, and the harder it will be for a programmer to reason about the code, and support it, long term.

Therefore LatLearn is designed to support both the notion of a "default" path for a span, as well as any number of *variants* upon it. And every variant will be considered to be part of the same "family" of spans, all sharing the same root name, in its report on metrics.

In the simplest case (when you just call B() and then A()) there is NO variant. It assumes that the code path between the two was the normal and default path, and therefore its outcome or results were also normal and the default. In that use case, the span is standalone, and not part of any family.

But by using the B2() function, an app can indicate that a variant on the span has just begun -- perhaps one which varies by some set of one or more parameters, or environmental conditions. And by calling A2() you can indicate also, if you wish, that an alternate ending, or outcome, has happened. The most common examples in practice of "end variants" are situations like an "early return" (perhaps carried out by a condition guard block), or, a triggered (or handled, or propagated untouched) panic.

You get to provide a string name for these variants, for both the B2 function (which marks when a span variant begins) and the A2 function (which marks when a span variant ends.) Metrics for these variants are reported as distinct entries in reports -- in their own rows -- but their underlying data is also shared by (and counted towards) the metrics for their family's single common "parent" span -- which is also listed as its own row in the report.

Variant names are a string. They are fairly free-form and their exact syntax is up to the application and their preferences. Only a few rules or patterns are enforced. First, that if both a B2() variant name was passed, and, an A2() variant name was passed, then they get combined into a single variant name string, separated by a comma, like: "my-B2-variant,my-A2-variant". Also, for purposes of reports (and for LatLearn's internal storage and lookups) it forms a compound key using a certain format rule. If a span has NO variants, or, is the parent of a family of variants, then it's key is just it's bare name. So span "foo" is also "foo" in the reports, and how it is looked-up under the hood. But to form a variant's full name (for purposes of key lookup, and reports, only), it *combines* them, in this format:

```    span-base-name(variant-name)```

To illustrate a little better:

```    "B-param"```
or:

```    "B2-param1(B2-param2,A2-param1)"```

Which could result in these examples of (totally arbitrary but) real world span names:

```
    "fn"
    "yourpkg/main/fn2"
    "twiddle_bits"
    "process-reports"
    "process-reports(N=100,R=3)"
    "process-reports(N=100,R=3,earlyreturn=due-to-foo-nil)"
    "whatever(goroutines=4,logging=off)"
    "do-task7(panicked,err=errtype,origin=X)"
    "go-work(timeout=100s,cancelled=45s)"
```

Making span variants a first class feature of LatLearn means that you can more easily tell why a particular metric looks much bigger (or much smaller!) than expected. Did some cases have lower latency because a function did an early return? Did some cases have a higher latency because the function parameters passed gave an argument (eg. some "n int") that caused the function body to perform more total compute work (or, simply spent more time waiting/asleep) than otherwise? And by *breaking* these cases out, explicitly, in our data set, it helps to refine the value of the "signal" you get from the reports, and from running your own benchmarks, or when doing your own regression testing, or tuning sprints. Indeed, *any* perf-relevant parameters, environmental conditions, or runtime edge cases can be syntactically "noted" (esp via dynamic iterpolation using string templates), easily, in LatLearn's instrumentation. And then the fact of their occurence gets to "pass through" into the reports, and be visible explicitly in the metric rows and their names, and how they are grouped there.

Span variants -- and our instrumentation's support for them -- are a phenomenon which are arguably a signature strength of LatLearn: a credible argument for *why* it might be worth adding to one's Golang development toolbox. Because it is how LatLearn can *differentiate* itself both from what Golang provides out-of-the-box, plus, differeentiate itself from more traditional, external and "hands off" profilers. Instrumenting with LatLearn allows you to profile your code in a way where the "profiler" in question (LatLearn) truly takes advantage both of Golang's language capabilities *and* the entire standard library, but *also* the fact that it gets instrumented by hand -- by an app's creator (or maintainer) -- and therefore is someone who has the knowledge of which *app-specific* factors & metadata to "bake-in" to the captured sample metrics. Thus, while it can take a little more work, upfront, to profile code with it, the trade-off's "win" is to get potentially much more value, in the long run, due to getting more *actionable* signal, and enabling finer-grained deductions. It has provided that for the author, anyway, so far.

Defers, Panics & Panic Recovery

LatLearn plays well with them, and in the ways you probably would expect.

For a concrete demonstration see [./example-app4.go](./example-app4.go).

By the way, the example code above also shows how LatLearn behaves in the case when pair-matching "end-of-span" calls fail to be made (the A()s or A2()s), for whatever reason, or, are made but *redundantly*. Hint: it does the *right* thing -- by silently ignoring them, and with no stat distortions, leaks or hangs.

Contexts

LatLearn has been designed to play well with Golang contexts. For concrete demos (including how it might interact with cancelled (and possibly deeply inherited/derived) contexts, deadlines, timeouts and "WithValue" per-context state) see [./example-app5.go](./example-app5.go)

Concurrency, Goroutines & Thread/Memory Safety

LatLearn is safe for use by processes running multiple goroutines, each with code paths instrumented via LatLearn. See [./latlearn/latlearn.go](./latlearn/latlearn.go) and [./example-app3.go](./example-app3.go) for more detail on exactly how and why.

Real Use Cases

Here's a brief write-up of a real use case where LatLearn's instrumentation and reporting  was used to help identify an inefficient code path, and then to confirm that a performance refactor was a success: [./benefits-example.md](./benefits-example.md)

Example App Code, in Action

To see many concrete demonstrations of LatLearn's features and supported use case permutations see the full set of example apps below (all of which are included in this repo):

* [./example-app1.go](./example-app1.go)
* [./example-app2.go](./example-app2.go)
* [./example-app3.go](./example-app3.go)
* [./example-app4.go](./example-app4.go)
* [./example-app5.go](./example-app5.go)

To build and run them: [./buildrun.sh](./buildrun.sh)

Reports

To help understand what LatLearn can do, there is a (very simple) sample of a latency report file included in this repo. It is at [./report-examples/latlearn-report.txt](./report-examples/latlearn-report.txt). But it is also recommended that you run [./buildrun.sh](./buildrun.sh) and poke around.

Caveats

The LatLearn code is NOT intended to meet everyone's needs. It scratched an itch, in-house. And it has the benefit of being well-understood by its creator, with no surprises. And it is easy to enhance or augment where desired.

There is MUCH more to come! There is more LL-related code to extract from Slartboz (and clean up, of course.) And there's a long list of in-house ideas for further enhancement.

Project Origin

By the way, the original version of LatLearn started life when it was extracted from it's author's "slartboz.go" file, on LatLearn's "birth day" of 2023 Sep 10. A file from his Slartboz game's private/closed Golang app source tree. It was homegrown there in order to meet that game's early goals/needs for:

* in-game UX monitoring & dynamic adjustment of task strategies, to maintain QoS
* benchmark regression tests (for basic QA automation & CI/CD pipelines)
* engine performance tuning & optimization refactors

And where it remains linked into and useful there, still today.

Consulting

The author specializes in the performance and scalability of software. And has long had a passion for it, going back to his days as a starry-eyed, would-be Physics major back in his college days. He can be available to help out with your system and legacy codebases, ideally as a remote contractor.

Performance upgrades. Troubleshooting. Improving the scalability of your architecture. Identifying and removing bottlenecks. Tuning pass follow-up to your own internal, biz-specific or feature-oriented efforts. Shrinking latencies can yield a better UX and more happy users. And a higher (better) SEO ranking in the eyes of Google, for example. By shrinking latencies it tends to also boost the total "payload" throughput. And reduce the compute load burden on your servers -- and *that* in turn can reduce your total *billing* spend each month. Spend a little money (on an expert consultant) to SAVE much more money going forward. And the benefits of a performance tuning effort yield even bigger profit boosts (or net savings) the *earlier* they are performed, and the more thorough.

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
2023 November 8

