package main

import (
	"io"
	"os/exec"
	"strings"
	"time"
)

func SshRunning(socksServer string) bool {
	c, err := dialTCP(socksServer)
	if err != nil {
		return false
	}
	defer c.Close()

	c.SetDeadline(time.Now().Add(effectiveDialTimeout()))
	defer c.SetDeadline(zeroTime)
	if _, err = c.Write(socksMsgVerMethodSelection); err != nil {
		return false
	}
	repBuf := make([]byte, 2)
	if _, err = io.ReadFull(c, repBuf); err != nil {
		return false
	}
	return repBuf[0] == 5 && repBuf[1] == 0
}

func runOneSSH(server string) {
	// config parsing canonicalize sshServer config value
	arr := strings.SplitN(server, ":", 3)
	sshServer, localPort, sshPort := arr[0], arr[1], arr[2]
	alreadyRunPrinted := false

	socksServer := "127.0.0.1:" + localPort
	for {
		if SshRunning(socksServer) {
			if !alreadyRunPrinted {
				debug.Println("ssh socks server", socksServer, "maybe already running")
				alreadyRunPrinted = true
			}
			time.Sleep(30 * time.Second)
			continue
		}

		// -n redirects stdin from /dev/null
		// -N do not execute remote command
		debug.Println("connecting to ssh server", sshServer+":"+sshPort)
		cmd := exec.Command("ssh", "-n", "-N", "-D", localPort, "-p", sshPort, sshServer)
		if err := cmd.Run(); err != nil {
			debug.Println("ssh:", err)
		}
		debug.Println("ssh", sshServer+":"+sshPort, "exited, reconnect")
		time.Sleep(5 * time.Second)
		alreadyRunPrinted = false
	}
}

func runSSH() {
	for _, server := range config.SshServer {
		go runOneSSH(server)
	}
}
