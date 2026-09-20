package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/jdevera/command-launcher/test/remoteconfig"
)

func main() {
	configurationFile := flag.String("config", "test/remote-fixture.json", "remote fixture configuration file")
	repositoryRoot := flag.String("repository-root", ".", "repository root used to inspect the Git origin")
	flag.Parse()

	config, err := remoteconfig.Load(*configurationFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	baseURL, err := remoteconfig.BaseURL(*repositoryRoot, config)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(baseURL)
}
