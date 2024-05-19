package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"

	"github.com/3rd/cloudwork/pkg/config"
	"github.com/3rd/cloudwork/pkg/ssh"
)

var interrupted bool

func main() {
	configPath := flag.String("config", "cloudwork.yml", "Path to the cloudwork configuration file")
	host := flag.String("host", "", "Specific host to run setup/run on (optional)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: cloudwork [options] <command>")
		fmt.Println("Commands:")
		fmt.Println("  bootstrap                 Creates the input/output directory structure for the configured workers")
		fmt.Println("  setup                     Runs the setup script on each worker")
		fmt.Println("  run                       Uploads inputs, executes the run script on all workers, and downloads outputs")
		fmt.Println("  download <remote path>    Downloads outputs from all workers (download-output <remote path>)")
		fmt.Println("  upload <remote path>      Uploads inputs to all workers (upload-input <remote path>)")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	cfg := config.Load(*configPath)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		fmt.Println("\nReceived interrupt signal. Terminating SSH processes...")
		interrupted = true
		ssh.TerminateAll()
		<-sigCh
		fmt.Println("Received second interrupt signal. Exiting the application...")
		os.Exit(1)
	}()

	switch flag.Arg(0) {
	case "bootstrap":
		bootstrap(cfg)
	case "setup":
		setup(cfg, *host)
	case "run":
		run(cfg, *host)
	case "download":
		if flag.NArg() < 2 {
			log.Fatal("Missing remote source path for download-output")
		}
		download(cfg, *host, flag.Arg(1))
	case "upload":
		if flag.NArg() < 2 {
			log.Fatal("Missing remote destination path for upload-input")
		}
		upload(cfg, *host, flag.Arg(1))
	default:
		log.Fatalf("Unknown command: %s", flag.Arg(0))
	}
}

func bootstrap(cfg config.Config) {
	fmt.Println("Bootstrapping worker directories...")
	for _, worker := range cfg.Workers {
		workerDir := filepath.Join("workers", worker.Host)
		if err := os.MkdirAll(workerDir, 0755); err != nil {
			log.Fatal(err)
		}
		inputDir := filepath.Join(workerDir, "input")
		if err := os.MkdirAll(inputDir, 0755); err != nil {
			log.Fatal(err)
		}
		outputDir := filepath.Join(workerDir, "output")
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Created directories for worker: %s\n", worker.Host)
	}
	fmt.Println("Bootstrap complete.")
}

func runScript(cfg config.Config, host string, script string, silent bool) {
	var wg sync.WaitGroup
	for _, worker := range cfg.Workers {
		if host != "" && worker.Host != host {
			continue
		}
		wg.Add(1)
		go func(worker config.Worker) {
			defer wg.Done()

			if !silent {
				log.Printf("Running script on worker: %s", worker.Host)
			}
			if err := ssh.Run(worker.Host, script, silent); err != nil {
				if interrupted {
					log.Printf("Worker %s interrupted", worker.Host)
				} else {
					log.Fatalf("Failed on worker %s: %v", worker.Host, err)
				}
			}
			if !silent && !interrupted {
				log.Printf("Script completed on worker: %s", worker.Host)
			}
		}(worker)
	}
	wg.Wait()
}

func setup(cfg config.Config, host string) {
	runScript(cfg, host, cfg.Setup, false)
	log.Println("Setup complete on all workers.")
}

func run(cfg config.Config, host string) {
	runScript(cfg, host, cfg.Run, false)
	fmt.Println("Run complete on all workers.")
}

func download(cfg config.Config, host, remoteSrc string) {
	runScript(cfg, host, fmt.Sprintf("download-output %s", remoteSrc), true)
	log.Println("Download complete on all workers.")
}

func upload(cfg config.Config, host, remoteDest string) {
	runScript(cfg, host, fmt.Sprintf("upload-input %s", remoteDest), true)
	log.Println("Upload complete on all workers.")
}
