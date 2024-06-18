package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
		fmt.Println("  bootstrap                Creates the input/output directory structure for the configured workers")
		fmt.Println("  run <script name|file>   Runs the specified script on all workers")
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
	case "run":
		script := "default"
		if flag.NArg() > 1 {
			script = flag.Arg(1)
		}
		if strings.HasPrefix(script, "./") || strings.HasPrefix(script, "/") {
			if _, err := os.Stat(script); err == nil {
				b, err := os.ReadFile(script)
				if err != nil {
					log.Fatalf("Failed to read script file: %v", err)
				}
				script = string(b)
				fmt.Printf("The script file '%s' will be executed on all workers. Are you sure you want to continue? (y/N): ", flag.Arg(1))
				reader := bufio.NewReader(os.Stdin)
				confirm, _ := reader.ReadString('\n')
				if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(confirm)), "y") {
					fmt.Println("Aborted.")
					os.Exit(0)
				}
			} else {
				log.Fatalf("Script file not found: %s", flag.Arg(1))
			}
		} else if scriptContent, ok := cfg.Scripts[script]; ok {
			script = scriptContent
		} else {
			log.Fatalf("Unknown script: %s", script)
		}
		run(cfg, *host, script)
	case "exec":
		if flag.NArg() < 2 {
			log.Fatal("Missing script argument for 'exec' command")
		}
		script := strings.Join(flag.Args()[1:], " ")
		run(cfg, *host, script)
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
					log.Printf("Failed on worker %s: %v", worker.Host, err)
				}
			}
			if !silent && !interrupted {
				log.Printf("Script completed on worker: %s", worker.Host)
			}
		}(worker)
	}
	wg.Wait()
}

func run(cfg config.Config, host, script string) {
	runScript(cfg, host, script, false)
	fmt.Println("Run complete on all workers.")
}
