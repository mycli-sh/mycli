package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"mycli.sh/cli/internal/auth"
	"mycli.sh/cli/internal/cache"
	"mycli.sh/cli/internal/client"
	"mycli.sh/cli/internal/config"
	"mycli.sh/cli/internal/library"
	"mycli.sh/pkg/spec"
)

var tagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

func isSystemOwner(owner string) bool {
	return owner == "" || strings.EqualFold(owner, "system")
}

func newLibraryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "library",
		Aliases: []string{"lib"},
		Short:   "Discover, install, and manage command libraries",
		Long:    "Search public libraries, install from the registry, publish, and manage installed libraries.",
	}

	cmd.AddCommand(newLibrarySearchCmd())
	cmd.AddCommand(newLibraryInstallCmd())
	cmd.AddCommand(newLibraryUninstallCmd())
	cmd.AddCommand(newLibrarySyncCmd())
	cmd.AddCommand(newLibraryListCmd())
	cmd.AddCommand(newLibraryReleaseCmd())
	cmd.AddCommand(newLibraryInfoCmd())
	cmd.AddCommand(newLibraryExploreCmd())

	return cmd
}

func newLibrarySearchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "search <query>",
		Short: "Search public libraries",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			resp, err := c.SearchPublicLibraries(args[0], 20, 0)
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if len(resp.Libraries) == 0 {
				fmt.Println("No libraries found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tDESCRIPTION\tINSTALLS")
			for _, lib := range resp.Libraries {
				desc := lib.Description
				if len(desc) > 60 {
					desc = desc[:57] + "..."
				}
				_, _ = fmt.Fprintf(w, "%s\t%s\t%d\n", lib.Slug, desc, lib.InstallCount)
			}
			_ = w.Flush()

			if resp.Total > len(resp.Libraries) {
				fmt.Printf("\nShowing %d of %d results.\n", len(resp.Libraries), resp.Total)
			}
			return nil
		},
	}
}

