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

package boilerplate

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

func TestFormatYears(t *testing.T) {
	Convey("Given unsorted years with duplicates", t, func() {
		years := []int{2026, 2019, 2024, 2021, 2023, 2025, 2023}

		Convey("It formats single years and contiguous ranges", func() {
			So(FormatYears(years), ShouldEqual, "2019, 2021, 2023-2026")
		})
	})
}

func TestRewrite(t *testing.T) {
	Convey("Given supported boilerplate styles", t, func() {
		meta := Metadata{
			Years: []int{2020, 2022, 2023},
			Authors: []Author{
				{Name: "Alice Example", Email: "alice@example.com"},
				{Name: "Bob Example", Email: "bob@example.com"},
			},
		}

		Convey("It rewrites Go block comments", func() {
			data := []byte(goHeader("2018", []Author{{Name: "Alice Example", Email: "old@example.com"}}) + "\npackage demo\n")

			updated, changed, err := Rewrite(data, meta)

			So(err, ShouldBeNil)
			So(changed, ShouldBeTrue)
			So(string(updated), ShouldContainSubstring, " * Copyright (c) 2020, 2022-2023 Genome Research Ltd.")
			So(string(updated), ShouldContainSubstring, " * Author: Alice Example <alice@example.com>\n * Author: Bob Example <bob@example.com>")
			So(string(updated), ShouldContainSubstring, "\npackage demo\n")
		})

		Convey("It rewrites Python line comments", func() {
			data := []byte(lineHeader("#", "2018", []Author{{Name: "Alice Example", Email: "old@example.com"}}) + "\nprint('ok')\n")

			updated, changed, err := Rewrite(data, meta)

			So(err, ShouldBeNil)
			So(changed, ShouldBeTrue)
			So(string(updated), ShouldContainSubstring, "# Copyright (c) 2020, 2022-2023 Genome Research Ltd.")
			So(string(updated), ShouldContainSubstring, "# Author: Alice Example <alice@example.com>\n# Author: Bob Example <bob@example.com>")
			So(string(updated), ShouldContainSubstring, "\nprint('ok')\n")
		})

		Convey("It rewrites Nextflow line comments", func() {
			data := []byte(lineHeader("//", "2018", []Author{{Name: "Alice Example", Email: "old@example.com"}}) + "\nnextflow.enable.dsl = 2\n")

			updated, changed, err := Rewrite(data, meta)

			So(err, ShouldBeNil)
			So(changed, ShouldBeTrue)
			So(string(updated), ShouldContainSubstring, "// Copyright (c) 2020, 2022-2023 Genome Research Ltd.")
			So(string(updated), ShouldContainSubstring, "// Author: Alice Example <alice@example.com>\n// Author: Bob Example <bob@example.com>")
			So(string(updated), ShouldContainSubstring, "\nnextflow.enable.dsl = 2\n")
		})
	})
}

