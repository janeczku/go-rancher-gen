package main

import (
	"flag"
	"fmt"
	"os"

	log "github.com/Sirupsen/logrus"
)

var (
	// Must be set at build time
	Version string = "Undefined"
	GitSHA  string = "N/A"

	configFile      string
	metadataVersion string
	logLevel        string
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

	flag.StringVar(&configFile, "config", "", "Path to config file")
	flag.StringVar(&metadataVersion, "metadata-version", "latest", "Metadata service version")
	flag.IntVar(&interval, "interval", 60, "Metadata polling interval (secs)")
	flag.BoolVar(&includeInactive, "include-inactive", false, "Include inactive services/stopped containers")
	flag.BoolVar(&onetime, "onetime", false, "Generate file once and exit")
	flag.StringVar(&logLevel, "log-level", "info", "Level of logging (debug,info,warn,error)")
	flag.BoolVar(&showVersion, "version", false, "Show application version and exit")
	flag.StringVar(&notifyCmd, "notify-cmd", "", "Optional command to run after generating the file")
	flag.BoolVar(&notifyOutput, "notify-output", false, "Log the output of the notify command")
	flag.Usage = printUsage
	flag.Parse()
}

func printUsage() {
	fmt.Println(`Usage: rancher-template [options] source [dest]

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
		fmt.Printf("rancher-template %s (%s) \n", Version, GitSHA)
		os.Exit(0)
	}

	if flag.NArg() < 1 && len(configFile) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	log.Infof("Starting rancher-template %s (%s)", Version, GitSHA)

	conf, err := initConfig()
	if err != nil {
		log.Fatal(err.Error())
	}

	r, err := NewRunner(conf)
	if err != nil {
		log.Fatal(err.Error())
	}

	if err := r.Run(); err != nil {
		log.Fatal(err.Error())
	}
}
