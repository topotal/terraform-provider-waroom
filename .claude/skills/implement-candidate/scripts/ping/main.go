// Command ping checks that the Waroom API used by the acceptance tests is reachable
// and the API key is accepted, by calling a read-only endpoint (list users) once.
//
// It reads E2E_WAROOM_API_ENDPOINT and E2E_WAROOM_API_KEY, the same variables as `task test:acc`.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/topotal/waroom-client-go/waroom"
)

func main() {
	endpoint := os.Getenv("E2E_WAROOM_API_ENDPOINT")
	apiKey := os.Getenv("E2E_WAROOM_API_KEY")
	if endpoint == "" || apiKey == "" {
		fmt.Fprintln(os.Stderr, "NG: E2E_WAROOM_API_ENDPOINT and E2E_WAROOM_API_KEY must be set")
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, err := waroom.NewClient(ctx, waroom.WithBaseURL(endpoint), waroom.WithAPIKey(apiKey))
	if err != nil {
		fmt.Fprintf(os.Stderr, "NG: create client: %s\n", err)
		os.Exit(1)
	}

	users, err := client.Users.List(ctx, waroom.ListUsersParams{PerPage: waroom.NewOptInt(1)})
	if err != nil {
		var apiErr *waroom.APIError
		if errors.As(err, &apiErr) {
			fmt.Fprintf(os.Stderr, "NG: reached %s but the API returned an error (check the API key / plan): %s\n", endpoint, err)
		} else {
			fmt.Fprintf(os.Stderr, "NG: cannot reach %s (is Waroom running? is the endpoint correct?): %s\n", endpoint, err)
		}
		os.Exit(1)
	}

	fmt.Printf("OK: %s is reachable (list users returned %d item(s))\n", endpoint, len(users))
}
