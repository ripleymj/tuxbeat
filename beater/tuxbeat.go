// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package beater

import (
	"bufio"
	"fmt"
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

		for _, domain := range bt.config.Domains {
			env := os.Environ()
			env = append(env, fmt.Sprintf("TUXCONFIG=%s", domain))

			tuxCmd := exec.Command(bt.config.TMAdmin, "-r")
			tuxCmd.Env = env

			tuxIn, _ := tuxCmd.StdinPipe()
			tuxOut, _ := tuxCmd.StdoutPipe()
			scanner := bufio.NewScanner(tuxOut)
			scanner.Split(bufio.ScanLines)

			tuxCmd.Start()

			tuxIn.Write([]byte("verbose on\n"))
			tuxIn.Write([]byte("page off\n"))

			if bt.config.PrintServer {
				tuxIn.Write([]byte("psr\n"))
			}
			if bt.config.PrintService {
				tuxIn.Write([]byte("psc\n"))
			}
			if bt.config.PrintQueue {
				tuxIn.Write([]byte("pq\n"))
			}
			if bt.config.PrintClient {
				tuxIn.Write([]byte("pclt\n"))
			}
			tuxIn.Write([]byte("quit\n"))

			message := ""
			for scanner.Scan() {
				temp := scanner.Text()
				line := strings.TrimLeft(temp, " >")
				if len(line) == 0 {
					HandleMsg(message, bt, domain)
					message = ""
				} else {
					message += line + "\n"
				}
			}

			HandleMsg(message, bt, domain)

			tuxIn.Close()
			tuxCmd.Wait()
		}
	}
}

func (bt *Tuxbeat) Stop() {
	bt.client.Close()
	close(bt.done)
}

func HandleMsg(message string, bt *Tuxbeat, tuxconfig string) {
	event := beat.Event{
		Timestamp: time.Now(),
		Fields: common.MapStr{
			"type": "tuxbeat",
		},
	}

	msgMap := make(map[string]string)
	for _, line := range strings.Split(message, "\n") {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			parts[0] = strings.Trim(parts[0], " \r")
			parts[1] = strings.Trim(parts[1], " \r")
			msgMap[parts[0]] = parts[1]
		}
	}
	if strings.Index(message, "Group ID:") == 0 {
		event.Fields.Put("msgtype", "printserver")
		HandleServerMsg(msgMap)
	} else if strings.Index(message, "Service Name:") == 0 {
		HandleServiceMsg(msgMap)
		event.Fields.Put("msgtype", "printservice")
	} else if strings.Index(message, "Prog Name:") == 0 {
		event.Fields.Put("msgtype", "printqueue")
	} else if strings.Index(message, "LMID:") == 0 {
		event.Fields.Put("msgtype", "printclient")
	} else {
		return
	}

	logp.Debug("Message", message)

	event.Fields.Put("tuxconfig", tuxconfig)
	event.Fields.Put("message", message)

	for key, value := range msgMap {
		event.Fields.Put(key, value)
	}
	bt.client.Publish(event)
}

func HandleServerMsg(msgMap map[string]string) {
	// Any specific server msg handling
}

func HandleServiceMsg(msgMap map[string]string) {
	//	string procID := msgMap[
}