func newLibraryInstallCmd() *cobra.Command {
	var profileOverride string

	cmd := &cobra.Command{
		Use:   "install <identifier>",
		Short: "Install a library into a profile (default: active profile)",
		Long: `Install a library from the registry into a profile.

  my library install kubernetes
  my library install owner/name
  my library install kubernetes --profile work

Without --profile, the library is added to your active profile (defaults to
"default"). The MY_PROFILE environment variable is also honored.

For git-backed sources, use 'my source add <git-url>' instead.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := args[0]

			if isGitURL(identifier) {
				return fmt.Errorf("git URLs are not supported here; use 'my source add %s' instead", identifier)
			}
			return installRegistryLibrary(identifier, profileOverride)
		},
	}

	cmd.Flags().StringVar(&profileOverride, "profile", "", "profile to install into (overrides the active profile)")

	return cmd
}

func isGitURL(s string) bool {
	return strings.HasPrefix(s, "https://") ||
		strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "git://") ||
		strings.HasPrefix(s, "ssh://") ||
		strings.HasPrefix(s, "file://") ||
		(strings.Contains(s, "@") && strings.Contains(s, ":"))
}

func installRegistryLibrary(identifier, profileOverride string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	c := client.New(resolveAPIURL(cfg))
	defer c.Close()

	// Parse owner/slug
	owner, slug := parseOwnerSlug(identifier)

	// Verify library exists
	detail, err := c.GetPublicLibrary(owner, slug)
	if err != nil {
		return fmt.Errorf("library %q not found: %w", identifier, err)
	}

	// Resolve target profile: --profile flag wins, else active profile.
	profile := profileOverride
	if profile == "" {
		profile = cfg.GetActiveProfile()
	}

	// Add to the target profile and sync.
	if auth.IsLoggedIn() {
		if err := c.AddLibraryToProfile(profile, identifier); err != nil {
			return fmt.Errorf("failed to add to profile %q: %w", profile, err)
		}
		fmt.Printf("Syncing profile %q...\n", profile)
		fetched, syncErr := cache.SyncProfile(c, profile, false)
		if syncErr != nil {
			fmt.Fprintf(os.Stderr, "warning: sync failed: %v\n", syncErr)
		} else if fetched > 0 {
			fmt.Printf("Synced %d command(s).\n", fetched)
		}
	}

	// Register in local registry
	reg, err := library.LoadRegistry()
	if err != nil {
		return err
	}

	displayName := slug

	// Check for duplicate
	if library.FindByName(reg, displayName) != nil {
		fmt.Printf("Library %q is already installed.\n", displayName)
		return nil
	}

	reg.Sources = append(reg.Sources, library.SourceEntry{
		Name:        displayName,
		Owner:       detail.Owner,
		Slug:        detail.Library.Slug,
		Kind:        "registry",
		AddedAt:     time.Now(),
		LastUpdated: time.Now(),
	})
	if err := library.SaveRegistry(reg); err != nil {
		return fmt.Errorf("save registry: %w", err)
	}

	fmt.Printf("Installed %s (%d commands).\n", displayName, len(detail.Commands))
	return nil
}

func parseOwnerSlug(identifier string) (string, string) {
	if strings.Contains(identifier, "/") {
		parts := strings.SplitN(identifier, "/", 2)
		return parts[0], parts[1]
	}
	// Flat name (e.g., "kubernetes") - use system owner
	return "system", identifier
}

func completeInstalledLibraries(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	reg, err := library.LoadRegistry()
	if err != nil || reg == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, entry := range reg.Sources {
		names = append(names, entry.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func newLibrarySyncCmd() *cobra.Command {
	var profileOverride string
	var syncAll bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Refresh the library catalog for a profile (default: active profile)",
		Long: `Downloads the latest library catalog so commands are available locally.

Profile resolution: --profile flag > MY_PROFILE env > active profile config > "default".

Examples:
  my library sync                       # sync the active profile
  my library sync --profile work        # sync 'work' explicitly
  my library sync --all                 # sync every profile you own
  MY_PROFILE=ci my library sync         # sync via env var`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !auth.IsLoggedIn() {
				return fmt.Errorf("not logged in (set MY_API_TOKEN or run 'my cli login')")
			}
			if syncAll && profileOverride != "" {
				return fmt.Errorf("--all and --profile are mutually exclusive")
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			if syncAll {
				return syncAllProfiles(c)
			}

			profile := profileOverride
			if profile == "" {
				profile = cfg.GetActiveProfile()
			}
			return syncOneProfile(c, profile)
		},
	}

	cmd.Flags().StringVar(&profileOverride, "profile", "", "profile to sync (overrides the active profile)")
	cmd.Flags().BoolVar(&syncAll, "all", false, "sync every profile you own")

	return cmd
}

func syncOneProfile(c *client.Client, profile string) error {
	fmt.Printf("Syncing profile %q...\n", profile)
	fetched, err := cache.SyncProfile(c, profile, true)
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}
	if fetched == 0 {
		fmt.Println("Already up to date.")
	} else {
		fmt.Printf("Synced %d command(s).\n", fetched)
	}
	return nil
}

