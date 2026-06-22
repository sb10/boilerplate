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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// ErrChangesNeeded is returned in check mode when files need rewriting.
var ErrChangesNeeded = errors.New("boilerplate changes needed")

const licenseLastLine = "SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE."

var authorLineRE = regexp.MustCompile(`^Authors?:\s*(.+)$`)

var authorRE = regexp.MustCompile(`^(.*?)\s*(?:<([^<>]+)>)?$`)

type headerStyle int

const (
	styleGo headerStyle = iota
	stylePython
	styleSlash
)

func commentPayload(style headerStyle, line string) string {
	line = strings.TrimSpace(line)

	switch style {
	case styleGo:
		if strings.HasPrefix(line, "*") {
			return strings.TrimSpace(strings.TrimPrefix(line, "*"))
		}
	case stylePython:
		if strings.HasPrefix(line, "#") {
			return strings.TrimSpace(strings.TrimPrefix(line, "#"))
		}
	case styleSlash:
		if strings.HasPrefix(line, "//") {
			return strings.TrimSpace(strings.TrimPrefix(line, "//"))
		}
	}

	return ""
}

func buildHeader(style headerStyle, newline string, metadata Metadata) string {
	var builder strings.Builder

	if style == styleGo {
		builder.WriteString("/*******************************************************************************" + newline)
	}

	prefix := commentPrefix(style)

	writeCommentLine(&builder, prefix, "Copyright (c) "+FormatYears(metadata.Years)+" Genome Research Ltd.", newline)
	writeCommentLine(&builder, prefix, "", newline)

	for _, author := range metadata.Authors {
		line := "Author: " + author.Name
		if author.Email != "" {
			line += " <" + author.Email + ">"
		}

		writeCommentLine(&builder, prefix, line, newline)
	}

	writeCommentLine(&builder, prefix, "", newline)

	for _, line := range mitLicenseLines() {
		writeCommentLine(&builder, prefix, line, newline)
	}

	if style == styleGo {
		builder.WriteString(" ******************************************************************************/" + newline)
	}

	return builder.String()
}

func writeCommentLine(builder *strings.Builder, prefix string, line string, newline string) {
	if line == "" {
		builder.WriteString(prefix + newline)

		return
	}

	builder.WriteString(prefix + " " + line + newline)
}

// FormatYears returns a compact comma-separated year list.
func FormatYears(years []int) string {
	if len(years) == 0 {
		return ""
	}

	unique := append([]int(nil), years...)
	slices.Sort(unique)
	unique = slices.Compact(unique)

	parts := make([]string, 0, len(unique))

	for index := 0; index < len(unique); index++ {
		start := unique[index]
		end := start

		for index+1 < len(unique) && unique[index+1] == end+1 {
			index++
			end = unique[index]
		}

		if start == end {
			parts = append(parts, strconv.Itoa(start))

			continue
		}

		parts = append(parts, fmt.Sprintf("%d-%d", start, end))
	}

	return strings.Join(parts, ", ")
}

func mitLicenseLines() []string {
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

func commentPrefix(style headerStyle) string {
	switch style {
	case styleGo:
		return " *"
	case stylePython:
		return "#"
	default:
		return "//"
	}
}

// Author is a contributor credited in a source file boilerplate.
type Author struct {
	Name  string
	Email string
}

func parseAuthor(text string) (Author, bool) {
	match := authorRE.FindStringSubmatch(strings.TrimSpace(text))
	if len(match) != 3 {
		return Author{}, false
	}

	name := strings.TrimSpace(match[1])
	email := strings.TrimSpace(match[2])
	if name == "" {
		return Author{}, false
	}

	return Author{Name: name, Email: email}, true
}

func parseHistory(output []byte) ([]historyEntry, error) {
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return nil, errors.New("no commits found")
	}

	lines := bytes.Split(output, []byte("\n"))
	entries := make([]historyEntry, 0, len(lines))

	for _, line := range lines {
		parts := bytes.Split(line, []byte{0})
		if len(parts) != 3 {
			return nil, fmt.Errorf("unexpected git log record %q", line)
		}

		year, err := strconv.Atoi(string(parts[0]))
		if err != nil {
			return nil, fmt.Errorf("invalid year %q: %w", parts[0], err)
		}

		name := strings.TrimSpace(string(parts[1]))
		email := strings.TrimSpace(string(parts[2]))
		if name == "" {
			return nil, errors.New("commit author name is empty")
		}

		entries = append(entries, historyEntry{
			Year:   year,
			Author: Author{Name: name, Email: email},
		})
	}

	return entries, nil
}

