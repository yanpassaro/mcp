package mcpserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"

	"github.com/containerd/errdefs"
	"github.com/docker/go-connections/nat"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	mobyclient "github.com/moby/moby/client"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"ntdsk.com/mcp/docker/internal/docker"
)

const (
	DEFAULT_TAIL = 200
	MAX_TAIL     = 10000
	MAX_LAST     = 1000
)

type Server struct {
	client *docker.Client
}

func New(client *docker.Client) *Server {
	return &Server{client: client}
}

func (s *Server) Register(server *mcp.Server) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_get",
		Description: "List Docker resources. 'resource': containers, images, volumes, networks, info, stats or disk. Read-only.",
	}, s.get)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_inspect",
		Description: "Container inspect JSON. Environment variables (Env) are removed.",
	}, s.inspect)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_logs",
		Description: "Container logs (stdout+stderr) with tail, since/until and optional timestamps. Read-only.",
	}, s.logs)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_run",
		Description: "Create and start a detached container. Image pull policy (missing/always/never), optional platform and timeout. No environment variables are accepted.",
	}, s.run)

	mcp.AddTool(server, &mcp.Tool{
		Name:        "docker_remove",
		Description: "Remove a container. Optional force and anonymous volume removal.",
	}, s.remove)
}

func (s *Server) get(ctx context.Context, _ *mcp.CallToolRequest, in getInput) (*mcp.CallToolResult, any, error) {
	ctx, cancel := s.client.WithTimeout(ctx)
	defer cancel()

	resource := strings.ToLower(strings.TrimSpace(in.Resource))
	switch resource {
	case "containers", "container", "ps":
		return s.getContainers(ctx, in)
	case "images", "image":
		return s.getImages(ctx, in)
	case "volumes", "volume":
		return s.getVolumes(ctx)
	case "networks", "network":
		return s.getNetworks(ctx)
	case "info":
		return s.getInfo(ctx)
	case "stats":
		return s.getStats(ctx, in)
	case "disk", "df", "disk_usage":
		return s.getDisk(ctx, in)
	default:
		return nil, nil, errors.New("'resource' inválido: use containers, images, volumes, networks, info, stats ou disk")
	}
}

