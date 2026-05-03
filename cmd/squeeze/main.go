package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"time"
)

const (
	Alphabet   = 256
	BufSize    = 300000
	DictSize   = BufSize
	Mod        = 354621
	OutBufSize = 2000
)

var (
	flagVerbose = flag.Bool("v", false, "announce compression results (``verbose'')")

	flagVersion = flag.Bool("V", false, "print version number")
	flagHelp    = flag.Bool("H", false, "print this notice")
	flagUsage   = flag.Bool("U", false, "print short usage summary")

	// Not yet implemented.
	// flagRandom   = flag.Bool("r", false, "``randomize'' output; useful before encryption")
	// flagNoRandom = flag.Bool("R", true, "do not randomize output (default)")
	// flagEOF      = flag.Bool("e", false, "explicitly indicate EOF")
	// flagNoEOF    = flag.Bool("E", true, "do not explicitly indicate EOF (default)")
	// flagBlocking = flag.Bool("b", true, "readapt to next block when out of memory (default)")
	// flagNoBlock  = flag.Bool("B", false, "freeze working adaptation when out of memory")
)

const version = "squeeze version 1.711, October 28, 1989.\nCopyright (c) 1989, Daniel J. Bernstein.\nAll rights reserved.\n"
const help = `squeeze compresses its input and prints the result, using adaptive Miller-
Wegman encoding, a variation on Ziv-Lempel encoding. It usually produces
files 10-40% shorter than compress does. To decompress, use unsqueeze.

squeeze -A: print authorship notice
squeeze -C: print copyright notice
squeeze -H: print this notice
squeeze -U: print short usage summary
squeeze -V: print version number
squeeze -W: print disclaimer of warranty

squeeze [ -eErRbBv ]: compress
  -e: explicitly indicate EOF
  -E: do not explicitly indicate EOF (default)
  -r: "randomize" output; useful before encryption
  -R: do not randomize output (default)
  -b: readapt to next block when out of memory (default)
  -B: freeze working adaptation when out of memory
  -v: announce compression results ("verbose")

If you have questions about or suggestions for squeeze, please feel free
to contact the author, Daniel J. Bernstein, at brnstnd@acf10.nyu.edu
on the Internet.
`
const usage = "Usage: squeeze [ -eErRbBvACHUVW ]\nHelp:  squeeze -H\n"

var (
	numberIn  int
	numberOut int

	table  [Mod]int
	parent [DictSize]int
	num    [DictSize]int
	next   [DictSize]int

	y [55]int
	j int
	k int

	buf      [BufSize]byte
	bufStart int
	bufEnd   int

	outN      [OutBufSize]int
	outB      [OutBufSize]int
	outBufPtr int
	outBitPos int
	outWord   int
	stdin     *bufio.Reader
	stdout    *bufio.Writer

	optRandom   bool
	optEOF      bool
	optBlocking bool
)

func main() {
	flag.Parse()

	if *flagVersion {
		fmt.Print(version)
		return
	}
	if *flagHelp {
		fmt.Print(help)
		return
	}
	if *flagUsage {
		fmt.Print(usage)
		return
	}

	parseFlags()

	if flag.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "squeeze: I am a filter, try sqz for squeezing files")
		os.Exit(1)
	}

	stdin = bufio.NewReader(os.Stdin)
	stdout = bufio.NewWriter(os.Stdout)
	defer stdout.Flush()

	if optRandom {
		initRandom()
	}

startOver:
	initDictionary()
	maxNum := Alphabet - 1
	maxNode := Alphabet

	ch, err := inChar()
	if err != nil {
		if optEOF {
			outNum(maxNum+1, maxNum)
		}
		outFinish()
		if *flagVerbose {
			goAheadAndBeVerbose()
		}
		return
	}
	outNum(ch, maxNum)
	numberIn++
	oldMatchHash := moreHash(0, ch)
	oldMatch := table[oldMatchHash]

	for {
		match := 0
		matchLen := 0
		curHash := 0
		dictMatch := 0
		dictMatchLen := 0
		dictMatchHash := 0

		for {
			ch, err = inChar()
			if err != nil {
				if matchLen == 0 {
					if optEOF {
						outNum(maxNum+1, maxNum)
					}
					outFinish()
					if *flagVerbose {
						goAheadAndBeVerbose()
					}
					return
				}
				break
			}
			curHash = moreHash(curHash, ch)
			i := table[curHash]
			found := false
			for i != 0 {
				if parent[i] == match {
					match = i
					matchLen++
					if num[i] != -1 {
						dictMatch = match
						dictMatchLen = matchLen
						dictMatchHash = curHash
					}
					found = true
					break
				}
				i = next[i]
			}
			if !found {
				break
			}
		}

		numberIn += dictMatchLen
		outNum(num[dictMatch], maxNum)
		inRepeat(boolToInt(err == nil) + matchLen)

		match = oldMatch
		curHash = oldMatchHash
		for dictMatchLen > 0 {
			ch, _ = inChar() // can't be EOF
			dictMatchLen--
			curHash = moreHash(curHash, ch)
			i := table[curHash]
			found := false
			for i != 0 {
				if parent[i] == match {
					match = i
					found = true
					break
				}
				i = next[i]
			}
			if !found {
				if maxNode+dictMatchLen+1 < DictSize {
					maxNode++
					parent[maxNode] = match
					next[maxNode] = table[curHash]
					num[maxNode] = -1
					match = maxNode
					table[curHash] = maxNode
					for dictMatchLen > 0 {
						ch, _ = inChar()
						dictMatchLen--
						curHash = moreHash(curHash, ch)
						maxNode++
						parent[maxNode] = match
						next[maxNode] = table[curHash]
						num[maxNode] = -1
						match = maxNode
						table[curHash] = maxNode
					}
				} else {
					maxNode = DictSize
					unInRepeat(dictMatchLen)
					dictMatchLen = 0
					if optBlocking {
						outNum(maxNum+boolToInt(optEOF)+2, maxNum+1)
						clearTable()
						goto startOver
					}
				}
				break
			}
		}
		maxNum++
		if maxNode < DictSize {
			if num[match] == -1 {
				num[match] = maxNum
			} else if optRandom {
				if randomBit() == 1 {
					num[match] = maxNum
				}
			}
		}
		oldMatch = dictMatch
		oldMatchHash = dictMatchHash
	}
}

