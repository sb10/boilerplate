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
	"io"
	"os"
	"os/signal"

	"github.com/sb10/boilerplate/internal/boilerplate"
)

const helpText = `boilerplate updates top-of-file MIT boilerplate comments using Git history.

For each supported source file, it finds the years in which that file changed
and rewrites the copyright line with only those years, compacting contiguous
years into ranges. It also rewrites the Author lines from the file's Git
history, deduplicating authors by name.

Existing header email addresses are preferred over commit emails. When an
author appears in multiple headers, the most commonly used real email address
is chosen; GitHub noreply addresses are avoided. If no real email is known for
an author, the author can be written without an email rather than adding a bad
noreply address.

Recognized comment styles match the Go, Python, and Nextflow convention
boilerplates from github.com/wtsi-hgi/agentskills. Supported tracked files are
.go, .py, .nf, and .config files.`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	var repo string
	var write bool
	var check bool

	flags := flag.NewFlagSet("boilerplate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&repo, "repo", ".", "Git repository to scan")
	flags.BoolVar(&write, "write", false, "rewrite files in place")
	flags.BoolVar(&check, "check", false, "exit non-zero when rewrites are needed")
	flags.Usage = func() {
		_, _ = fmt.Fprintf(flags.Output(), `%s

Usage:
  boilerplate [-repo DIR] [-write] [-check] [path ...]

By default boilerplate does a dry run and prints each tracked source file whose
header would change. Pass one or more paths to limit which files are checked;
repo-wide header emails are still considered so known authors keep their usual
addresses.

Options:
`, helpText)
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}

		return 2
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	result, err := boilerplate.Run(ctx, boilerplate.Options{
		Repo:   repo,
		Paths:  flags.Args(),
		Write:  write,
		Check:  check,
		Stdout: stdout,
		Stderr: stderr,
	})
	if errors.Is(err, boilerplate.ErrChangesNeeded) {
		_, _ = fmt.Fprintf(stderr, "%d file(s) need boilerplate updates\n", len(result.Changed))

		return 1
	}

	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)

		return 1
	}

	return 0
}
