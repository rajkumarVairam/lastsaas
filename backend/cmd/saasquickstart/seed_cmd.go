package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"saasquickstart/internal/auth"
	"saasquickstart/internal/seed"
)

func cmdSeed() {
	fs := flag.NewFlagSet("seed", flag.ExitOnError)
	reset := fs.Bool("reset", false, "Wipe existing seed data before seeding")
	output := fs.String("output", "", "Path for seed-manifest.json (default: project root)")
	fs.Parse(os.Args[2:])

	database, cfg, cleanup := connectDB()
	defer cleanup()

	jwtSvc := auth.NewJWTService(
		cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret,
		cfg.JWT.AccessTTLMin, cfg.JWT.RefreshTTLDay,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if *reset {
		fmt.Println("Resetting seed data...")
		if err := seed.Reset(ctx, database); err != nil {
			fmt.Fprintf(os.Stderr, "Reset failed: %v\n", err)
			os.Exit(1)
		}
	}

	// Default manifest path: two levels up from the binary (project root)
	manifestPath := *output
	if manifestPath == "" {
		exe, err := os.Executable()
		if err != nil {
			manifestPath = "seed-manifest.json"
		} else {
			// Walk up to find the project root (contains go.mod)
			dir := filepath.Dir(exe)
			for i := 0; i < 5; i++ {
				if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
					break
				}
				dir = filepath.Dir(dir)
			}
			manifestPath = filepath.Join(filepath.Dir(dir), "seed-manifest.json")
		}
		// Fallback: current directory
		if _, err := os.Stat(filepath.Dir(manifestPath)); err != nil {
			manifestPath = "seed-manifest.json"
		}
	}

	fmt.Println("Seeding scenarios...")
	fmt.Println()

	if err := seed.Run(ctx, database, manifestPath, jwtSvc); err != nil {
		fmt.Fprintf(os.Stderr, "\nSeed failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Printf("Manifest written to: %s\n", manifestPath)
	fmt.Println()
	fmt.Println("Seeded accounts (all use password: " + seed.SeedPassword + "):")
	fmt.Println()
	fmt.Printf("  %-22s  %s\n", "KEY", "EMAIL")
	fmt.Printf("  %-22s  %s\n", "---", "-----")
	accounts := []string{
		"rootAdmin", "freeOwner", "trialOwner", "activeOwner", "annualOwner",
		"lifetimeOwner", "pastDueOwner", "canceledOwner", "enterpriseOwner",
		"teamOwner", "teamAdmin", "teamMember",
		"aiFullOwner", "aiLowOwner", "aiEmptyOwner", "apiKeyOwner",
	}
	emails := map[string]string{
		"rootAdmin":       "root-admin@seed.local",
		"freeOwner":       "free@seed.local",
		"trialOwner":      "trial@seed.local",
		"activeOwner":     "active@seed.local",
		"annualOwner":     "annual@seed.local",
		"lifetimeOwner":   "lifetime@seed.local",
		"pastDueOwner":    "pastdue@seed.local",
		"canceledOwner":   "canceled@seed.local",
		"enterpriseOwner": "enterprise@seed.local",
		"teamOwner":       "team-owner@seed.local",
		"teamAdmin":       "team-admin@seed.local",
		"teamMember":      "team-member@seed.local",
		"aiFullOwner":     "ai-full@seed.local",
		"aiLowOwner":      "ai-low@seed.local",
		"aiEmptyOwner":    "ai-empty@seed.local",
		"apiKeyOwner":     "apikey-owner@seed.local",
	}
	for _, k := range accounts {
		fmt.Printf("  %-22s  %s\n", k, emails[k])
	}
	fmt.Println()
	fmt.Println("Run `saasquickstart seed --reset` to wipe and re-seed.")
}
