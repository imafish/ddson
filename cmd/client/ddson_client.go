package main

import (
	"flag"
	"log"
	"net/url"
	"os"
	"path"

	"internal/version"
)

var (
	addr        = flag.String("addr", "localhost:5510", "the address to connect to")
	clientName  = flag.String("name", "", "the name of the client")
	downloadUrl = flag.String("url", "", "URL to download from")
	output      = flag.String("output", "", "output file name")
	servicePort = flag.Int("port", 5510, "the port to listen on")
)

func main() {
	flag.Parse()

	// TODO: include both mode in the same process
	if *downloadUrl != "" {
		// downloader mode
		if *output == "" {
			parsedURL, err := url.Parse(*downloadUrl)
			if err != nil {
				log.Fatalf("failed to parse URL: %v", err)
			}

			pathSegments := parsedURL.Path
			*output = path.Base(pathSegments)
			log.Printf("Extracted file name from URL: %s", *output)
		}

		log.Printf("Downloading from %s to %s", *downloadUrl, *output)
		download()

	} else {

		// client agent mode
		if *clientName == "" {
			hostname, err := os.Hostname()
			if err != nil {
				log.Fatalf("failed to get hostname: %v", err)
			}
			*clientName = hostname
		}

		log.Printf("starting agent mode as '%s' (version: %s)", *clientName, version.VersionString)
		log.Printf("server is: %s", *addr)

		runAgent()
	}
}
