// latlearn/example-app5.go
//     project: https://github.com/mkramlich/LatLearn

package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "./latlearn"
)

// PURPOSE:
//
// * LatLearn use with Golang context.Context:
//     -- with Background, WithCancel (and cancel() called),
//     -- WithTimeout (implying WithDeadline) & its triggering,
//     -- and WithValue (for potentially deeply passed-around latency span names)

type SpanKeyFn2 struct {}

func fn( ctx context.Context) {
    pre :=      "fn"
    log.Printf( "%s\n", pre)

    // Passing span_name down into here via Context.Value was overkill, in this
    // use case. We did it *only* to demo how a LatLearn span name can potentially
    // be handed across multiple API boundaries, goroutines & even processes
    // or hosts.
    span_name, _ := ctx.Value( SpanKeyFn2 {}).( string)

    for { // this loop (& thus this goroutine) will run forever, unless ctx cancelled
        ll := latlearn.B( span_name)
        time.Sleep( 10 * time.Millisecond) // standin for some periodic/polled work
        ll.A()
        // this loop will check, periodically, when it can, if it should end:
        if err := context.Cause( ctx); err != nil {
            log.Printf( "%s: this goroutine's context has cancelled: span %s, reason %v\n", pre, span_name, err)
            break
        }
    }
}

func run_workers( task_id, worker_qty, timeout_secs, cancel_secs int) {
    pre := "run_workers"

    ctx, cancel := context.WithCancel( context.Background())

    log.Printf( "%s: fn goroutines to create: %d\n", pre, worker_qty)

    for i := 0; i < worker_qty; i++ {
        ctx2, _   := context.WithTimeout( ctx, time.Duration( timeout_secs) * time.Second)

        span_name := fmt.Sprintf( "task-%d/worker-%d", task_id, i)
        ctx3      := context.WithValue( ctx2, SpanKeyFn2{}, span_name)

        go fn( ctx3)
    }

    time.Sleep( time.Duration( cancel_secs) * time.Second)

    cancel()
}

func main() {
    pre :=      "example-app5/main"
    log.Printf( "%s\n", pre)

    latlearn.Init()

    latlearn.Report_fpath             = "latlearn-report-app5.txt"
    latlearn.Should_report_builtins   = false
    latlearn.Should_subtract_overhead = true

    defer func() {
        ok  := latlearn.Report()
        ok2 := latlearn.Stop()
        log.Printf( "%s: stopped LatLearn: last report ok %v, stop ok %v\n", pre, ok, ok2)
    }()

    latlearn.Latency_measure_self_sample( -1)

    run_workers( 537, 10, 20, 40) // task id 537 has no special meaning or signif

    // All work under above call will end (typically) sometime around lesser of
    // 20 or 40s. Thus, around 20s -- after all the goroutine Timeouts hit. This
    // is *approx* -- cuz OS scheduling etc, and cuz fn's loop has Sleeps. Note
    // that we provide each goroutine with a unique span name it should use with
    // LatLearn, one which identifies both its relative goroutine id, and a task
    // id. So their metrics show up as separate rows in the report.

    // Now let's do it again! But this time we'll configure it so the orchestrator
    // (the function named run_workers) calls cancel() well before any of the
    // goroutines reach their timeout. Notice task id is diff, and less workers.

    run_workers( 296,  5, 40, 20) // task id 296 has no special meaning or signif

    // Somewhere here, in effect, defer will ensure LatLearn gens report & stops.
}
