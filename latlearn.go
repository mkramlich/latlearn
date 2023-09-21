// latlearn.go aka LatencyLearner
//     by Mike Kramlich
//
//     started  2023 September
//     last rev 2023 September 21
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
    "strings"
    "time"
)

type LatencyLearner struct { // assumes only single-thread/one-goroutine-at-a-time access
    name                     string
    t1                       time.Time     // NOTE there is no (need for a) t2 field
    latency_last             time.Duration // int64
    cumul_latency            time.Duration // int64
    weight_of_cumul_latency  int
    min                      time.Duration // int64
    max                      time.Duration // int64
    pair_underway            bool
    pair_ever_completed      bool
}

// tracked_spans built/modified ONLY by the latlearn_init and llB fns
// it keeps a stable order of keys, for a better UX of the report
var tracked_spans                   []string
var latlearn_report_fpath           string = "latlearn-report.txt"
var latency_learners                map[string]*LatencyLearner
var init_time                       time.Time
var init_completed                  bool = false // explicit. we expect this starts false
var latlearn_should_report_builtins bool = true


// for latlearn's internal use only
func latency_learner( span string) (ll *LatencyLearner, found bool) {
    ll, found = latency_learners[ span]
    if !found {
        ll                      = new( LatencyLearner)
        ll.name                 = span
        latency_learners[ span] = ll
    }
    return ll, found
}

func latlearn_init2( spans_app []string) {
    pre :=      "latlearn_init2"
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
        latency_learner( span)
    }

    tracked_spans  = spans
    init_time      = time.Now()
    init_completed = true

    //log.Printf( "%s: END\n", pre)
}

func latlearn_init() {
     latlearn_init2( []string {})
}

func (ll *LatencyLearner) before() {
    //log.Printf( "LatencyLearner.before: name %s\n", ll.name)

    // Capture Time Before
    ll.t1            = time.Now() // type is time.Time
    ll.pair_underway = true
}

func (ll *LatencyLearner) B() {
    // trade-off: since call wrapped with another call, overhead latency impact a tiny bit higher. but gives a smaller code-on-screen footprint at point-of-instrumentation, for dev to parse
    ll.before()
}

func llB( name string) *LatencyLearner {
    //log.Printf( "llB: name %s\n", name)

    if !init_completed { // allows lazy init of latlearn, upon first call to llB
        latlearn_init()
    }

    ll, found := latency_learner( name)
    if !found { // if was an ad hoc span? meaning this span was NOT already tracked
        tracked_spans = append( tracked_spans, name) // we assume here that tracked_spans will stay in sych with the set of keys in latency_learners. except tracked_spans adds extra notion of preserving a stable order to the keys (relied on in the report)
    }
    ll.before()
    return ll
}

