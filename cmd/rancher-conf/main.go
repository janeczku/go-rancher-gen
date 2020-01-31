package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

var (
	// Must be set at build time
	Version string = "UNDEFINED"
	GitSHA  string = "UNDEFINED"

	configFile      string
	metadataUrl     string
	metadataVersion string
	logLevel        string
	checkCmd        string
	pollCmd         string
	notifyCmd       string
	onetime         bool
	showVersion     bool
	notifyOutput    bool
	includeInactive bool
	interval        int
)

func init() {
	log.SetFormatter(&log.TextFormatter{DisableTimestamp: true})
	log.SetOutput(os.Stdout)

	flag.StringVar(&configFile, "config", "", "Path to optional config file")
	flag.StringVar(&metadataUrl, "metadata-url", "http://rancher-metadata", "Metadata endpoint to use for querying the Metadata API")
	flag.StringVar(&metadataVersion, "metadata-version", "latest", "Metadata version to use for querying the Metadata API")
	flag.IntVar(&interval, "interval", 60, "Interval (in seconds) for polling the Metadata API for changes")
	flag.BoolVar(&includeInactive, "include-inactive", false, "Not yet implemented")
	flag.BoolVar(&onetime, "onetime", false, "Process all templates once and exit")
	flag.StringVar(&logLevel, "log-level", "info", "Verbosity of log output (debug,info,warn,error)")
	flag.StringVar(&checkCmd, "check-cmd", "", "Command to check the content before updating the destination file.")
	flag.StringVar(&pollCmd, "poll-cmd", "", "Command to run after each polling interval.")
	flag.StringVar(&notifyCmd, "notify-cmd", "", "Command to run after the destination file has been updated.")
	flag.BoolVar(&notifyOutput, "notify-output", false, "Print the result of the notify command to STDOUT")
	flag.BoolVar(&showVersion, "version", false, "Show application version and exit")
	flag.Usage = printUsage
	flag.Parse()
}

func printUsage() {
	fmt.Println(`Usage: rancher-conf [options] source [destination]

Options:`)
	flag.VisitAll(func(fg *flag.Flag) {
		fmt.Printf("\t--%s=%s\n\t\t%s\n", fg.Name, fg.DefValue, fg.Usage)
	})
	fmt.Println(`
Arguments:
	source - Path to the template file
	dest - Path to the output file. If ommited result is printed to STDOUT.`)
}

func main() {
	if showVersion {
		fmt.Printf("rancher-conf version %s (%s) \n", Version, GitSHA)
		os.Exit(0)
	}

	if flag.NArg() < 1 && len(configFile) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	log.Infof("Starting rancher-conf %s (%s)", Version, GitSHA)

	conf, err := initConfig(configFile)
	if err != nil {
		log.Fatal(err.Error())
	}

	r, err := NewRunner(conf)
	if err != nil {
		log.Fatal(err.Error())
	}

	if err := r.Run(); err != nil {
		log.Fatal(err)
	}
}
