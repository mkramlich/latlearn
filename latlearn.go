// latlearn.go aka LatencyLearner
//     by Mike Kramlich
//
//     started  2023 September
//     last rev 2023 October 9
//
//     contact: groglogic@gmail.com
//     project: https://github.com/mkramlich/latlearn

package main

import (
    "fmt"
    "io"
    "log"
    "os"
    "os/exec"
    "runtime"
    "runtime/debug"
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

type VariantLatencyLearner struct {
    *LatencyLearner
    parent                   *LatencyLearner
}

type LatencyLearnerI interface {
    getLL()                  *LatencyLearner
    getVLL()                 *VariantLatencyLearner

    before()
    after()

    B()
    A()

    report( *os.File, time.Duration, time.Duration)
}

func ( ll *LatencyLearner)        getLL()  *LatencyLearner        { return  ll}
func ( ll *LatencyLearner)        getVLL() *VariantLatencyLearner { return nil}
func (vll *VariantLatencyLearner) getLL()  *LatencyLearner        { return vll.LatencyLearner}
func (vll *VariantLatencyLearner) getVLL() *VariantLatencyLearner { return vll}

// tracked_spans built/modified ONLY by the latlearn_init and llB fns
// it keeps a stable order of keys, for a better UX of the report
var tracked_spans                     []string
var latlearn_report_fpath             string = "latlearn-report.txt"
var latency_learners                  map[string]LatencyLearnerI
var init_time                         time.Time
var init_completed                    bool = false // explicit. we expect this starts false
var latlearn_should_report_builtins   bool = true
var latlearn_should_subtract_overhead bool = false


// for latlearn's internal use only
func latency_learner( span string) (ll *LatencyLearner, found bool) {
    lli, found                 := latency_learners[ span]
    if  !found {
        ll                      = new( LatencyLearner)
        ll.name                 = span
        latency_learners[ span] = ll
    } else {
        ll                      = lli.getLL()
    }
    return ll, found
}

// for latlearn's internal use only
func variant_latency_learner( span string) (vll *VariantLatencyLearner, found bool) {
    lli, found                 := latency_learners[ span]
    if  !found {
        ll                     := new( LatencyLearner)
        ll.name                 = span
        vll                     = new( VariantLatencyLearner)
        vll.LatencyLearner      =  ll
        latency_learners[ span] = vll
    } else {
        vll                     = lli.getVLL()
    }
    return vll, found
}

func latlearn_init2( spans_app []string) { // span list should be for LLs (parent spans) not VLLs
    pre :=      "latlearn_init2"
    log.Printf( "%s\n", pre)

    latency_learners = make( map[string]LatencyLearnerI)

    // latlearn's built-in benchmark spans
    //     for purposes of comparison with the enduser's reported span metrics
    spans_latlearn_builtin := []string {
        "LL.no-op",              "LL.span-map-lookup", "LL.add-2-int-literals",
        "LL.add-2-str-literals", "LL.sort-10-strs",    "LL.log-10-hellos",
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

func (vll *VariantLatencyLearner) before() {
    //log.Printf( "LatencyLearner.before: name %s\n", ll.name)

    vll.parent.t1            = time.Now() // we CAN assume safely that vll.parent != nil
    vll.parent.pair_underway = true

    vll.t1                   = vll.parent.t1
    vll.pair_underway        = true

}

func (ll *LatencyLearner) B() {
    // trade-off: since call wrapped with another call, overhead latency impact a tiny bit higher. but gives a smaller code-on-screen footprint at point-of-instrumentation, for dev to parse
    ll.before()
}

func (vll *VariantLatencyLearner) B() {
    // trade-off: since call wrapped with another call, overhead latency impact a tiny bit higher. but gives a smaller code-on-screen footprint at point-of-instrumentation, for dev to parse
    vll.before()
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

func llB2( name string, variant string) *VariantLatencyLearner {
    //log.Printf( "llB: name %s\n", name)

    if !init_completed { // allows lazy init of latlearn, upon first call to llB2
        latlearn_init()
    }

    ll, found    := latency_learner( name) // example name: "somefn"
    if !found   { tracked_spans = append( tracked_spans, name)}

    variant_name := fmt.Sprintf( "%s(%s)", name, variant) // example variant_name: "somefn(N=200)"
    vll, found2  := variant_latency_learner( variant_name)
    if  !found2 { tracked_spans = append( tracked_spans, variant_name)}
    vll.parent    = ll // to note this span is a variant of a parent span, and part of a span family
    vll.before() // the called before fn here will ALSO in effect call ll.before()

    return vll
}

func (ll *LatencyLearner) after2( dur time.Duration) { // dur is int64. of ns. legit & precise?
    //log.Printf("LatencyLearner.after: name %s\n", ll.name)

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
}

func (ll *LatencyLearner) after() {
    t2  := time.Now()
    dur := t2.Sub( ll.t1) // time.Duration. int64. of ns. legit & precise?

    ll.after2( dur)
}

func (vll *VariantLatencyLearner) after() {
    t2  := time.Now()
    dur := t2.Sub( vll.t1) // time.Duration. int64. of ns. legit & precise?

    vll.parent.after2(         dur)
    vll.LatencyLearner.after2( dur)
}

func (ll *LatencyLearner) A() {
    ll.after()
}

func (vll *VariantLatencyLearner) A() {
    vll.after()
}

func llA( name string) {
    if !init_completed { return}

    latency_learners[ name].after() // TODO map found guard
}

func latency_measure_self_sample() {
    if !init_completed { return} // TODO auto-init, or, return error

    // below is to (try to) measure/estimate the latency cost of a latlearn measurement
    // but lots of subtle issues and complexity lurk here
    ll    := latency_learners[ "LL.no-op"]
    for i := 0; i < 1000000; i++ { // we'll do it 1 million times, hoping to mitigate (somewhat, maybe) the effects of host load spikes, GC runs, etc
        ll.before()
        ll.after()
    }
}

func latlearn_measure_overhead_estimate() (overhead time.Duration, exists bool) {
    if !init_completed              { return -1, false}

    lli, found := latency_learners[ "LL.no-op"]

    if  !found                      { return -1, false}

    ll_noop    := lli.getLL()

    if !ll_noop.pair_ever_completed { return -1, false}

    return ll_noop.min, true
}

func latlearn_benchmarks() {
    if !init_completed { return} // TODO auto-init, or, return error

    pre      :=      "latlearn_benchmarks"
    ll_bt    := llB( "LL.benchmarks-total")

    latency_measure_self_sample()

    for i    := 0; i < 1000; i++ {
        ll   := llB( "LL.add-2-int-literals")
        a    := (1 + 2)
        _     = a // make closer to real, and compiler happy
        ll.A()
    }

    for i    := 0; i < 1000; i++ {
        ll   := llB( "LL.add-2-str-literals")
        c    := ("a" + "b")
        _     = c // make closer to real, and compiler happy
        ll.A()
    }

    for i    := 0; i < 1000; i++ {
        ll   := llB( "LL.span-map-lookup")
        a, b := latency_learners[ "LL.no-op"]
        ll.A()
        _     = a // yes, is reason why we are doing this
        _     = b // ditto
    }

    strs         := []string { "Zelda", "Hoth", "Abro",  "Daneel", "Tempest", "Cthulhu", "Bonk", "Arky","Ys", "Jude Law"}
    for i        := 0; i < 1000; i++ {
        strs2    := []string {}
        for _, s := range strs {
            strs2 = append( strs2, s)
        }
        ll   := llB( "LL.sort-10-strs")
        sort.Strings( strs2) // sorts the given slice in-place
        ll.A()
    }

    ll       := llB( "LL.log-10-hellos")
    for i    := 0; i < 10; i++ {
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

func overhead_comp( metric_in int64, overhead int64) (metric_out int64) { // "comp" for compensate
    if latlearn_should_subtract_overhead {
        metric_out = (metric_in - overhead)
    } else {
        metric_out = metric_in
    }
    return metric_out
}

func (ll *LatencyLearner) report( f *os.File, since_init time.Duration, overhead time.Duration) { // time.Duration is int64 ns
    line := ""

    if ll.pair_ever_completed {
        min              := int64(ll.min)
        if (overhead != -1) && (ll.name != "LL.no-op") {
            min           = overhead_comp( int64(ll.min),          int64(overhead))
        }
        min_txt          := fmt.Sprintf( "%15s", number_grouped( int64( min), ","))

        last             := overhead_comp( int64(ll.latency_last), int64(overhead))
        last_txt         := fmt.Sprintf( "%15s", number_grouped( int64( last), ","))

        max              := overhead_comp( int64(ll.max),          int64(overhead))
        max_txt          := fmt.Sprintf( "%15s", number_grouped( int64( max), ","))

        mean_txt         := "???,???,???,???"
        weight_txt       :=     "???,???,???"
        tf_txt           :=        "????????"
        weight           := ll.weight_of_cumul_latency

        if weight         > 0 {
            cum_ns       := ll.cumul_latency.Nanoseconds() // int64. ns

            lat_mean     := cum_ns / int64( weight)

            mean         := overhead_comp(         lat_mean, int64(overhead))
            mean_txt      = number_grouped( int64( mean),   ",")

            weight_txt    = number_grouped( int64( weight), ",")
            my_frac      := float64( cum_ns) / float64( since_init) // float64. fraction
            tf_txt        = fmt.Sprintf( "%8f", my_frac)
        }
        line = fmt.Sprintf(
                   "%-22s: %15s | %15s | %15s | %15s | w %11s | tf %8s | %-21s",
                   ll.name, min_txt, last_txt, max_txt, mean_txt, weight_txt, tf_txt, ll.name)
    } else {
        // min, last, max, mean, weight of mean (# of calls for this span), time fraction (of current time difference since latlearn_init, in/under this span)
        line = fmt.Sprintf(
                   "%-22s: ???,???,???,??? | ???,???,???,??? | ???,???,???,??? | ???,???,???,??? | w ???,???,??? | tf ???????? | %-21s",
                   ll.name, ll.name)
    }

    to_file( f, line)
}

func mac_sysctl( key string) (value string) {
    cmd := exec.Command( "/usr/sbin/sysctl", key)

    var out []byte
    var err error

    if  out, err = cmd.CombinedOutput(); (err != nil) {
        //log.Printf( "latearn report's mac sysctl exec failed. output: %s\n", string( out))
        //log.Fatal( err)
        return ""
    }
    out_str := string( out)
    value_to_line_end := strings.TrimPrefix( out_str, key + ": ")
    return strings.TrimSpace( value_to_line_end)
}

func mac_sysctl_report_line( key string, f *os.File) {
    value := mac_sysctl( key)
    line  := fmt.Sprintf( "%-27s: %s\n", key, value)
    io.WriteString( f, line)
}

func write_info_about_mac_host_to_report( f *os.File) {
    mac_sysctl_report_line( "kern.ostype",                 f)
    mac_sysctl_report_line( "kern.osproductversion",       f)
    mac_sysctl_report_line( "kern.osrelease",              f)
    mac_sysctl_report_line( "kern.osrevision",             f)
    mac_sysctl_report_line( "kern.version",                f)
    mac_sysctl_report_line( "user.posix2_version",         f)
    mac_sysctl_report_line( "machdep.cpu.brand_string",    f)
    mac_sysctl_report_line( "machdep.cpu.core_count",      f)
    mac_sysctl_report_line( "machdep.cpu.thread_count",    f)
    mac_sysctl_report_line( "machdep.memmap.Conventional", f)
    mac_sysctl_report_line( "hw.memsize",                  f)
    mac_sysctl_report_line( "hw.pagesize",                 f)
    mac_sysctl_report_line( "hw.cpufrequency",             f)
    mac_sysctl_report_line( "hw.busfrequency",             f)
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

    io.WriteString( f, "Latency Report (https://github.com/mkramlich/latlearn)\n\n")
    io.WriteString( f, fmt.Sprintf("latlearn_should_subtract_overhead: %v\n", latlearn_should_subtract_overhead))
    if latlearn_should_subtract_overhead {
        io.WriteString( f, "metric treated as overhead: LL.no-op, min\n")
    }

    t2         := time.Now()         // time.Time
    since_init := t2.Sub( init_time) // time.Duration. int64. ns. legit/precise?
    si_txt     := number_grouped( int64( since_init), ",")
    time_param := fmt.Sprintf( "since LL init: %s ns\n\n", si_txt)
    io.WriteString( f, time_param)

    io.WriteString( f, fmt.Sprintf( "Go ver:         %s\n", runtime.Version()))
    io.WriteString( f, fmt.Sprintf( "GOARCH:         %s\n", runtime.GOARCH))
    io.WriteString( f, fmt.Sprintf( "GOOS:           %s\n", runtime.GOOS))
    io.WriteString( f, fmt.Sprintf( "NumCPU:         %d\n", runtime.NumCPU()))
    io.WriteString( f, fmt.Sprintf( "GOMAXPROCS:     %d\n", runtime.GOMAXPROCS( -1)))
    io.WriteString( f, fmt.Sprintf( "NumGoroutine:   %d\n", runtime.NumGoroutine()))
    //io.WriteString( f, fmt.Sprintf( "NumCgoCall:   %d\n", runtime.NumCgoCall()))

    mem_limit     := debug.SetMemoryLimit( -1)
    mem_limit_str := number_grouped( mem_limit, ",")
    io.WriteString( f, fmt.Sprintf( "SetMemoryLimit: %s bytes\n", mem_limit_str))

    gogc := ""
    if val, ok := os.LookupEnv(     "GOGC"); ok {           gogc = val}
    io.WriteString( f, fmt.Sprintf( "GOGC:           %s\n", gogc))

    if (runtime.GOOS == "darwin") {
        write_info_about_mac_host_to_report( f)
    }

    term_rows  := "?"
    term_cols  := "?"
    if val, ok := os.LookupEnv(     "LINES");   ok {        term_rows = val}
    if val, ok := os.LookupEnv(     "COLUMNS"); ok {        term_cols = val}
    io.WriteString( f, fmt.Sprintf( "LINES:          %s\n", term_rows))
    io.WriteString( f, fmt.Sprintf( "COLUMNS:        %s\n", term_cols))

    host       := ""
    if val, ok := os.LookupEnv(     "HOST"); ok {           host = val}
    io.WriteString( f, fmt.Sprintf( "HOST:           %s\n", host))

    term       := ""
    if val, ok := os.LookupEnv(     "TERM"); ok {           term = val}
    io.WriteString( f, fmt.Sprintf( "TERM:           %s\n", term))

    io.WriteString( f, "\n")

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
                   "%-22s: %15s | %15s | %15s | %15s | %13s | %11s | %-21s",
                   "span", "min (ns)", "last (ns)", "max (ns)", "mean (ns)", "weight (B&As)", "time frac", "span")
    to_file( f, header)

    var overhead time.Duration = -1 // this value signals that we have no usable estimate
    if latlearn_should_subtract_overhead {
        overhead, _ = latlearn_measure_overhead_estimate()
    }

    for _, span := range tracked_spans {
        if !latlearn_should_report_builtins && strings.HasPrefix( span,"LL.") { continue}
        latency_learners[ span].report( f, since_init, overhead) // TODO add found-in-map guard
    }

    ll.A()
}


func latlearn_report() {
     latlearn_report2( []string {})
}