func (ll *LatencyLearner) after() {
    //log.Printf("LatencyLearner.after: name %s\n", ll.name)

    // Capture Time After
    t2  := time.Now()     // time.Time
    dur := t2.Sub( ll.t1) // time.Duration. int64. of ns. legit/precise?

    //log.Printf( "%s before: %#v ms\n",         ll.name, ll.t1) // lg num printed is ms beyond the sec
    //log.Printf( "%s after : %#v ms\n",         ll.name, t2)
    //log.Printf( "%s dur   : %#v ns precise\n", ll.name, dur) // nanos. (1/1000 of a milli)

    ll.latency_last             = dur
    ll.cumul_latency           += dur
    ll.weight_of_cumul_latency ++

    if ll.pair_ever_completed {
        if ( dur < ll.min) {ll.min = dur}
        if ( dur > ll.max) {ll.max = dur}
    } else {
        ll.min = dur
        ll.max = dur
    }

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

func latlearn_benchmarks() {
    if !init_completed { return}

    pre   :=      "latlearn_benchmarks"
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

func (ll *LatencyLearner) values() ( string, time.Duration, time.Duration, int, time.Duration, time.Duration, bool, bool) {
    return ll.name, ll.latency_last, ll.cumul_latency, ll.weight_of_cumul_latency, ll.min, ll.max, ll.pair_underway, ll.pair_ever_completed
}

func (ll *LatencyLearner) mean_latency() ( mean_latency int64, weight int) {
    mean_latency      = int64( -1)
    weight            = ll.weight_of_cumul_latency
    if weight         > 0 {
        cumul        := ll.cumul_latency.Nanoseconds()
        mean_latency  = cumul / int64( weight)
    }
    return mean_latency, weight
}

// for latlearn's internal use only
func to_file(              f *os.File, txt string) {
    _, _ = io.WriteString( f,          txt + "\n") // TODO error handling
}

// for latlearn's internal use only
func reverse_string_array( in []string) (out []string) {
    // NOTE: compiler refuses to let one use: sort.Reverse( in)
    // TODO  I should not need to write my own fn to do this

    out     = []string {}
    for i  := (len( in) - 1); i >= 0; i-- {
        out = append( out, in[i])
    }
    return out
}

// for latlearn's internal use only
func number_grouped( val int64, sep string) string { // sep value like "," or " "
    // for the "digit grouping" formatted print of a number
    // example of results:
    //      12 (with any sep)    to        "12"
    //    1234 (with sep of ",") to     "1,234"
    // 1222333 (with sep of " ") to "1 222 333"

    s      := fmt.Sprintf(   "%d", val)
    s_pcs  := strings.Split( s,    "")

    // reverse it
    s_pcs2 := reverse_string_array( s_pcs)

    // add comma delim, every 3 digits, to group them for easier human reading:
    s2         := ""
    for i, pc  := range s_pcs2 {
        if (i  != 0) && ((i % 3) == 0) {
            s2 += sep
        }
        s2     += pc
    }

    s_pcs3 := strings.Split(        s2,     "")
    s_pcs4 := reverse_string_array( s_pcs3)
    s3     := strings.Join(         s_pcs4, "")

    return s3
}

func (ll *LatencyLearner) report( f *os.File, since_init time.Duration /*int64. ns*/) {

    line    := ""

    if ll.pair_ever_completed {
        lat_min_txt      := fmt.Sprintf( "%15s", number_grouped( int64( ll.min), ","))
        lat_last_txt     := fmt.Sprintf( "%15s", number_grouped( int64( ll.latency_last), ","))
        lat_max_txt      := fmt.Sprintf( "%15s", number_grouped( int64( ll.max), ","))
        lat_mean_txt     := "???,???,???,???"
        weight_txt       := "???,???,???"
        tf_txt           := "????????"
        weight           := ll.weight_of_cumul_latency
        if weight         > 0 {
            cum_ns       := ll.cumul_latency.Nanoseconds() // int64. ns
            lat_mean     := cum_ns / int64( weight)
            lat_mean_txt  = number_grouped( int64( lat_mean), ",")
            weight_txt    = number_grouped( int64( weight),   ",")
            my_frac      := float64( cum_ns) / float64( since_init) // float64. fraction
            tf_txt        = fmt.Sprintf( "%8f", my_frac)
        }
        line = fmt.Sprintf(
                   "%-20s: %15s | %15s | %15s | %15s | w %11s | tf %8s",
                   ll.name, lat_min_txt, lat_last_txt, lat_max_txt, lat_mean_txt, weight_txt, tf_txt)
    } else {
        // min, last, max, mean, weight of mean (# of calls for this span), time fraction (of current time difference since latlearn_init, in/under this span)
        line = fmt.Sprintf(
                   "%-20s: ???,???,???,??? | ???,???,???,??? | ???,???,???,??? | ???,???,???,??? | w ???,???,??? | tf ????????",
                   ll.name)
    }

    to_file( f, line)
}

func latlearn_report2( params []string) {
    if !init_completed { return}
    pre := "latlearn_report2"

    ll      := llB( "LL.lat-report")

    f, err  := os.Create( latlearn_report_fpath)
    if err  != nil {
        msg := fmt.Sprintf( "latlearn/%s could not create file for report: path '%s', err %#v", pre, latlearn_report_fpath, err)
        log.Printf( "%s\n", msg)
        panic( msg)
    }
    defer f.Close()

    io.WriteString( f, "Latency Report (https://github.com/mkramlich/latlearn)\n")

    t2         := time.Now()         // time.Time
    since_init := t2.Sub( init_time) // time.Duration. int64. ns. legit/precise?
    si_txt     := number_grouped( int64( since_init), ",")
    time_param := fmt.Sprintf( "since LL init: %s ns\n", si_txt)
    io.WriteString( f, time_param)

    // Context Params (which may impact interpretation of the reported span metrics)
    for i, param     := range params {
        txt          := ""
        if        i  == 0 {
            txt       = param
        } else if ((i != 0) && ((i % 4) == 0)) { // TODO do this better
              txt     = "\n" + param
        } else if i > 0 {
              txt     = ", " + param
        }
        io.WriteString( f, txt)
    }
    if len( params) > 0 {
        io.WriteString( f, "\n")
    }
    io.WriteString(     f, "\n")

    // write a report entry (to the file) for the latency stats on each tracked span:
    header  := fmt.Sprintf(
                   "%-20s: %15s | %15s | %15s | %15s | %13s | %11s",
                   "span", "min (ns)", "last (ns)", "max (ns)", "mean (ns)", "weight (B&As)", "time frac")
    to_file( f, header)
    for _, span := range tracked_spans {
        if !latlearn_should_report_builtins && strings.HasPrefix( span,"LL.") { continue}
        latency_learners[ span].report( f, since_init) // TODO add found-in-map guard
    }

    ll.A()
}


func latlearn_report() {
     latlearn_report2( []string {})
}

