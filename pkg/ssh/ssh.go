package ssh

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
)

func Upload(host, localPath, remotePath string) error {
	fmt.Printf("Uploading %s to %s:%s\n", localPath, host, remotePath)
	cmd := exec.Command("rsync", "-r", "--mkpath", localPath, host+":"+remotePath)
	return runCommand(cmd)
}

func Download(host, remotePath, localPath string) error {
	fmt.Printf("Downloading %s:%s to %s\n", host, remotePath, localPath)
	cmd := exec.Command("rsync", "-r", "--mkpath", host+":"+remotePath, localPath)
	return runCommand(cmd)
}

func Run(host, script string) error {
	tmpFile, err := os.CreateTemp("", "cloudwork_*")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

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

	_, err = tmpFile.WriteString(processedScript)
	if err != nil {
		return err
	}

	err = Upload(host, tmpFile.Name(), "/tmp/cloudwork-exec.sh")
	if err != nil {
		return err
	}

	cmd := exec.Command("ssh", host, "-t", "bash --login -c 'sh /tmp/cloudwork-exec.sh'")
	cmd.Stdin = os.NewFile(0, os.DevNull)
	return runCommand(cmd)
}

func runCommand(cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	go func() {
		io.Copy(os.Stdout, stdout)
		stdout.Close()
	}()
	go func() {
		io.Copy(os.Stderr, stderr)
		stderr.Close()
	}()
	if err := cmd.Start(); err != nil {
		return err
	}

	return cmd.Wait()
}
