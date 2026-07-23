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
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		printHelp()
		return nil
	}
	jsonOut := false
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--json" {
			jsonOut = true
			continue
		}
		filtered = append(filtered, a)
	}
	args = filtered
	cmd := args[0]
	client := &apiClient{
		baseURL: strings.TrimRight(envOr("OPENMCP_URL", "http://127.0.0.1:8080"), "/"),
		token:   os.Getenv("OPENMCP_TOKEN"),
		http:    &http.Client{Timeout: 15 * time.Second},
		jsonOut: jsonOut,
	}
	switch cmd {
	case "status":
		return client.getPrint("/v1/info")
	case "search":
		q := ""
		if len(args) > 1 {
			q = args[1]
		}
		return client.getPrint("/v1/catalog/search?q=" + urlQuery(q))
	case "inspect":
		if len(args) < 2 {
			return fmt.Errorf("usage: mcpctl inspect <package-id>")
		}
		return client.getPrint("/v1/catalog/packages/" + args[1])
	case "plan":
		return client.handlePlan(args[1:])
	case "approval":
		return client.handleApproval(args[1:])
	case "install":
		return client.handleInstall(args[1:])
	case "profile":
		return client.handleProfile(args[1:])
	case "client":
		return client.handleClient(args[1:])
	case "gateway":
		if len(args) < 2 || args[1] != "status" {
			return fmt.Errorf("usage: mcpctl gateway status")
		}
		return client.getPrintAuth("/v1/admin/gateway/status")
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

type apiClient struct {
	baseURL string
	token   string
	http    *http.Client
	jsonOut bool
}

func (c *apiClient) handlePlan(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mcpctl plan <create|show|list>")
	}
	switch args[0] {
	case "list":
		return c.getPrintAuth("/v1/admin/plans")
	case "show":
		if len(args) < 2 {
			return fmt.Errorf("usage: mcpctl plan show <id>")
		}
		return c.getPrintAuth("/v1/admin/plans/" + args[1])
	case "create":
		if len(args) < 2 {
			return fmt.Errorf("usage: mcpctl plan create '<json>'")
		}
		return c.postPrintAuth("/v1/admin/plans", []byte(args[1]))
	default:
		return fmt.Errorf("usage: mcpctl plan <create|show|list>")
	}
}

func (c *apiClient) handleApproval(args []string) error {
	if len(args) == 0 || args[0] != "create" || len(args) < 3 {
		return fmt.Errorf("usage: mcpctl approval create <plan-id> <reason>")
	}
	body, _ := json.Marshal(map[string]string{"plan_id": args[1], "reason": strings.Join(args[2:], " ")})
	return c.postPrintAuth("/v1/admin/approvals", body)
}

func (c *apiClient) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mcpctl install <apply|list|health|logs|start|stop|restart|disable|uninstall>")
	}
	switch args[0] {
	case "list":
		return c.getPrintAuth("/v1/admin/installations")
	case "apply":
		if len(args) < 2 {
			return fmt.Errorf("usage: mcpctl install apply <plan-id>")
		}
		body, _ := json.Marshal(map[string]string{"plan_id": args[1]})
		return c.postPrintAuth("/v1/admin/installations/apply", body)
	case "health", "logs", "start", "stop", "restart", "disable", "uninstall":
		if len(args) < 2 {
			return fmt.Errorf("usage: mcpctl install %s <installation-id>", args[0])
		}
		id := args[1]
		switch args[0] {
		case "health":
			return c.getPrintAuth("/v1/admin/installations/" + id + "/health")
		case "logs":
			return c.getPrintAuth("/v1/admin/installations/" + id + "/logs")
		default:
			return c.postPrintAuth("/v1/admin/installations/"+id+"/"+args[0], []byte("{}"))
		}
	default:
		return fmt.Errorf("unknown install subcommand")
	}
}

func (c *apiClient) handleProfile(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mcpctl profile <create|list|add|tools>")
	}
	switch args[0] {
	case "list":
		return c.getPrintAuth("/v1/admin/profiles")
	case "create":
		if len(args) < 2 {
			return fmt.Errorf("usage: mcpctl profile create <name>")
		}
		body, _ := json.Marshal(map[string]string{"name": args[1]})
		return c.postPrintAuth("/v1/admin/profiles", body)
	case "add":
		if len(args) < 3 {
			return fmt.Errorf("usage: mcpctl profile add <profile-id> <installation-id>[,installation-id...]")
		}
		ids := strings.Split(args[2], ",")
		body, _ := json.Marshal(map[string]any{"installation_ids": ids})
		return c.postPrintAuth("/v1/admin/profiles/"+args[1]+"/installations", body)
	case "tools":
		if len(args) < 3 {
			return fmt.Errorf("usage: mcpctl profile tools <profile-id> <tool>[,tool...]")
		}
		tools := strings.Split(args[2], ",")
		body, _ := json.Marshal(map[string]any{"tools": tools})
		return c.postPrintAuth("/v1/admin/profiles/"+args[1]+"/tools", body)
	default:
		return fmt.Errorf("unknown profile subcommand")
	}
}

func (c *apiClient) handleClient(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: mcpctl client <create|list|revoke>")
	}
	switch args[0] {
	case "list":
		return c.getPrintAuth("/v1/admin/clients")
	case "create":
		if len(args) < 3 {
			return fmt.Errorf("usage: mcpctl client create <profile-id> <name>")
		}
		body, _ := json.Marshal(map[string]string{"profile_id": args[1], "name": args[2]})
		return c.postPrintAuth("/v1/admin/clients", body)
	case "revoke":
		if len(args) < 2 {
			return fmt.Errorf("usage: mcpctl client revoke <client-id>")
		}
		return c.postPrintAuth("/v1/admin/clients/"+args[1]+"/revoke", []byte("{}"))
	default:
		return fmt.Errorf("unknown client subcommand")
	}
}

func (c *apiClient) getPrint(path string) error {
	return c.doPrint(http.MethodGet, path, nil, false)
}

func (c *apiClient) getPrintAuth(path string) error {
	return c.doPrint(http.MethodGet, path, nil, true)
}

func (c *apiClient) postPrintAuth(path string, body []byte) error {
	return c.doPrint(http.MethodPost, path, body, true)
}

func (c *apiClient) doPrint(method, path string, body []byte, auth bool) error {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		if c.token == "" {
			return fmt.Errorf("OPENMCP_TOKEN is required for admin commands")
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(out)))
	}
	if c.jsonOut {
		fmt.Println(string(out))
		return nil
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, out, "", "  "); err != nil {
		fmt.Println(string(out))
		return nil
	}
	fmt.Println(pretty.String())
	return nil
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func urlQuery(v string) string {
	return strings.ReplaceAll(v, " ", "+")
}

func printHelp() {
	fmt.Print(`mcpctl - Open MCP Control Plane CLI

Environment:
  OPENMCP_URL    Control plane base URL (default http://127.0.0.1:8080)
  OPENMCP_TOKEN  Admin bearer token

Commands:
  status
  search <query>
  inspect <package-id>
  plan create '<json>' | show <id> | list
  approval create <plan-id> <reason>
  install apply <plan-id> | list | health|logs|start|stop|restart|disable|uninstall <id>
  profile create <name> | list | add <profile-id> <installation-ids> | tools <profile-id> <tools>
  client create <profile-id> <name> | list | revoke <client-id>
  gateway status

Flags:
  --json   emit raw JSON
`)
}
