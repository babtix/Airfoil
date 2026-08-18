// Command airfoil runs the Airfoil pipeline: ingest, normalize, embed, cluster,
// score, summarize, write, publish.
//
// This main is thin by design. All logic lives in internal/.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "airfoil:", err)
		os.Exit(1)
	}
}