func collectGitAuthors(ctx context.Context, root string) ([]Author, error) {
	output, err := gitRaw(ctx, root, "log", "--all", "--format=%an%x00%ae")
	if err != nil {
		return nil, fmt.Errorf("git authors: %w", err)
	}

	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return nil, nil
	}

	lines := bytes.Split(output, []byte("\n"))
	authors := make([]Author, 0, len(lines))

	for _, line := range lines {
		parts := bytes.Split(line, []byte{0})
		if len(parts) != 2 {
			return nil, fmt.Errorf("unexpected git author record %q", line)
		}

		name := strings.TrimSpace(string(parts[0]))
		email := strings.TrimSpace(string(parts[1]))
		if name == "" {
			return nil, errors.New("commit author name is empty")
		}

		authors = append(authors, Author{Name: name, Email: email})
	}

	return authors, nil
}

type authorEmailChoice struct {
	Email string
	Count int
}

type authorEmailIndex struct {
	counts map[string]map[string]int
}

func newAuthorEmailIndex() authorEmailIndex {
	return authorEmailIndex{counts: make(map[string]map[string]int)}
}

func (index *authorEmailIndex) add(author Author) {
	if author.Email == "" {
		return
	}

	if index.counts == nil {
		index.counts = make(map[string]map[string]int)
	}

	if index.counts[author.Name] == nil {
		index.counts[author.Name] = make(map[string]int)
	}

	index.counts[author.Name][author.Email]++
}

func (index *authorEmailIndex) addHeader(header headerBlock) {
	for _, author := range existingAuthors(header) {
		index.add(author)
	}
}

func existingAuthors(header headerBlock) []Author {
	authors := make([]Author, 0)
	for _, line := range strings.Split(header.text, header.newline) {
		payload := commentPayload(header.style, line)
		match := authorLineRE.FindStringSubmatch(payload)
		if len(match) != 2 {
			continue
		}

		author, ok := parseAuthor(match[1])
		if !ok {
			continue
		}

		authors = append(authors, author)
	}

	return authors
}

func (index authorEmailIndex) bestNonGitHubNoreply(name string) (authorEmailChoice, bool) {
	counts := index.counts[name]
	if len(counts) == 0 {
		return authorEmailChoice{}, false
	}

	var best authorEmailChoice
	found := false

	for email, count := range counts {
		if isGitHubNoreply(email) {
			continue
		}

		choice := authorEmailChoice{Email: email, Count: count}
		if !found || betterEmailChoice(choice, best) {
			best = choice
			found = true
		}
	}

	return best, found
}

func isGitHubNoreply(email string) bool {
	return strings.Contains(strings.ToLower(email), "noreply.github.com")
}

func betterEmailChoice(candidate authorEmailChoice, current authorEmailChoice) bool {
	if candidate.Count != current.Count {
		return candidate.Count > current.Count
	}

	return candidate.Email < current.Email
}

// Metadata is the Git-derived data used to rewrite a boilerplate.
type Metadata struct {
	Years   []int
	Authors []Author
}

func historyMetadata(ctx context.Context, root string, rel string, emailResolver authorEmailResolver) (Metadata, error) {
	output, err := gitRaw(ctx, root, "log", "--follow", "--date=format:%Y", "--format=%ad%x00%an%x00%ae", "--", rel)
	if err != nil {
		return Metadata{}, fmt.Errorf("git history for %s: %w", rel, err)
	}

	entries, err := parseHistory(output)
	if err != nil {
		return Metadata{}, fmt.Errorf("parse git history for %s: %w", rel, err)
	}

	years := make([]int, 0, len(entries))
	authorsByName := make(map[string]Author)
	authorNames := make([]string, 0)

	for index := len(entries) - 1; index >= 0; index-- {
		entry := entries[index]
		years = append(years, entry.Year)

		if _, seen := authorsByName[entry.Author.Name]; seen {
			continue
		}

		author := emailResolver.resolve(entry.Author)
		authorsByName[author.Name] = author
		authorNames = append(authorNames, author.Name)
	}

	authors := make([]Author, 0, len(authorNames))
	for _, name := range authorNames {
		authors = append(authors, authorsByName[name])
	}

	return Metadata{Years: years, Authors: authors}, nil
}

// Rewrite updates a supported top-of-file boilerplate using metadata.
func Rewrite(data []byte, metadata Metadata) ([]byte, bool, error) {
	header, ok := detectHeader(string(data))
	if !ok {
		return data, false, nil
	}

	if len(metadata.Years) == 0 {
		return nil, false, errors.New("metadata has no years")
	}

	if len(metadata.Authors) == 0 {
		return nil, false, errors.New("metadata has no authors")
	}

	updatedHeader := buildHeader(header.style, header.newline, metadata)
	updated := append([]byte(updatedHeader), data[header.end:]...)

	return updated, !bytes.Equal(data, updated), nil
}

