// latlearn/example-app4.go
//     project: https://github.com/mkramlich/LatLearn

package main

import (
    "fmt"
    "log"
    "runtime/debug"

    "./latlearn"
)

// PURPOSE:
//
// * use of Golang defer
// * dealing with panics and panic recover
// * alternate span ending cases -- via variants (VLL's) -- identified by calling ll.A2()
// * demonstrate fact that "never ended" spans wont break anything -- quietly ignored
// * demonstrate fact that "redundantly ended" spans wont break anything -- quietly ignored

func fn( id, id_last int) {
    pre :=            "fn"
    log.Printf(       "%s: id %d\n", pre, id)

    ll := latlearn.B( "example-app4/fn")
    defer ll.A()

    // ... assume that the code span to measure is here ...

    return_case    := (id % 2) // yields values in range 0 to 1
    if (id == id_last) {
        return_case = 2
    }

    switch return_case { // by here, return_case has value in range 0 to 2

    case 0: // normal return, via last line of fn
        log.Printf( "%s: case 0\n", pre)

    case 1: // early return, and inside this case:
        log.Printf( "%s: case 1\n", pre)
        ll.A2( "earlyreturn")
        return

    case 2:
        // leave fn via panic(), to help demo the LL span will still get
        // its default call to A() (via defer)
        log.Printf( "%s: case 2\n", pre)
        ll.A2( "panic")
        panic( "OMG")
    }

    // if we left this fn via case 1 or 2 above (early return or panic) then
    // the "defer A()" will be called, but it will be ignored. Because, prior to
    // that, A2() will have been called, and actually used. It will have recorded
    // the latency metrics under a *variant* span name. One passed to A2() above.

    return
}

func panic_handler_for_main( panicked *bool) {
    if err          := recover(); err != nil {
        *panicked    = true

        stack_trace := string( debug.Stack())

        msg         := fmt.Sprintf(
                           "panic bubbled up to main(), so will attempt to make LatLearn generate report before program exit; panic err: %p, %v, %#v\npanicked goroutine's stacktrace:\n%s",
                               &err, err, err, stack_trace)
        log.Printf( "%s\n", msg)
        log.Printf( "NOTE: if you saw a scary stack trace just above, that is expected. Like how \"DON'T PANIC!\" was written on the cover of a book once, in a large and comforting font.\n")

        // NOTE: The following Report call is not necessarily *wise* to do in an
        // app's panic recovery handler. Why? Because of the possibility of cases
        // where panic happened in/under a LatLearn call, or, if it might NOT
        // have BUT instead a subsequent call to Latlearn.Report might itself
        // hang (due, for example, to runtime conditions having gone bad). We
        // *only* demonstrate here that it *is* a useful pattern, in the general
        // case: where LatLearn's runtime state remains ok and we want to
        // maximize chance that a report is generated before process exit.
        ok := latlearn.Report()
        log.Printf("latlearn report generated: %v\n", ok)
    }
}

func main() {
    pre :=      "example-app4/main"
    log.Printf( "%s\n", pre)

    latlearn.Init()

    defer func() {
        // NOTE: There is no strict need by LatLearn to ensure that either Report
        // or Stop get called before an app ceases its use of it, or before
        // program exit. We only register "defer" calls here as an excuse to
        // demonstrate a usage pattern that is supported (and known to work
        // correctly.) However, if your program reaches a point where you are
        // sure it will not make any further calls to the LatLearn API, and you
        // want to minimize resource usage a tad more, feel free to call its Stop.
        // In that scenario, a failure to call Stop means only that there will
        // remain 1 extra idle (ie. waiting) goroutine than otherwise, blocked
        // (inside latlearn.go's serve()) trying to read from it's "outer" comm
        // channel. If there's a chance your process *might* resume calls to the
        // API then NOT stopping it is likely a small enough price, and wiser.
        log.Printf( "%s: stopping LatLearn\n",       pre)
        ok := latlearn.Stop()
        log.Printf( "%s: stopped LatLearn: ok %v\n", pre, ok)
    }()

    latlearn.Report_fpath             = "latlearn-report-app4.txt"
    latlearn.Should_report_builtins   = false
    latlearn.Should_subtract_overhead = true // default for this is to use the min field of OVERHEAD_SPAN

    latlearn.Latency_measure_self_sample(-1) // default attempts capture of 1M samples of OVERHEAD_SPAN

    panicked := false
    defer panic_handler_for_main( &panicked)

    //////////////////////

    // The purpose of the following code is to demonstrate that it is possible to NOT end a span (in other words, to simply fail to ever make its pair-corresponding call to A() or A2()) and it will NOT hurt your app, mess up the stats, or break LatLearn.
    span_name := fmt.Sprintf( "%s/big-iloop-iter", pre)
    n         := 10_000_000
    log.Printf( "%s: will now do %d iters using span name %s\n", pre, n, span_name)
    for i := 0; i < n; i++ {
        variant        := ""
        if (i < 100) {
            variant     = "i<100"
            if (i < 10) {
                variant = "i<10"
            }
        }
        ll := latlearn.B2( span_name, variant)
        if (i < 100) {
            // These iterations WILL get their span's A() called, as normal.
            ll.A()
            if (i < 10) {
                // These iterations will get their span's A() called MANY times,
                // redundantly. Though all but the 1st A() call will be silently
                // ignored by LatLearn. It will not mess up the stats.
                for j := 0; j < 5; j++ {
                    ll.A()
                    ll.A2( "")
                    ll.A()
                    ll.A2( "this-is-irrelevant-cuz-will-be-ignored")
                }
            }
        }
    }
    // In the report, you should see that the code block just above results in
    // data for only 100 samples of that span -- not 10M. The remaining 9,999,900
    // iters will NOT be reflected in the metrics there. Also, within those 100
    // samples, even though the first 10 of them had their spans ended MULTIPLE
    // times, redundantly, it did NOT distort the stats (ie. did NOT artificially
    // inflate the reported weight, or cumul, or impact the mean.) And, it broke
    // neither LatLearn or the app. The un-ended "ll" objects (which, under the
    // hood, are instances of the struct latlearn.SpanSampleUnderway) simply go
    // away, cleanly, once Golang's GC gets around to it.

    ///////////////////////////

    // The purpose of the next code is to demonstrate situations where a span might
    // be ended in multiple distinct ways -- for example, via an early return
    // condition, or, a panic -- and how those cases can be made to work cleanly
    // with LatLearn, via *distinctly* tracked latency stats, though stats which
    // all still share the same family of span variants, via a common "parent" span
    // entry (where the latter's key is the base "name" of every span in the variant
    // family.) The reason we registered the so-called panic handler earlier in main
    // was in order so our program could recover gracefully in the case where a
    // panic might be induced down below. (And it will happen, to demonstrate.)
    n  = 10
    log.Printf( "%s: calls to fn: %d\n", pre, n)

    for i := 0; i < n; i++ {
        fn( i, (n - 1)) // in last iter of this loop, this fn call will panic()
    }
}
