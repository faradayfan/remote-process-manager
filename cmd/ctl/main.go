package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/faradayfan/remote-process-manager/internal/protocol"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	baseURL := getenvDefault("GAMESVC_URL", "http://127.0.0.1:8080")
	client := &http.Client{Timeout: 10 * time.Second}

	cmd := os.Args[1]
	args := os.Args[2:]

	switch cmd {
	case "agents":
		doGET(client, baseURL+"/agents")

	case "instances":
		if len(args) != 1 {
			fmt.Println("instances requires: <agentID>")
			os.Exit(2)
		}
		doGET(client, fmt.Sprintf("%s/agents/%s/instances", baseURL, args[0]))

	case "instance-create":
		if len(args) < 3 {
			fmt.Println("instance-create requires: <agentID> <name> <template> [key=value ...]")
			os.Exit(2)
		}

		agentID := args[0]
		name := args[1]
		template := args[2]
		params := parseKeyValues(args[3:])

		req := protocol.CreateInstanceRequest{
			Name:     name,
			Template: template,
			Enabled:  true,
			Params:   params,
		}

		doPOST(client, fmt.Sprintf("%s/agents/%s/instances/create", baseURL, agentID), req)

	case "instance-delete":
		if len(args) < 2 {
			fmt.Println("instance-delete requires: <agentID> <name> [--force] [--delete-data]")
			os.Exit(2)
		}

		agentID := args[0]
		name := args[1]
		force := hasFlag(args[2:], "--force")
		deleteData := hasFlag(args[2:], "--delete-data")

		req := protocol.DeleteInstanceRequest{
			Name:       name,
			Force:      force,
			DeleteData: deleteData,
		}

		doPOST(client, fmt.Sprintf("%s/agents/%s/instances/delete", baseURL, agentID), req)

	case "start":
		if len(args) != 2 {
			fmt.Println("start requires: <agentID> <instance>")
			os.Exit(2)
		}
		agentID := args[0]
		instance := args[1]
		doPOST(client, fmt.Sprintf("%s/agents/%s/servers/%s/start", baseURL, agentID, instance), nil)

	case "stop":
		if len(args) != 2 {
			fmt.Println("stop requires: <agentID> <instance>")
			os.Exit(2)
		}
		agentID := args[0]
		instance := args[1]
		doPOST(client, fmt.Sprintf("%s/agents/%s/servers/%s/stop", baseURL, agentID, instance), nil)

	case "status":
		if len(args) != 2 {
			fmt.Println("status requires: <agentID> <instance>")
			os.Exit(2)
		}
		agentID := args[0]
		instance := args[1]
		doGET(client, fmt.Sprintf("%s/agents/%s/servers/%s/status", baseURL, agentID, instance))

	default:
		fmt.Printf("unknown command: %s\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Println(strings.TrimSpace(`
Usage:
  gamesvcctl agents
  gamesvcctl instances <agentID>

  gamesvcctl instance-create <agentID> <name> <template> [key=value ...]
  gamesvcctl instance-delete <agentID> <name> [--force] [--delete-data]

  gamesvcctl start  <agentID> <instance>
  gamesvcctl stop   <agentID> <instance>
  gamesvcctl status <agentID> <instance>

Environment:
  GAMESVC_URL=http://127.0.0.1:8080
`))
}

func getenvDefault(k, def string) string {
	v := os.Getenv(k)
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func doGET(client *http.Client, url string) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		fatal(err)
	}

	res, err := client.Do(req)
	if err != nil {
		fatal(err)
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)
	fmt.Printf("%s\n", prettyJSON(body))
	if res.StatusCode >= 400 {
		os.Exit(1)
	}
}

func doPOST(client *http.Client, url string, payload any) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			fatal(err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		fatal(err)
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(res.Body)
	fmt.Printf("%s\n", prettyJSON(respBody))
	if res.StatusCode >= 400 {
		os.Exit(1)
	}
}

func prettyJSON(b []byte) string {
	var v any
	if err := json.Unmarshal(b, &v); err != nil {
		return string(b)
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(b)
	}
	return string(out)
}

func parseKeyValues(args []string) map[string]string {
	out := map[string]string{}
	for _, a := range args {
		parts := strings.SplitN(a, "=", 2)
		if len(parts) != 2 {
			continue
		}
		k := strings.TrimSpace(parts[0])
		v := strings.TrimSpace(parts[1])
		if k != "" {
			out[k] = v
		}
	}
	return out
}

func hasFlag(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func fatal(err error) {
	fmt.Printf("error: %v\n", err)
	os.Exit(1)
}