func TestRunUsesGitHistoryPerFile(t *testing.T) {
	Convey("Given a Git repo with file-specific history", t, func() {
		repo := t.TempDir()
		runGit(t, repo, nil, "init")

		sourcePath := filepath.Join(repo, "example.go")
		initial := goHeader("2020", []Author{{Name: "Alice Example", Email: "alice@header.test"}}) +
			"\npackage demo\n\nconst First = 1\n"
		So(os.WriteFile(sourcePath, []byte(initial), 0o600), ShouldBeNil)
		commitAll(t, repo, "2019-01-02T03:04:05Z", "Alice Example", "alice@git.test", "initial")

		So(os.WriteFile(sourcePath, []byte(initial+"\nconst Second = 2\n"), 0o600), ShouldBeNil)
		commitAll(t, repo, "2021-02-03T04:05:06Z", "Bob Example", "bob@git.test", "bob change")

		So(os.WriteFile(sourcePath, []byte(initial+"\nconst Second = 2\nconst Third = 3\n"), 0o600), ShouldBeNil)
		commitAll(t, repo, "2023-03-04T05:06:07Z", "Alice Example", "alice@new.test", "alice change")

		So(os.WriteFile(sourcePath, []byte(initial+"\nconst Second = 2\nconst Third = 3\nconst Fourth = 4\n"), 0o600), ShouldBeNil)
		commitAll(t, repo, "2024-04-05T06:07:08Z", "Bob Example", "bob@new.test", "bob again")

		Convey("It keeps existing emails by author name and adds missing author emails from commits", func() {
			stdout := &bytes.Buffer{}
			result, err := Run(context.Background(), Options{
				Repo:   repo,
				Write:  true,
				Stdout: stdout,
			})

			So(err, ShouldBeNil)
			So(result.Changed, ShouldResemble, []string{"example.go"})
			So(stdout.String(), ShouldContainSubstring, "updated: example.go")

			updated, err := os.ReadFile(sourcePath)
			So(err, ShouldBeNil)
			So(string(updated), ShouldContainSubstring, " * Copyright (c) 2019, 2021, 2023-2024 Genome Research Ltd.")
			So(string(updated), ShouldContainSubstring, " * Author: Alice Example <alice@header.test>\n * Author: Bob Example <bob@git.test>")
			So(string(updated), ShouldNotContainSubstring, "alice@git.test")
			So(string(updated), ShouldNotContainSubstring, "bob@new.test")
		})

		Convey("It can report needed changes without writing", func() {
			stdout := &bytes.Buffer{}
			result, err := Run(context.Background(), Options{
				Repo:   repo,
				Paths:  []string{"example.go"},
				Stdout: stdout,
			})

			So(err, ShouldBeNil)
			So(result.Changed, ShouldResemble, []string{"example.go"})
			So(stdout.String(), ShouldContainSubstring, "would update: example.go")

			updated, err := os.ReadFile(sourcePath)
			So(err, ShouldBeNil)
			So(string(updated), ShouldContainSubstring, " * Copyright (c) 2020 Genome Research Ltd.")
		})
	})
}

func TestRunUsesRepositoryHeaderEmailsForKnownAuthors(t *testing.T) {
	Convey("Given a Git repo with an author email in another tracked header", t, func() {
		repo := t.TempDir()
		runGit(t, repo, nil, "init")

		targetPath := filepath.Join(repo, "target.go")
		target := goHeader("2020", []Author{{Name: "Alice Example", Email: "alice@target-header.test"}}) +
			"\npackage demo\n\nconst First = 1\n"
		So(os.WriteFile(targetPath, []byte(target), 0o600), ShouldBeNil)

		knownPath := filepath.Join(repo, "known.go")
		known := goHeader("2020", []Author{{Name: "Bob Example", Email: "bob@repo-header.test"}}) +
			"\npackage demo\n\nconst Known = 1\n"
		So(os.WriteFile(knownPath, []byte(known), 0o600), ShouldBeNil)
		commitAll(t, repo, "2020-01-02T03:04:05Z", "Alice Example", "alice@git.test", "initial")

		So(os.WriteFile(targetPath, []byte(target+"\nconst Second = 2\n"), 0o600), ShouldBeNil)
		commitAll(t, repo, "2021-02-03T04:05:06Z", "Bob Example", "bob@commit.test", "bob change")

		So(os.WriteFile(targetPath, []byte(target+"\nconst Second = 2\nconst Third = 3\n"), 0o600), ShouldBeNil)
		commitAll(t, repo, "2022-03-04T05:06:07Z", "Carol Example", "carol@commit.test", "carol change")

		Convey("It uses known header emails before falling back to commit emails", func() {
			stdout := &bytes.Buffer{}
			result, err := Run(context.Background(), Options{
				Repo:   repo,
				Paths:  []string{"target.go"},
				Write:  true,
				Stdout: stdout,
			})

			So(err, ShouldBeNil)
			So(result.Changed, ShouldResemble, []string{"target.go"})
			So(stdout.String(), ShouldContainSubstring, "updated: target.go")

			updated, err := os.ReadFile(targetPath)
			So(err, ShouldBeNil)
			So(string(updated), ShouldContainSubstring, " * Copyright (c) 2020-2022 Genome Research Ltd.")
			So(string(updated), ShouldContainSubstring, " * Author: Alice Example <alice@target-header.test>\n * Author: Bob Example <bob@repo-header.test>\n * Author: Carol Example <carol@commit.test>")
			So(string(updated), ShouldNotContainSubstring, "bob@commit.test")
			So(string(updated), ShouldNotContainSubstring, "alice@git.test")
		})
	})
}

