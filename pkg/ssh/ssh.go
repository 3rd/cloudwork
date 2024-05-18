package ssh

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

var processes sync.Map

func TerminateAll() {
	processes.Range(func(key, value interface{}) bool {
		host := key.(string)
		process := value.(*os.Process)
		if err := process.Signal(os.Interrupt); err != nil {
			log.Printf("Failed to send interrupt signal to process on host %s: %v", host, err)
		}
		return true
	})
}

func Upload(host, localPath, remotePath string) error {
	log.Printf("Uploading %s to %s:%s", localPath, host, remotePath)
	cmd := exec.Command("rsync", "-r", "--mkpath", localPath, host+":"+remotePath)
	return waitCommand(cmd, "local")
}

func Download(host, remotePath, localPath string) error {
	log.Printf("Downloading %s:%s to %s", host, remotePath, localPath)
	cmd := exec.Command("rsync", "-r", "--mkpath", host+":"+remotePath, localPath)
	return waitCommand(cmd, "local")
}

func Run(host, script string) error {

	// handle special commands
	processedScript := ""
	for _, line := range strings.Split(script, "\n") {

		if strings.HasPrefix(line, "upload ") {
			// upload
			parts := strings.Split(strings.TrimPrefix(line, "upload "), " ")
			if len(parts) == 2 {
				localPath := strings.TrimSpace(parts[0])
				remotePath := strings.TrimSpace(parts[1])
				if err := Upload(host, localPath, remotePath); err != nil {
					return err
				}
			}
		} else if strings.HasPrefix(line, "download ") {
			// deferred download
			downloadLine := line
			defer func() {
				parts := strings.Split(strings.TrimPrefix(downloadLine, "download "), " ")
				if len(parts) == 2 {
					remotePath := strings.TrimSpace(parts[0])
					localPath := strings.TrimSpace(parts[1])
					if err := Download(host, remotePath, localPath); err != nil {
						log.Printf("Failed to download %s to %s: %v", remotePath, localPath, err)
					}
				}
			}()
		} else {
			processedScript += line + "\n"
		}
	}

	tmpFile, err := os.CreateTemp("", "cloudwork_*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())
	_, err = tmpFile.WriteString(processedScript)
	if err != nil {
		return err
	}

	err = Upload(host, tmpFile.Name(), "/tmp/cloudwork-exec.sh")
	if err != nil {
		return err
	}

	cmd := exec.Command("ssh", host, "bash --login -c 'sh /tmp/cloudwork-exec.sh'")
	cmd.Stdin = os.NewFile(0, os.DevNull)

	return waitCommand(cmd, host)
}

func waitCommand(cmd *exec.Cmd, host string) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	ch := make(chan string, 100)

	stdoutScanner := bufio.NewScanner(stdout)
	wg.Add(1)
	go func() {
		for stdoutScanner.Scan() {
			ch <- strings.TrimSpace(stdoutScanner.Text())
		}
		wg.Done()
	}()

	stderrScanner := bufio.NewScanner(stderr)
	wg.Add(1)
	go func() {
		for stderrScanner.Scan() {
			text := fmt.Sprintf("[%s] %s", host, strings.TrimSpace(stderrScanner.Text()))
			ch <- strings.TrimSpace(text)
		}
		wg.Done()
	}()

	go func() {
		wg.Wait()
		close(ch)
	}()

	for text := range ch {
		formatted := fmt.Sprintf("[%s] %s", host, strings.TrimSpace(text))
		log.Printf("%s", formatted)
	}

	return cmd.Wait()
}
