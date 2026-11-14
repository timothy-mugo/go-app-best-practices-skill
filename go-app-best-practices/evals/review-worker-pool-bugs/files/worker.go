package checker

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Result struct {
	URL    string
	Status int
	Err    error
}

// CheckAll fans out one goroutine per URL and returns every result.
func CheckAll(urls []string) ([]Result, error) {
	var wg sync.WaitGroup
	var mu sync.Mutex
	var results []Result

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	var err error
	for _, u := range urls {
		go func() {
			wg.Add(1)
			defer wg.Done()

			res, err := check(ctx, u)
			if err != nil {
				fmt.Println("check failed:", err)
			}
			mu.Lock()
			results = append(results, res)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return results, err
}

func check(ctx context.Context, url string) (Result, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{URL: url, Err: err}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return Result{URL: url, Err: err}, err
	}
	return Result{URL: url, Status: resp.StatusCode}, nil
}