func parseFlags() {
	optRandom = false
	optEOF = false
	optBlocking = true

	for _, arg := range os.Args[1:] {
		if len(arg) > 0 && arg[0] == '-' {
			for i := 1; i < len(arg); i++ {
				switch arg[i] {
				case 'r':
					optRandom = true
				case 'R':
					optRandom = false
				case 'e':
					optEOF = true
				case 'E':
					optEOF = false
				case 'b':
					optBlocking = true
				case 'B':
					optBlocking = false
				case 'v':
					*flagVerbose = true
				}
			}
		}
	}
}

func moreHash(curHash, ch int) int {
	return (curHash*256 + ch + 1) % Mod
}

func clearTable() {
	for h := 0; h < Mod; h++ {
		table[h] = 0
	}
}

func initDictionary() {
	clearTable()
	for ch := 0; ch < Alphabet; ch++ {
		parent[ch+1] = 0
		num[ch+1] = ch
		next[ch+1] = 0
		table[moreHash(0, ch)] = ch + 1
	}
}

func initRandom() {
	now := time.Now()
	usec := now.Nanosecond() / 1000
	sec := int(now.Unix())

	for j = 0; j < 20; j++ {
		y[j] = ((usec >> uint(j)) + (sec >> uint(j))) % 2
	}
	for j = 0; j < 20; j++ {
		y[j+20] = (usec >> uint(j)) % 2
	}
	y[54] = 1
	j = 24
	k = 0
}

func randomBit() int {
	j = (j + 54) % 55
	k = (k + 54) % 55
	y[k] = y[k] ^ y[j]
	return y[k]
}

func inChar() (int, error) {
	if bufStart != bufEnd {
		result := int(buf[bufStart])
		bufStart = (bufStart + 1) % BufSize
		return result, nil
	}
	b, err := stdin.ReadByte()
	if err != nil {
		return 0, err
	}
	buf[bufStart] = b
	result := int(b)
	bufStart = (bufStart + 1) % BufSize
	bufEnd = bufStart
	return result, nil
}

func inRepeat(n int) {
	bufStart = (bufStart + BufSize - n) % BufSize
}

func unInRepeat(n int) {
	bufStart = (bufStart + n) % BufSize
}

func outNum(ch, max int) {
	m := (max + boolToInt(optEOF) + boolToInt(optBlocking)) >> 8
	bits := 8
	for m > 0 {
		bits++
		m >>= 1
	}

	if optRandom {
		codeSpace := max + boolToInt(optEOF) + boolToInt(optBlocking) + 1
		if ch < (1<<uint(bits))-codeSpace {
			if randomBit() == 1 {
				ch += codeSpace
			}
		}
	}

	outN[outBufPtr] = ch
	outB[outBufPtr] = bits
	outBufPtr++

	if outBufPtr == OutBufSize {
		for i := 0; i < OutBufSize; i++ {
			outWord += outN[i] << uint(outBitPos)
			outBitPos += outB[i]
			for outBitPos > 7 {
				stdout.WriteByte(byte(outWord & 255))
				numberOut++
				outWord >>= 8
				outBitPos -= 8
			}
		}
		outBufPtr = 0
	}
}

func outFinish() {
	for i := 0; i < outBufPtr; i++ {
		outWord += outN[i] << uint(outBitPos)
		outBitPos += outB[i]
		for outBitPos > 7 {
			stdout.WriteByte(byte(outWord & 255))
			numberOut++
			outWord >>= 8
			outBitPos -= 8
		}
	}
	if optRandom {
		for outBitPos < 7 {
			outWord += randomBit() << uint(outBitPos)
			outBitPos++
		}
	}
	stdout.WriteByte(byte(outWord))
	numberOut++
}

func goAheadAndBeVerbose() {
	ratio := 0
	if numberIn > 0 {
		ratio = (100*numberOut + numberIn/2) / numberIn
	}
	fmt.Fprintf(os.Stderr, "In: %d chars  Out: %d chars  Squeezed to: %d%%\n",
		numberIn, numberOut, ratio)
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