func TestRunAvoidsGitHubNoreplyHeaderEmails(t *testing.T) {
	Convey("Given a Git repo with real and GitHub noreply author identities", t, func() {
		repo := t.TempDir()
		runGit(t, repo, nil, "init")

		targetPath := filepath.Join(repo, "target.go")
		target := goHeader("2020", []Author{
			{Name: "Current Example", Email: "current@target.test"},
			{Name: "Nora Example", Email: "333+nora@users.noreply.github.com"},
		}) + "\npackage demo\n\nconst First = 1\n"
		So(os.WriteFile(targetPath, []byte(target), 0o600), ShouldBeNil)
		commitAll(t, repo, "2020-01-02T03:04:05Z", "Current Example", "current@commit.test", "target initial")

		knownFiles := map[string]string{
			"alias-real.go": goHeader("2020", []Author{{Name: "Real Name", Email: "real@header.test"}}) +
				"\npackage demo\n\nconst AliasReal = 1\n",
			"chris-best-a.go": goHeader("2020", []Author{{Name: "Chris Example", Email: "chris@best.test"}}) +
				"\npackage demo\n\nconst ChrisBestA = 1\n",
			"chris-best-b.go": goHeader("2020", []Author{{Name: "Chris Example", Email: "chris@best.test"}}) +
				"\npackage demo\n\nconst ChrisBestB = 1\n",
			"chris-old.go": goHeader("2020", []Author{{Name: "Chris Example", Email: "chris@old.test"}}) +
				"\npackage demo\n\nconst ChrisOld = 1\n",
			"current-repo.go": goHeader("2020", []Author{{Name: "Current Example", Email: "current@repo.test"}}) +
				"\npackage demo\n\nconst CurrentRepo = 1\n",
			"nora-real.go": goHeader("2020", []Author{{Name: "Nora Example", Email: "nora@real.test"}}) +
				"\npackage demo\n\nconst NoraReal = 1\n",
		}

		for name, content := range knownFiles {
			So(os.WriteFile(filepath.Join(repo, name), []byte(content), 0o600), ShouldBeNil)
		}

		commitAll(t, repo, "2020-06-02T03:04:05Z", "Real Name", "111+alias@users.noreply.github.com", "known headers")

		target += "\nconst ChrisChange = 2\n"
		So(os.WriteFile(targetPath, []byte(target), 0o600), ShouldBeNil)
		commitAll(t, repo, "2021-01-02T03:04:05Z", "Chris Example", "222+chris@users.noreply.github.com", "chris")

		target += "const NoraChange = 3\n"
		So(os.WriteFile(targetPath, []byte(target), 0o600), ShouldBeNil)
		commitAll(t, repo, "2022-01-02T03:04:05Z", "Nora Example", "333+nora@users.noreply.github.com", "nora")

		target += "const AliasChange = 4\n"
		So(os.WriteFile(targetPath, []byte(target), 0o600), ShouldBeNil)
		commitAll(t, repo, "2023-01-02T03:04:05Z", "Alias Login", "111+alias@users.noreply.github.com", "alias")

		target += "const NoreplyChange = 5\n"
		So(os.WriteFile(targetPath, []byte(target), 0o600), ShouldBeNil)
		commitAll(t, repo, "2024-01-02T03:04:05Z", "Only Noreply", "444+only@users.noreply.github.com", "noreply")

		target += "const FallbackChange = 6\n"
		So(os.WriteFile(targetPath, []byte(target), 0o600), ShouldBeNil)
		commitAll(t, repo, "2025-01-02T03:04:05Z", "Fallback Example", "fallback@commit.test", "fallback")

		Convey("It rewrites headers without GitHub noreply email addresses", func() {
			stdout := &bytes.Buffer{}
			result, err := Run(context.Background(), Options{
				Repo:   repo,
				Paths:  []string{"target.go"},
				Write:  true,
				Stdout: stdout,
			})

			So(err, ShouldBeNil)
			So(result.Changed, ShouldResemble, []string{"target.go"})
			So(stdout.String(), ShouldContainSubstring, "updated: target.go")

			updated, err := os.ReadFile(targetPath)
			So(err, ShouldBeNil)
			So(string(updated), ShouldContainSubstring, " * Author: Current Example <current@target.test>\n"+
				" * Author: Chris Example <chris@best.test>\n"+
				" * Author: Nora Example <nora@real.test>\n"+
				" * Author: Alias Login <real@header.test>\n"+
				" * Author: Only Noreply\n"+
				" * Author: Fallback Example <fallback@commit.test>")
			So(string(updated), ShouldNotContainSubstring, "noreply.github.com")
			So(string(updated), ShouldNotContainSubstring, "chris@old.test")
			So(string(updated), ShouldNotContainSubstring, "current@repo.test")
		})
	})
}

