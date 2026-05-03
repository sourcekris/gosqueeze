# gosqueeze
A modern Go re-implementation of the 1989 `squeeze` and `unsqueeze` adaptive compression tools.

## Overview
This project is a bit-compatible port of the Miller-Wegman adaptive encoding tools originally developed by **Daniel J. Bernstein** in 1989. It replicates the behavior of the original version 1.711, allowing for the compression and decompression of files using the same historical algorithm.

## Credits & History
The original `squeeze` and `unsqueeze` utilities were written by Daniel J. Bernstein (brnstnd@acf10.nyu.edu) and released in October 1989. 

This Go version is a functional re-interpretation of that 40-year-old code, aimed at historical preservation and compatibility. While the logic remains faithful to the original, the implementation has been modernized for the Go ecosystem.

## Building
To build the binaries, run:
```bash
go build -o squeeze ./cmd/squeeze
go build -o unsqueeze ./cmd/unsqueeze
```

## Usage
Both tools act as filters, reading from stdin and writing to stdout.

### Compress
```bash
./squeeze < input_file > output.sqz
```

### Decompress
```bash
./unsqueeze < output.sqz > output_file
```

### Options
**Squeeze:**
- `-e`: Explicitly indicate EOF
- `-r`: "Randomize" output; useful before encryption
- `-b`: Readapt to next block when out of memory (default)
- `-B`: Freeze working adaptation when out of memory
- `-v`: Announce compression results (verbose)

**Unsqueeze:**
- `-e`: Look for explicit indication of EOF
- `-r`: Accept randomized input (default)
- `-b`: Assume input is blocked (default)
- `-v`: Announce decompression results (verbose)

## License
The original C implementation was Copyright (c) 1989, Daniel J. Bernstein.
This Go implementation is licensed under the **Apache License, Version 2.0**. See the `LICENSE` file for details.
