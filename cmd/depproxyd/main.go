package main

import (
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"src.agwa.name/go-listener"
	_ "src.agwa.name/go-listener/tls"

	"src.agwa.name/depproxy"
)

func usageError(message string) {
	fmt.Fprintln(os.Stderr, message)
	flag.Usage()
	os.Exit(2)
}

func simplifyError(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}

	return err
}

func readAllowedModulesFile(filename string) ([]depproxy.AllowedModule, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, simplifyError(err)
	}
	defer file.Close()
	return depproxy.ReadAllowedModules(file)
}

func main() {
	var flags struct {
		allowlist string
		listen    []string
		upstream  string
	}
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(flag.CommandLine.Output(), "The allowlist file should contain lines consisting of: module pattern, whitespace, version pattern\n")
		fmt.Fprintf(flag.CommandLine.Output(), "For go-listener syntax, see https://pkg.go.dev/src.agwa.name/go-listener#section-readme\n")
	}
	flag.StringVar(&flags.allowlist, "allowlist", "", "Path to allowed modules list")
	flag.Func("listen", "Socket to listen on, in go-listener syntax (repeatable)", func(arg string) error {
		flags.listen = append(flags.listen, arg)
		return nil
	})
	flag.StringVar(&flags.upstream, "upstream", "https://proxy.golang.org", "URL of upstream module proxy")
	flag.Parse()

	if flags.allowlist == "" {
		usageError("-allowlist flag required")
	}
	if len(flags.listen) == 0 {
		usageError("At least one -listen flag required")
	}

	allowedModules, err := readAllowedModulesFile(flags.allowlist)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading allowlist file from %q: %s\n", flags.allowlist, err)
		os.Exit(1)
	}
	upstreamProxy, err := url.Parse(flags.upstream)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error parsing upstream proxy URL %q: %s\n", flags.upstream, err)
		os.Exit(1)
	}

	server := &depproxy.Server{
		AllowedModules: allowedModules,
		UpstreamProxy:  upstreamProxy,
	}

	httpServer := http.Server{
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  30 * time.Second,
		Handler:      server.HTTPHandler(),
	}

	listeners, err := listener.OpenAll(flags.listen)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.CloseAll(listeners)

	for _, l := range listeners {
		go func(l net.Listener) {
			log.Fatal(httpServer.Serve(l))
		}(l)
	}

	select {}
}