func lineHeader(prefix string, years string, authors []Author) string {
	var builder strings.Builder
	writeHeaderLines(&builder, prefix, years, authors)

	return builder.String()
}

func goHeader(years string, authors []Author) string {
	var builder strings.Builder
	builder.WriteString("/*******************************************************************************\n")
	writeHeaderLines(&builder, " *", years, authors)
	builder.WriteString(" ******************************************************************************/\n")

	return builder.String()
}

func writeHeaderLines(builder *strings.Builder, prefix string, years string, authors []Author) {
	lines := append([]string{
		"Copyright (c) " + years + " Genome Research Ltd.",
		"",
	}, authorLines(authors)...)
	lines = append(lines, append([]string{""}, licenseText()...)...)

	for _, line := range lines {
		if line == "" {
			builder.WriteString(prefix + "\n")

			continue
		}

		builder.WriteString(prefix + " " + line + "\n")
	}
}

func authorLines(authors []Author) []string {
	lines := make([]string, 0, len(authors))

	for _, author := range authors {
		lines = append(lines, "Author: "+author.Name+" <"+author.Email+">")
	}

	return lines
}

func licenseText() []string {
	return []string{
		"Permission is hereby granted, free of charge, to any person obtaining",
		"a copy of this software and associated documentation files (the",
		"\"Software\"), to deal in the Software without restriction, including",
		"without limitation the rights to use, copy, modify, merge, publish,",
		"distribute, sublicense, and/or sell copies of the Software, and to",
		"permit persons to whom the Software is furnished to do so, subject to",
		"the following conditions:",
		"",
		"The above copyright notice and this permission notice shall be included",
		"in all copies or substantial portions of the Software.",
		"",
		"THE SOFTWARE IS PROVIDED \"AS IS\", WITHOUT WARRANTY OF ANY KIND,",
		"EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF",
		"MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.",
		"IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY",
		"CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,",
		"TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE",
		"SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.",
	}
}

func commitAll(t *testing.T, repo string, date string, name string, email string, message string) {
	t.Helper()

	runGit(t, repo, nil, "add", ".")
	runGit(t, repo, []string{
		"GIT_AUTHOR_DATE=" + date,
		"GIT_COMMITTER_DATE=" + date,
		"GIT_AUTHOR_NAME=" + name,
		"GIT_AUTHOR_EMAIL=" + email,
		"GIT_COMMITTER_NAME=" + name,
		"GIT_COMMITTER_EMAIL=" + email,
	}, "commit", "-m", message)
}

func runGit(t *testing.T, repo string, env []string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", append([]string{"-C", repo}, args...)...)
	cmd.Env = append(os.Environ(), env...)
	output, err := cmd.CombinedOutput()
	So(err, ShouldBeNil)
	So(string(output), ShouldNotContainSubstring, "fatal:")
}