func detectHeader(content string) (headerBlock, bool) {
	newline := "\n"
	if strings.Contains(content, "\r\n") {
		newline = "\r\n"
	}

	lines := strings.SplitAfter(content, newline)
	if len(lines) == 0 {
		return headerBlock{}, false
	}

	firstLine := trimLineEnding(lines[0])

	switch {
	case firstLine == "/*******************************************************************************":
		return detectGoHeader(content, lines, newline)
	case strings.HasPrefix(strings.TrimLeft(firstLine, " \t"), "# Copyright (c)"):
		return detectLineHeader(content, lines, newline, "#", stylePython)
	case strings.HasPrefix(strings.TrimLeft(firstLine, " \t"), "// Copyright (c)"):
		return detectLineHeader(content, lines, newline, "//", styleSlash)
	default:
		return headerBlock{}, false
	}
}

type authorEmailResolver struct {
	repoEmails               authorEmailIndex
	currentEmails            authorEmailIndex
	realEmailByGitHubNoreply map[string]authorEmailChoice
}

func newAuthorEmailResolver(repoEmails authorEmailIndex, gitAuthors []Author) authorEmailResolver {
	realEmailByGitHubNoreply := make(map[string]authorEmailChoice)

	for _, author := range gitAuthors {
		if !isGitHubNoreply(author.Email) {
			continue
		}

		choice, ok := repoEmails.bestNonGitHubNoreply(author.Name)
		if !ok {
			continue
		}

		key := strings.ToLower(author.Email)
		current, seen := realEmailByGitHubNoreply[key]
		if !seen || betterEmailChoice(choice, current) {
			realEmailByGitHubNoreply[key] = choice
		}
	}

	return authorEmailResolver{
		repoEmails:               repoEmails,
		realEmailByGitHubNoreply: realEmailByGitHubNoreply,
	}
}

func (resolver authorEmailResolver) withCurrentHeader(header headerBlock) authorEmailResolver {
	currentEmails := newAuthorEmailIndex()
	currentEmails.addHeader(header)
	resolver.currentEmails = currentEmails

	return resolver
}

func (resolver authorEmailResolver) resolve(author Author) Author {
	if choice, ok := resolver.currentEmails.bestNonGitHubNoreply(author.Name); ok {
		author.Email = choice.Email

		return author
	}

	if choice, ok := resolver.repoEmails.bestNonGitHubNoreply(author.Name); ok {
		author.Email = choice.Email

		return author
	}

	if !isGitHubNoreply(author.Email) {
		return author
	}

	if choice, ok := resolver.realEmailByGitHubNoreply[strings.ToLower(author.Email)]; ok {
		author.Email = choice.Email

		return author
	}

	author.Email = ""

	return author
}

func trimLineEnding(line string) string {
	line = strings.TrimSuffix(line, "\n")

	return strings.TrimSuffix(line, "\r")
}

func detectGoHeader(content string, lines []string, newline string) (headerBlock, bool) {
	end := len(lines[0])

	for _, line := range lines[1:] {
		end += len(line)

		if trimLineEnding(line) != " ******************************************************************************/" {
			continue
		}

		text := content[:end]
		if !recognizedHeader(text) {
			return headerBlock{}, false
		}

		return headerBlock{style: styleGo, end: end, newline: newline, text: text}, true
	}

	return headerBlock{}, false
}

func recognizedHeader(text string) bool {
	return strings.Contains(text, "Copyright (c)") &&
		strings.Contains(text, "Genome Research Ltd.") &&
		strings.Contains(text, "Permission is hereby granted") &&
		strings.Contains(text, licenseLastLine)
}

func detectLineHeader(content string, lines []string, newline string, prefix string, style headerStyle) (headerBlock, bool) {
	end := 0

	for _, line := range lines {
		raw := trimLineEnding(line)
		trimmed := strings.TrimLeft(raw, " \t")

		if !strings.HasPrefix(trimmed, prefix) {
			return headerBlock{}, false
		}

		end += len(line)

		payload := strings.TrimSpace(strings.TrimPrefix(trimmed, prefix))
		if payload != licenseLastLine {
			continue
		}

		text := content[:end]
		if !recognizedHeader(text) {
			return headerBlock{}, false
		}

		return headerBlock{style: style, end: end, newline: newline, text: text}, true
	}

	return headerBlock{}, false
}

// Options controls a boilerplate run.
type Options struct {
	Repo   string
	Paths  []string
	Write  bool
	Check  bool
	Stdout io.Writer
	Stderr io.Writer
}

// Result describes files examined and updated by a run.
type Result struct {
	Scanned int
	Changed []string
}