func syncAllProfiles(c *client.Client) error {
	resp, err := c.ListProfiles()
	if err != nil {
		return fmt.Errorf("list profiles: %w", err)
	}
	if len(resp.Profiles) == 0 {
		fmt.Println("No profiles to sync.")
		return nil
	}

	var failed []string
	for _, p := range resp.Profiles {
		fmt.Printf("Syncing profile %q...\n", p.Slug)
		fetched, err := cache.SyncProfile(c, p.Slug, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
			failed = append(failed, p.Slug)
			continue
		}
		if fetched == 0 {
			fmt.Println("  Already up to date.")
		} else {
			fmt.Printf("  Synced %d command(s).\n", fetched)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("%d profile(s) failed to sync: %s", len(failed), strings.Join(failed, ", "))
	}
	return nil
}

func newLibraryUninstallCmd() *cobra.Command {
	var profileOverride string

	cmd := &cobra.Command{
		Use:               "uninstall <name>",
		Short:             "Uninstall a registry library from a profile (default: active profile)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeInstalledLibraries,
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			reg, err := library.LoadRegistry()
			if err != nil {
				return err
			}

			entry := library.FindByName(reg, name)
			if entry == nil {
				return fmt.Errorf("library %q not found", name)
			}

			if entry.Kind == "git" {
				return fmt.Errorf("%q is a git source; use 'my source remove %s' instead", name, name)
			}

			// Remove from the target profile on the server (best-effort).
			if entry.Kind == "registry" && auth.IsLoggedIn() {
				cfg, err := config.Load()
				if err == nil {
					c := client.New(resolveAPIURL(cfg))
					defer c.Close()
					profile := profileOverride
					if profile == "" {
						profile = cfg.GetActiveProfile()
					}
					_ = c.RemoveLibraryFromProfile(profile, entry.Owner, entry.Slug)
				}
			}

			library.Remove(reg, name)
			if err := library.SaveRegistry(reg); err != nil {
				return fmt.Errorf("save registry: %w", err)
			}

			fmt.Printf("Uninstalled library %q.\n", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&profileOverride, "profile", "", "profile to remove the library from (overrides the active profile)")

	return cmd
}

func newLibraryListCmd() *cobra.Command {
	var jsonOutput bool

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List installed libraries",
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := library.LoadRegistry()
			if err != nil {
				return err
			}

			// Merge API-installed libraries from the active profile's cached catalog
			cfg, _ := config.Load()
			var catalog *cache.CachedCatalog
			if cfg != nil {
				catalog, _ = cache.GetCatalog(cfg.GetActiveProfile())
			}
			if catalog != nil {
				seen := map[string]bool{}
				for _, entry := range reg.Sources {
					seen[entry.Slug] = true
				}
				for _, item := range catalog.Items {
					if item.Library == "" {
						continue
					}
					if seen[item.Library] {
						continue
					}
					seen[item.Library] = true
					reg.Sources = append(reg.Sources, library.SourceEntry{
						Name:        item.Library,
						Owner:       item.LibraryOwner,
						Slug:        item.Library,
						Kind:        "registry",
						LastUpdated: catalog.SyncedAt,
					})
				}
			}

			if len(reg.Sources) == 0 {
				fmt.Println("No libraries installed. Run 'my library install <name>' or 'my source add <git-url>' to add one.")
				return nil
			}

			if jsonOutput {
				data, _ := json.MarshalIndent(reg.Sources, "", "  ")
				fmt.Println(string(data))
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			_, _ = fmt.Fprintln(w, "NAME\tSOURCE\tUPDATED")
			for _, entry := range reg.Sources {
				updated := entry.LastUpdated.Format("2006-01-02 15:04")
				_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", entry.Name, entry.Kind, updated)
			}
			_ = w.Flush()
			return nil
		},
	}

	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output as JSON")
	return cmd
}

func newLibraryReleaseCmd() *cobra.Command {
	var pushFlag bool
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "release [tag]",
		Short: "Create a versioned release of all libraries in the manifest",
		Long: `Create a release from a git tag (e.g., v1.0.0). Reads the manifest and specs
at the tagged commit and publishes them to the registry as one atomic call.
All libraries in the manifest are released under the same tag. Requires login.

Without a tag argument, prompts interactively for a version bump.
With a tag that doesn't exist yet, validates everything first and only then
creates the tag at HEAD.
With a tag that already exists, publishes from that tag's contents.

The release is content-addressable: retrying the same command after any
partial failure (network blip, killed process, etc.) is safe — matching
content returns 200 idempotent, differing content returns a distinct error.

Use --push to push the tag to origin after releasing (reliably, even when the
tag was created in a prior attempt).
Use --dry-run to preview per-library outcomes without creating anything.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !auth.IsLoggedIn() {
				return fmt.Errorf("not logged in (run 'my cli login' first)")
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("get working directory: %w", err)
			}

			// 1. Resolve the tag. Do NOT create it here — validation runs first
			//    so a bad manifest or spec at HEAD never leaves an orphan tag.
			tag, tagExisted, err := resolveReleaseTag(cwd, args)
			if err != nil {
				return err
			}

			// 2. Working-tree check only matters when we're about to base a new
			//    tag on HEAD. For an existing tag, the release comes from the
			//    tag ref and the working tree is irrelevant.
			if !tagExisted {
				clean, err := library.IsWorkingTreeClean(cwd)
				if err != nil {
					return fmt.Errorf("check working tree: %w", err)
				}
				if !clean {
					return fmt.Errorf("working tree has uncommitted changes; commit or stash them first")
				}
			}

			// 3. Archive the commit that will be released (HEAD for new tags,
			//    the existing tag ref otherwise).
			ref := "HEAD"
			if tagExisted {
				ref = tag
			}
			tmpDir, err := os.MkdirTemp("", "my-release-*")
			if err != nil {
				return fmt.Errorf("create temp dir: %w", err)
			}
			defer func() { _ = os.RemoveAll(tmpDir) }()

			if err := library.ArchiveCommit(cwd, ref, tmpDir); err != nil {
				return fmt.Errorf("extract %s: %w", ref, err)
			}

			// 4. Load and validate the manifest.
			manifest, err := library.LoadManifest(tmpDir)
			if err != nil {
				return fmt.Errorf("no valid manifest at %s: %w", ref, err)
			}
			if len(manifest.Libraries) == 0 {
				return fmt.Errorf("manifest declares no libraries")
			}

			// 5. + 6. Discover specs strictly and build the bundled payload.
			bundledLibs, err := buildBundledLibraries(tmpDir, manifest)
			if err != nil {
				return err
			}

			// 7. Commit hash & git URL for the record.
			var commitHash string
			if tagExisted {
				commitHash, err = library.TagCommitHash(cwd, tag)
				if err != nil {
					return fmt.Errorf("get tag commit: %w", err)
				}
			} else {
				commitHash, err = headCommitHash(cwd)
				if err != nil {
					return fmt.Errorf("get HEAD commit: %w", err)
				}
			}
			var gitURL string
			if remote, err := getGitRemoteURL(cwd); err == nil {
				gitURL = remote
			}

			req := &client.CreateBundledReleaseRequest{
				Tag:        tag,
				CommitHash: commitHash,
				GitURL:     gitURL,
				Namespace:  manifest.Namespace,
				Libraries:  bundledLibs,
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			// 8. --dry-run: preview without creating a tag or POSTing.
			if dryRun {
				return runReleaseDryRun(c, req, manifest.Namespace, tag)
			}

			// 9. Create the tag NOW that everything client-side is validated.
			if !tagExisted {
				if err := library.CreateTag(cwd, tag); err != nil {
					return fmt.Errorf("create tag: %w", err)
				}
				fmt.Printf("Created tag %s\n", tag)
			}

			// 10. POST the bundle.
			resp, err := c.CreateBundledRelease(req)
			if err != nil {
				printReleaseRecoveryGuidance(tag, commitHash, err)
				return err
			}
			printReleaseSummary(resp)

			// 11. Push. Reliable even for tags that already existed locally.
			if pushFlag {
				return ensureTagPushed(cwd, tag)
			}
			if !tagExisted {
				fmt.Printf("Tag %s is local-only. Run 'my library release %s --push' to push it.\n", tag, tag)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&pushFlag, "push", false, "push the tag to origin after releasing (idempotent — safe to re-run)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview per-library outcomes without creating anything")

	return cmd
}

// resolveReleaseTag returns the tag to release plus whether it already exists.
// It never mutates the repo (tag creation happens later, only after everything
// has been validated).
func resolveReleaseTag(cwd string, args []string) (string, bool, error) {
	if len(args) == 1 {
		tag := args[0]
		if !tagPattern.MatchString(tag) {
			return "", false, fmt.Errorf("invalid tag format %q (must match vX.Y.Z)", tag)
		}
		return tag, library.TagExists(cwd, tag), nil
	}

	// Interactive: prompt for a semver bump.
	latest, err := library.LatestSemverTag(cwd)
	if err != nil {
		return "", false, fmt.Errorf("find latest tag: %w", err)
	}
	if latest == "" {
		latest = "v0.0.0"
		fmt.Println("No existing tags found. Starting from v0.0.0.")
	} else {
		fmt.Printf("Latest release: %s\n", latest)
	}

	patchBump, _ := library.BumpSemver(latest, "patch")
	minorBump, _ := library.BumpSemver(latest, "minor")
	majorBump, _ := library.BumpSemver(latest, "major")

	fmt.Println()
	fmt.Println("Bump to:")
	fmt.Printf("  1) %s (patch)\n", patchBump)
	fmt.Printf("  2) %s (minor)\n", minorBump)
	fmt.Printf("  3) %s (major)\n", majorBump)
	fmt.Println("  4) Custom")
	fmt.Println()

	var choice string
	fmt.Print("Choose [1-4]: ")
	if _, err := fmt.Scanln(&choice); err != nil {
		return "", false, fmt.Errorf("read input: %w", err)
	}

	var tag string
	switch strings.TrimSpace(choice) {
	case "1":
		tag = patchBump
	case "2":
		tag = minorBump
	case "3":
		tag = majorBump
	case "4":
		fmt.Print("Enter tag (vX.Y.Z): ")
		if _, err := fmt.Scanln(&tag); err != nil {
			return "", false, fmt.Errorf("read input: %w", err)
		}
		tag = strings.TrimSpace(tag)
		if !tagPattern.MatchString(tag) {
			return "", false, fmt.Errorf("invalid tag format %q (must match vX.Y.Z)", tag)
		}
	default:
		return "", false, fmt.Errorf("invalid choice %q", choice)
	}

	if library.TagExists(cwd, tag) {
		return "", false, fmt.Errorf("tag %s already exists; run 'my library release %s' to publish it", tag, tag)
	}
	return tag, false, nil
}

// buildBundledLibraries validates every spec in every library of the manifest
// and computes each library's canonical content hash. Any failure here — bad
// spec, empty library, missing directory — halts the release before any
// state-mutating step, so a broken input never creates an orphan tag.
func buildBundledLibraries(tmpDir string, manifest *library.Manifest) ([]client.BundledLibraryRelease, error) {
	// Sort library keys so the request order is deterministic; server hashing
	// is per-library so this is cosmetic, but stable output helps humans read
	// the release log.
	keys := make([]string, 0, len(manifest.Libraries))
	for k := range manifest.Libraries {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]client.BundledLibraryRelease, 0, len(keys))
	for _, libKey := range keys {
		libDef := manifest.Libraries[libKey]
		items, err := library.DiscoverSpecsStrict(tmpDir, libKey, libDef)
		if err != nil {
			return nil, fmt.Errorf("library %q: %w", libKey, err)
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("library %q at %s: no command specs found", libKey, libDef.Path)
		}

		commandBytes := make([]json.RawMessage, 0, len(items))
		hashEntries := make([]spec.SpecHashEntry, 0, len(items))
		for _, item := range items {
			raw, err := os.ReadFile(item.SpecPath)
			if err != nil {
				return nil, fmt.Errorf("library %q: read %s: %w", libKey, item.SpecPath, err)
			}
			canon, err := spec.CanonicalSpecBytes(raw)
			if err != nil {
				return nil, fmt.Errorf("library %q: canonicalize %s: %w", libKey, item.SpecPath, err)
			}
			commandBytes = append(commandBytes, canon)
			hashEntries = append(hashEntries, spec.SpecHashEntry{Slug: item.Slug, Bytes: canon})
		}

		contentHash := spec.LibraryReleaseHash(spec.LibraryReleaseHashInput{
			Slug:        libKey,
			Name:        libDef.Name,
			Description: libDef.Description,
			Aliases:     libDef.Aliases,
			Specs:       hashEntries,
		})

		out = append(out, client.BundledLibraryRelease{
			Slug:          libKey,
			Name:          libDef.Name,
			Description:   libDef.Description,
			Aliases:       libDef.Aliases,
			ContentSHA256: contentHash,
			Commands:      commandBytes,
		})
	}
	return out, nil
}

// runReleaseDryRun asks the API what would happen for each library at the
// requested version. It prints one line per library:
//
//	would-create     — no such release yet
//	would-idempotent — release exists with the same content_sha256
//	would-conflict   — release exists with different content_sha256
//	unknown          — API call failed and we can't tell
func runReleaseDryRun(c *client.Client, req *client.CreateBundledReleaseRequest, namespace, tag string) error {
	owner, err := dryRunOwner(c, namespace)
	if err != nil {
		return err
	}
	version := strings.TrimPrefix(tag, "v")

	fmt.Printf("[dry-run] tag %s (namespace: %s, owner: %s)\n", tag, valOrDefault(namespace, "(user)"), owner)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "LIBRARY\tCOMMANDS\tHASH\tSTATUS")
	for _, lib := range req.Libraries {
		status := "would-create"
		existing, err := c.GetLibraryRelease(owner, lib.Slug, version)
		if err != nil {
			status = fmt.Sprintf("unknown (%s)", err.Error())
		} else if existing != nil {
			switch {
			case existing.ContentSHA256 == nil:
				status = "would-conflict (legacy, no hash on record)"
			case *existing.ContentSHA256 == lib.ContentSHA256:
				status = "would-idempotent"
			default:
				status = "would-conflict"
			}
		}
		_, _ = fmt.Fprintf(w, "%s\t%d\t%s\t%s\n", lib.Slug, len(lib.Commands), shortHash(lib.ContentSHA256), status)
	}
	_ = w.Flush()
	fmt.Println("\n[dry-run] No tag created, nothing published.")
	return nil
}

// dryRunOwner picks the owner used to look up existing releases. "system"
// releases live under the system user; anything else is looked up under the
// authenticated user's username.
func dryRunOwner(c *client.Client, namespace string) (string, error) {
	if namespace == "system" {
		return "system", nil
	}
	me, err := c.GetMe()
	if err != nil {
		return "", fmt.Errorf("get current user: %w", err)
	}
	if me.Username == nil || *me.Username == "" {
		return "", fmt.Errorf("your account has no username set (run 'my cli login' to complete setup)")
	}
	return *me.Username, nil
}

func valOrDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

func shortHash(h string) string {
	if strings.HasPrefix(h, "sha256:") && len(h) >= 7+12 {
		return h[:7+12] + "…"
	}
	return h
}

func headCommitHash(cwd string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse HEAD: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// printReleaseSummary reports what happened per library after a successful
// bundled release.
func printReleaseSummary(resp *client.CreateBundledReleaseResponse) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "LIBRARY\tSTATUS\tCOMMANDS")
	for _, lib := range resp.Libraries {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%d\n", lib.Slug, lib.Status, lib.PublishedCount)
	}
	_ = w.Flush()
}

// printReleaseRecoveryGuidance tells the user how to retry after a network
// failure. Because the API is content-addressed, an identical retry is safe
// even if the previous call actually reached the server.
func printReleaseRecoveryGuidance(tag, commitHash string, err error) {
	if apiErr, ok := err.(*client.APIError); ok {
		// Deterministic API errors (validation, conflict, stale) aren't
		// retry-recoverable — no guidance would help.
		switch apiErr.Code {
		case "RELEASE_CONTENT_MISMATCH", "RELEASE_STALE", "HASH_MISMATCH",
			"INVALID_REQUEST", "INVALID_TAG", "INVALID_SLUG", "INVALID_SPEC",
			"EMPTY_LIBRARY", "FORBIDDEN":
			return
		}
	}
	fmt.Fprintf(os.Stderr, "\nRelease did not complete. The tag %s references commit %s locally.\n", tag, commitHash)
	fmt.Fprintf(os.Stderr, "Retry with:  my library release %s --push\n", tag)
	fmt.Fprintln(os.Stderr, "Releases are content-addressed, so an identical retry is safe.")
}

// ensureTagPushed pushes the tag to origin when it's missing there, is a no-op
// when the remote already has it pointing at the same commit, and refuses to
// overwrite when the remote points somewhere else. A network error on the
// ls-remote check falls back to attempting the push — better than leaving a
// successful release with the tag stuck locally.
func ensureTagPushed(cwd, tag string) error {
	localSHA, err := library.TagCommitHash(cwd, tag)
	if err != nil {
		return fmt.Errorf("get local tag commit: %w", err)
	}
	remoteSHA, exists, err := library.RemoteTagInfo(cwd, tag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not check remote tag (%v); attempting push anyway\n", err)
		return pushTag(cwd, tag)
	}
	if !exists {
		return pushTag(cwd, tag)
	}
	if !strings.EqualFold(remoteSHA, localSHA) {
		return fmt.Errorf("tag %s exists on origin at commit %s but local tag points to %s; refusing to overwrite (delete or move the tag manually if this is intentional)", tag, remoteSHA, localSHA)
	}
	fmt.Printf("Tag %s already on origin (commit %s).\n", tag, remoteSHA[:min(len(remoteSHA), 12)])
	return nil
}

func pushTag(cwd, tag string) error {
	fmt.Printf("Pushing tag %s to origin...\n", tag)
	if err := library.PushTag(cwd, tag); err != nil {
		return fmt.Errorf("push tag: %w", err)
	}
	fmt.Println("Done.")
	return nil
}

func getGitRemoteURL(dir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func newLibraryInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "info <identifier>",
		Short:             "Show details about a library",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeInstalledLibraries,
		RunE: func(cmd *cobra.Command, args []string) error {
			identifier := args[0]

			// Try local registry first
			reg, _ := library.LoadRegistry()
			if reg != nil {
				if entry := library.FindByName(reg, identifier); entry != nil {
					fmt.Printf("Name:    %s\n", entry.Name)
					fmt.Printf("Source:  %s\n", entry.Kind)
					if isSystemOwner(entry.Owner) {
						fmt.Printf("Status:  official\n")
					} else {
						fmt.Printf("Owner:   %s\n", entry.Owner)
					}
					if entry.GitURL != "" {
						fmt.Printf("Git URL: %s\n", entry.GitURL)
					}
					if entry.LastCommit != "" {
						fmt.Printf("Commit:  %s\n", entry.LastCommit)
					}
					fmt.Printf("Updated: %s\n", entry.LastUpdated.Format("2006-01-02 15:04"))

					if entry.Kind == "git" && entry.LocalPath != "" {
						// Show libraries and commands from manifest
						manifest, err := library.LoadManifest(entry.LocalPath)
						if err == nil {
							totalCmds := 0
							for libKey, libDef := range manifest.Libraries {
								items, _ := library.DiscoverSpecs(entry.LocalPath, libKey, libDef)
								totalCmds += len(items)
								fmt.Printf("\nLibrary %s (%d commands):\n", libKey, len(items))
								for _, item := range items {
									fmt.Printf("  %s — %s\n", item.Slug, item.Description)
								}
							}
							fmt.Printf("\nTotal: %d commands\n", totalCmds)
						}
					}
					return nil
				}
			}

			// Try API
			owner, slug := parseOwnerSlug(identifier)
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("library %q not found", identifier)
			}
			c := client.New(resolveAPIURL(cfg))
			defer c.Close()

			detail, err := c.GetPublicLibrary(owner, slug)
			if err != nil {
				return fmt.Errorf("library %q not found", identifier)
			}

			fmt.Printf("Name:        %s\n", detail.Library.Name)
			fmt.Printf("Slug:        %s\n", detail.Library.Slug)
			if isSystemOwner(detail.Owner) {
				fmt.Printf("Status:      official\n")
			} else {
				fmt.Printf("Owner:       %s\n", detail.Owner)
			}
			if detail.Library.Description != "" {
				fmt.Printf("Description: %s\n", detail.Library.Description)
			}
			fmt.Printf("Installs:    %d\n", detail.Library.InstallCount)
			if len(detail.Commands) > 0 {
				fmt.Printf("\nCommands (%d):\n", len(detail.Commands))
				for _, cmd := range detail.Commands {
					fmt.Printf("  %s — %s\n", cmd.Slug, cmd.Description)
				}
			}

			// Show releases
			releases, err := c.ListReleases(owner, slug)
			if err == nil && len(releases) > 0 {
				fmt.Printf("\nReleases:\n")
				for _, rel := range releases {
					fmt.Printf("  %s  %s  (%d commands)\n", rel.Tag, rel.ReleasedAt, rel.CommandCount)
				}
			}
			return nil
		},
	}
}
