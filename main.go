/*******************************************************************************
 * Copyright (c) 2026 Genome Research Ltd.
 *
 * Author: Sendu Bala <sb10@sanger.ac.uk>
 *
 * Permission is hereby granted, free of charge, to any person obtaining
 * a copy of this software and associated documentation files (the
 * "Software"), to deal in the Software without restriction, including
 * without limitation the rights to use, copy, modify, merge, publish,
 * distribute, sublicense, and/or sell copies of the Software, and to
 * permit persons to whom the Software is furnished to do so, subject to
 * the following conditions:
 *
 * The above copyright notice and this permission notice shall be included
 * in all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
 * EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
 * MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
 * IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
 * CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
 * TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
 * SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 ******************************************************************************/

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/sb10/boilerplate/internal/boilerplate"
)

func main() {
	os.Exit(run())
}

func run() int {
	var repo string
	var write bool
	var check bool

	flags := flag.NewFlagSet("boilerplate", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	flags.StringVar(&repo, "repo", ".", "Git repository to scan")
	flags.BoolVar(&write, "write", false, "rewrite files in place")
	flags.BoolVar(&check, "check", false, "exit non-zero when rewrites are needed")
	flags.Usage = func() {
		_, _ = fmt.Fprintf(flags.Output(), "Usage: boilerplate [-repo DIR] [-write] [-check] [path ...]\n")
		flags.PrintDefaults()
	}

	if err := flags.Parse(os.Args[1:]); err != nil {
		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	result, err := boilerplate.Run(ctx, boilerplate.Options{
		Repo:   repo,
		Paths:  flags.Args(),
		Write:  write,
		Check:  check,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	if errors.Is(err, boilerplate.ErrChangesNeeded) {
		fmt.Fprintf(os.Stderr, "%d file(s) need boilerplate updates\n", len(result.Changed))

		return 1
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	return 0
}
