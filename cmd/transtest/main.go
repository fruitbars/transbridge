package main

import (
	"fmt"
	"os"
)

const usage = `transtest - TransBridge API testing CLI

USAGE:
  transtest smoke   [flags]   # Smoke test all API endpoints
  transtest cache   [flags]   # Verify cache behavior
  transtest bench   [flags]   # Concurrent load test
  transtest quality [flags]   # Golden set regression test

FLAGS:
  Common:
    --base URL          API base URL (default: http://localhost:8080)
    --token TOKEN       Auth token (default: uses config token)
    --timeout SECS      Request timeout (default: 60)
    --verbose           Enable verbose output
    --json              Output results as JSON

  bench specific:
    -c, --concurrency N     Number of concurrent workers (default: 10)
    -d, --duration DURATION Run duration, e.g. 30s, 5m (mutually exclusive with -n)
    -n, --requests N        Total requests to send (mutually exclusive with -d)
    --rps RATE              Target requests per second (optional rate limiting)

  quality specific:
    --golden FILE           Golden test cases YAML file (default: testdata/golden.yml)
    --update                Update golden file with actual results
    --diff                  Show diff for failed cases

EXAMPLES:
  transtest smoke --base http://localhost:8080 --token your-token
  transtest bench --base http://localhost:8080 --token your-token -c 50 -d 30s
  transtest quality --golden testdata/golden.yml --update
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "smoke":
		runSmoke(os.Args[2:])
	case "cache":
		runCache(os.Args[2:])
	case "bench":
		runBench(os.Args[2:])
	case "quality":
		runQuality(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n%s", command, usage)
		os.Exit(1)
	}
}