// Run scans and optionally rewrites supported source files in a Git repo.
func Run(ctx context.Context, options Options) (Result, error) {
	repo := options.Repo
	if repo == "" {
		repo = "."
	}

	root, err := gitText(ctx, repo, "rev-parse", "--show-toplevel")
	if err != nil {
		return Result{}, err
	}

	files, err := gitFiles(ctx, root, options.Paths)
	if err != nil {
		return Result{}, err
	}

	emailFiles := files
	if len(options.Paths) > 0 {
		emailFiles, err = gitFiles(ctx, root, nil)
		if err != nil {
			return Result{}, err
		}
	}

	repoEmails, err := collectExistingAuthorEmails(root, emailFiles)
	if err != nil {
		return Result{}, err
	}

	gitAuthors, err := collectGitAuthors(ctx, root)
	if err != nil {
		return Result{}, err
	}

	emailResolver := newAuthorEmailResolver(repoEmails, gitAuthors)

	stdout := options.Stdout
	if stdout == nil {
		stdout = io.Discard
	}

	result := Result{}

	for _, rel := range files {
		result.Scanned++

		abs := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if err != nil {
			return result, fmt.Errorf("read %s: %w", rel, err)
		}

		header, ok := detectHeader(string(data))
		if !ok {
			continue
		}

		metadata, err := historyMetadata(ctx, root, rel, emailResolver.withCurrentHeader(header))
		if err != nil {
			return result, err
		}

		updated, changed, err := Rewrite(data, metadata)
		if err != nil {
			return result, fmt.Errorf("rewrite %s: %w", rel, err)
		}

		if !changed {
			continue
		}

		result.Changed = append(result.Changed, rel)

		if options.Write {
			info, err := os.Stat(abs)
			if err != nil {
				return result, fmt.Errorf("stat %s: %w", rel, err)
			}

			if err := os.WriteFile(abs, updated, info.Mode()); err != nil {
				return result, fmt.Errorf("write %s: %w", rel, err)
			}

			if _, err := fmt.Fprintf(stdout, "updated: %s\n", rel); err != nil {
				return result, fmt.Errorf("write stdout: %w", err)
			}

			continue
		}

		if _, err := fmt.Fprintf(stdout, "would update: %s\n", rel); err != nil {
			return result, fmt.Errorf("write stdout: %w", err)
		}
	}

	if options.Check && len(result.Changed) > 0 {
		return result, ErrChangesNeeded
	}

	return result, nil
}

type headerBlock struct {
	style   headerStyle
	end     int
	newline string
	text    string
}

func collectExistingAuthorEmails(root string, files []string) (authorEmailIndex, error) {
	emails := newAuthorEmailIndex()

	for _, rel := range files {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		data, err := os.ReadFile(abs)
		if err != nil {
			return authorEmailIndex{}, fmt.Errorf("read %s: %w", rel, err)
		}

		header, ok := detectHeader(string(data))
		if !ok {
			continue
		}

		emails.addHeader(header)
	}

	return emails, nil
}

type historyEntry struct {
	Year   int
	Author Author
}

func gitText(ctx context.Context, repo string, args ...string) (string, error) {
	output, err := gitRaw(ctx, repo, args...)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

func gitFiles(ctx context.Context, root string, paths []string) ([]string, error) {
	args := []string{"ls-files", "-z"}

	if len(paths) > 0 {
		cleaned, err := cleanPaths(root, paths)
		if err != nil {
			return nil, err
		}

		args = append(args, "--")
		args = append(args, cleaned...)
	}

	output, err := gitRaw(ctx, root, args...)
	if err != nil {
		return nil, err
	}

	files := strings.Split(strings.TrimSuffix(string(output), "\x00"), "\x00")
	filtered := make([]string, 0, len(files))

	for _, file := range files {
		if file == "" || !supportedSource(file) {
			continue
		}

		filtered = append(filtered, file)
	}

	slices.Sort(filtered)

	return filtered, nil
}

func cleanPaths(root string, paths []string) ([]string, error) {
	cleaned := make([]string, 0, len(paths))

	for _, path := range paths {
		rel := path

		if filepath.IsAbs(path) {
			var err error
			rel, err = filepath.Rel(root, path)
			if err != nil {
				return nil, fmt.Errorf("make %s relative to repo: %w", path, err)
			}
		}

		rel = filepath.Clean(rel)
		if rel == "." {
			cleaned = append(cleaned, ".")

			continue
		}

		if strings.HasPrefix(rel, ".."+string(filepath.Separator)) || rel == ".." {
			return nil, fmt.Errorf("%s is outside repo %s", path, root)
		}

		cleaned = append(cleaned, filepath.ToSlash(rel))
	}

	return cleaned, nil
}

func gitRaw(ctx context.Context, repo string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repo}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git -C %s %s: %w\n%s", repo, strings.Join(args, " "), err, output)
	}

	return output, nil
}

func supportedSource(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".go", ".py", ".nf", ".config":
		return true
	default:
		return false
	}
}
