// latlearn/example-app2.go
//     project: https://github.com/mkramlich/latlearn

package main

import (
    "fmt"
    "math/rand"
    "time"
)

func needed_tasks() {
    sleep := time.Duration(1 + rand.Intn(  25)) // for a sleep of 1 to  25 ms in duration
    time.Sleep( sleep * time.Millisecond) // standin for some task that blocks for some amount of time
}

func optional_tasks() {
    sleep := time.Duration(1 + rand.Intn( 200)) // for a sleep of 1 to 200 ms in duration
    time.Sleep( sleep * time.Millisecond) // standin for some task that blocks for some amount of time
}

func should_do_optional_tasks( span string) bool {
    lli, found := latency_learners[ span]

    if  !found                 { return true}

    // By here we don't know if lli points at a LatencyLearner or a
    // VariantLatencyLearner. However they do both contain an LL instance, so
    // lets grab that and use it below:
    ll := lli.getLL()

    if !ll.pair_ever_completed { return true}

    loop_latency_mean, weight := ll.mean_latency() // mean value returned is int64 of ns

    // The case we guard for here SHOULD never happen. Only to be rigorous:
    if (weight < 1)            { return true}

    if (loop_latency_mean > int64( 50000000)) { // 50,000,000 ns is 50 ms
        // We don't want our loop to exceed 1 ms in total latency,
        // if we can help it. Therefore lets skip over all optional tasks during
        // this iteration
        return false
    }
    return true
}

func engine_loop( dyn_adjust bool) {
    ll_variant := fmt.Sprintf( "dyn_adjust=%v", dyn_adjust)
    fmt.Printf( "engine_loop: %s\n", ll_variant)

    for  i := 0; i < 40; i++ {
        ll := llB2( "loop", ll_variant)

        needed_tasks()

        if (!dyn_adjust || (dyn_adjust && should_do_optional_tasks( ll.name))) {
            fmt.Printf("%2d: WILL do optional_tasks\n", i)
            optional_tasks()
        } else {
            fmt.Printf("%2d: will NOT do optional_tasks\n", i)
        }

        ll.A()
    }
}

func main() {
    // Here we'll show off how latlearn can be used by a program to implement
    // dynamic adjustment of execution strategies -- while running "in prod"
    // -- in order to maintain some ideal QoS goal, or business requirement.
    //
    // In this case, to help ensure a peppy UX by dropping unnecessary (and
    // therefore optional) tasks (or sleeps, as their standins here) but only
    // IF/WHEN the overall "engine" loop latency begins to trend too high.

    // We call init here so its latency cost not incurred while inside engine_loop.
    latlearn_init()

    // We do benchmarks here mainly because we'd like the LL.no-op min metric
    // to be populated (and with a reasonable value to use), so that we can take
    // advantage of the "subtract_overhead" feature later on, during report gen.
    latlearn_benchmarks()

    engine_loop( false) // WITHOUT enablement of dynamic adjustments to maintain QoS
    engine_loop( true)  // WITH it

    latlearn_report_fpath             = "latlearn-report-dynadj.txt"
    latlearn_should_report_builtins   = true
    latlearn_should_subtract_overhead = true

    latlearn_report()

    // Now look at the report generated. Compare the diff in UX/QoS delivered
    // between cases when using a (latlearn-based) "dynamic adjustment" strategy,
    // versus, not. Obviously, much more sophisticated strategies can be devised,
    // and they should always be tailored to the unique characteristics of a
    // system's technical and business requirements.
}