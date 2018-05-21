package beater

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"

	"github.com/ripleymj/tuxbeat/config"
)

type Tuxbeat struct {
	done   chan struct{}
	config config.Config
	client beat.Client
}

var pidWorkStats map[string]int

// Creates beater
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &Tuxbeat{
		done:   make(chan struct{}),
		config: c,
	}
	return bt, nil
}

func (bt *Tuxbeat) Run(b *beat.Beat) error {
	logp.Info("tuxbeat is running! Hit CTRL-C to stop it.")

	var err error
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	pidWorkStats = make(map[string]int)

	ticker := time.NewTicker(bt.config.Period)
	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:
		}

		for _, domain := range bt.config.Domains {
			env := os.Environ()
			env = append(env, fmt.Sprintf("TUXCONFIG=%s", domain))

			tuxCmd := exec.Command(bt.config.TMAdmin, "-r")
			tuxCmd.Env = env

			tuxIn, _ := tuxCmd.StdinPipe()
			tuxOut, _ := tuxCmd.StdoutPipe()
			oBuf := bufio.NewReader(tuxOut)

			tuxCmd.Start()

			tuxIn.Write([]byte("verbose on\n"))
			tuxIn.Write([]byte("page off\n"))

			if bt.config.PrintServer {
				tuxIn.Write([]byte("psr\n"))
			}
			if bt.config.PrintQueue {
				tuxIn.Write([]byte("pq\n"))
			}
			if bt.config.PrintClient {
				tuxIn.Write([]byte("pclt\n"))
			}
			tuxIn.Write([]byte("quit\n"))

		tmadminRead:
			for {
				message := ""
				event := beat.Event{
					Timestamp: time.Now(),
					Fields: common.MapStr{
						"type": b.Info.Name,
					},
				}
			MessageRead:
				for {

					line, err := oBuf.ReadString('\n')
					line = strings.TrimLeft(line, " >")

					if err != nil {
						if err != io.EOF {
							fmt.Printf("Error: %s\n", err)
						}
						break tmadminRead
					} else if (line == "\n" || line == "\r\n") {
						break MessageRead
					}
					message += line
				}

				msgMap := make(map[string]string)
				if strings.Index(message, "Group ID:") == 0 {
					event.Fields.Put("msgtype", "printserver")
					msgMap = HandleServerMsg(message, (int)(bt.config.Period.Seconds()))
				} else if strings.Index(message, "Prog Name:") == 0 {
					event.Fields.Put("msgtype", "printqueue")
				} else if strings.Index(message, "LMID:") == 0 {
					event.Fields.Put("msgtype", "printclient")
				} else {
					continue tmadminRead
				}

				event.Fields.Put("tuxconfig", domain)
				event.Fields.Put("message", message)

				for key, value := range msgMap {
					event.Fields.Put(key, value)
				}
				bt.client.Publish(event)
			}
			tuxIn.Close()
			tuxCmd.Wait()
		}
	}
}

func (bt *Tuxbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}

func HandleServerMsg(message string, period int) map[string]string {
	msgMap := make(map[string]string)
	var pid string
	var req int
	for _, line := range strings.Split(message, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			parts[0] = strings.Trim(parts[0], " \r")
			parts[1] = strings.Trim(parts[1], " \r")
			msgMap[parts[0]] = parts[1]
			if parts[0] == "Process ID" {
				pid = strings.SplitN(parts[1], " ", 2)[0]
			} else if parts[0] == "Requests done" {
				req, _ = strconv.Atoi(parts[1])
			}
		}
	}

	var reqDone, reqPerSec float64
	_, ok := pidWorkStats[pid]
	if ok {
		reqDone = float64(req) - float64(pidWorkStats[pid])
		pidWorkStats[pid] = req
	} else {
		reqDone = 0
		pidWorkStats[pid] = req
	}
	reqPerSec = reqDone / float64(period)
	msgMap["reqPerSec"] = strconv.FormatFloat(reqPerSec, 'f', 2, 32)

	return msgMap
}
