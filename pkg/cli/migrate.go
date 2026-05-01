package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sort"
	"strings"

	"github.com/fatih/color"
	"github.com/m-mizutani/fireconf"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/shepherd/pkg/cli/config"
	firestoreRepo "github.com/m-mizutani/shepherd/pkg/repository/firestore"
	"github.com/urfave/cli/v3"
)

func cmdMigrate() *cli.Command {
	var (
		repoCfg config.Repository
		dryRun  bool
	)

	flags := repoCfg.Flags()
	flags = append(flags, &cli.BoolFlag{
		Name:        "dry-run",
		Usage:       "Show planned changes without applying",
		Destination: &dryRun,
	})

	return &cli.Command{
		Name:  "migrate",
		Usage: "Apply Firestore index / TTL configuration via fireconf",
		Flags: flags,
		Action: func(ctx context.Context, c *cli.Command) error {
			if repoCfg.Backend() != "firestore" {
				return goerr.New("migrate requires --repository-backend=firestore",
					goerr.V("backend", repoCfg.Backend()))
			}
			if repoCfg.ProjectID() == "" {
				return goerr.New("--firestore-project-id is required")
			}
			databaseID := repoCfg.DatabaseID()
			if databaseID == "" {
				databaseID = "(default)"
			}

			desired := firestoreRepo.DesiredConfig()

			// Suppress fireconf's verbose internal logging — we render our
			// own focused summary below. fireconf still gets a working
			// logger so genuine errors propagate, just to /dev/null.
			silent := slog.New(slog.NewTextHandler(io.Discard, nil))
			opts := []fireconf.Option{fireconf.WithLogger(silent)}
			if dryRun {
				opts = append(opts, fireconf.WithDryRun(true))
			}

			client, err := fireconf.New(ctx, repoCfg.ProjectID(), databaseID, desired, opts...)
			if err != nil {
				return goerr.Wrap(err, "failed to create fireconf client",
					goerr.V("project", repoCfg.ProjectID()),
					goerr.V("database", databaseID),
				)
			}
			defer func() { _ = client.Close() }()

			collections := collectionNames(desired)
			current, err := client.Import(ctx, collections...)
			if err != nil {
				return goerr.Wrap(err, "failed to import current Firestore configuration")
			}

			diff, err := client.DiffConfigs(current)
			if err != nil {
				return goerr.Wrap(err, "failed to diff configurations")
			}

			printPlan(os.Stdout, repoCfg.ProjectID(), databaseID, dryRun, diff)

			if !diffHasChanges(diff) {
				fmt.Fprintln(os.Stdout, color.GreenString("✓ Already up to date. Nothing to apply."))
				return nil
			}

			if dryRun {
				fmt.Fprintln(os.Stdout, color.YellowString("(dry-run) No changes applied. Re-run without --dry-run to apply."))
				return nil
			}

			if err := client.Migrate(ctx); err != nil {
				return goerr.Wrap(err, "fireconf migrate failed")
			}
			fmt.Fprintln(os.Stdout, color.GreenString("✓ Migration applied."))
			return nil
		},
	}
}

func collectionNames(cfg *fireconf.Config) []string {
	if cfg == nil {
		return nil
	}
	out := make([]string, 0, len(cfg.Collections))
	for _, col := range cfg.Collections {
		out = append(out, col.Name)
	}
	return out
}

func diffHasChanges(d *fireconf.DiffResult) bool {
	if d == nil {
		return false
	}
	for _, col := range d.Collections {
		if len(col.IndexesToAdd) > 0 || len(col.IndexesToDelete) > 0 || col.TTLAction != "" {
			return true
		}
	}
	return false
}

