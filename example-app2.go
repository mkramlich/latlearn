// latlearn/example-app2.go
//     project: https://github.com/mkramlich/LatLearn

package main

import (
    "fmt"
    "math/rand"
    "time"

    "./latlearn"
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
    replyMsg, ok := latlearn.Values( span)
    if       !ok                     { return true}
    if !replyMsg.Pair_ever_completed { return true}

    // The case we guard for here SHOULD never happen. Only to be rigorous:
    if (replyMsg.Weight < 1)         { return true}

    if (replyMsg.Mean > int64( 50_000_000)) { // 50,000,000 ns is 50 ms
        // We don't want our loop to exceed 50 ms in total latency,
        // if we can help it. Therefore lets SKIP over all OPTIONAL tasks
        // during this iteration
        return false
    }
    return true
}

func engine_loop( dyn_adjust bool) {
    ll_variant := fmt.Sprintf( "dyn_adjust=%v", dyn_adjust)
    fmt.Printf( "engine_loop: %s\n", ll_variant)

    for  i := 0; i < 40; i++ {
        ll := latlearn.B2( "loop", ll_variant)

        needed_tasks()

        if (!dyn_adjust || (dyn_adjust && should_do_optional_tasks( ll.Name))) {
            fmt.Printf("%2d: WILL do optional_tasks\n", i)
            optional_tasks()
        } else {
            fmt.Printf("%2d: will NOT do optional_tasks\n", i)
        }

        ll.A()
    }
}

func main() {
    fmt.Printf( "example-app2\n")
    // Here we'll show off how latlearn can be used by a program to implement
    // dynamic adjustment of execution strategies -- while running "in prod"
    // -- in order to maintain some ideal QoS goal, or business requirement.
    //
    // In this case, to help ensure a peppy UX by dropping unnecessary (and
    // therefore optional) tasks (or sleeps, as their standins here) but only
    // IF/WHEN the overall "engine" loop latency begins to trend too high.

    // We call init here so its latency cost not incurred while inside engine_loop.
    latlearn.Init()

    // We do this next step because we'd like the LL.no-op min metric
    // to be populated (and with a reasonable value to use), so that we can take
    // advantage of the "subtract_overhead" feature later on, during report gen.
    latlearn.Latency_measure_self_sample(-1) // default attempts capture of 1M samples of LL.no-op

    engine_loop( false) // WITHOUT enablement of dynamic adjustments to maintain QoS
    engine_loop( true)  // WITH it

    latlearn.Report_fpath             = "latlearn-report-dynadj.txt"
    latlearn.Should_report_builtins   = true
    latlearn.Should_subtract_overhead = true

    latlearn.Report()

    // Now look at the report generated. Compare the diff in UX/QoS delivered
    // between cases when using a (latlearn-based) "dynamic adjustment" strategy,
    // versus, not. Obviously, much more sophisticated strategies can be devised,
    // and they should always be tailored to the unique characteristics of a
    // system's technical and business requirements.
}
