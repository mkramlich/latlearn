// latlearn.go aka LatencyLearner
//     by Mike Kramlich
//
//     started  2023 September
//     last rev 2023 September 13
//
//     contact: groglogic@gmail.com
//     project: https://github.com/mkramlich/latlearn

package main

import (
    "fmt"
    "io"
    "log"
    "os"
    "sort"
    "time"
)

type LatencyLearner struct { // assumes only single-thread/one-goroutine-at-a-time access
    name                     string
    t1                       time.Time     // NOTE there is no (need for a) t2 field
    latency_recent           time.Duration // int64
    cumul_latency            time.Duration // int64
    weight_of_cumul_latency  int
    pair_underway            bool
    pair_ever_completed      bool
}

// tracked_spans built/modified ONLY by latlearn_init. stable sort order wise for report
var   tracked_spans           []string
const latlearn_report_fpath = "logs/latency.txt"
var   latency_learners        map[string]*LatencyLearner
var   init_time               time.Time
var init_completed          bool = false// we rely on this defaulting to false

func latlearn_init( spans_app []string) {
    pre :=      "latlearn_init"
    log.Printf( "%s\n", pre)

    latency_learners = make( map[string]*LatencyLearner)

    // latlearn's built-in benchmark spans
    //     for purposes of comparison with the enduser's reported span metrics
    spans_latlearn_builtin := []string {
        "LL.no-op",            "LL.sort-10-strs", "LL.log-10-hellos",
        "LL.benchmarks-total", "LL.lat-report"}

    spans       := []string {}

    for _, span := range   spans_app {
        spans    = append( spans, span)
    }

    for _, span := range   spans_latlearn_builtin {
        spans    = append( spans, span)
    }

    log.Printf( "%s: spans: %#v\n", pre, spans)

    for _, span := range spans {
        ll                     := new( LatencyLearner)
        ll.name                 = span
        latency_learners[ span] = ll
    }

    tracked_spans  = spans
    init_time      = time.Now()
    init_completed = true

    log.Printf( "%s: END\n", pre)
}

func (ll *LatencyLearner) before() {
    //log.Printf( "LatencyLearner.before: name %s\n", ll.name)

    // Capture Time Before
    ll.t1            = time.Now() // type is time.Time
    ll.pair_underway = true
}

func (ll *LatencyLearner) B() {
    // since call wrapped with another call, overhead latency impact a tiny bit higher. but gives a smaller code-on-screen footprint at point-of-instrumentation, for dev to parse
    // tradeoffs, haha
    ll.before()
}

func llB( name string) *LatencyLearner {
    //log.Printf( "llB: name %s\n", name)

    if !init_completed { return nil}

    ll := latency_learners[ name]
    ll.before()
    return ll
}

// TODO opt string arg to indicate was alternate/non-normal path to reach the after point
func (ll *LatencyLearner) after() {
    //log.Printf("LatencyLearner.after: name %s\n", ll.name)

    // Capture Time After
    t2  := time.Now()     // time.Time
    dur := t2.Sub( ll.t1) // time.Duration. int64. of ns. legit/precise?

    //log.Printf( "%s before: %#v ms\n",         ll.name, ll.t1) // lg num printed is ms beyond the sec
    //log.Printf( "%s after : %#v ms\n",         ll.name, t2)
    //log.Printf( "%s dur   : %#v ns precise\n", ll.name, dur) // nanos. (1/1000 of a milli)

    ll.latency_recent           = dur
    ll.cumul_latency           += dur
    ll.weight_of_cumul_latency ++

    ll.pair_underway            = false
    ll.pair_ever_completed      = true
    // NOTE that both A() and llA() include a self_sample call. but after() does not
}

func (ll *LatencyLearner) A() {
    ll.after()
    latency_measure_self_sample()
    // NOTE that both A() and llA() include a self_sample call. but after() does not
}

func llA( name string) {
    if !init_completed { return}

    latency_learners[ name].after()
    latency_measure_self_sample()
    // NOTE that both A() and llA() include a self_sample call. but after() does not
}

