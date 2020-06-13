package httpcache

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	TurnOnCommand         = "turn-on-diag"
	TurnOffCommand        = "turn-off-diag"
	DumpStatsCommand      = "dump-stats"
	ShowSummaryCommand    = "show-summary"
	ShowProxyStatsCommand = "show-proxy-stats"
	ShowCacheStatsCommand = "show-cache-stats"
	ShowBEStatsCommand    = "show-backend-stats"
	HelpCommand           = "help"

	DumpFile = "/tmp/dump-stats-httpCache.json"

	LineDelimiter = "========================================="
)

type (
	CommandCtxt struct {
		httpCacheCtxt *HttpCacheCtxt
		listener      net.Listener

		noOfConns uint32
	}
)

func NewCommandCtxt(httpCacheCtxt *HttpCacheCtxt) (cmdCtxt *CommandCtxt, err error) {

	cmdCtxt = &CommandCtxt{
		httpCacheCtxt: httpCacheCtxt,
	}
	return
}

func (cmdCtxt *CommandCtxt) startListening() (err error) {

	if err = os.RemoveAll(cmdCtxt.httpCacheCtxt.Config.Server.DiagHost); err != nil {
		return
	}

	if cmdCtxt.listener, err = net.Listen("unix",
		cmdCtxt.httpCacheCtxt.Config.Server.DiagHost); err != nil {

		return

	}

	return
}

func (cmdCtxt *CommandCtxt) switchDiagnose(status bool) (output string, err error) {

	if cmdCtxt.httpCacheCtxt.Config.Server.Diagnose != status {
		swLock := &sync.RWMutex{}
		swLock.Lock()
		defer swLock.Unlock()

		output = fmt.Sprintf("Switching Diagnose status %t", status)
		cmdCtxt.httpCacheCtxt.Config.Server.Diagnose = status
	}
	return
}

func (cmdCtxt *CommandCtxt) showHelpStats() (output string, err error) {

	output += "Following commands are available :\n\n"
	output += fmt.Sprintf("%s\n", ShowSummaryCommand)
	output += fmt.Sprintf("%s\n", ShowCacheStatsCommand)
	output += fmt.Sprintf("%s\n", ShowProxyStatsCommand)
	output += fmt.Sprintf("%s\n", ShowBEStatsCommand)
	output += fmt.Sprintf("%s\n", DumpStatsCommand)

	return
}

func (cmdCtxt *CommandCtxt) runCommand(cmd string) (output string, err error) {

	var (
		spCmd []string
	)

	spCmd = strings.Split(strings.TrimSpace(cmd), " ")

	if len(spCmd) == 0 {
		err = errors.New("No Input specified")

	} else {

		switch spCmd[0] {

		default:
			output, err = cmdCtxt.showHelpStats()

		}
	}

	if err != nil {
		output = err.Error()
	}

	output = fmt.Sprintf("\n\n\n%s\n%s\n%s\n\n\n", LineDelimiter, output, LineDelimiter)

	return
}

func (cmdCtxt *CommandCtxt) startSession(conn net.Conn) (err error) {

	cmdCtxt.switchDiagnose(true)

	for {

		var (
			buf       [1024]byte
			cmd       string
			output    string
			noOfBytes int
		)

		conn.Write([]byte(">> "))

		noOfBytes, err = conn.Read(buf[:])
		cmd = string(buf[:noOfBytes])

		log.Println("Received --> " + cmd)

		if output, err = cmdCtxt.runCommand(cmd); err != nil {
			output = "Error in running command --> " + cmd + "\nError --> " + err.Error() + "\n"
			log.Println(err)
		}

		if _, err = conn.Write([]byte(output)); err != nil {
			log.Println(err)
			return
		}

	}

	cmdCtxt.switchDiagnose(false)

	return
}

// Command format
// fetch_stats cp api stat
func (cmdCtxt *CommandCtxt) Process() (err error) {

	for {

		if err = cmdCtxt.startListening(); err == nil {
			log.Println(err)
			break
		}

		time.Sleep(time.Second * 10)
	}

	for {

		var (
			conn net.Conn
		)

		if conn, err = cmdCtxt.listener.Accept(); err != nil {
			log.Println(err)
			continue
		}

		go cmdCtxt.startSession(conn)
	}

	return
}
