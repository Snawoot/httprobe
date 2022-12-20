package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	ProgName = "httprobe"
)

var (
	timeout   = flag.Duration("timeout", 5*time.Second, "operation timeout")
	method    = flag.String("method", "GET", "HTTP method")
	proxyURL  = flag.String("proxy", "", "proxy URL")
	targetURL = ""
)

func usage() {
	out := flag.CommandLine.Output()
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out)
	fmt.Fprintf(out, "%s <URL>\n", ProgName)
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Options:")
	flag.PrintDefaults()
}

func run() int {
	flag.Parse()
	if len(flag.Args()) != 1 {
		usage()
		return 2
	}
	targetURL = flag.Arg(0)
	err := makeRequest()
	if err != nil {
		log.Printf("can't make request: %v", err)
		return 1
	}
	return 0
}

func makeRequest() error {
	ctx, cl := context.WithTimeout(context.Background(), *timeout)
	defer cl()

	req, err := http.NewRequestWithContext(ctx, *method, targetURL, nil)
	if err != nil {
		return fmt.Errorf("can't construct HTTP request: %w", err)
	}

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	if *proxyURL != "" {
		log.Printf("using proxy override: %q", *proxyURL)
		parsedProxyURL, err := url.Parse(*proxyURL)
		if err != nil {
			return fmt.Errorf("unparseable proxy URL: %q", *proxyURL)
		}
		transport.Proxy = http.ProxyURL(parsedProxyURL)
	}

	httpClient := &http.Client{
		Transport: transport,
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	fmt.Printf("%s %s\n", resp.Proto, resp.Status)
	resp.Header.WriteSubset(os.Stdout, nil)
	fmt.Println()
	io.Copy(os.Stdout, resp.Body)
	fmt.Println()
	return nil
}

func main() {
	log.Default().SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
	log.Default().SetPrefix(strings.ToUpper(ProgName) + ": ")
	os.Exit(run())
}