func (s *Server) getContainers(ctx context.Context, in getInput) (*mcp.CallToolResult, any, error) {
	result, err := s.client.API().ContainerList(ctx, mobyclient.ContainerListOptions{
		All:   !in.RunningOnly,
		Limit: clampLast(in.Last),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list containers: %w", err)
	}
	return textResult(formatContainers(result.Items))
}

func (s *Server) getImages(ctx context.Context, in getInput) (*mcp.CallToolResult, any, error) {
	result, err := s.client.API().ImageList(ctx, mobyclient.ImageListOptions{All: in.All})
	if err != nil {
		return nil, nil, fmt.Errorf("list images: %w", err)
	}
	return textResult(formatImages(result.Items))
}

func (s *Server) getVolumes(ctx context.Context) (*mcp.CallToolResult, any, error) {
	result, err := s.client.API().VolumeList(ctx, mobyclient.VolumeListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("list volumes: %w", err)
	}
	return textResult(formatVolumes(result.Items))
}

func (s *Server) getNetworks(ctx context.Context) (*mcp.CallToolResult, any, error) {
	result, err := s.client.API().NetworkList(ctx, mobyclient.NetworkListOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("list networks: %w", err)
	}
	return textResult(formatNetworks(result.Items))
}

func (s *Server) getInfo(ctx context.Context) (*mcp.CallToolResult, any, error) {
	result, err := s.client.API().Info(ctx, mobyclient.InfoOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("docker info: %w", err)
	}
	return textResult(formatInfo(result.Info))
}

func (s *Server) getStats(ctx context.Context, in getInput) (*mcp.CallToolResult, any, error) {
	api := s.client.API()
	targets := []string{}
	requested := strings.TrimSpace(in.Container)
	if requested != "" {
		targets = append(targets, requested)
	}
	if requested == "" {
		list, err := api.ContainerList(ctx, mobyclient.ContainerListOptions{})
		if err != nil {
			return nil, nil, fmt.Errorf("list containers: %w", err)
		}
		for _, item := range list.Items {
			targets = append(targets, item.ID)
		}
	}
	stats := make([]container.StatsResponse, 0, len(targets))
	for _, target := range targets {
		result, err := api.ContainerStats(ctx, target, mobyclient.ContainerStatsOptions{IncludePreviousSample: true})
		if err != nil {
			return nil, nil, fmt.Errorf("stats %s: %w", target, err)
		}
		var stat container.StatsResponse
		decodeErr := json.NewDecoder(result.Body).Decode(&stat)
		closeErr := result.Body.Close()
		if decodeErr != nil {
			return nil, nil, fmt.Errorf("decode stats %s: %w", target, decodeErr)
		}
		if closeErr != nil {
			return nil, nil, fmt.Errorf("close stats %s: %w", target, closeErr)
		}
		stats = append(stats, stat)
	}
	return textResult(formatStats(stats))
}

func (s *Server) getDisk(ctx context.Context, in getInput) (*mcp.CallToolResult, any, error) {
	result, err := s.client.API().DiskUsage(ctx, mobyclient.DiskUsageOptions{
		Containers: true,
		Images:     true,
		Volumes:    true,
		BuildCache: true,
		Verbose:    in.Verbose,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("docker disk usage: %w", err)
	}
	return textResult(formatDiskUsage(result))
}

func (s *Server) inspect(ctx context.Context, _ *mcp.CallToolRequest, in inspectInput) (*mcp.CallToolResult, any, error) {
	containerID := strings.TrimSpace(in.Container)
	if containerID == "" {
		return nil, nil, errors.New("container é obrigatório")
	}
	ctx, cancel := s.client.WithTimeout(ctx)
	defer cancel()

	result, err := s.client.API().ContainerInspect(ctx, containerID, mobyclient.ContainerInspectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("inspect %s: %w", containerID, err)
	}
	var value any
	if err := json.Unmarshal(result.Raw, &value); err != nil {
		return nil, nil, fmt.Errorf("decode inspect %s: %w", containerID, err)
	}
	return textResult(formatInspect(containerID, redactEnv(value)))
}

func (s *Server) logs(ctx context.Context, _ *mcp.CallToolRequest, in logsInput) (*mcp.CallToolResult, any, error) {
	containerID := strings.TrimSpace(in.Container)
	if containerID == "" {
		return nil, nil, errors.New("container é obrigatório")
	}
	ctx, cancel := s.client.WithTimeout(ctx)
	defer cancel()

	info, err := s.client.API().ContainerInspect(ctx, containerID, mobyclient.ContainerInspectOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("inspect %s: %w", containerID, err)
	}
	tty, err := inspectTTY(info.Raw)
	if err != nil {
		return nil, nil, err
	}
	tail := clampTail(in.Tail)
	result, err := s.client.API().ContainerLogs(ctx, containerID, mobyclient.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Since:      strings.TrimSpace(in.Since),
		Until:      strings.TrimSpace(in.Until),
		Timestamps: in.Timestamps,
		Tail:       strconv.Itoa(tail),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("logs %s: %w", containerID, err)
	}

	var out bytes.Buffer
	var readErr error
	if tty {
		_, readErr = io.Copy(&out, result)
	}
	if !tty {
		var stderr bytes.Buffer
		_, readErr = stdcopy.StdCopy(&out, &stderr, result)
		if stderr.Len() > 0 {
			out.WriteString("\n")
			out.Write(stderr.Bytes())
		}
	}
	closeErr := result.Close()
	if readErr != nil {
		return nil, nil, fmt.Errorf("read logs %s: %w", containerID, readErr)
	}
	if closeErr != nil {
		return nil, nil, fmt.Errorf("close logs %s: %w", containerID, closeErr)
	}
	return textResult(formatLogs(containerID, tail, out.String()))
}

func (s *Server) run(ctx context.Context, _ *mcp.CallToolRequest, in runInput) (*mcp.CallToolResult, any, error) {
	rawImage := strings.TrimSpace(in.Image)
	if rawImage == "" {
		return nil, nil, errors.New("image é obrigatório")
	}
	imageName := normalizeImage(rawImage)
	policy, err := pullPolicy(in.Pull)
	if err != nil {
		return nil, nil, err
	}
	platform, err := parsePlatform(in.Platform)
	if err != nil {
		return nil, nil, err
	}
	ctx, cancel := s.client.WithTimeoutSeconds(ctx, in.TimeoutSeconds)
	defer cancel()
	pulled, err := s.ensureImage(ctx, imageName, policy, platform)
	if err != nil {
		return nil, nil, err
	}

	natExposed, natBindings, err := nat.ParsePortSpecs(in.Ports)
	if err != nil {
		return nil, nil, fmt.Errorf("portas inválidas: %w", err)
	}
	ports := network.PortMap{}
	for natPort, bindings := range natBindings {
		port, err := network.ParsePort(string(natPort))
		if err != nil {
			return nil, nil, fmt.Errorf("porta inválida %q: %w", natPort, err)
		}
		converted := make([]network.PortBinding, 0, len(bindings))
		for _, binding := range bindings {
			hostIP := netip.Addr{}
			if strings.TrimSpace(binding.HostIP) != "" {
				parsed, err := netip.ParseAddr(binding.HostIP)
				if err != nil {
					return nil, nil, fmt.Errorf("host IP inválido %q: %w", binding.HostIP, err)
				}
				hostIP = parsed
			}
			converted = append(converted, network.PortBinding{HostIP: hostIP, HostPort: binding.HostPort})
		}
		ports[port] = converted
	}
	exposed := network.PortSet{}
	for natPort := range natExposed {
		port, err := network.ParsePort(string(natPort))
		if err != nil {
			return nil, nil, fmt.Errorf("porta inválida %q: %w", natPort, err)
		}
		exposed[port] = struct{}{}
	}
	name := strings.TrimSpace(in.Name)
	config := &container.Config{Image: imageName, Cmd: in.Command, ExposedPorts: exposed}
	host := &container.HostConfig{
		PortBindings: ports,
		Binds:        in.Volumes,
		AutoRemove:   in.AutoRemove,
	}
	networkName := strings.TrimSpace(in.Network)
	if networkName != "" {
		host.NetworkMode = container.NetworkMode(networkName)
	}
	created, err := s.client.API().ContainerCreate(ctx, mobyclient.ContainerCreateOptions{
		Config:     config,
		HostConfig: host,
		Name:       name,
		Platform:   platform,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create container: %w", err)
	}
	_, err = s.client.API().ContainerStart(ctx, created.ID, mobyclient.ContainerStartOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("container %s criado mas não iniciado: %w", created.ID, err)
	}
	return textResult(formatRun(name, imageName, created, pulled))
}

func (s *Server) remove(ctx context.Context, _ *mcp.CallToolRequest, in removeInput) (*mcp.CallToolResult, any, error) {
	containerID := strings.TrimSpace(in.Container)
	if containerID == "" {
		return nil, nil, errors.New("container é obrigatório")
	}
	ctx, cancel := s.client.WithTimeout(ctx)
	defer cancel()

	_, err := s.client.API().ContainerRemove(ctx, containerID, mobyclient.ContainerRemoveOptions{
		Force:         in.Force,
		RemoveVolumes: in.Volumes,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("remove %s: %w", containerID, err)
	}
	return textResult(formatRemove(containerID))
}

func inspectTTY(raw json.RawMessage) (bool, error) {
	var meta struct {
		Config struct {
			Tty bool `json:"Tty"`
		} `json:"Config"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return false, fmt.Errorf("decode container inspect: %w", err)
	}
	return meta.Config.Tty, nil
}

func pullPolicy(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "missing":
		return "missing", nil
	case "always":
		return "always", nil
	case "never":
		return "never", nil
	default:
		return "", fmt.Errorf("pull inválido: use missing, always ou never")
	}
}

func parsePlatform(value string) (*ocispec.Platform, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parts := strings.Split(value, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("platform inválida %q: use os/arch[/variant]", value)
	}
	if len(parts) > 3 {
		return nil, fmt.Errorf("platform inválida %q: use os/arch[/variant]", value)
	}
	platform := &ocispec.Platform{OS: parts[0], Architecture: parts[1]}
	if len(parts) == 3 {
		platform.Variant = parts[2]
	}
	if platform.OS == "" {
		return nil, fmt.Errorf("platform inválida %q: OS vazio", value)
	}
	if platform.Architecture == "" {
		return nil, fmt.Errorf("platform inválida %q: arquitetura vazia", value)
	}
	return platform, nil
}

func normalizeImage(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if strings.Contains(value, "@") {
		return value
	}
	slash := strings.LastIndex(value, "/")
	colon := strings.LastIndex(value, ":")
	if colon > slash {
		return value
	}
	return fmt.Sprintf("%s:latest", value)
}

func imageExists(ctx context.Context, api *mobyclient.Client, image string) (bool, error) {
	_, err := api.ImageInspect(ctx, image)
	if err == nil {
		return true, nil
	}
	if errdefs.IsNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("inspect image %s: %w", image, err)
}

func (s *Server) ensureImage(ctx context.Context, image, policy string, platform *ocispec.Platform) (bool, error) {
	exists, err := imageExists(ctx, s.client.API(), image)
	if err != nil {
		return false, err
	}
	if policy == "never" {
		if !exists {
			return false, fmt.Errorf("imagem %q não encontrada e pull=never", image)
		}
		return false, nil
	}
	if policy == "always" {
		if err := s.pullImage(ctx, image, platform); err != nil {
			return false, err
		}
		return true, nil
	}
	if exists {
		return false, nil
	}
	if err := s.pullImage(ctx, image, platform); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Server) pullImage(ctx context.Context, image string, platform *ocispec.Platform) error {
	options := mobyclient.ImagePullOptions{RegistryAuth: s.client.RegistryAuth()}
	if platform != nil {
		options.Platforms = []ocispec.Platform{*platform}
	}
	response, err := s.client.API().ImagePull(ctx, image, options)
	if err != nil {
		return fmt.Errorf("pull %s: %w", image, err)
	}
	waitErr := response.Wait(ctx)
	closeErr := response.Close()
	if waitErr != nil {
		return fmt.Errorf("pull %s: %w", image, waitErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close pull %s: %w", image, closeErr)
	}
	return nil
}

func clampTail(n int) int {
	if n <= 0 {
		return DEFAULT_TAIL
	}
	if n > MAX_TAIL {
		return MAX_TAIL
	}
	return n
}

func clampLast(n int) int {
	if n <= 0 {
		return 0
	}
	if n > MAX_LAST {
		return MAX_LAST
	}
	return n
}

type getInput struct {
	Resource    string `json:"resource" jsonschema:"What to get: containers, images, volumes, networks, info, stats or disk (required)."`
	RunningOnly bool   `json:"runningOnly,omitempty" jsonschema:"For containers: if true, shows only running containers; default includes stopped."`
	All         bool   `json:"all,omitempty" jsonschema:"For images: include intermediate images."`
	Last        int    `json:"last,omitempty" jsonschema:"For containers: last N created containers."`
	Verbose     bool   `json:"verbose,omitempty" jsonschema:"For disk: detailed per-object usage."`
	Container   string `json:"container,omitempty" jsonschema:"For stats: container name or ID; default all running containers."`
}

type inspectInput struct {
	Container string `json:"container" jsonschema:"Container name or ID (required)."`
}

type logsInput struct {
	Container  string `json:"container" jsonschema:"Container name or ID (required)."`
	Tail       int    `json:"tail,omitempty" jsonschema:"Number of lines from the end (default 200, max 10000)."`
	Since      string `json:"since,omitempty" jsonschema:"Show logs since a timestamp or relative duration (e.g. 10m)."`
	Until      string `json:"until,omitempty" jsonschema:"Show logs before a timestamp or relative duration."`
	Timestamps bool   `json:"timestamps,omitempty" jsonschema:"If true, prefixes each line with its timestamp."`
}

type runInput struct {
	Image          string   `json:"image" jsonschema:"Image to run (required)."`
	Name           string   `json:"name,omitempty" jsonschema:"Container name (optional)."`
	Command        []string `json:"command,omitempty" jsonschema:"Command and arguments after the image (optional)."`
	Ports          []string `json:"ports,omitempty" jsonschema:"Port mappings, e.g. [\"8080:80\"]."`
	Volumes        []string `json:"volumes,omitempty" jsonschema:"Volume mappings, e.g. [\"C:/data:/data\"]."`
	Network        string   `json:"network,omitempty" jsonschema:"Network name (optional)."`
	AutoRemove     bool     `json:"autoRemove,omitempty" jsonschema:"If true, removes the container when it stops."`
	Pull           string   `json:"pull,omitempty" jsonschema:"Image pull policy: missing (default), always or never."`
	Platform       string   `json:"platform,omitempty" jsonschema:"Optional platform, e.g. linux/amd64."`
	TimeoutSeconds int      `json:"timeoutSeconds,omitempty" jsonschema:"Optional timeout override for pull/create/start, in seconds."`
}

type removeInput struct {
	Container string `json:"container" jsonschema:"Container name or ID (required)."`
	Force     bool   `json:"force,omitempty" jsonschema:"Force removal of a running container."`
	Volumes   bool   `json:"volumes,omitempty" jsonschema:"Remove anonymous volumes."`
}

func textResult(text string) (*mcp.CallToolResult, any, error) {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: text}},
	}, nil, nil
}
