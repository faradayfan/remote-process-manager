package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/faradayfan/remote-process-manager/internal/protocol"
)

func main() {
	if err := run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	if len(argv) < 2 {
		usage()
		return errors.New("missing command")
	}

	baseURL := getenvDefault("GAMESVC_URL", "http://127.0.0.1:8080")
	apiPrefix := "/v1" // ✅ enforce consistent API prefix

	client := &http.Client{Timeout: 10 * time.Second}
	api := NewAPI(client, baseURL, apiPrefix)

	cmd := argv[1]
	args := argv[2:]

	switch cmd {
	case "agents":
		return api.PrintGET("/agents")

	case "instances":
		if len(args) != 1 {
			return fmt.Errorf("instances requires: <agentID>")
		}
		return api.PrintGET(fmt.Sprintf("/agents/%s/instances", url.PathEscape(args[0])))

	case "instance-create":
		if len(args) < 3 {
			return fmt.Errorf("instance-create requires: <agentID> <name> <template> [key=value ...]")
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

		return api.PrintPOST(fmt.Sprintf("/agents/%s/instances/create", url.PathEscape(agentID)), req)

	case "instance-delete":
		if len(args) < 2 {
			return fmt.Errorf("instance-delete requires: <agentID> <name> [--force] [--delete-data]")
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

		return api.PrintPOST(fmt.Sprintf("/agents/%s/instances/delete", url.PathEscape(agentID)), req)

	case "start":
		if len(args) != 2 {
			return fmt.Errorf("start requires: <agentID> <instance>")
		}
		agentID := args[0]
		instance := args[1]
		return api.PrintPOST(fmt.Sprintf("/agents/%s/servers/%s/start", url.PathEscape(agentID), url.PathEscape(instance)), nil)

	case "stop":
		if len(args) != 2 {
			return fmt.Errorf("stop requires: <agentID> <instance>")
		}
		agentID := args[0]
		instance := args[1]
		return api.PrintPOST(fmt.Sprintf("/agents/%s/servers/%s/stop", url.PathEscape(agentID), url.PathEscape(instance)), nil)

	case "status":
		if len(args) != 2 {
			return fmt.Errorf("status requires: <agentID> <instance>")
		}
		agentID := args[0]
		instance := args[1]
		return api.PrintGET(fmt.Sprintf("/agents/%s/servers/%s/status", url.PathEscape(agentID), url.PathEscape(instance)))

	case "templates":
		if len(args) != 1 {
			return fmt.Errorf("templates requires: <agentID>")
		}
		agentID := args[0]
		return api.PrintGET(fmt.Sprintf("/agents/%s/templates", url.PathEscape(agentID)))

	case "template":
		if len(args) != 2 {
			return fmt.Errorf("template requires: <agentID> <templateName>")
		}
		agentID := args[0]
		name := args[1]
		return api.PrintGET(fmt.Sprintf("/agents/%s/templates/%s", url.PathEscape(agentID), url.PathEscape(name)))

	case "help", "-h", "--help":
		usage()
		return nil

	default:
		usage()
		return fmt.Errorf("unknown command: %s", cmd)
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

  gamesvcctl templates <agentID>                 List templates
  gamesvcctl template  <agentID> <templateName>  Inspect one template

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

//
// API client
//

type API struct {
	client    *http.Client
	baseURL   string
	apiPrefix string
}

func NewAPI(client *http.Client, baseURL, apiPrefix string) *API {
	return &API{
		client:    client,
		baseURL:   strings.TrimRight(baseURL, "/"),
		apiPrefix: apiPrefix,
	}
}

func (a *API) url(path string) string {
	// path should begin with "/..."
	return a.baseURL + a.apiPrefix + path
}

func (a *API) PrintGET(path string) error {
	body, status, err := a.GET(path)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", prettyJSON(body))
	if status >= 400 {
		return fmt.Errorf("request failed: HTTP %d", status)
	}
	return nil
}

func (a *API) PrintPOST(path string, payload any) error {
	body, status, err := a.POST(path, payload)
	if err != nil {
		return err
	}
	fmt.Printf("%s\n", prettyJSON(body))
	if status >= 400 {
		return fmt.Errorf("request failed: HTTP %d", status)
	}
	return nil
}

func (a *API) GET(path string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", a.url(path), nil)
	if err != nil {
		return nil, 0, err
	}

	res, err := a.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	b, _ := io.ReadAll(res.Body)
	return b, res.StatusCode, nil
}

func (a *API) POST(path string, payload any) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest("POST", a.url(path), body)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := a.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(res.Body)
	return respBody, res.StatusCode, nil
}

//
// helpers
//

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
