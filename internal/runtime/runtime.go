package runtime

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type Status struct {
	State   string `json:"state"`
	Health  string `json:"health"`
	Runtime string `json:"runtime"`
}

type Runtime interface {
	Inspect(context.Context, string) (Status, error)
	Install(context.Context, InstallSpec) (string, error)
	Start(context.Context, string) error
	Stop(context.Context, string) error
	Restart(context.Context, string) error
	Health(context.Context, string) (Status, error)
	Logs(context.Context, string, int) (string, error)
	PrepareUpdate(context.Context, string) (map[string]string, error)
	Uninstall(context.Context, string) error
}

type InstallSpec struct {
	Name       string
	Image      string
	EnvNames   []string
	CPU        string
	Memory     string
	PIDs       int
	Network    string
	Labels     map[string]string
	ReadOnlyFS bool
}

type commandRunner func(context.Context, string, ...string) ([]byte, error)

type DockerCLI struct {
	binary  string
	timeout time.Duration
	runner  commandRunner
}

func NewDockerCLI(binary string, timeout time.Duration) *DockerCLI {
	if binary == "" {
		binary = "docker"
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &DockerCLI{binary: binary, timeout: timeout, runner: defaultRunner}
}

func defaultRunner(ctx context.Context, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("docker cli: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return out, nil
}

func (d *DockerCLI) run(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, d.timeout)
	defer cancel()
	return d.runner(ctx, d.binary, args...)
}

func (d *DockerCLI) Inspect(ctx context.Context, ref string) (Status, error) {
	out, err := d.run(ctx, "inspect", "--format", `{{.State.Status}}|{{.State.Health.Status}}`, ref)
	if err != nil {
		return Status{}, err
	}
	parts := strings.Split(strings.TrimSpace(string(out)), "|")
	status := Status{Runtime: "docker"}
	if len(parts) > 0 {
		status.State = parts[0]
	}
	if len(parts) > 1 {
		status.Health = parts[1]
	}
	return status, nil
}

func (d *DockerCLI) Install(ctx context.Context, spec InstallSpec) (string, error) {
	if spec.Image == "" {
		return "", errors.New("image is required")
	}
	args := []string{"run", "-d", "--name", spec.Name, "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--user", "65532:65532"}
	if spec.PIDs > 0 {
		args = append(args, "--pids-limit", fmt.Sprint(spec.PIDs))
	}
	if spec.CPU != "" {
		args = append(args, "--cpus", spec.CPU)
	}
	if spec.Memory != "" {
		args = append(args, "--memory", spec.Memory)
	}
	if spec.Network != "" {
		args = append(args, "--network", spec.Network)
	}
	for k, v := range spec.Labels {
		args = append(args, "--label", k+"="+v)
	}
	for _, env := range spec.EnvNames {
		args = append(args, "-e", env)
	}
	args = append(args, spec.Image)
	out, err := d.run(ctx, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (d *DockerCLI) Start(ctx context.Context, ref string) error {
	_, err := d.run(ctx, "start", ref)
	return err
}
func (d *DockerCLI) Stop(ctx context.Context, ref string) error {
	_, err := d.run(ctx, "stop", ref)
	return err
}
func (d *DockerCLI) Restart(ctx context.Context, ref string) error {
	_, err := d.run(ctx, "restart", ref)
	return err
}
func (d *DockerCLI) Health(ctx context.Context, ref string) (Status, error) {
	return d.Inspect(ctx, ref)
}
func (d *DockerCLI) Logs(ctx context.Context, ref string, lines int) (string, error) {
	if lines <= 0 {
		lines = 200
	}
	out, err := d.run(ctx, "logs", "--tail", fmt.Sprint(lines), ref)
	return string(out), err
}
func (d *DockerCLI) PrepareUpdate(_ context.Context, _ string) (map[string]string, error) {
	return map[string]string{"strategy": "recreate", "rollback": "image_digest"}, nil
}
func (d *DockerCLI) Uninstall(ctx context.Context, ref string) error {
	_, err := d.run(ctx, "rm", "-f", ref)
	return err
}

type FakeRuntime struct {
	mu     sync.Mutex
	states map[string]Status
	logs   map[string]string
}

func NewFakeRuntime() *FakeRuntime {
	return &FakeRuntime{states: make(map[string]Status), logs: make(map[string]string)}
}

func (f *FakeRuntime) Inspect(_ context.Context, ref string) (Status, error) {
	return f.Health(context.Background(), ref)
}

func (f *FakeRuntime) Install(_ context.Context, spec InstallSpec) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := "fake-" + spec.Name
	f.states[id] = Status{State: "running", Health: "healthy", Runtime: "fake"}
	f.logs[id] = "installed " + spec.Image
	return id, nil
}

func (f *FakeRuntime) Start(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	status := f.states[ref]
	status.State = "running"
	status.Health = "healthy"
	status.Runtime = "fake"
	f.states[ref] = status
	return nil
}
func (f *FakeRuntime) Stop(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	status := f.states[ref]
	status.State = "stopped"
	status.Runtime = "fake"
	f.states[ref] = status
	return nil
}
func (f *FakeRuntime) Restart(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	status := f.states[ref]
	status.State = "running"
	status.Health = "healthy"
	status.Runtime = "fake"
	f.states[ref] = status
	return nil
}
func (f *FakeRuntime) Health(_ context.Context, ref string) (Status, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	st, ok := f.states[ref]
	if !ok {
		return Status{}, ErrRuntimeNotFound
	}
	return st, nil
}
func (f *FakeRuntime) Logs(_ context.Context, ref string, _ int) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.logs[ref], nil
}
func (f *FakeRuntime) PrepareUpdate(_ context.Context, _ string) (map[string]string, error) {
	return map[string]string{"strategy": "fake-recreate", "rollback": "stored"}, nil
}
func (f *FakeRuntime) Uninstall(_ context.Context, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.states, ref)
	delete(f.logs, ref)
	return nil
}

var ErrRuntimeNotFound = errors.New("runtime instance not found")
