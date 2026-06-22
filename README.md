# boilerplate
Fix copyright year and author in boilerplate comments

## boilerplate

`boilerplate` rewrites supported top-of-file MIT boilerplate comments using
per-file Git history. It currently recognizes the Go, Python, and Nextflow
comment styles used by the convention skills in
[`wtsi-hgi/agentskills`](https://github.com/wtsi-hgi/agentskills), such as
`skills/go-conventions/SKILL.md`, `skills/python-conventions/SKILL.md`, and
`skills/nextflow-conventions/SKILL.md`.

Dry run:

```bash
go run . -repo /path/to/repo
```

Rewrite files in place:

```bash
go run . -repo /path/to/repo -write
```

Check mode exits non-zero if any tracked source file needs a rewrite:

```bash
go run . -repo /path/to/repo -check
```
