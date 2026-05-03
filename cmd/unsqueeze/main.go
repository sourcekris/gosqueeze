package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
)

const (
	Alphabet  = 256
	DictSize  = 2000000
	StackSize = 50000
)

var (
	flagEOF     = flag.Bool("e", false, "look for explicit indication of EOF")
	flagDisplay = flag.String("d", "", "display string between output bunches, for debugging")
	flagVerbose = flag.Bool("v", false, "announce decompression results (``verbose'')")

	flagVersion = flag.Bool("V", false, "print version number")
	flagHelp    = flag.Bool("H", false, "print this notice")
	flagUsage   = flag.Bool("U", false, "print short usage summary")

	// Not yet implemented.
	// flagNoEOF    = flag.Bool("E", true, "don't look for explicit EOF (default)")
	// flagRandom   = flag.Bool("r", true, "accept ``randomized'' input (default)")
	// flagNoRandom = flag.Bool("R", false, "do not accept randomized input")
	// flagBlocking = flag.Bool("b", true, "assume input is ``blocked'' (default)")
	// flagNoBlock  = flag.Bool("B", false, "assume input is not blocked")

)

const author = "unsqueeze was originally written by Daniel J. Bernstein in 1989"
const version = "unsqueeze Golang port of version 1.711\n"

const help = `unsqueeze decompresses its input and prints the result. The input is a
file compressed with squeeze, which uses adaptive Miller-Wegman encoding.

unsqueeze -A: print authorship notice
unsqueeze -C: print copyright notice
unsqueeze -H: print this notice
unsqueeze -U: print short usage summary
unsqueeze -V: print version number

unsqueeze [ -eErRbBdv ]: decompress
  -e: look for explicit indication of EOF
  -E: don't look for explicit EOF (default)
  -r: accept "randomized" input (default)
  -R: do not accept randomized input
  -b: assume input is "blocked" (default)
  -B: assume input is not blocked
  -dstring: display string between output bunches, for debugging
  -v: announce decompression results ("verbose")
`
const usage = "Usage: unsqueeze [ -eErRbBACHUVW ] [ -dstring ]\nHelp:  unsqueeze -H\n"

var (
	numberIn  int
	numberOut int
	startCh   int
	dict      [DictSize]int
	stackBuf  [StackSize]int

	inWord     int
	inWordBits int
	stdin      *bufio.Reader
	stdout     *bufio.Writer

	optEOF      bool
	optRandom   bool
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

	// Correctly handle flags that have defaults and inverse flags
	optEOF = *flagEOF
	if flag.Lookup("E").Value.String() == "true" {
		// This is tricky because both can be set. The C version's while loop over argv
		// means the LAST flag wins.
	}
	// Let's re-parse manually or use a more robust way to match C behavior.
	parseFlags()

	if flag.NArg() > 0 {
		fmt.Fprintln(os.Stderr, "unsqueeze: I am a filter, try unsqz for unsqueezing files")
		os.Exit(1)
	}

	stdin = bufio.NewReader(os.Stdin)
	stdout = bufio.NewWriter(os.Stdout)
	defer stdout.Flush()

startOver:
	max := Alphabet - 1
	ch, err := inChar(max)
	if err == nil {
		codeSpace := max + boolToInt(optEOF) + boolToInt(optBlocking) + 1
		if ch >= codeSpace {
			if optRandom {
				ch %= codeSpace
			} else {
				fmt.Fprintln(os.Stderr, "unsqueeze: bad (randomized?) input")
				os.Exit(1)
			}
		}

		if optEOF && ch == max+1 {
			goto eofSignalled
		}
		if optBlocking && ch == max+boolToInt(optEOF)+1 {
			goto startOver
		}

		startCh = ch
		output(ch)
		if *flagDisplay != "" {
			fmt.Print(*flagDisplay)
		}

		for {
			ch, err = inChar(max)
			if err != nil {
				break
			}

			if max >= DictSize-1 {
				fmt.Fprintln(os.Stderr, "unsqueeze: not enough memory")
				os.Exit(1)
			}

			codeSpace = max + boolToInt(optEOF) + boolToInt(optBlocking) + 1
			if ch >= codeSpace {
				if optRandom {
					ch %= codeSpace
				} else {
					fmt.Fprintln(os.Stderr, "unsqueeze: bad (randomized?) input")
					os.Exit(1)
				}
			}

			if optEOF && ch == max+1 {
				goto eofSignalled
			}
			if optBlocking && ch == max+boolToInt(optEOF)+1 {
				goto startOver
			}

			max++
			dict[max] = ch
			output(ch)
			if *flagDisplay != "" {
				fmt.Print(*flagDisplay)
			}
		}
	}

	if optEOF {
		fmt.Fprintln(os.Stderr, "unsqueeze: EOF not signalled?")
		if *flagVerbose {
			goAheadAndBeVerbose()
		}
		os.Exit(1)
	}

eofSignalled:
	if *flagVerbose {
		goAheadAndBeVerbose()
	}
}

func parseFlags() {
	// Re-implementing the C-style flag parsing to ensure last flag wins
	// and handle the -dstring case correctly.
	optEOF = false
	optRandom = true
	optBlocking = true

	for _, arg := range os.Args[1:] {
		if len(arg) > 0 && arg[0] == '-' {
			for i := 1; i < len(arg); i++ {
				switch arg[i] {
				case 'e':
					optEOF = true
				case 'E':
					optEOF = false
				case 'r':
					optRandom = true
				case 'R':
					optRandom = false
				case 'b':
					optBlocking = true
				case 'B':
					optBlocking = false
				case 'd':
					// Already handled by flag package or we can handle here
					if i+1 < len(arg) {
						// -dstring in same arg
						*flagDisplay = arg[i+1:]
						goto nextArg
					}
				case 'v':
					*flagVerbose = true
				case 'A', 'C', 'V', 'W', 'H', 'U':
					// These are handled by the main function's early return
				}
			}
		}
	nextArg:
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func goAheadAndBeVerbose() {
	ratio := 0
	if numberOut > 0 {
		ratio = (100*numberIn + numberOut/2) / numberOut
	}
	fmt.Fprintf(os.Stderr, "In: %d chars  Out: %d chars  Unsqueezed from: %d%%\n",
		numberIn, numberOut, ratio)
}

func output(ch int) {
	stackPos := 1
	stackBuf[0] = ch
	for stackPos > 0 {
		stackPos--
		ch = stackBuf[stackPos]
		for ch >= Alphabet {
			stackBuf[stackPos] = dict[ch]
			stackPos++
			if ch == Alphabet {
				ch = startCh
			} else {
				ch = dict[ch-1]
			}
		}
		stdout.WriteByte(byte(ch))
		numberOut++
	}
}

func inChar(max int) (int, error) {
	inb := 8
	m := (max + boolToInt(optEOF) + boolToInt(optBlocking)) >> 8
	for m > 0 {
		inb++
		m >>= 1
	}

	for inb > inWordBits {
		b, err := stdin.ReadByte()
		if err != nil {
			return 0, err
		}
		numberIn++
		inWord += int(b) << inWordBits
		inWordBits += 8
	}

	result := inWord % (1 << uint(inb))
	inWord >>= uint(inb)
	inWordBits -= inb
	return result, nil
}
