# The project board

How the roadmap is steered day to day:
[github.com/users/stephrobert/projects/2](https://github.com/users/stephrobert/projects/2),
one item per batch issue of [roadmap.md](roadmap.md)'s operational view.

The board duplicates nothing that is already native. The wave is the milestone,
the provider is a label, open or closed is the issue state. It adds the two
things that exist nowhere else: **Status** (`Todo`, `In Progress`, `Done` — the
project's own field) and **Size** (`S`/`M`/`L`, from the batch headings of the
three `roadmap-*-iaas.md` documents, which meet each other only here). Both are
scripted from the versioned batch definitions in `tools/issues/batches/`.

## What "ready to start" means, and who maintains it

A batch is ready when every batch it waits on is closed. That fact lives in
three layers, each with one job:

1. **Native issue dependencies** are the ground truth.
   `tools/issues/dependencies.sh` mirrors the `after:` front matter of the
   batch definitions into GitHub blocked-by relationships — the same edges the
   issue bodies state in prose.
2. **The `blocked` label** is the projection of that truth into the only space
   a board filter can see. Measured 2026-07-30: the Projects filter language
   has no qualifier for dependencies, so views filter on the label.
3. **The `Unblock` workflow** (`.github/workflows/unblock.yml`) keeps 2 equal
   to 1: on every issue close or reopen it recounts open blockers from the
   dependency lists and adds or removes the label. Nobody maintains it by hand,
   because a label maintained by memory is a comment.

An issue labelled `blocked` by hand, blocker named in the body per
`CONTRIBUTING.md`, carries no dependency records and the workflow leaves it
alone.

## The views

| View | Layout | Filter | Answers |
|---|---|---|---|
| **Waves** | Table, grouped by Milestone | *(none)* | How far along is each wave, closed items included |
| **Now** | Board, columns by Status | `is:open -label:blocked` | What is actionable, and what is in flight |
| **Ready to start** | Table | `is:open status:Todo -label:blocked` | What can begin today — pick by Milestone first, Size second |
| **Providers** | Table, sliced by Labels | *(none)* | One provider's whole story, via the slice panel |

The filters are written by `tools/issues/project.sh`. Three settings are not,
because the view mutations carry only name, layout and filter (measured against
the GraphQL schema): **Waves** is grouped by Milestone, **Ready to start** is
sorted by Milestone ascending, **Providers** is sliced by Labels. Each is one
click, once, in the view's menu.

## What moves a card

- **Todo → In Progress**: by hand. Dragging the card is the one human signal
  the board asks for, and nothing ever drags it back for you.
- **→ Done**: close the issue. The built-in "Item closed" workflow sets the
  status. The reverse is also live — the built-in "Auto-close issue" workflow
  closes the issue when a card is dragged to Done — so do not use the drag as a
  closing gesture: a batch closes on the four conditions in its body, command
  first.
- **`blocked` on and off**: the `Unblock` workflow, as above. Closing `X-1`
  frees `SW-1`, `OSC-1` and `EXO-1` on the board without anyone touching them.

## Adding a batch

Write the definition in `tools/issues/batches/` (with `size:` and `after:`),
then run, in order and each idempotent:

```bash
tools/issues/create.sh        # the issue, milestone, labels, blocked-by prose
tools/issues/dependencies.sh  # the native blocked-by relationships
tools/issues/sync-blocked.sh  # the blocked label, until the workflow's next run
tools/issues/project.sh       # onto the board, Status and Size filled
```

`project.sh` needs the `project` OAuth scope: `gh auth refresh -s project`.