func latency_measure_self_sample() {
    if !init_completed { return}

    // Below is to help measure (1) the cost of measuring itself, and
    // and (2) to do so around the time of a real measure occurring
    ll_noop := latency_learners[ "LL.no-op"]

    ll_noop.before()
    ll_noop.after() // NOTE: we dont call A() so dont unbounded-recurse back into this fn
}

func latency_measure_of_various_benchmark_tasks() {
    if !init_completed { return}

    pre   := "latlearn.latency_measure_of_various_benchmark_tasks"

    ll_bt := llB( "LL.benchmarks-total")

    strs  := []string { "Zelda", "Hoth", "Abro",  "Daneel", "Tempest", "Cthulhu", "Bonk", "Arky","Ys", "Jude Law"}

    ll    := llB( "LL.sort-10-strs")
    sort.Strings( strs)
    ll.A()

    ll     = llB( "LL.log-10-hellos")
    for i := 0; i < 10; i++ {
        log.Printf( "%s: log measure test\n", pre)
    }
    ll.A()

    ll_bt.A()
}

func (ll *LatencyLearner) values() ( string, time.Duration, time.Duration, int, bool, bool) {
    return ll.name, ll.latency_recent, ll.cumul_latency, ll.weight_of_cumul_latency, ll.pair_underway, ll.pair_ever_completed
}

func (ll *LatencyLearner) avg_latency() ( avg_latency int64, weight int) {
    avg_latency      = int64( -1)
    weight           = ll.weight_of_cumul_latency
    if weight        > 0 {
        cumul       := ll.cumul_latency.Nanoseconds()
        avg_latency  = cumul / int64( weight)
    }
    return avg_latency, weight
}

func to_file(              f *os.File, txt string) {
    _, _ = io.WriteString( f,          txt + "\n") // TODO error handling
}

func (ll *LatencyLearner) report( f *os.File) {
    lat_rec := "?????????"
    txt2    := "????????? ns AVG   w0     (timefrac ????????"

    if ll.pair_ever_completed {
        lat_rec         = fmt.Sprintf( "%9d", ll.latency_recent)
        weight         := ll.weight_of_cumul_latency
        if weight       > 0 {
            cum_ns     := ll.cumul_latency.Nanoseconds() // int64. ns
            avg_lat    := cum_ns / int64( weight)
            t2         := time.Now()         // time.Time
            since_init := t2.Sub( init_time) // time.Duration. int64. ns. legit/precise?
            my_frac    := float64( cum_ns) / float64( since_init) // float64. fraction

            txt2        = fmt.Sprintf( "%9d ns AVG   w%-5d (timefrac %8f)",
                                       avg_lat, weight, my_frac)
        }
    }

    txt1               := fmt.Sprintf( "%9s ns LAST %s",
                                       lat_rec, ll.name)

    to_file( f, txt1)
    to_file( f, txt2)
}

func latency_report_gen( params []string) {
    if !init_completed { return}

    ll    := llB( "LL.lat-report")

    var f *os.File
    lat_stats_file, err := os.Create( latlearn_report_fpath)
    if err  != nil {
        msg := fmt.Sprintf( "latlearn.latency_report_gen could not create file for report: path '%s', err %#v", latlearn_report_fpath, err)
        log.Printf( "%s\n", msg)
        panic( msg)
    }
    defer lat_stats_file.Close()
    f =   lat_stats_file

    // Context Params (which may impact interpretation of the reported span metrics)
    for i, param     := range params {
        txt          := ""
        if        i  == 0 {
            txt       = param
        } else if is_int_equal_to_any_of( i, []int {9, 18}) { // TODO do this right
              txt     = "\n" + param
        } else if i > 0 {
              txt     = ", " + param
        }
        _, _        = io.WriteString( f, txt)
    }
    _, _            = io.WriteString( f, "\n")

    // write (to the file) a report entry for each span:
    for _, span := range tracked_spans {
        latency_learners[ span].report( f)
    }

    ll.A()
}

