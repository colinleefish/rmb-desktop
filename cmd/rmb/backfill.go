package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strconv"

	"github.com/colinleefish/rmb-desktop/internal/client"
)

// backfillCmd runs the one-time provenance backfill on the daemon (issue #31).
//
//	rmb backfill-provenance [--url=http://…] [--dry-run] [--threshold=0.9] [--max-scenes=5] [--categories=events]
func backfillCmd(args []string) int {
	fs := flag.NewFlagSet("backfill-provenance", flag.ExitOnError)
	dryRun := fs.Bool("dry-run", false, "report only; do not write")
	threshold := fs.Float64("threshold", 0.90, "min cosine similarity to link a scene")
	maxScenes := fs.Int("max-scenes", 5, "max provenance scenes per memory")
	categories := fs.String("categories", "", "comma list of memory categories to backfill (default all)")
	baseURL := fs.String("url", "", "rmbd base URL (default from config)")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "backfill-provenance: %v\n", err)
		return 2
	}

	var cl *client.Client
	if *baseURL != "" {
		cl = client.New(*baseURL)
	} else {
		sc, err := apiClient()
		if err != nil {
			fmt.Fprintf(os.Stderr, "backfill-provenance: %v\n", err)
			return 1
		}
		cl = sc
	}

	q := url.Values{}
	q.Set("threshold", strconv.FormatFloat(*threshold, 'f', -1, 64))
	q.Set("max-scenes", strconv.Itoa(*maxScenes))
	q.Set("dry-run", strconv.FormatBool(*dryRun))
	if *categories != "" {
		q.Set("categories", *categories)
	}

	body, err := cl.BackfillProvenance(context.Background(), q)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backfill-provenance: %v\n", err)
		return 1
	}
	fmt.Println(body)
	return 0
}
