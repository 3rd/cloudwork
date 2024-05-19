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

func Upload(host, localPath, remotePath string, silent bool) error {
	if !silent {
		log.Printf("Uploading %s to %s:%s", localPath, host, remotePath)
	}
	cmd := exec.Command("rsync", "-r", localPath, host+":"+remotePath)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
	// return waitCommand(cmd, "")
}

func Download(host, remotePath, localPath string, silent bool) error {
	log.Printf("Downloading %s:%s to %s", host, remotePath, localPath)
	cmd := exec.Command("rsync", "-r", host+":"+remotePath, localPath)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Wait()
	// return waitCommand(cmd, "")
}

func Run(host, script string, silent bool) error {

	// handle special commands
	processedScript := ""
	for _, line := range strings.Split(script, "\n") {

		// upload <local path> <remote path>
		if strings.HasPrefix(line, "upload ") {
			parts := strings.Split(strings.TrimPrefix(line, "upload "), " ")
			if len(parts) == 2 {
				localPath := strings.TrimSpace(parts[0])
				remotePath := strings.TrimSpace(parts[1])
				if err := Upload(host, localPath, remotePath, silent); err != nil {
					return err
				}
			}
		} else if strings.HasPrefix(line, "download ") {
			// download <remote path> <local path> (deferred)
			downloadLine := line
			defer func() {
				parts := strings.Split(strings.TrimPrefix(downloadLine, "download "), " ")
				if len(parts) == 2 {
					remotePath := strings.TrimSpace(parts[0])
					localPath := strings.TrimSpace(parts[1])
					if err := Download(host, remotePath, localPath, silent); err != nil {
						log.Printf("Failed to download %s to %s: %v", remotePath, localPath, err)
					}
				}
			}()
		} else if strings.HasPrefix(line, "upload-input") {
			// upload-input <remote path>
			remotePath := strings.TrimSpace(strings.TrimPrefix(line, "upload-input "))
			localPath := fmt.Sprintf("./workers/%s/input/", host)
			if err := Upload(host, localPath, remotePath, silent); err != nil {
				return err
			}
		} else if strings.HasPrefix(line, "download-output") {
			// download-output <remote path> (deferred)
			downloadLine := line
			defer func() {
				remotePath := strings.TrimSpace(strings.TrimPrefix(downloadLine, "download-output "))
				localPath := fmt.Sprintf("./workers/%s/output/", host)
				if err := Download(host, remotePath, localPath, silent); err != nil {
					log.Printf("Failed to download %s to %s: %v", remotePath, localPath, err)
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

	err = Upload(host, tmpFile.Name(), "/tmp/cloudwork-exec.sh", silent)
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
			text := fmt.Sprintf("[%s] %s", host, strings.TrimSpace(stdoutScanner.Text()))
			ch <- strings.TrimSpace(text)
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
		formatted := strings.TrimSpace(text)
		if host != "" {
			formatted = fmt.Sprintf("[%s] %s", host, strings.TrimSpace(text))
		}
		if formatted != "" {
			log.Printf("%s", formatted)
		}
	}

	return cmd.Wait()
}
