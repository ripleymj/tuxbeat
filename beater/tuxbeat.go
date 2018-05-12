package beater

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
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

	ticker := time.NewTicker(bt.config.Period)
	for {
		select {
		case <-bt.done:
			return nil
		case <-ticker.C:
		}

		env := os.Environ()
		tuxConfig := "/home/psadm2/psft/pt/8.56/appserv/APPDOM/PSTUXCFG"
		env = append(env, fmt.Sprintf("TUXCONFIG=%s", tuxConfig))

		tuxCmd := exec.Command("tmadmin", "-r")
		tuxCmd.Env = env

		tuxIn, _ := tuxCmd.StdinPipe()
		tuxOut, _ := tuxCmd.StdoutPipe()
		oBuf := bufio.NewReader(tuxOut)

		tuxCmd.Start()

		tuxIn.Write([]byte("verbose on\n"))
		tuxIn.Write([]byte("page off\n"))

		tuxIn.Write([]byte("psr\n"))
		tuxIn.Write([]byte("pq\n"))
		tuxIn.Write([]byte("pclt\n"))
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
				} else if line == "\n" {
					break MessageRead
				}
				message += line
			}

			msgMap := make(map[string]string)
			if strings.Index(message, "Group ID:") == 0 {
				event.Fields.Put("msgtype", "printserver")
				msgMap = HandleServerMsg(message)
			} else if strings.Index(message, "Prog Name:") == 0 {
				event.Fields.Put("msgtype", "printqueue")
			} else if strings.Index(message, "LMID:") == 0 {
				event.Fields.Put("msgtype", "printclient")
			} else {
				continue tmadminRead
			}

			event.Fields.Put("tuxconfig", tuxConfig)
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

func (bt *Tuxbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}

func HandleServerMsg(message string) map[string]string {
	msgMap := make(map[string]string)
	for _, line := range strings.Split(message, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			parts[0] = strings.Trim(parts[0], " ")
			parts[1] = strings.Trim(parts[1], " ")
			msgMap[parts[0]] = parts[1]
		}
	}
	return msgMap
}
