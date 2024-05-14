package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/3rd/cloudwork/pkg/config"
	"github.com/3rd/cloudwork/pkg/ssh"
)

func main() {
	configPath := flag.String("config", "cloudwork.yml", "Path to the cloudwork configuration file")
	host := flag.String("host", "", "Specific host to run setup/run on (optional)")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Println("Usage: cloudwork [options] <command>")
		fmt.Println("Commands:")
		fmt.Println("  bootstrap  Creates the input/output directory structure for the configured workers")
		fmt.Println("  setup      Runs the setup script on each worker")
		fmt.Println("  run        Uploads inputs, executes the run script on all workers, and downloads outputs")
		fmt.Println("Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	cfg := config.Load(*configPath)

	// log host
	fmt.Printf("Running on host: %s\n", *host)

	switch flag.Arg(0) {
	case "bootstrap":
		bootstrap(cfg)
	case "setup":
		setup(cfg, *host)
	case "run":
		run(cfg, *host)
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

func setup(cfg config.Config, host string) {
	var wg sync.WaitGroup
	for _, worker := range cfg.Workers {
		if host != "" && worker.Host != host {
			continue
		}
		wg.Add(1)
		go func(worker config.Worker) {
			defer wg.Done()
			fmt.Printf("Running setup on worker: %s\n", worker.Host)
			if err := ssh.Run(worker.Host, cfg.Setup); err != nil {
				log.Fatalf("Setup failed on worker %s: %v", worker.Host, err)
			}
			fmt.Printf("Setup complete on worker: %s\n", worker.Host)
		}(worker)
	}
	wg.Wait()
	fmt.Println("Setup complete on all workers.")
}

func run(cfg config.Config, host string) {
	var wg sync.WaitGroup
	for _, worker := range cfg.Workers {
		if host != "" && worker.Host != host {
			continue
		}
		wg.Add(1)
		go func(worker config.Worker) {
			defer wg.Done()
			fmt.Printf("Running script on worker: %s\n", worker.Host)
			if err := ssh.Run(worker.Host, cfg.Run); err != nil {
				log.Fatalf("Run failed on worker %s: %v", worker.Host, err)
			}
			fmt.Printf("Run complete on worker: %s\n", worker.Host)
		}(worker)
	}
	wg.Wait()
	fmt.Println("Run complete on all workers.")
}
