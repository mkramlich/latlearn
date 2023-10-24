// latlearn/example-app1.go
//     project: https://github.com/mkramlich/LatLearn

package main

import (
    "fmt"

    "./latlearn"
)

func fn1() {
    span :=      "fn1"
    ll   := latlearn.B( span)
    fmt.Printf(  "%s\n", span)
    ll.A()
}

func fn2() {
    ll := latlearn.B(  "fn2")
    ll.A()
}

func fn3( n int) {
    ll    := latlearn.B2( "fn3", fmt.Sprintf( "n=%d", n))
    v     := 0
    for i := 0; i < n; i++ {
        v += (i * 2) // do something here. what matters not. we just wanted to do n iters
    }
    ll.A()
}

func fn4( a int, b int) {
    ll     := latlearn.B2( "fn4", fmt.Sprintf( "a=%d,b=%d", a, b))

    // what we do here exactly does not matter. we only want the total compute to depend on a and b
    v      := 0
    as     := []int {}
    for ai := 0; ai < a; ai++ {
        as  = append( as, 5)
    }
    for bi := 0; bi < b; bi++ {
        for _, aa := range as {
            v     += (bi * aa * 2)
        }
    }

    ll.A()
}

// NOTE: On a slower Mac laptop, this main() takes over 1 minute to finish,
// normally. Most of the running time is caused by calling latlearn.Benchmarks.
func main() {
    fmt.Printf( "example-app1\n")

    // For all the following, we assume latlearn.go is a local file. In your compile path & brought into your own module's namespace. You can look in ./buildrun.sh to see this example app's buildtime and runtime assumptions.

    latlearn.Init2( []string { "print-yo", "fn1", "fn2"}) // spans of yours it should expect. in report order

    ll := latlearn.B(  "print-yo")
    fmt.Printf( "yo\n")
    ll.A()

    fn1()

    // let's print a basic report:
    latlearn.Report()

    // We might want to include some context params in our report. If so, give the report generating function a list of strings. A context parameter might be some fact or metadata that the audience would feel is relevant, especially to make a correct interpretation of the metrics, and a correct deduction about their own next course of action.

    params := []string { "ver=1.2", "commit=whatever", "N=2", "cores=2"}
    latlearn.Report2( params)

    // It just wrote a report (on latency stats) into a file at "./latlearn-report.txt"

    // If you look at the report you'll see a lot of ??? for some LL benchmark spans
    // because they were not performed. They are purely optional.

    // If you wish to perform them call Benchmarks:
    log.Printf( "about to call latlearn.Benchmarks. can take over 1 min on slower Mac\n")
    latlearn.Benchmarks()

    // Now let's write a new report. but let us put it in a different file

    // To change the report's file path, do like:
    latlearn.Report_fpath = "latlearn-report2.txt"
    // There were no changes to any context param. we just want to see the benchmark metrics which should now appear in the report:
    latlearn.Report2( params)

    // NOTE: All of LL's built-in spans (like for benchmarks) have names starting with "LL."
    // By the way, latlearn also measures the latency of its own report generation. It names that span "LL.lat-report"

    // Let's do some more measurements
    for i := 0; i < 100_234; i++ {
        fn2()
    }

    // The below may start to feel a little verbose (boilerplatey) but we express it like this to reinforce that you can continually adjust the report params and file path as you go. which is helpful for testing, and for doing comparisons while doing troubleshooting or tuning work

    latlearn.Report_fpath = "latlearn-report3.txt"
    latlearn.Report2( params)

    // In this report you'll see metrics for fn2 reported for the first time.
    // Note that fn2's AVG latency is (almost certainly) smaller (faster) than fn1. That is because its code body is similar, except it lacks any Printf call. Typically, turning off any output writes (esp to log files) yields a significant reduction in latency (on a typical machine, ayway.) Though in this case fn2 contributes to much more of the program's overall total run latency, measured from process start to end. Because, unlike fn1, it was called 100,234 times.

    for i := 0; i < 2; i++ {
        ll = latlearn.B(  "totally-adhoc") // this was not in the initial set of tracked_spans
        // but this ad hoc span measurement will work correctly, anyway
        ll.A()
    }
    latlearn.Report_fpath = "latlearn-report4.txt"
    latlearn.Report() // next time you gen any report, that span will be added to end

    latlearn.Report_fpath = "latlearn-report5.txt"
    latlearn.Should_report_builtins = false
    latlearn.Report() // this report will NOT include any of latlearn's built-in spans

    // Now let's make a report where lastlearn will try to subtract the measuring cost
    // from all reported span metrics. It is only a "good faith" effort. An estimate or
    // guess. For this purpose it uses the metrics for the built-in "LL.no-op" and uses
    // it's currwnt "min" value for latency. An observed minimum is a good faith attempt
    // to try to determine the true, inescapable latency cost of any task (at least on
    // the current hardware, and under ideal conditions) but there are too many other
    // impacting conditions which are outside our control (typically) to be able to say
    // with 100% confidence that we know the TRUE minimum and therefore "correct" cost.
    // By this time, in everything we've told you and shown you so far about latlearn,
    // you should appreciate why we did make an effort to "self-sample" and include
    // various built-in benchmarks, especially the "no-op" span. So we *would* be able
    // to make a reasonable estimate of the latency costs that were imposed/experienced
    // by latlearn, itself.

    latlearn.Report_fpath = "latlearn-report6.txt"
    latlearn.Should_report_builtins   = true
    latlearn.Should_subtract_overhead = true // this is false by default
    latlearn.Latency_measure_self_sample(-1) // default attempts capture of 1M samples of LL.no-op
    latlearn.Report() // in report notice that all the previous reported metrics shrunk
    // Except... for LL.no-op's min. That is the only span and field we EXEMPT from this overhead compensation feature. We exempt it so that it's original value passes thru into the generated report. So you know by *how* much the other reported numbers have shrunk!

    fn3(                 1)
    fn3(                50)
    fn3(            20_000)
    fn4(         1,      1)
    fn4(         1,    100)
    fn4(        10,     20)
    for i := 0; i < 100; i++ {
        fn4( 1_000,     15)
    }
    // in report we gen here, notice added stats for all fn3 & fn4 param variants, tracked separately:
    latlearn.Report_fpath = "latlearn-report7.txt"
    latlearn.Report()
}
