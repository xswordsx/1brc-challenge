//go:build goexperiment.arenas

package main

import (
	"arena"
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"log"
	stdlog "log"
	"os"
	"runtime"
	"runtime/pprof"
	"slices"
	"time"
)

var cpuprofile = flag.String("cpuprofile", "", "write cpu profile to file")
var debug = flag.Bool("debug", false, "print out debug logs")

var dbg = func(string, ...any) {}

const MAX_LINE_LEN = 100

type temp struct {
	min   float64
	max   float64
	sum   float64
	count uint
}

type line struct {
	num  uint64
	line []byte
}

type entry struct {
	name    string
	reading float64
}

func fastparse(b []byte) float64 {
	// f, _ := strconv.ParseFloat(string(b), 32)
	// return f
	if len(b) == 0 {
		return 0
	}
	base := int16(0)
	neg := b[0] == '-'
	i := 0
	if neg {
		i = 1
	}
	k := bytes.IndexRune(b, '.')
	if k == -1 || k == len(b) {
		panic("invalid value: " + string(b))
	}
	for ; i < k; i++ {
		base = base*10 + int16(b[i]-'0')
	}
	total := float64(base)
	if b[k+1]-'0' != 0 {
		total += float64(10) / float64(b[k+1]-'0')
	}
	if neg {
		total = -total
	}
	return total
}

func parseline(l line) entry {
	dbg("parsing line %5d: %s\n", l.num, string(l.line))
	i := bytes.IndexByte(l.line, ';')
	if i == -1 || i == len(l.line) {
		fmt.Fprintf(os.Stderr, "line %d (%q) not ok\n", l.num, l.line)
		os.Exit(1)
	}
	return entry{
		name:    string(l.line[:i]),
		reading: fastparse(l.line[i+1:]),
	}
}

func main() {
	flag.Parse()
	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
		defer dbg("Saving CPU profile to %q", *cpuprofile)
	}
	if *debug {
		dbg = stdlog.Printf
	}
	start := time.Now()
	defer func() { fmt.Println("Took ", time.Since(start)) }()

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	theArena := arena.NewArena()
	defer theArena.Free()

	nCores := runtime.GOMAXPROCS(0)
	doneCh := make(chan struct{})
	lineCh := make(chan line, nCores)
	entryCh := make(chan entry, nCores)

	dbg("Starting %d routines", nCores)

	processed := uint64(0)
	i := uint64(1)
	// start up core-count line parsers.
	// these will be stopped via a `line{}` entry.
	for i := range nCores {
		go func(i int) {
			dbg("[parsing] Routine #%d started", i)
			defer dbg("[parsing] Routine #%d stopped", i)

			buf := make([]byte, MAX_LINE_LEN)
			n := 0

			for {
				select {
				case <-doneCh:
					return
				case e := <-lineCh:
					if e.num == 0 && e.line == nil {
						return
					}
					n = copy(buf, e.line)
					entryCh <- parseline(line{e.num, buf[:n]})
				}
			}
		}(i + 1)
	}

	counts := map[string]temp{}
	// store results in a map.
	// busy loop instead of a mutex (maybe faster)?
	go func() {
		for {
			select {
			case <-doneCh:
				return
			case e := <-entryCh:
				cn, ok := counts[e.name]
				if !ok {
					counts[e.name] = temp{
						min:   e.reading,
						max:   e.reading,
						sum:   e.reading,
						count: 1,
					}
				} else {
					cn.sum += e.reading
					cn.count++
					if e.reading > float64(cn.max) {
						cn.max = e.reading
					}
					if e.reading < float64(cn.min) {
						cn.min = e.reading
					}
					counts[e.name] = cn
				}
				processed++
				dbg("proc: %d", processed)
				dbg("i: %d", i)
			default:
				continue
			}
		}
	}()

	scanner := bufio.NewScanner(f)
	// scanBuf := make([]byte, 16<<20)
	// scanner.Buffer(scanBuf, 16<<20)
	for scanner.Scan() {
		if scanner.Err() != nil {
			log.Fatalf("line %d: %v,", i, scanner.Err())
		}
		bb := arena.MakeSlice[byte](theArena, len(scanner.Bytes()), MAX_LINE_LEN)
		n := copy(bb, scanner.Bytes())
		lineCh <- line{i, bb[:n]}
		i++
	}
	// send the Close line to all listeners
	for range nCores {
		lineCh <- line{}
	}
	i--
	// busy loop wait
	for processed != i {
	}
	close(doneCh)

	sorted := make([]string, 0, len(counts))
	for name := range counts {
		sorted = append(sorted, name)
	}
	slices.Sort(sorted)

	fmt.Print("{")
	lastIdx := len(sorted) - 1
	for i, name := range sorted {
		fmt.Printf(
			"%s=%.1f/%.1f/%.1f",
			name,
			counts[name].min,
			counts[name].sum/float64(counts[name].count),
			counts[name].max,
		)
		if i != lastIdx {
			fmt.Print(", ")
		}
	}
	fmt.Println("}")
}
