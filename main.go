package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"slices"
	"time"
)

type temp struct {
	min   float64
	max   float64
	sum   float64
	count uint
}

func fastparse(b []byte) float64 {
	if len(b) == 0 {
		return 0
	}
	base := int8(0)
	neg := b[0] == '-'
	i := 0
	if neg {
		i = 1
	}
	k := bytes.IndexRune(b, '.')
	for ; i < k; i++ {
		base = base*10 + int8(b[i]-'0')
	}
	total := float64(base)
	if b[k+1]-'0' != 0 {
		total += float64(10) / float64(b[k+1]-'0')
	}
	if neg {
		return -1 * total
	}
	return total
}

func main() {
	start := time.Now()
	defer func() { fmt.Println("Took ", time.Since(start)) }()

	f, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	counts := map[string]temp{}

	scanner := bufio.NewScanner(f)
	var name string
	var tempS []byte
	for scanner.Scan() {
		if scanner.Err() != nil {
			log.Fatal(scanner.Err())
		}
		line := scanner.Bytes()
		i := bytes.IndexByte(line, ';')
		name = string(line[:i])
		tempS = line[i+1:]

		val := fastparse(tempS)
		cn, ok := counts[name]
		if !ok {
			counts[name] = temp{
				min:   val,
				max:   val,
				sum:   val,
				count: 1,
			}
		} else {
			cn.sum += val
			cn.count++
			if val > float64(cn.max) {
				cn.max = val
			}
			if val < float64(cn.min) {
				cn.min = val
			}
			counts[name] = cn
		}
	}

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