// printPlan renders a focused summary of what `migrate` is about to do.
// Two design choices worth calling out:
//   - We render through fmt.Fprintln (not slog) so the output is for humans
//     reading a terminal, not log infrastructure.
//   - We sort everything alphabetically so consecutive runs render
//     deterministically — easier for diffs in CI snapshots and easier on
//     the eyes when scanning.
func printPlan(w io.Writer, project, database string, dryRun bool, diff *fireconf.DiffResult) {
	mode := color.GreenString("APPLY")
	if dryRun {
		mode = color.YellowString("DRY-RUN")
	}

	fmt.Fprintln(w, color.CyanString("─── Firestore migration ──────────────────────────────────"))
	fmt.Fprintf(w, "  %s  %s\n", label("Project"), project)
	fmt.Fprintf(w, "  %s  %s\n", label("Database"), database)
	fmt.Fprintf(w, "  %s  %s\n", label("Mode"), mode)
	fmt.Fprintln(w, color.CyanString("──────────────────────────────────────────────────────────"))

	if !diffHasChanges(diff) {
		return
	}

	addCount, delCount, ttlCount := 0, 0, 0
	cols := append([]fireconf.CollectionDiff(nil), diff.Collections...)
	sort.Slice(cols, func(i, j int) bool { return cols[i].Name < cols[j].Name })

	for _, col := range cols {
		fmt.Fprintln(w)
		fmt.Fprintf(w, "  collection %s\n", color.New(color.Bold).Sprint(col.Name))

		// Indexes to add / delete are sorted by their human-readable
		// description so identical configs render identically across runs.
		toAdd := append([]fireconf.Index(nil), col.IndexesToAdd...)
		sort.Slice(toAdd, func(i, j int) bool { return describeIndex(toAdd[i]) < describeIndex(toAdd[j]) })
		toDel := append([]fireconf.Index(nil), col.IndexesToDelete...)
		sort.Slice(toDel, func(i, j int) bool { return describeIndex(toDel[i]) < describeIndex(toDel[j]) })

		for _, idx := range toAdd {
			fmt.Fprintf(w, "    %s %s\n", color.GreenString("+ create"), describeIndex(idx))
			addCount++
		}
		for _, idx := range toDel {
			fmt.Fprintf(w, "    %s %s\n", color.RedString("- delete"), describeIndex(idx))
			delCount++
		}
		if col.TTLAction != "" {
			ttlCount++
			switch col.TTLAction {
			case fireconf.ActionAdd:
				fmt.Fprintf(w, "    %s TTL on %q\n", color.GreenString("+ add"), col.TTL.Field)
			case fireconf.ActionDelete:
				fmt.Fprintf(w, "    %s TTL\n", color.RedString("- remove"))
			case fireconf.ActionModify:
				fmt.Fprintf(w, "    %s TTL → %q\n", color.YellowString("~ modify"), col.TTL.Field)
			}
		}
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %s  %s create, %s delete, %s ttl\n",
		label("Summary"),
		color.GreenString("%d", addCount),
		color.RedString("%d", delCount),
		color.YellowString("%d", ttlCount),
	)
	fmt.Fprintln(w)
}

func label(s string) string {
	// Pads the field name to a fixed width so values line up.
	const width = 9
	if len(s) < width {
		s += strings.Repeat(" ", width-len(s))
	}
	return color.New(color.Faint).Sprint(s)
}

// describeIndex renders an Index in a one-line human-readable form. Vector
// indexes get a compact `vector(dim=N)` token, scalar fields render as
// `path:ASC` / `path:DESC`, and array-contains indexes render as
// `path:CONTAINS`. The QueryScope is appended in square brackets so the
// reader can tell collection vs collection-group at a glance.
func describeIndex(idx fireconf.Index) string {
	parts := make([]string, 0, len(idx.Fields))
	for _, f := range idx.Fields {
		switch {
		case f.Vector != nil:
			parts = append(parts, fmt.Sprintf("%s:vector(dim=%d)", f.Path, f.Vector.Dimension))
		case f.Array != "":
			parts = append(parts, fmt.Sprintf("%s:%s", f.Path, f.Array))
		case f.Order != "":
			parts = append(parts, fmt.Sprintf("%s:%s", f.Path, shortOrder(f.Order)))
		default:
			parts = append(parts, f.Path)
		}
	}
	scope := string(idx.QueryScope)
	if scope == "" {
		scope = string(fireconf.QueryScopeCollection)
	}
	return fmt.Sprintf("%s [%s]", strings.Join(parts, ", "), scope)
}

func shortOrder(o fireconf.Order) string {
	switch o {
	case fireconf.OrderAscending:
		return "ASC"
	case fireconf.OrderDescending:
		return "DESC"
	default:
		return string(o)
	}
}
